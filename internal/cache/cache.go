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

	"github.com/google/uuid"
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

// Bump increments the report-cache version for a branch and the chain-wide
// scope, instantly invalidating any cached report whose key embeds that version
// (see reporting.reportKey). Fire-and-forget: a Redis error just means the 60s
// TTL is the fallback. Pass every branch a write touched (e.g. both ends of a
// transfer).
func (c *Cache) Bump(ctx context.Context, branchIDs ...uuid.UUID) {
	if c == nil || c.rdb == nil {
		return
	}
	pipe := c.rdb.Pipeline()
	pipe.Incr(ctx, "rptver:all")
	for _, id := range branchIDs {
		pipe.Incr(ctx, "rptver:"+id.String())
	}
	if _, err := pipe.Exec(ctx); err != nil {
		slog.Warn("cache: bump failed", "error", err)
	}
}

// Version returns the current cache version for a scope ("all" or a branch id),
// or 0 when Redis is absent. Folding it into a cache key makes a Bump on that
// scope miss every prior entry.
func (c *Cache) Version(ctx context.Context, scope string) int64 {
	if c == nil || c.rdb == nil {
		return 0
	}
	n, err := c.rdb.Get(ctx, "rptver:"+scope).Int64()
	if err != nil && err != redis.Nil {
		slog.Warn("cache: version read failed", "scope", scope, "error", err)
	}
	return n
}

func marshal(compute func() (any, error)) ([]byte, error) {
	v, err := compute()
	if err != nil {
		return nil, err
	}
	return json.Marshal(v)
}
