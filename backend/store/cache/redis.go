package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache wraps go-redis/v9 to implement the Cache interface for distributed caching.
// Requires a Codec[V] for serialization since Redis stores bytes.
type RedisCache[K comparable, V any] struct {
	client    *redis.Client
	codec     Codec[V]
	keyPrefix string
}

// NewRedis creates a Redis-backed distributed cache.
// keyPrefix is prepended to all keys to avoid collisions across domains.
func NewRedis[K comparable, V any](url string, codec Codec[V], keyPrefix string) (*RedisCache[K, V], error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("cache/redis: parse URL: %w", err)
	}
	client := redis.NewClient(opts)
	return &RedisCache[K, V]{
		client:    client,
		codec:     codec,
		keyPrefix: keyPrefix,
	}, nil
}

// Get retrieves a cached value from Redis.
func (c *RedisCache[K, V]) Get(ctx context.Context, key K) (V, bool, error) {
	var zero V
	data, err := c.client.Get(ctx, c.fmtKey(key)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return zero, false, nil
		}
		return zero, false, fmt.Errorf("cache/redis: get %v: %w", key, err)
	}
	v, err := c.codec.Unmarshal(data)
	if err != nil {
		return zero, false, fmt.Errorf("cache/redis: unmarshal %v: %w", key, err)
	}
	return v, true, nil
}

// Set stores a value in Redis with an optional TTL.
func (c *RedisCache[K, V]) Set(ctx context.Context, key K, value V, ttl time.Duration) error {
	data, err := c.codec.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache/redis: marshal %v: %w", key, err)
	}
	return c.client.Set(ctx, c.fmtKey(key), data, ttl).Err()
}

// Delete removes a single key from Redis.
func (c *RedisCache[K, V]) Delete(ctx context.Context, key K) error {
	return c.client.Del(ctx, c.fmtKey(key)).Err()
}

// Purge removes all keys with the configured prefix using SCAN + DEL.
func (c *RedisCache[K, V]) Purge(ctx context.Context) error {
	var cursor uint64
	prefix := c.keyPrefix + ":"
	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, prefix+"*", 100).Result()
		if err != nil {
			return fmt.Errorf("cache/redis: scan for purge: %w", err)
		}
		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("cache/redis: del during purge: %w", err)
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

// Close closes the Redis client connection.
func (c *RedisCache[K, V]) Close() error {
	return c.client.Close()
}

func (c *RedisCache[K, V]) fmtKey(key K) string {
	return fmt.Sprintf("%s:%v", c.keyPrefix, key)
}
