package customer

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ebnsina/saydalah-api/internal/httpx"
	"github.com/ebnsina/saydalah-api/internal/store"
)

// Service holds customer business logic.
type Service struct {
	repo Repository
}

// NewService constructs a customer Service.
func NewService(repo Repository) *Service { return &Service{repo: repo} }

// ListResult is a page of customers plus the total count for that filter.
type ListResult struct {
	Items []store.Customer
	Total int64
}

// Create adds a customer.
func (s *Service) Create(ctx context.Context, in CreateRequest) (store.Customer, error) {
	c, err := s.repo.Create(ctx, store.CreateCustomerParams{
		Name:    in.Name,
		Phone:   in.Phone,
		Address: in.Address,
	})
	if err != nil {
		return store.Customer{}, fmt.Errorf("customer: create: %w", err)
	}
	return c, nil
}

// Get returns a customer by ID or httpx.ErrNotFound.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (store.Customer, error) {
	c, err := s.repo.Get(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Customer{}, httpx.ErrNotFound
	}
	if err != nil {
		return store.Customer{}, fmt.Errorf("customer: get: %w", err)
	}
	return c, nil
}

// List returns a page of customers, optionally filtered by a name/phone search.
func (s *Service) List(ctx context.Context, search *string, p httpx.Pagination) (ListResult, error) {
	search = clean(search)
	items, err := s.repo.List(ctx, store.ListCustomersParams{
		Search: search,
		Limit:  p.Limit,
		Offset: p.Offset,
	})
	if err != nil {
		return ListResult{}, fmt.Errorf("customer: list: %w", err)
	}
	total, err := s.repo.Count(ctx, search)
	if err != nil {
		return ListResult{}, fmt.Errorf("customer: count: %w", err)
	}
	return ListResult{Items: items, Total: total}, nil
}

// Update replaces a customer's mutable fields.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateRequest) (store.Customer, error) {
	c, err := s.repo.Update(ctx, store.UpdateCustomerParams{
		ID:      id,
		Name:    in.Name,
		Phone:   in.Phone,
		Address: in.Address,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Customer{}, httpx.ErrNotFound
	}
	if err != nil {
		return store.Customer{}, fmt.Errorf("customer: update: %w", err)
	}
	return c, nil
}

func clean(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}
