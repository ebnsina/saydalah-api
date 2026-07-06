package middleware

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"

	"github.com/ebnsina/saydalah-api/internal/httpx"
)

// visitor is a per-client token bucket with a last-seen timestamp for eviction.
type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// rateLimiter keeps one token bucket per client IP, evicting idle ones.
type rateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rps      rate.Limit
	burst    int
}

// RateLimit returns middleware that limits each client IP to rps requests per
// second with the given burst. Over-limit requests get 429 with Retry-After.
//
// When rdb is non-nil the counter lives in Redis, so the limit is shared across
// every API instance (prefix namespaces the two limiters). When rdb is nil it
// falls back to a per-process in-memory bucket, with a background sweeper that
// evicts idle clients so the map does not grow unbounded.
func RateLimit(rdb *redis.Client, prefix string, rps float64, burst int) func(http.Handler) http.Handler {
	if rdb != nil {
		return redisLimit(rdb, prefix, rps, burst)
	}
	rl := &rateLimiter{
		visitors: make(map[string]*visitor),
		rps:      rate.Limit(rps),
		burst:    burst,
	}
	go rl.sweep()
	return rl.handle
}

// redisLimit implements the same limit backed by Redis (shared across instances).
func redisLimit(rdb *redis.Client, prefix string, rps float64, burst int) func(http.Handler) http.Handler {
	limiter := redis_rate.NewLimiter(rdb)
	lim := limitFrom(rps, burst)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			res, err := limiter.Allow(r.Context(), prefix+":"+clientIP(r), lim)
			if err != nil {
				// Fail open: a Redis blip must not take the API down.
				next.ServeHTTP(w, r)
				return
			}
			if res.Allowed == 0 {
				w.Header().Set("Retry-After", strconv.Itoa(int(res.RetryAfter.Seconds())+1))
				httpx.Error(w, r, httpx.NewError(http.StatusTooManyRequests, "rate limit exceeded"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// limitFrom converts rps + burst into a redis_rate.Limit, handling sub-1 rps
// (e.g. the login limiter) by widening the period instead of rounding to zero.
func limitFrom(rps float64, burst int) redis_rate.Limit {
	if rps >= 1 {
		return redis_rate.Limit{Rate: int(rps), Burst: burst, Period: time.Second}
	}
	period := time.Duration(float64(time.Second) / rps)
	return redis_rate.Limit{Rate: 1, Burst: burst, Period: period}
}

func (rl *rateLimiter) limiterFor(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if v, ok := rl.visitors[ip]; ok {
		v.lastSeen = time.Now()
		return v.limiter
	}
	lim := rate.NewLimiter(rl.rps, rl.burst)
	rl.visitors[ip] = &visitor{limiter: lim, lastSeen: time.Now()}
	return lim
}

func (rl *rateLimiter) sweep() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > 3*time.Minute {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *rateLimiter) handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.limiterFor(clientIP(r)).Allow() {
			w.Header().Set("Retry-After", "1")
			httpx.Error(w, r, httpx.NewError(http.StatusTooManyRequests, "rate limit exceeded"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP extracts the caller's IP, honoring X-Forwarded-For (first hop) when
// the API sits behind a proxy/load balancer.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first, _, ok := strings.Cut(xff, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
