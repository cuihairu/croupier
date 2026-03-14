package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache Redis 缓存实现
type RedisCache struct {
	client     *redis.Client
	defaultTTL time.Duration
}

// NewRedisCache 创建 Redis 缓存实例
func NewRedisCache(addr, password string, db int, poolSize int, defaultTTL time.Duration) (*RedisCache, error) {
	if poolSize <= 0 {
		poolSize = 10
	}
	if defaultTTL == 0 {
		defaultTTL = 5 * time.Minute
	}

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
		PoolSize: poolSize,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisCache{
		client:     client,
		defaultTTL: defaultTTL,
	}, nil
}

func (c *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, ErrCacheMiss
	}
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (c *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl == 0 {
		ttl = c.defaultTTL
	}
	return c.client.Set(ctx, key, value, ttl).Err()
}

func (c *RedisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

func (c *RedisCache) DeletePattern(ctx context.Context, pattern string) error {
	iter := c.client.Scan(ctx, 0, pattern, 100).Iterator()
	pipe := c.client.Pipeline()

	count := 0
	for iter.Next(ctx) {
		pipe.Del(ctx, iter.Val())
		count++

		// 每 100 个键执行一次批量删除
		if count%100 == 0 {
			if _, err := pipe.Exec(ctx); err != nil {
				return err
			}
			pipe = c.client.Pipeline()
		}
	}

	if err := iter.Err(); err != nil {
		return err
	}

	// 执行剩余的删除操作
	if count%100 != 0 {
		_, err := pipe.Exec(ctx)
		return err
	}

	return nil
}

func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (c *RedisCache) Close() error {
	return c.client.Close()
}

// Client 返回底层 Redis 客户端（用于高级操作）
func (c *RedisCache) Client() *redis.Client {
	return c.client
}
