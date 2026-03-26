package jsworker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dop251/goja"
)

type singleUseRuntimeAsyncState struct {
	vm       *goja.Runtime
	execCtx  context.Context
	deadline time.Time

	mu      sync.Mutex
	active  bool
	timerSeq int64
	timers  map[int64]*time.Timer
	tasks   []func() error
	wake    chan struct{}
}

func newSingleUseRuntimeAsyncState(
	vm *goja.Runtime,
	execCtx context.Context,
	deadline time.Time,
) *singleUseRuntimeAsyncState {
	if vm == nil {
		return nil
	}
	if execCtx == nil {
		execCtx = context.Background()
	}
	return &singleUseRuntimeAsyncState{
		vm:       vm,
		execCtx:  execCtx,
		deadline: deadline,
		active:   true,
		wake:     make(chan struct{}, 1),
	}
}

func (s *singleUseRuntimeAsyncState) enqueue(task func() error) bool {
	if s == nil || task == nil {
		return false
	}
	s.mu.Lock()
	if !s.active {
		s.mu.Unlock()
		return false
	}
	s.tasks = append(s.tasks, task)
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return true
}

func (s *singleUseRuntimeAsyncState) takeTasks() []func() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.tasks) == 0 {
		return nil
	}
	tasks := append([]func() error(nil), s.tasks...)
	s.tasks = nil
	return tasks
}

func (s *singleUseRuntimeAsyncState) drainTasks() error {
	if s == nil {
		return nil
	}
	for {
		tasks := s.takeTasks()
		if len(tasks) == 0 {
			return nil
		}
		for _, task := range tasks {
			if task == nil {
				continue
			}
			if err := task(); err != nil {
				return err
			}
			if s.vm == nil {
				continue
			}
			if _, err := s.vm.RunString(""); err != nil {
				return fmt.Errorf("flush async jobs failed: %w", err)
			}
		}
	}
}

func (s *singleUseRuntimeAsyncState) nextTimerID() int64 {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.timerSeq++
	if s.timers == nil {
		s.timers = make(map[int64]*time.Timer)
	}
	return s.timerSeq
}

func (s *singleUseRuntimeAsyncState) registerTimer(id int64, timer *time.Timer) bool {
	if s == nil || id <= 0 || timer == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.active {
		return false
	}
	if s.timers == nil {
		s.timers = make(map[int64]*time.Timer)
	}
	s.timers[id] = timer
	return true
}

func (s *singleUseRuntimeAsyncState) deleteTimer(id int64) *time.Timer {
	if s == nil || id <= 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.timers) == 0 {
		return nil
	}
	timer := s.timers[id]
	delete(s.timers, id)
	return timer
}

func (s *singleUseRuntimeAsyncState) clearTimer(id int64) bool {
	timer := s.deleteTimer(id)
	if timer == nil {
		return false
	}
	timer.Stop()
	return true
}

func (s *singleUseRuntimeAsyncState) isActive() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

func (s *singleUseRuntimeAsyncState) finish() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if !s.active {
		s.mu.Unlock()
		return
	}
	s.active = false
	timers := s.timers
	s.timers = nil
	s.tasks = nil
	s.mu.Unlock()
	for _, timer := range timers {
		if timer != nil {
			timer.Stop()
		}
	}
}

func (s *singleUseRuntimeAsyncState) queueMicrotask(callback goja.Callable) error {
	if s == nil || callback == nil {
		return fmt.Errorf("queueMicrotask is unavailable")
	}
	if !s.isActive() {
		return fmt.Errorf("queueMicrotask is unavailable for the current invocation")
	}
	s.enqueue(func() error {
		if !s.isActive() {
			return nil
		}
		_, err := callback(goja.Undefined())
		return err
	})
	return nil
}

func (s *singleUseRuntimeAsyncState) scheduleTimeout(
	callback goja.Callable,
	delay time.Duration,
	args []goja.Value,
) (int64, error) {
	if s == nil || s.vm == nil {
		return 0, fmt.Errorf("setTimeout is unavailable")
	}
	if callback == nil || !s.isActive() {
		return 0, fmt.Errorf("setTimeout is unavailable for the current invocation")
	}
	if delay < 0 {
		delay = 0
	}
	timerID := s.nextTimerID()
	task := func() error {
		if !s.isActive() {
			return nil
		}
		s.deleteTimer(timerID)
		_, err := callback(goja.Undefined(), args...)
		return err
	}
	timer := time.AfterFunc(delay, func() {
		if !s.isActive() {
			return
		}
		s.enqueue(task)
	})
	if !s.registerTimer(timerID, timer) {
		timer.Stop()
		return 0, fmt.Errorf("setTimeout is unavailable for the current invocation")
	}
	return timerID, nil
}

func (s *singleUseRuntimeAsyncState) resolveAsyncResult(value goja.Value) (goja.Value, error) {
	promise := exportGojaPromise(value)
	if promise == nil {
		return value, nil
	}
	return s.awaitPromise(promise)
}

func (s *singleUseRuntimeAsyncState) awaitPromise(promise *goja.Promise) (goja.Value, error) {
	if promise == nil {
		return goja.Undefined(), nil
	}
	for {
		if err := s.drainTasks(); err != nil {
			return nil, err
		}
		switch promise.State() {
		case goja.PromiseStateFulfilled:
			return promise.Result(), nil
		case goja.PromiseStateRejected:
			return nil, runtimePromiseRejectionError(promise.Result())
		}

		var timer *time.Timer
		var deadlineCh <-chan time.Time
		if !s.deadline.IsZero() {
			wait := time.Until(s.deadline)
			if wait <= 0 {
				return nil, fmt.Errorf("execution timeout")
			}
			timer = time.NewTimer(wait)
			deadlineCh = timer.C
		}

		select {
		case <-s.wake:
		case <-s.execCtx.Done():
			if timer != nil {
				timer.Stop()
			}
			return nil, s.execCtx.Err()
		case <-deadlineCh:
			if timer != nil {
				timer.Stop()
			}
			return nil, fmt.Errorf("execution timeout")
		}

		if timer != nil {
			timer.Stop()
		}
	}
}
