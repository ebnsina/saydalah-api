//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/ebnsina/saydalah-api/internal/auth"
	"github.com/ebnsina/saydalah-api/internal/httpx"
)

func TestRefreshTokenRotationAndReuse(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	// Login yields an access + refresh token pair.
	login, err := e.auth.Login(ctx, auth.LoginRequest{Email: e.adminEmail, Password: e.adminPass})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if login.AccessToken == "" || login.RefreshToken == "" {
		t.Fatalf("login should return both tokens: %+v", login)
	}

	// Refreshing rotates: a new pair is issued.
	rotated, err := e.auth.Refresh(ctx, auth.RefreshRequest{RefreshToken: login.RefreshToken})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if rotated.RefreshToken == login.RefreshToken {
		t.Error("refresh token should be rotated (new value)")
	}

	// Reusing the now-rotated original token is rejected...
	if _, err := e.auth.Refresh(ctx, auth.RefreshRequest{RefreshToken: login.RefreshToken}); !errors.Is(err, httpx.ErrUnauthorized) {
		t.Errorf("reused refresh token should be unauthorized, got %v", err)
	}
	// ...and reuse-detection revokes the whole family, so the rotated token dies too.
	if _, err := e.auth.Refresh(ctx, auth.RefreshRequest{RefreshToken: rotated.RefreshToken}); !errors.Is(err, httpx.ErrUnauthorized) {
		t.Errorf("rotated token should be revoked after reuse detection, got %v", err)
	}
}

func TestLogoutRevokesRefreshToken(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	login, err := e.auth.Login(ctx, auth.LoginRequest{Email: e.adminEmail, Password: e.adminPass})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if err := e.auth.Logout(ctx, auth.RefreshRequest{RefreshToken: login.RefreshToken}); err != nil {
		t.Fatalf("logout: %v", err)
	}
	// The token no longer works.
	if _, err := e.auth.Refresh(ctx, auth.RefreshRequest{RefreshToken: login.RefreshToken}); !errors.Is(err, httpx.ErrUnauthorized) {
		t.Errorf("refresh after logout should be unauthorized, got %v", err)
	}
	// Logout is idempotent.
	if err := e.auth.Logout(ctx, auth.RefreshRequest{RefreshToken: login.RefreshToken}); err != nil {
		t.Errorf("second logout should be a no-op, got %v", err)
	}
}

func TestLoginRejectsBadCredentials(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if _, err := e.auth.Login(ctx, auth.LoginRequest{Email: e.adminEmail, Password: "wrong"}); !errors.Is(err, httpx.ErrUnauthorized) {
		t.Errorf("bad password should be unauthorized, got %v", err)
	}
}
