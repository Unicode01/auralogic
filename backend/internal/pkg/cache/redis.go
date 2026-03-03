package cache

import (
	"context"
	"fmt"
	"log"
	"time"

	"auralogic/internal/config"
	"github.com/go-redis/redis/v8"
)

var (
	RedisClient *redis.Client
	ctx         = context.Background()
)

// InitRedis 初始化Redis连接
func InitRedis(cfg *config.RedisConfig) error {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     cfg.GetRedisAddr(),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	// 测试连接
	_, err := RedisClient.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to redis: %w", err)
	}

	log.Println("Redis connected successfully")
	return nil
}

// Get get缓存
func Get(key string) (string, error) {
	return RedisClient.Get(ctx, key).Result()
}

// Set 设置缓存
func Set(key string, value interface{}, expiration time.Duration) error {
	return RedisClient.Set(ctx, key, value, expiration).Err()
}

// Del Delete缓存
func Del(keys ...string) error {
	return RedisClient.Del(ctx, keys...).Err()
}

// Exists 检查键是否存在
func Exists(keys ...string) (int64, error) {
	return RedisClient.Exists(ctx, keys...).Result()
}

// Expire 设置过期时间
func Expire(key string, expiration time.Duration) error {
	return RedisClient.Expire(ctx, key, expiration).Err()
}

// Incr 增加计数
func Incr(key string) (int64, error) {
	return RedisClient.Incr(ctx, key).Result()
}

// SetNX 仅当key不存在时设置，返回是否设置成功
func SetNX(key string, value interface{}, expiration time.Duration) (bool, error) {
	return RedisClient.SetNX(ctx, key, value, expiration).Result()
}

// DeleteByPatterns 按通配符模式删除缓存键，返回删除数量
func DeleteByPatterns(patterns ...string) (int64, error) {
	if RedisClient == nil {
		return 0, fmt.Errorf("redis client not initialized")
	}

	var deleted int64
	for _, pattern := range patterns {
		var cursor uint64
		for {
			keys, nextCursor, err := RedisClient.Scan(ctx, cursor, pattern, 500).Result()
			if err != nil {
				return deleted, err
			}

			if len(keys) > 0 {
				n, err := RedisClient.Del(ctx, keys...).Result()
				if err != nil {
					return deleted, err
				}
				deleted += n
			}

			cursor = nextCursor
			if cursor == 0 {
				break
			}
		}
	}

	return deleted, nil
}

// Close 关闭Redis连接
func Close() error {
	if RedisClient != nil {
		return RedisClient.Close()
	}
	return nil
}
