// Package config loads and validates application configuration from the environment.
package config

import (
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
	JWTSecret       string        // signing key for access tokens
	JWTTTL          time.Duration // access-token lifetime
	CORSOrigins     []string      // allowed browser origins (future SvelteKit app)
	ShutdownTimeout time.Duration // graceful shutdown grace period

	// Optional first-admin bootstrap. When both are set and the users table is
	// empty, an admin account is created at startup so a fresh deployment is
	// immediately usable. Leave unset in environments that seed users elsewhere.
	AdminEmail    string
	AdminPassword string
}

// Load reads configuration from the environment, applying sensible development
// defaults. It returns an error if a required value is missing or malformed so
// the process fails fast at startup rather than at first use.
func Load() (Config, error) {
	cfg := Config{
		Env:             getEnv("APP_ENV", "development"),
		HTTPAddr:        getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		JWTSecret:       os.Getenv("JWT_SECRET"),
		JWTTTL:          getDuration("JWT_TTL", 24*time.Hour),
		CORSOrigins:     getCSV("CORS_ORIGINS", []string{"http://localhost:5173"}),
		ShutdownTimeout: getDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		AdminEmail:      os.Getenv("ADMIN_EMAIL"),
		AdminPassword:   os.Getenv("ADMIN_PASSWORD"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("config: DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("config: JWT_SECRET is required")
	}
	return cfg, nil
}

// IsProduction reports whether the API is running in production mode.
func (c Config) IsProduction() bool { return c.Env == "production" }

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	// Allow a bare integer to mean seconds.
	if n, err := strconv.Atoi(v); err == nil {
		return time.Duration(n) * time.Second
	}
	return fallback
}

func getCSV(key string, fallback []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
