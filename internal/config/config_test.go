package config

import (
	"testing"
	"time"
)

// setEnv sets env vars for a test and clears them afterward.
func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	for k, v := range kv {
		t.Setenv(k, v)
	}
}

func baseEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL": "postgres://x",
		"JWT_SECRET":   "secret",
	}
}

func TestLoadDefaults(t *testing.T) {
	setEnv(t, baseEnv())
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.RateLimitRPS != 20 || cfg.RateLimitBurst != 40 {
		t.Errorf("unexpected rate defaults: %+v", cfg)
	}
	if cfg.JWTTTL != 24*time.Hour {
		t.Errorf("JWTTTL default = %s", cfg.JWTTTL)
	}
}

func TestLoadRequiresSecrets(t *testing.T) {
	// Neither DATABASE_URL nor JWT_SECRET set.
	if _, err := Load(); err == nil {
		t.Fatal("expected error when required vars are missing")
	}
}

func TestLoadFailsOnMalformedNumber(t *testing.T) {
	env := baseEnv()
	env["RATE_LIMIT_RPS"] = "not-a-number"
	setEnv(t, env)
	if _, err := Load(); err == nil {
		t.Fatal("expected error for malformed RATE_LIMIT_RPS, got nil (silent fallback)")
	}
}

func TestLoadFailsOnOutOfRange(t *testing.T) {
	env := baseEnv()
	env["RATE_LIMIT_BURST"] = "0" // must be >= 1
	setEnv(t, env)
	if _, err := Load(); err == nil {
		t.Fatal("expected error for RATE_LIMIT_BURST below minimum")
	}
}

func TestLoadFailsOnMalformedDuration(t *testing.T) {
	env := baseEnv()
	env["JWT_TTL"] = "banana"
	setEnv(t, env)
	if _, err := Load(); err == nil {
		t.Fatal("expected error for malformed JWT_TTL")
	}
}

func TestLoadAcceptsValidOverrides(t *testing.T) {
	env := baseEnv()
	env["RATE_LIMIT_RPS"] = "5"
	env["JWT_TTL"] = "30m"
	setEnv(t, env)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.RateLimitRPS != 5 || cfg.JWTTTL != 30*time.Minute {
		t.Errorf("overrides not applied: %+v", cfg)
	}
}
