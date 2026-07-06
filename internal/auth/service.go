package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ebnsina/saydalah-api/internal/httpx"
	"github.com/ebnsina/saydalah-api/internal/store"
)

// Service performs authentication: verifying credentials, issuing short-lived
// access tokens paired with rotating server-side refresh tokens, and revoking
// them on logout.
type Service struct {
	repo       Repository
	tm         *TokenManager
	refreshTTL time.Duration
	now        func() time.Time
}

// NewService constructs an auth Service. refreshTTL is the lifetime of issued
// refresh tokens.
func NewService(repo Repository, tm *TokenManager, refreshTTL time.Duration) *Service {
	return &Service{repo: repo, tm: tm, refreshTTL: refreshTTL, now: time.Now}
}

// Login verifies email/password and returns an access + refresh token pair. A
// missing user and a bad password both yield the same httpx.ErrUnauthorized to
// avoid leaking which emails exist; deactivated accounts are also rejected.
func (s *Service) Login(ctx context.Context, in LoginRequest) (LoginResponse, error) {
	u, err := s.repo.GetByEmail(ctx, in.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return LoginResponse{}, httpx.ErrUnauthorized
	}
	if err != nil {
		return LoginResponse{}, fmt.Errorf("auth: lookup user: %w", err)
	}
	if !u.Active || !CheckPassword(u.PasswordHash, in.Password) {
		return LoginResponse{}, httpx.ErrUnauthorized
	}

	var resp LoginResponse
	err = s.repo.Tx(ctx, func(tx Repository) error {
		resp, err = s.issuePair(ctx, tx, u)
		return err
	})
	if err != nil {
		return LoginResponse{}, err
	}
	return resp, nil
}

// Refresh validates a refresh token, rotates it (revoking the presented token
// and issuing a fresh pair), and returns the new tokens. A revoked-but-presented
// token signals theft/replay: all of the user's tokens are revoked and the
// request is rejected.
func (s *Service) Refresh(ctx context.Context, in RefreshRequest) (LoginResponse, error) {
	rec, err := s.repo.GetRefreshToken(ctx, HashToken(in.RefreshToken))
	if errors.Is(err, pgx.ErrNoRows) {
		return LoginResponse{}, httpx.ErrUnauthorized
	}
	if err != nil {
		return LoginResponse{}, fmt.Errorf("auth: lookup refresh token: %w", err)
	}

	if rec.RevokedAt != nil {
		// Reuse of an already-rotated token — revoke the whole family.
		_ = s.repo.RevokeUserRefreshTokens(ctx, rec.UserID)
		return LoginResponse{}, httpx.ErrUnauthorized
	}
	if s.now().After(rec.ExpiresAt) {
		return LoginResponse{}, httpx.ErrUnauthorized
	}

	u, err := s.repo.GetByID(ctx, rec.UserID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !u.Active) {
		return LoginResponse{}, httpx.ErrUnauthorized
	}
	if err != nil {
		return LoginResponse{}, fmt.Errorf("auth: lookup user: %w", err)
	}

	var resp LoginResponse
	err = s.repo.Tx(ctx, func(tx Repository) error {
		if err := tx.RevokeRefreshToken(ctx, rec.ID); err != nil {
			return err
		}
		resp, err = s.issuePair(ctx, tx, u)
		return err
	})
	if err != nil {
		return LoginResponse{}, err
	}
	return resp, nil
}

// Logout revokes the presented refresh token. It is idempotent — an unknown or
// already-revoked token is not an error, so clients can always "log out".
func (s *Service) Logout(ctx context.Context, in RefreshRequest) error {
	rec, err := s.repo.GetRefreshToken(ctx, HashToken(in.RefreshToken))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("auth: lookup refresh token: %w", err)
	}
	if rec.RevokedAt != nil {
		return nil
	}
	if err := s.repo.RevokeRefreshToken(ctx, rec.ID); err != nil {
		return fmt.Errorf("auth: revoke refresh token: %w", err)
	}
	return nil
}

// issuePair mints an access token and a new refresh token (persisting only the
// refresh token's hash) for the user, using the given transaction.
func (s *Service) issuePair(ctx context.Context, tx Repository, u store.User) (LoginResponse, error) {
	now := s.now()
	id := Identity{UserID: u.ID, Role: u.Role, BranchID: u.BranchID}
	access, expiresAt, err := s.tm.Issue(id, now)
	if err != nil {
		return LoginResponse{}, err
	}

	refresh, err := GenerateRefreshToken()
	if err != nil {
		return LoginResponse{}, err
	}
	if _, err := tx.CreateRefreshToken(ctx, u.ID, HashToken(refresh), now.Add(s.refreshTTL)); err != nil {
		return LoginResponse{}, fmt.Errorf("auth: store refresh token: %w", err)
	}

	return LoginResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresAt:    expiresAt,
		User:         userInfo(u),
	}, nil
}

// Me returns the profile of the user identified by id.
func (s *Service) Me(ctx context.Context, id uuid.UUID) (UserInfo, error) {
	u, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return UserInfo{}, httpx.ErrUnauthorized
	}
	if err != nil {
		return UserInfo{}, fmt.Errorf("auth: me: %w", err)
	}
	return userInfo(u), nil
}
