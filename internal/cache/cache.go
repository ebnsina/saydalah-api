// Package cache provides an optional Redis-backed read cache. When no Redis
// client is configured every call simply computes the value, so callers behave
// identically with or without Redis.
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// Connect opens a Redis client from a URL (redis://host:port/db) and pings it.
func Connect(ctx context.Context, url string) (*redis.Client, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("cache: parse url: %w", err)
	}
	rdb := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("cache: ping: %w", err)
	}
	return rdb, nil
}

// Cache is a thin JSON read-cache over Redis. A nil client disables caching.
type Cache struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Cache { return &Cache{rdb: rdb} }

// GetOrSet returns the cached JSON for key, or computes it, stores it under key
// with the given TTL, and returns the marshalled bytes. Redis errors are
// non-fatal: they log and fall back to computing the value.
func (c *Cache) GetOrSet(ctx context.Context, key string, ttl time.Duration, compute func() (any, error)) ([]byte, error) {
	if c == nil || c.rdb == nil {
		return marshal(compute)
	}
	if b, err := c.rdb.Get(ctx, key).Bytes(); err == nil {
		return b, nil
	} else if err != redis.Nil {
		slog.Warn("cache: get failed", "key", key, "error", err)
	}
	body, err := marshal(compute)
	if err != nil {
		return nil, err
	}
	if err := c.rdb.Set(ctx, key, body, ttl).Err(); err != nil {
		slog.Warn("cache: set failed", "key", key, "error", err)
	}
	return body, nil
}

func marshal(compute func() (any, error)) ([]byte, error) {
	v, err := compute()
	if err != nil {
		return nil, err
	}
	return json.Marshal(v)
}
