package service

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestRunBackgroundServiceWithStopChanRestartsAfterPanic(t *testing.T) {
	stopChan := make(chan struct{})
	doneChan := make(chan struct{})
	restartedChan := make(chan struct{}, 1)
	var runCount atomic.Int32

	go func() {
		defer close(doneChan)
		runBackgroundServiceWithStopChan("test.background", stopChan, func(stop <-chan struct{}) {
			currentRun := runCount.Add(1)
			if currentRun == 1 {
				panic("boom")
			}
			select {
			case restartedChan <- struct{}{}:
			default:
			}
			<-stop
		})
	}()

	select {
	case <-restartedChan:
	case <-time.After(5 * time.Second):
		t.Fatal("expected background worker to restart after panic")
	}

	close(stopChan)

	select {
	case <-doneChan:
	case <-time.After(5 * time.Second):
		t.Fatal("expected background worker wrapper to exit after stop")
	}

	if runCount.Load() < 2 {
		t.Fatalf("expected at least two loop runs, got %d", runCount.Load())
	}
}
