package service

import (
	"fmt"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/pkg/cache"
	"github.com/go-redis/redis/v8"
)

var reserveMessageRateLimitScript = redis.NewScript(`
local hourly_limit = tonumber(ARGV[1])
local daily_limit = tonumber(ARGV[2])
local hour_expire = tonumber(ARGV[3])
local day_expire = tonumber(ARGV[4])

local function current(key)
	local value = redis.call('GET', key)
	if not value then
		return 0
	end
	return tonumber(value)
end

local hour_count = current(KEYS[1])
local day_count = current(KEYS[2])

if hourly_limit > 0 and hour_count >= hourly_limit then
	return {0, 'hour'}
end

if daily_limit > 0 and day_count >= daily_limit then
	return {0, 'day'}
end

if hourly_limit > 0 then
	local next_hour = redis.call('INCR', KEYS[1])
	if next_hour == 1 then
		redis.call('EXPIRE', KEYS[1], hour_expire)
	end
	if next_hour > hourly_limit then
		redis.call('DECR', KEYS[1])
		return {0, 'hour'}
	end
end

if daily_limit > 0 then
	local next_day = redis.call('INCR', KEYS[2])
	if next_day == 1 then
		redis.call('EXPIRE', KEYS[2], day_expire)
	end
	if next_day > daily_limit then
		redis.call('DECR', KEYS[2])
		if hourly_limit > 0 then
			redis.call('DECR', KEYS[1])
		end
		return {0, 'day'}
	end
end

return {1, ''}
`)

func messageRateLimitKeys(prefix, recipient string, now time.Time) (string, string) {
	return fmt.Sprintf("%s_rate:%s:%s", prefix, recipient, now.Format("2006010215")),
		fmt.Sprintf("%s_rate:%s:%s", prefix, recipient, now.Format("20060102"))
}

func messageRateLimitAvailableAt(now time.Time, scope string) time.Time {
	switch scope {
	case "day":
		return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	case "hour":
		fallthrough
	default:
		return now.Truncate(time.Hour).Add(time.Hour)
	}
}

func messageRateLimitTTLSeconds(now time.Time, scope string) int {
	availableAt := messageRateLimitAvailableAt(now, scope)
	ttl := int(time.Until(availableAt).Seconds())
	if ttl < 1 {
		return 1
	}
	return ttl
}

// reserveMessageRateLimitSlot atomically checks and consumes one rate-limit slot.
// When Redis is unavailable, it fails open to avoid breaking transactional flows.
func reserveMessageRateLimitSlot(prefix, recipient string, rl config.MessageRateLimit) (bool, time.Time, error) {
	if rl.Hourly <= 0 && rl.Daily <= 0 {
		return true, time.Time{}, nil
	}
	if cache.RedisClient == nil {
		return true, time.Time{}, nil
	}

	now := time.Now()
	hourKey, dayKey := messageRateLimitKeys(prefix, recipient, now)
	values, err := reserveMessageRateLimitScript.Run(
		cache.RedisClient.Context(),
		cache.RedisClient,
		[]string{hourKey, dayKey},
		rl.Hourly,
		rl.Daily,
		messageRateLimitTTLSeconds(now, "hour"),
		messageRateLimitTTLSeconds(now, "day"),
	).Result()
	if err != nil {
		return true, time.Time{}, err
	}

	result, ok := values.([]interface{})
	if !ok || len(result) < 2 {
		return true, time.Time{}, fmt.Errorf("unexpected rate limit script result: %T", values)
	}

	allowed, ok := result[0].(int64)
	if !ok {
		return true, time.Time{}, fmt.Errorf("unexpected rate limit allowed result: %T", result[0])
	}
	if allowed == 1 {
		return true, time.Time{}, nil
	}

	scope, _ := result[1].(string)
	return false, messageRateLimitAvailableAt(now, scope), nil
}
