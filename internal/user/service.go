package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ebnsina/saydalah-api/internal/auth"
	"github.com/ebnsina/saydalah-api/internal/httpx"
	"github.com/ebnsina/saydalah-api/internal/store"
)

// Service holds user business logic: password hashing and the invariant that
// branch-scoped roles are assigned to a branch while admins are chain-wide.
type Service struct {
	repo Repository
}

// NewService constructs a user Service.
func NewService(repo Repository) *Service { return &Service{repo: repo} }

// ListResult is a page of users plus the total count.
type ListResult struct {
	Items []store.User
	Total int64
}

// Create hashes the password, validates the role/branch pairing, and inserts
// the user. A duplicate email surfaces as httpx.ErrConflict.
func (s *Service) Create(ctx context.Context, in CreateRequest) (store.User, error) {
	branchID, err := normalizeBranch(in.Role, in.BranchID)
	if err != nil {
		return store.User{}, err
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return store.User{}, fmt.Errorf("user: hash password: %w", err)
	}

	u, err := s.repo.Create(ctx, store.CreateUserParams{
		Email:        in.Email,
		PasswordHash: hash,
		FullName:     in.FullName,
		Role:         in.Role,
		BranchID:     branchID,
	})
	switch {
	case store.IsUniqueViolation(err):
		return store.User{}, fmt.Errorf("email already registered: %w", httpx.ErrConflict)
	case store.IsForeignKeyViolation(err):
		return store.User{}, fmt.Errorf("branch does not exist: %w", httpx.ErrInvalidInput)
	case err != nil:
		return store.User{}, fmt.Errorf("user: create: %w", err)
	}
	return u, nil
}

// Get returns a user by ID or httpx.ErrNotFound.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (store.User, error) {
	u, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.User{}, httpx.ErrNotFound
	}
	if err != nil {
		return store.User{}, fmt.Errorf("user: get: %w", err)
	}
	return u, nil
}

// List returns a page of users with the total count.
func (s *Service) List(ctx context.Context, p httpx.Pagination) (ListResult, error) {
	items, err := s.repo.List(ctx, store.ListUsersParams{Limit: p.Limit, Offset: p.Offset})
	if err != nil {
		return ListResult{}, fmt.Errorf("user: list: %w", err)
	}
	total, err := s.repo.Count(ctx)
	if err != nil {
		return ListResult{}, fmt.Errorf("user: count: %w", err)
	}
	return ListResult{Items: items, Total: total}, nil
}

// Update replaces a user's role, branch, name, and active flag.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateRequest) (store.User, error) {
	branchID, err := normalizeBranch(in.Role, in.BranchID)
	if err != nil {
		return store.User{}, err
	}
	u, err := s.repo.Update(ctx, store.UpdateUserParams{
		ID:       id,
		FullName: in.FullName,
		Role:     in.Role,
		BranchID: branchID,
		Active:   in.Active,
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return store.User{}, httpx.ErrNotFound
	case store.IsForeignKeyViolation(err):
		return store.User{}, fmt.Errorf("branch does not exist: %w", httpx.ErrInvalidInput)
	case err != nil:
		return store.User{}, fmt.Errorf("user: update: %w", err)
	}
	return u, nil
}

// ResetPassword sets a new password hash for a user (admin-initiated, no
// current-password check).
func (s *Service) ResetPassword(ctx context.Context, id uuid.UUID, newPassword string) error {
	if _, err := s.repo.GetByID(ctx, id); errors.Is(err, pgx.ErrNoRows) {
		return httpx.ErrNotFound
	} else if err != nil {
		return fmt.Errorf("user: lookup: %w", err)
	}
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("user: hash password: %w", err)
	}
	if err := s.repo.SetPassword(ctx, id, hash); err != nil {
		return fmt.Errorf("user: set password: %w", err)
	}
	return nil
}

// normalizeBranch enforces the role/branch invariant: admins are chain-wide
// (branch cleared), everyone else must be assigned to a branch.
func normalizeBranch(role store.UserRole, branchID *uuid.UUID) (*uuid.UUID, error) {
	if role == store.UserRoleAdmin {
		return nil, nil
	}
	if branchID == nil {
		return nil, fmt.Errorf("branch_id is required for non-admin roles: %w", httpx.ErrInvalidInput)
	}
	return branchID, nil
}
