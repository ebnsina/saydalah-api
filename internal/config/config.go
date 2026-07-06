// Package config loads and validates application configuration from the
// environment. Loading is fail-fast: an unset variable falls back to a sensible
// default, but a variable that is set to a malformed or out-of-range value
// causes Load to return an error at startup rather than silently reverting to a
// default — a bad override should never ship wrong behavior to production.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration for the API. Values are read from the
// environment once at startup and passed explicitly to the components that need
// them — no package-level globals.
type Config struct {
	Env             string        // "development" | "production"
	HTTPAddr        string        // e.g. ":8080"
	DatabaseURL     string        // postgres connection string
	RedisURL        string        // optional; enables shared rate-limit + report cache
	JWTSecret       string        // signing key for access tokens
	JWTTTL          time.Duration // access-token lifetime
	RefreshTTL      time.Duration // refresh-token lifetime
	CORSOrigins     []string      // allowed browser origins (future SvelteKit app)
	ShutdownTimeout time.Duration // graceful shutdown grace period

	RateLimitRPS   float64 // per-client requests/sec (global)
	RateLimitBurst int     // per-client burst (global)
	LoginRateRPS   float64 // per-client requests/sec on /auth/login
	LoginRateBurst int     // per-client burst on /auth/login

	// TaxRate is the sales tax/VAT rate as a fraction (e.g. 0.15 = 15%), applied
	// to (subtotal - discount) at checkout. Default 0 = tax-free.
	TaxRate float64

	// Optional first-admin bootstrap. When both are set and the users table is
	// empty, an admin account is created at startup so a fresh deployment is
	// immediately usable. Leave unset in environments that seed users elsewhere.
	AdminEmail    string
	AdminPassword string
}

// Load reads and validates configuration from the environment. It collects all
// problems and returns them together so a misconfigured deployment sees every
// issue at once.
func Load() (Config, error) {
	p := &parser{}

	cfg := Config{
		Env:             p.str("APP_ENV", "development"),
		HTTPAddr:        p.str("HTTP_ADDR", ":8080"),
		DatabaseURL:     p.require("DATABASE_URL"),
		RedisURL:        p.str("REDIS_URL", ""),
		JWTSecret:       p.require("JWT_SECRET"),
		JWTTTL:          p.duration("JWT_TTL", 24*time.Hour),
		RefreshTTL:      p.duration("REFRESH_TTL", 30*24*time.Hour),
		CORSOrigins:     p.csv("CORS_ORIGINS", []string{"http://localhost:5173"}),
		ShutdownTimeout: p.duration("SHUTDOWN_TIMEOUT", 10*time.Second),
		RateLimitRPS:    p.floatMin("RATE_LIMIT_RPS", 20, 0),
		RateLimitBurst:  p.intMin("RATE_LIMIT_BURST", 40, 1),
		LoginRateRPS:    p.floatMin("LOGIN_RATE_RPS", 0.5, 0),
		LoginRateBurst:  p.intMin("LOGIN_RATE_BURST", 5, 1),
		TaxRate:         p.floatMin("TAX_RATE", 0, 0),
		AdminEmail:      p.str("ADMIN_EMAIL", ""),
		AdminPassword:   p.str("ADMIN_PASSWORD", ""),
	}

	if err := p.err(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// IsProduction reports whether the API is running in production mode.
func (c Config) IsProduction() bool { return c.Env == "production" }

// parser reads env vars and accumulates validation errors so Load can report
// them all at once.
type parser struct{ errs []error }

func (p *parser) err() error {
	if len(p.errs) == 0 {
		return nil
	}
	return fmt.Errorf("config: %w", errors.Join(p.errs...))
}

func (p *parser) str(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func (p *parser) require(key string) string {
	v := os.Getenv(key)
	if v == "" {
		p.errs = append(p.errs, fmt.Errorf("%s is required", key))
	}
	return v
}

func (p *parser) duration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	if d, err := time.ParseDuration(v); err == nil {
		if d <= 0 {
			p.errs = append(p.errs, fmt.Errorf("%s must be positive, got %q", key, v))
			return fallback
		}
		return d
	}
	// Allow a bare integer to mean seconds.
	if n, err := strconv.Atoi(v); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	p.errs = append(p.errs, fmt.Errorf("%s is not a valid duration: %q", key, v))
	return fallback
}

func (p *parser) floatMin(key string, fallback, min float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		p.errs = append(p.errs, fmt.Errorf("%s is not a valid number: %q", key, v))
		return fallback
	}
	if f <= min {
		p.errs = append(p.errs, fmt.Errorf("%s must be greater than %g, got %g", key, min, f))
		return fallback
	}
	return f
}

func (p *parser) intMin(key string, fallback, min int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		p.errs = append(p.errs, fmt.Errorf("%s is not a valid integer: %q", key, v))
		return fallback
	}
	if n < min {
		p.errs = append(p.errs, fmt.Errorf("%s must be at least %d, got %d", key, min, n))
		return fallback
	}
	return n
}

func (p *parser) csv(key string, fallback []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		p.errs = append(p.errs, fmt.Errorf("%s contained no valid entries: %q", key, v))
		return fallback
	}
	return out
}
