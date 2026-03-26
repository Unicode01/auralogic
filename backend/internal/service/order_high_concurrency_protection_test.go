package service

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/pkg/cache"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
)

func TestOrderHighConcurrencyProtectionMemoryModeLimitsConcurrency(t *testing.T) {
	cfg := &config.Config{}
	cfg.Order.HighConcurrencyProtection = config.OrderHighConcurrencyProtectionConfig{
		Enabled:       true,
		Mode:          "memory",
		MaxInFlight:   2,
		WaitTimeoutMs: 1000,
		RedisLeaseMs:  5000,
	}

	var current atomic.Int64
	var maxSeen atomic.Int64
	start := make(chan struct{})
	errs := make(chan error, 8)
	var wg sync.WaitGroup

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			release, err := acquireOrderHighConcurrencyProtection(cfg, orderHotPathSubmitShippingForm)
			if err != nil {
				errs <- err
				return
			}
			defer release()

			active := current.Add(1)
			for {
				previous := maxSeen.Load()
				if active <= previous || maxSeen.CompareAndSwap(previous, active) {
					break
				}
			}
			time.Sleep(40 * time.Millisecond)
			current.Add(-1)
		}()
	}

	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("unexpected acquire error: %v", err)
		}
	}
	if maxSeen.Load() > 2 {
		t.Fatalf("expected max concurrency <= 2, got %d", maxSeen.Load())
	}
}

func TestOrderHighConcurrencyProtectionRedisModeLimitsConcurrency(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis failed: %v", err)
	}
	defer mr.Close()

	previousClient := cache.RedisClient
	cache.RedisClient = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() {
		if cache.RedisClient != nil {
			_ = cache.RedisClient.Close()
		}
		cache.RedisClient = previousClient
	}()

	cfg := &config.Config{}
	cfg.Order.HighConcurrencyProtection = config.OrderHighConcurrencyProtectionConfig{
		Enabled:       true,
		Mode:          "redis",
		MaxInFlight:   2,
		WaitTimeoutMs: 1000,
		RedisLeaseMs:  5000,
	}

	var current atomic.Int64
	var maxSeen atomic.Int64
	start := make(chan struct{})
	errs := make(chan error, 8)
	var wg sync.WaitGroup

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			release, err := acquireOrderHighConcurrencyProtection(cfg, orderHotPathConfirmPaymentResult)
			if err != nil {
				errs <- err
				return
			}
			defer release()

			active := current.Add(1)
			for {
				previous := maxSeen.Load()
				if active <= previous || maxSeen.CompareAndSwap(previous, active) {
					break
				}
			}
			time.Sleep(40 * time.Millisecond)
			current.Add(-1)
		}()
	}

	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("unexpected acquire error: %v", err)
		}
	}
	if maxSeen.Load() > 2 {
		t.Fatalf("expected max concurrency <= 2, got %d", maxSeen.Load())
	}
}

func TestOrderHighConcurrencyProtectionReturnsBusyOnTimeout(t *testing.T) {
	cfg := &config.Config{}
	cfg.Order.HighConcurrencyProtection = config.OrderHighConcurrencyProtectionConfig{
		Enabled:       true,
		Mode:          "memory",
		MaxInFlight:   1,
		WaitTimeoutMs: 25,
		RedisLeaseMs:  5000,
	}

	release, err := acquireOrderHighConcurrencyProtection(cfg, orderHotPathCreateUserOrder)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	defer release()

	_, err = acquireOrderHighConcurrencyProtection(cfg, orderHotPathCreateUserOrder)
	if err == nil {
		t.Fatal("expected busy error, got nil")
	}
	if !isOrderHighConcurrencyBusyError(err) {
		t.Fatalf("expected busy error type, got %v", err)
	}
}
