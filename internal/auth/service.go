package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ebnsina/saydalah-api/internal/httpx"
)

// Service performs authentication: verifying credentials and issuing tokens.
type Service struct {
	repo Repository
	tm   *TokenManager
	now  func() time.Time
}

// NewService constructs an auth Service.
func NewService(repo Repository, tm *TokenManager) *Service {
	return &Service{repo: repo, tm: tm, now: time.Now}
}

// Login verifies email/password and returns an access token. To avoid leaking
// which emails exist, a missing user and a bad password both yield the same
// httpx.ErrUnauthorized. Deactivated accounts are also rejected.
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

	id := Identity{UserID: u.ID, Role: u.Role, BranchID: u.BranchID}
	token, expiresAt, err := s.tm.Issue(id, s.now())
	if err != nil {
		return LoginResponse{}, err
	}
	return LoginResponse{AccessToken: token, ExpiresAt: expiresAt, User: userInfo(u)}, nil
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
