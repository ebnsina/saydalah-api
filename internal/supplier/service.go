package supplier

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ebnsina/saydalah-api/internal/httpx"
	"github.com/ebnsina/saydalah-api/internal/store"
)

// Service holds supplier business logic.
type Service struct {
	repo Repository
}

// NewService constructs a supplier Service.
func NewService(repo Repository) *Service { return &Service{repo: repo} }

// ListResult is a page of suppliers plus the total count.
type ListResult struct {
	Items []store.Supplier
	Total int64
}

// Create adds a supplier.
func (s *Service) Create(ctx context.Context, in CreateRequest) (store.Supplier, error) {
	sup, err := s.repo.Create(ctx, store.CreateSupplierParams{
		Name:    in.Name,
		Contact: in.Contact,
		Phone:   in.Phone,
		Email:   in.Email,
	})
	if err != nil {
		return store.Supplier{}, fmt.Errorf("supplier: create: %w", err)
	}
	return sup, nil
}

// Get returns a supplier by ID or httpx.ErrNotFound.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (store.Supplier, error) {
	sup, err := s.repo.Get(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Supplier{}, httpx.ErrNotFound
	}
	if err != nil {
		return store.Supplier{}, fmt.Errorf("supplier: get: %w", err)
	}
	return sup, nil
}

// List returns a page of suppliers with the total count.
func (s *Service) List(ctx context.Context, p httpx.Pagination) (ListResult, error) {
	items, err := s.repo.List(ctx, store.ListSuppliersParams{Limit: p.Limit, Offset: p.Offset})
	if err != nil {
		return ListResult{}, fmt.Errorf("supplier: list: %w", err)
	}
	total, err := s.repo.Count(ctx)
	if err != nil {
		return ListResult{}, fmt.Errorf("supplier: count: %w", err)
	}
	return ListResult{Items: items, Total: total}, nil
}

// Update replaces a supplier's mutable fields.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateRequest) (store.Supplier, error) {
	sup, err := s.repo.Update(ctx, store.UpdateSupplierParams{
		ID:      id,
		Name:    in.Name,
		Contact: in.Contact,
		Phone:   in.Phone,
		Email:   in.Email,
		Active:  in.Active,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Supplier{}, httpx.ErrNotFound
	}
	if err != nil {
		return store.Supplier{}, fmt.Errorf("supplier: update: %w", err)
	}
	return sup, nil
}
