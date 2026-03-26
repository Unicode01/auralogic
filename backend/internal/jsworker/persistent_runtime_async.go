package jsworker

import (
	"fmt"
	"strings"
	"time"

	"github.com/dop251/goja"
)

type runtimeAsyncTask struct {
	invocation        *persistentPluginRuntimeInvocation
	action            string
	run               func() error
	releaseInvocation bool
}

func (r *persistentPluginRuntime) wakeAsyncLoop() {
	if r == nil {
		return
	}
	select {
	case r.asyncWake <- struct{}{}:
	default:
	}
}

func (r *persistentPluginRuntime) enqueueAsyncTask(task runtimeAsyncTask) bool {
	if r == nil || task.run == nil {
		return false
	}
	r.asyncMu.Lock()
	if r.asyncClosed {
		r.asyncMu.Unlock()
		return false
	}
	r.asyncTasks = append(r.asyncTasks, task)
	r.asyncMu.Unlock()
	r.wakeAsyncLoop()
	return true
}

func (r *persistentPluginRuntime) scheduleAsyncRetry() {
	if r == nil {
		return
	}
	r.asyncMu.Lock()
	if r.asyncClosed || r.asyncRetryPending {
		r.asyncMu.Unlock()
		return
	}
	r.asyncRetryPending = true
	r.asyncMu.Unlock()

	time.AfterFunc(2*time.Millisecond, func() {
		if r == nil {
			return
		}
		r.asyncMu.Lock()
		if r.asyncClosed {
			r.asyncMu.Unlock()
			return
		}
		r.asyncRetryPending = false
		r.asyncMu.Unlock()
		r.wakeAsyncLoop()
	})
}

func (r *persistentPluginRuntime) enqueueInvocationAsyncTask(
	invocation *persistentPluginRuntimeInvocation,
	action string,
	task func() error,
) bool {
	if r == nil || invocation == nil || task == nil {
		return false
	}
	if !invocation.retainAsync() {
		return false
	}
	if !r.enqueueAsyncTask(runtimeAsyncTask{
		invocation:        invocation,
		action:            strings.TrimSpace(action),
		run:               task,
		releaseInvocation: true,
	}) {
		invocation.releaseAsync()
		return false
	}
	return true
}

func (r *persistentPluginRuntime) takeAsyncTasks() []runtimeAsyncTask {
	if r == nil {
		return nil
	}
	r.asyncMu.Lock()
	defer r.asyncMu.Unlock()
	if len(r.asyncTasks) == 0 {
		return nil
	}
	tasks := append([]runtimeAsyncTask(nil), r.asyncTasks...)
	r.asyncTasks = nil
	return tasks
}

func (r *persistentPluginRuntime) nextRuntimeTimerID() int64 {
	if r == nil {
		return 0
	}
	r.asyncMu.Lock()
	defer r.asyncMu.Unlock()
	r.timerSeq++
	return r.timerSeq
}

func (r *persistentPluginRuntime) registerRuntimeTimer(id int64, invocation *persistentPluginRuntimeInvocation) {
	if r == nil || id <= 0 || invocation == nil {
		return
	}
	r.asyncMu.Lock()
	if !r.asyncClosed {
		if r.timerOwners == nil {
			r.timerOwners = make(map[int64]*persistentPluginRuntimeInvocation)
		}
		r.timerOwners[id] = invocation
	}
	r.asyncMu.Unlock()
}

func (r *persistentPluginRuntime) unregisterRuntimeTimer(id int64) {
	if r == nil || id <= 0 {
		return
	}
	r.asyncMu.Lock()
	if len(r.timerOwners) > 0 {
		delete(r.timerOwners, id)
	}
	r.asyncMu.Unlock()
}

func (r *persistentPluginRuntime) lookupRuntimeTimerOwner(id int64) *persistentPluginRuntimeInvocation {
	if r == nil || id <= 0 {
		return nil
	}
	r.asyncMu.Lock()
	defer r.asyncMu.Unlock()
	return r.timerOwners[id]
}

func (r *persistentPluginRuntime) runAsyncLoop() {
	if r == nil {
		return
	}
	for {
		select {
		case <-r.asyncStop:
			return
		case <-r.asyncWake:
		}
		r.drainAsyncTasksInBackground()
	}
}

func (r *persistentPluginRuntime) drainAsyncTasksInBackground() {
	if r == nil {
		return
	}
	for {
		r.mu.Lock()
		if r.currentInvoke != nil || r.vm == nil {
			r.mu.Unlock()
			if r.currentInvoke != nil {
				r.scheduleAsyncRetry()
			}
			return
		}
		tasks := r.takeAsyncTasks()
		if len(tasks) == 0 {
			r.mu.Unlock()
			return
		}
		for _, task := range tasks {
			r.executeAsyncTaskLocked(task)
		}
		r.mu.Unlock()
	}
}

func (r *persistentPluginRuntime) drainAsyncTasksLocked() error {
	if r == nil {
		return nil
	}
	for {
		tasks := r.takeAsyncTasks()
		if len(tasks) == 0 {
			return nil
		}
		for _, task := range tasks {
			if err := r.executeAsyncTaskLocked(task); err != nil {
				return err
			}
		}
	}
}

func exportGojaPromise(value goja.Value) *goja.Promise {
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return nil
	}
	exported := value.Export()
	promise, _ := exported.(*goja.Promise)
	return promise
}

func runtimePromiseRejectionError(reason goja.Value) error {
	if reason == nil || goja.IsUndefined(reason) || goja.IsNull(reason) {
		return fmt.Errorf("promise rejected")
	}
	message := strings.TrimSpace(reason.String())
	if message == "" {
		return fmt.Errorf("promise rejected")
	}
	return fmt.Errorf("promise rejected: %s", message)
}

func (r *persistentPluginRuntime) resolveAsyncResultLocked(
	invocation *persistentPluginRuntimeInvocation,
	value goja.Value,
) (goja.Value, error) {
	promise := exportGojaPromise(value)
	if promise == nil {
		if err := r.drainAsyncTasksLocked(); err != nil {
			return nil, err
		}
		return value, nil
	}
	return r.awaitPromiseLocked(invocation, promise)
}

func (r *persistentPluginRuntime) awaitPromiseLocked(
	invocation *persistentPluginRuntimeInvocation,
	promise *goja.Promise,
) (goja.Value, error) {
	if promise == nil {
		return goja.Undefined(), nil
	}
	for {
		if err := r.drainAsyncTasksLocked(); err != nil {
			return nil, err
		}
		switch promise.State() {
		case goja.PromiseStateFulfilled:
			return promise.Result(), nil
		case goja.PromiseStateRejected:
			return nil, runtimePromiseRejectionError(promise.Result())
		}

		if invocation == nil {
			return nil, fmt.Errorf("pending promise cannot settle without invocation context")
		}

		var timer *time.Timer
		var deadlineCh <-chan time.Time
		if !invocation.deadline.IsZero() {
			wait := time.Until(invocation.deadline)
			if wait <= 0 {
				return nil, fmt.Errorf("execution timeout")
			}
			timer = time.NewTimer(wait)
			deadlineCh = timer.C
		}
		pollTimer := time.NewTimer(2 * time.Millisecond)

		select {
		case <-r.asyncWake:
		case <-pollTimer.C:
		case <-invocation.execCtx.Done():
			if timer != nil {
				timer.Stop()
			}
			pollTimer.Stop()
			return nil, invocation.execCtx.Err()
		case <-deadlineCh:
			if timer != nil {
				timer.Stop()
			}
			pollTimer.Stop()
			return nil, fmt.Errorf("execution timeout")
		}

		if timer != nil {
			timer.Stop()
		}
		pollTimer.Stop()
	}
}

func (i *persistentPluginRuntimeInvocation) registerTimer(id int64, timer *persistentPluginRuntimeTimer) bool {
	if i == nil || id <= 0 || timer == nil || timer.timer == nil {
		return false
	}
	i.timerMu.Lock()
	defer i.timerMu.Unlock()
	if i.closed {
		return false
	}
	if i.timers == nil {
		i.timers = make(map[int64]*persistentPluginRuntimeTimer)
	}
	i.timers[id] = timer
	return true
}

func (i *persistentPluginRuntimeInvocation) deleteTimer(id int64) *persistentPluginRuntimeTimer {
	if i == nil || id <= 0 {
		return nil
	}
	i.timerMu.Lock()
	defer i.timerMu.Unlock()
	if len(i.timers) == 0 {
		return nil
	}
	timer := i.timers[id]
	delete(i.timers, id)
	return timer
}

func (i *persistentPluginRuntimeInvocation) clearTimer(id int64) bool {
	timer := i.deleteTimer(id)
	if timer == nil {
		return false
	}
	if timer.timer != nil {
		timer.timer.Stop()
	}
	if timer.done != nil {
		close(timer.done)
	}
	return true
}

func (i *persistentPluginRuntimeInvocation) isActive() bool {
	if i == nil {
		return false
	}
	i.timerMu.Lock()
	defer i.timerMu.Unlock()
	return !i.closed
}

func (i *persistentPluginRuntimeInvocation) retainAsync() bool {
	if i == nil {
		return false
	}
	i.timerMu.Lock()
	defer i.timerMu.Unlock()
	if i.closed {
		return false
	}
	i.asyncRefs++
	return true
}

func (i *persistentPluginRuntimeInvocation) releaseAsync() {
	if i == nil {
		return
	}
	i.timerMu.Lock()
	if i.asyncRefs > 0 {
		i.asyncRefs--
	}
	shouldClose := !i.closed && !i.requestActive && i.asyncRefs == 0
	i.timerMu.Unlock()
	if shouldClose {
		i.forceClose()
	}
}

func (i *persistentPluginRuntimeInvocation) endRequest() bool {
	if i == nil {
		return false
	}
	i.timerMu.Lock()
	if i.closed {
		i.timerMu.Unlock()
		return false
	}
	i.requestActive = false
	shouldClose := i.asyncRefs == 0
	i.timerMu.Unlock()
	if shouldClose {
		i.forceClose()
		return true
	}
	return false
}

func (i *persistentPluginRuntimeInvocation) forceClose() {
	if i == nil {
		return
	}
	i.timerMu.Lock()
	if i.closed {
		i.timerMu.Unlock()
		return
	}
	i.closed = true
	i.requestActive = false
	timers := i.timers
	i.timers = nil
	workerGroup := i.workerGroup
	i.workerGroup = nil
	hostSessionStop := i.hostSessionStop
	i.hostSessionStop = nil
	i.timerMu.Unlock()
	for _, timer := range timers {
		if timer == nil {
			continue
		}
		if timer.timer != nil {
			timer.timer.Stop()
		}
		if timer.done != nil {
			close(timer.done)
		}
	}
	if workerGroup != nil {
		workerGroup.close()
	}
	if hostSessionStop != nil {
		hostSessionStop()
	}
}

func (i *persistentPluginRuntimeInvocation) finish() {
	i.forceClose()
}

func (r *persistentPluginRuntime) clearRuntimeTimeout(id int64) bool {
	if r == nil || id <= 0 {
		return false
	}
	invocation := r.lookupRuntimeTimerOwner(id)
	if invocation == nil {
		return false
	}
	if !invocation.clearTimer(id) {
		return false
	}
	r.unregisterRuntimeTimer(id)
	invocation.releaseAsync()
	return true
}

func (r *persistentPluginRuntime) scheduleInvocationTimeout(
	invocation *persistentPluginRuntimeInvocation,
	callback goja.Callable,
	delay time.Duration,
	args []goja.Value,
) (int64, error) {
	if r == nil || r.vm == nil {
		return 0, fmt.Errorf("setTimeout is unavailable")
	}
	if invocation == nil || callback == nil {
		return 0, fmt.Errorf("setTimeout is unavailable for the current invocation")
	}
	if delay < 0 {
		delay = 0
	}
	if !invocation.retainAsync() {
		return 0, fmt.Errorf("setTimeout is unavailable for the current invocation")
	}
	timerID := r.nextRuntimeTimerID()
	timer := &persistentPluginRuntimeTimer{
		timer: time.NewTimer(delay),
		done:  make(chan struct{}),
	}
	if !invocation.registerTimer(timerID, timer) {
		timer.timer.Stop()
		close(timer.done)
		invocation.releaseAsync()
		return 0, fmt.Errorf("setTimeout is unavailable for the current invocation")
	}
	r.registerRuntimeTimer(timerID, invocation)
	go func() {
		select {
		case <-timer.done:
			return
		case <-timer.timer.C:
		}
		if invocation.deleteTimer(timerID) == nil {
			return
		}
		select {
		case <-timer.done:
			return
		default:
		}
		r.unregisterRuntimeTimer(timerID)
		if !invocation.isActive() {
			invocation.releaseAsync()
			return
		}
		if !r.enqueueAsyncTask(runtimeAsyncTask{
			invocation:        invocation,
			action:            "setTimeout",
			releaseInvocation: true,
			run: func() error {
				_, err := callback(goja.Undefined(), args...)
				return err
			},
		}) {
			invocation.releaseAsync()
		}
	}()
	return timerID, nil
}

func (r *persistentPluginRuntime) queueInvocationMicrotask(
	invocation *persistentPluginRuntimeInvocation,
	callback goja.Callable,
) error {
	if r == nil || callback == nil {
		return fmt.Errorf("queueMicrotask is unavailable")
	}
	if invocation == nil || !invocation.isActive() {
		return fmt.Errorf("queueMicrotask is unavailable for the current invocation")
	}
	if !r.enqueueInvocationAsyncTask(invocation, "queueMicrotask", func() error {
		if !invocation.isActive() {
			return nil
		}
		_, err := callback(goja.Undefined())
		return err
	}) {
		return fmt.Errorf("queueMicrotask is unavailable for the current invocation")
	}
	return nil
}

func (r *persistentPluginRuntime) installAsyncRuntimeGlobals() {
	if r == nil || r.vm == nil {
		return
	}
	installRuntimeAsyncCompatibilityGlobals(r.vm, runtimeAsyncGlobalHooks{
		StructuredClone: func(value goja.Value) (goja.Value, error) {
			return runtimeStructuredCloneValue(r.vm, value)
		},
		QueueMicrotask: func(callback goja.Callable) error {
			return r.queueInvocationMicrotask(r.currentInvocation(), callback)
		},
		SetTimeout: func(callback goja.Callable, delay time.Duration, args []goja.Value) (int64, error) {
			return r.scheduleInvocationTimeout(r.currentInvocation(), callback, delay, args)
		},
		ClearTimeout: func(id int64) bool {
			return r.clearRuntimeTimeout(id)
		},
	})
}

func (r *persistentPluginRuntime) executeAsyncTaskLocked(task runtimeAsyncTask) (err error) {
	if r == nil || task.run == nil {
		return nil
	}
	invocation := task.invocation
	if invocation != nil && !invocation.isActive() {
		if task.releaseInvocation {
			invocation.releaseAsync()
		}
		return nil
	}

	action := strings.TrimSpace(task.action)
	if action == "" && invocation != nil {
		action = strings.ToLower(strings.TrimSpace(invocation.sandboxCfg.CurrentAction))
	}
	if action == "" {
		action = "runtime.async"
	}

	prevInvoke := r.currentInvoke
	prevAction := r.currentAction
	r.currentInvoke = invocation
	r.currentAction = action
	if invocation != nil && prevInvoke != invocation {
		if applyErr := r.applyInvocation(invocation); applyErr != nil {
			r.currentInvoke = prevInvoke
			r.currentAction = prevAction
			if task.releaseInvocation {
				invocation.releaseAsync()
			}
			return applyErr
		}
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic in async script runtime: %v", recovered)
			r.lastError = err.Error()
			_ = r.resetLocked()
		}
		if task.releaseInvocation && invocation != nil {
			invocation.releaseAsync()
		}
		if invocation != nil && invocation.workspaceState != nil && err != nil {
			invocation.workspaceState.write(
				"stderr",
				"error",
				err.Error(),
				"runtime.async",
				map[string]string{"action": action},
			)
		}
		if prevInvoke != nil && prevInvoke != invocation && prevInvoke.isActive() && r.vm != nil {
			_ = r.applyInvocation(prevInvoke)
		}
		r.currentInvoke = prevInvoke
		r.currentAction = prevAction
		if r.vm != nil {
			r.vm.ClearInterrupt()
		}
	}()

	if err = task.run(); err != nil {
		r.lastError = err.Error()
		return err
	}
	if r.vm != nil {
		if _, flushErr := r.vm.RunString(""); flushErr != nil {
			err = fmt.Errorf("flush async jobs failed: %w", flushErr)
			r.lastError = err.Error()
			return err
		}
	}
	r.lastError = ""
	return nil
}
