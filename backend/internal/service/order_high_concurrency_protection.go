package service

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/pkg/cache"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

const (
	orderHotPathCreateUserOrder      = "create_user_order"
	orderHotPathSubmitShippingForm   = "submit_shipping_form"
	orderHotPathConfirmPaymentResult = "confirm_payment_result"
)

var (
	orderHighConcurrencyAcquireScript = redis.NewScript(`
local key = KEYS[1]
local token = ARGV[1]
local now = tonumber(ARGV[2])
local expires = tonumber(ARGV[3])
local limit = tonumber(ARGV[4])
redis.call('ZREMRANGEBYSCORE', key, '-inf', now)
if redis.call('ZCARD', key) >= limit then
  return 0
end
redis.call('ZADD', key, expires, token)
redis.call('PEXPIRE', key, math.max(expires - now, 1000))
return 1
`)
	orderHighConcurrencyReleaseScript = redis.NewScript(`
local key = KEYS[1]
local token = ARGV[1]
redis.call('ZREM', key, token)
if redis.call('ZCARD', key) <= 0 then
  redis.call('DEL', key)
end
return 1
`)
	orderHighConcurrencyMemoryGates sync.Map
)

type orderHighConcurrencyBusyError struct {
	HotPath     string
	Mode        string
	MaxInFlight int
	WaitTimeout time.Duration
}

func (e *orderHighConcurrencyBusyError) Error() string {
	if e == nil {
		return "order hot path is busy"
	}
	if e.WaitTimeout > 0 {
		return fmt.Sprintf(
			"order hot path %s is busy (mode=%s, max_inflight=%d, wait_timeout=%s)",
			e.HotPath,
			e.Mode,
			e.MaxInFlight,
			e.WaitTimeout,
		)
	}
	return fmt.Sprintf(
		"order hot path %s is busy (mode=%s, max_inflight=%d)",
		e.HotPath,
		e.Mode,
		e.MaxInFlight,
	)
}

func isOrderHighConcurrencyBusyError(err error) bool {
	var target *orderHighConcurrencyBusyError
	return errors.As(err, &target)
}

type orderHighConcurrencyMemoryGate struct {
	sem chan struct{}
}

func acquireOrderHighConcurrencyProtection(cfg *config.Config, hotPath string) (func(), error) {
	if cfg == nil {
		return func() {}, nil
	}

	protection := cfg.Order.HighConcurrencyProtection
	if !protection.Enabled || protection.MaxInFlight <= 0 {
		return func() {}, nil
	}

	mode := strings.ToLower(strings.TrimSpace(protection.Mode))
	if mode == "" {
		mode = "auto"
	}

	waitTimeout := time.Duration(protection.WaitTimeoutMs) * time.Millisecond
	if waitTimeout < 0 {
		waitTimeout = 0
	}
	redisLease := time.Duration(protection.RedisLeaseMs) * time.Millisecond
	if redisLease <= 0 {
		redisLease = 30 * time.Second
	}

	switch mode {
	case "memory":
		return acquireOrderHighConcurrencyMemoryGate(hotPath, protection.MaxInFlight, waitTimeout)
	case "redis":
		release, err := acquireOrderHighConcurrencyRedisGate(hotPath, protection.MaxInFlight, waitTimeout, redisLease)
		if err == nil {
			return release, nil
		}
		log.Printf("order high concurrency protection redis mode fallback to memory: path=%s err=%v", hotPath, err)
		return acquireOrderHighConcurrencyMemoryGate(hotPath, protection.MaxInFlight, waitTimeout)
	default:
		if cache.RedisClient != nil {
			release, err := acquireOrderHighConcurrencyRedisGate(hotPath, protection.MaxInFlight, waitTimeout, redisLease)
			if err == nil {
				return release, nil
			}
			log.Printf("order high concurrency protection auto mode fallback to memory: path=%s err=%v", hotPath, err)
		}
		return acquireOrderHighConcurrencyMemoryGate(hotPath, protection.MaxInFlight, waitTimeout)
	}
}

func acquireOrderHighConcurrencyMemoryGate(hotPath string, maxInFlight int, waitTimeout time.Duration) (func(), error) {
	if maxInFlight <= 0 {
		return func() {}, nil
	}

	key := fmt.Sprintf("%s#%d", hotPath, maxInFlight)
	gateAny, _ := orderHighConcurrencyMemoryGates.LoadOrStore(key, &orderHighConcurrencyMemoryGate{
		sem: make(chan struct{}, maxInFlight),
	})
	gate := gateAny.(*orderHighConcurrencyMemoryGate)

	if waitTimeout <= 0 {
		select {
		case gate.sem <- struct{}{}:
			return func() { <-gate.sem }, nil
		default:
			return nil, &orderHighConcurrencyBusyError{
				HotPath:     hotPath,
				Mode:        "memory",
				MaxInFlight: maxInFlight,
				WaitTimeout: waitTimeout,
			}
		}
	}

	timer := time.NewTimer(waitTimeout)
	defer timer.Stop()

	select {
	case gate.sem <- struct{}{}:
		return func() { <-gate.sem }, nil
	case <-timer.C:
		return nil, &orderHighConcurrencyBusyError{
			HotPath:     hotPath,
			Mode:        "memory",
			MaxInFlight: maxInFlight,
			WaitTimeout: waitTimeout,
		}
	}
}

func acquireOrderHighConcurrencyRedisGate(hotPath string, maxInFlight int, waitTimeout time.Duration, redisLease time.Duration) (func(), error) {
	if maxInFlight <= 0 {
		return func() {}, nil
	}
	if cache.RedisClient == nil {
		return nil, fmt.Errorf("redis client is not initialized")
	}

	key := fmt.Sprintf("order:hot_path:%s", hotPath)
	token := uuid.NewString()
	deadline := time.Now().Add(waitTimeout)
	for {
		now := time.Now()
		granted, err := orderHighConcurrencyAcquireScript.Run(
			cache.RedisClient.Context(),
			cache.RedisClient,
			[]string{key},
			token,
			now.UnixMilli(),
			now.Add(redisLease).UnixMilli(),
			maxInFlight,
		).Int()
		if err != nil {
			return nil, err
		}
		if granted == 1 {
			return func() {
				if releaseErr := orderHighConcurrencyReleaseScript.Run(
					cache.RedisClient.Context(),
					cache.RedisClient,
					[]string{key},
					token,
				).Err(); releaseErr != nil {
					log.Printf("order high concurrency protection redis release failed: path=%s err=%v", hotPath, releaseErr)
				}
			}, nil
		}
		if waitTimeout <= 0 || !time.Now().Before(deadline) {
			return nil, &orderHighConcurrencyBusyError{
				HotPath:     hotPath,
				Mode:        "redis",
				MaxInFlight: maxInFlight,
				WaitTimeout: waitTimeout,
			}
		}

		sleepFor := 25 * time.Millisecond
		remaining := time.Until(deadline)
		if remaining < sleepFor {
			sleepFor = remaining
		}
		if sleepFor <= 0 {
			return nil, &orderHighConcurrencyBusyError{
				HotPath:     hotPath,
				Mode:        "redis",
				MaxInFlight: maxInFlight,
				WaitTimeout: waitTimeout,
			}
		}
		time.Sleep(sleepFor)
	}
}
