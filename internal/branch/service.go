package branch

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ebnsina/saydalah-api/internal/httpx"
	"github.com/ebnsina/saydalah-api/internal/store"
)

// Service holds branch business logic. It depends on the Repository interface,
// never on HTTP or the concrete database.
type Service struct {
	repo Repository
}

// NewService constructs a branch Service.
func NewService(repo Repository) *Service { return &Service{repo: repo} }

// ListResult is a page of branches plus the total count for pagination.
type ListResult struct {
	Items []store.Branch
	Total int64
}

// Create adds a new branch.
func (s *Service) Create(ctx context.Context, in CreateRequest) (store.Branch, error) {
	return s.repo.Create(ctx, store.CreateBranchParams{
		Name:    in.Name,
		Address: in.Address,
		Phone:   in.Phone,
	})
}

// Get returns a branch by ID, or httpx.ErrNotFound if it does not exist.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (store.Branch, error) {
	b, err := s.repo.Get(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Branch{}, httpx.ErrNotFound
	}
	if err != nil {
		return store.Branch{}, fmt.Errorf("branch: get: %w", err)
	}
	return b, nil
}

// List returns a page of branches with the total count.
func (s *Service) List(ctx context.Context, p httpx.Pagination) (ListResult, error) {
	items, err := s.repo.List(ctx, store.ListBranchesParams{Limit: p.Limit, Offset: p.Offset})
	if err != nil {
		return ListResult{}, fmt.Errorf("branch: list: %w", err)
	}
	total, err := s.repo.Count(ctx)
	if err != nil {
		return ListResult{}, fmt.Errorf("branch: count: %w", err)
	}
	return ListResult{Items: items, Total: total}, nil
}

// Update replaces a branch's mutable fields.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateRequest) (store.Branch, error) {
	b, err := s.repo.Update(ctx, store.UpdateBranchParams{
		ID:      id,
		Name:    in.Name,
		Address: in.Address,
		Phone:   in.Phone,
		Active:  in.Active,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Branch{}, httpx.ErrNotFound
	}
	if err != nil {
		return store.Branch{}, fmt.Errorf("branch: update: %w", err)
	}
	return b, nil
}
