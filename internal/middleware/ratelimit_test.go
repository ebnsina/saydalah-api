package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimitAllowsBurstThenBlocks(t *testing.T) {
	// 0 rps, burst 2: two requests allowed, the third is limited.
	h := RateLimit(0.0001, 2)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	do := func() int {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "1.2.3.4:5555"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	if got := do(); got != http.StatusOK {
		t.Fatalf("request 1: got %d, want 200", got)
	}
	if got := do(); got != http.StatusOK {
		t.Fatalf("request 2: got %d, want 200", got)
	}
	if got := do(); got != http.StatusTooManyRequests {
		t.Fatalf("request 3: got %d, want 429", got)
	}
}

func TestRateLimitIsPerClient(t *testing.T) {
	h := RateLimit(0.0001, 1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	call := func(ip string) int {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = ip + ":1000"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	// Each distinct client gets its own bucket.
	if call("10.0.0.1") != http.StatusOK || call("10.0.0.2") != http.StatusOK {
		t.Fatal("first request per client should be allowed")
	}
	// Second hit from the same client is limited.
	if call("10.0.0.1") != http.StatusTooManyRequests {
		t.Error("second request from same client should be limited")
	}
}

func TestClientIPHonorsForwardedFor(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:9999"
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
	if got := clientIP(req); got != "203.0.113.7" {
		t.Errorf("clientIP = %q, want 203.0.113.7", got)
	}
}
