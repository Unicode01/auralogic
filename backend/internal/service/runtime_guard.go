package service

import (
	"context"
	"log"
	"runtime/debug"
	"time"
)

func recoverBackgroundServicePanic(name string) {
	if recovered := recover(); recovered != nil {
		log.Printf("[panic-guard] %s panic recovered: %v\n%s", name, recovered, debug.Stack())
	}
}

const backgroundServiceRestartDelay = time.Second

func runBackgroundServiceWithStopChan(name string, stopChan <-chan struct{}, loop func(stopChan <-chan struct{})) {
	runBackgroundServiceWithRestart(
		name,
		func() bool { return isBackgroundServiceStopChanClosed(stopChan) },
		func(delay time.Duration) bool { return waitBackgroundServiceStopChan(stopChan, delay) },
		func() { loop(stopChan) },
	)
}

func runBackgroundServiceWithContext(name string, ctx context.Context, loop func(ctx context.Context)) {
	if ctx == nil {
		ctx = context.Background()
	}
	runBackgroundServiceWithRestart(
		name,
		func() bool { return isBackgroundServiceContextDone(ctx) },
		func(delay time.Duration) bool { return waitBackgroundServiceContext(ctx, delay) },
		func() { loop(ctx) },
	)
}

func runBackgroundServiceWithRestart(
	name string,
	shouldStop func() bool,
	waitStop func(delay time.Duration) bool,
	loop func(),
) {
	for {
		panicked := false
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					panicked = true
					log.Printf("[panic-guard] %s panic recovered: %v\n%s", name, recovered, debug.Stack())
				}
			}()
			loop()
		}()

		if !panicked {
			return
		}
		if shouldStop != nil && shouldStop() {
			return
		}

		log.Printf("[panic-guard] %s restarting after panic", name)
		if waitStop != nil && waitStop(backgroundServiceRestartDelay) {
			return
		}
	}
}

func isBackgroundServiceStopChanClosed(stopChan <-chan struct{}) bool {
	if stopChan == nil {
		return false
	}
	select {
	case <-stopChan:
		return true
	default:
		return false
	}
}

func waitBackgroundServiceStopChan(stopChan <-chan struct{}, delay time.Duration) bool {
	if stopChan == nil {
		time.Sleep(delay)
		return false
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-stopChan:
		return true
	case <-timer.C:
		return false
	}
}

func isBackgroundServiceContextDone(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func waitBackgroundServiceContext(ctx context.Context, delay time.Duration) bool {
	if ctx == nil {
		time.Sleep(delay)
		return false
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return true
	case <-timer.C:
		return false
	}
}
