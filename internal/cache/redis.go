package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache is a Cache backed by Redis.
// Redis errors are treated as cache misses so the cache is never a blocking
// dependency in the request path (RULES §0.3, §4).
type RedisCache struct {
	client      *redis.Client
	ttlPositive time.Duration
	ttlNegative time.Duration
	timeout     time.Duration
	log         *slog.Logger
}

// NewRedisCache constructs a RedisCache. It does not verify connectivity at
// startup; the app starts and resolves live if Redis is unavailable.
func NewRedisCache(redisURL string, ttlPositive, ttlNegative, timeout time.Duration, log *slog.Logger) (*RedisCache, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}
	return &RedisCache{
		client:      redis.NewClient(opts),
		ttlPositive: ttlPositive,
		ttlNegative: ttlNegative,
		timeout:     timeout,
		log:         log,
	}, nil
}

// Get implements [Cache].
func (c *RedisCache) Get(ctx context.Context, key string) (Entry, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return Entry{}, false, nil
	}
	if err != nil {
		c.log.WarnContext(ctx, "redis get error", "key", key, "err", err)
		return Entry{}, false, nil // graceful degradation: treat as miss
	}

	var e Entry
	if err := json.Unmarshal([]byte(val), &e); err != nil {
		c.log.WarnContext(ctx, "redis unmarshal error", "key", key, "err", err)
		return Entry{}, false, nil
	}
	return e, true, nil
}

// Set implements [Cache].
func (c *RedisCache) Set(ctx context.Context, key string, e Entry) error {
	e.StoredAt = time.Now()

	data, err := json.Marshal(e)
	if err != nil {
		// Entry fields are all basic types; Marshal failure here is a programmer error.
		return fmt.Errorf("marshal cache entry: %w", err)
	}

	ttl := c.ttlNegative
	if e.Positive {
		ttl = c.ttlPositive
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	if err := c.client.Set(ctx, key, data, ttl).Err(); err != nil {
		c.log.WarnContext(ctx, "redis set error", "key", key, "err", err)
		// graceful: don't propagate; next request resolves live
	}
	return nil
}

// Delete implements [Cache].
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	if err := c.client.Del(ctx, key).Err(); err != nil {
		c.log.WarnContext(ctx, "redis delete error", "key", key, "err", err)
	}
	return nil
}

// Close releases the underlying Redis connection pool.
func (c *RedisCache) Close() error {
	return c.client.Close()
}
