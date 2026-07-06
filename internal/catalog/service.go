package catalog

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

// Service holds product catalog business logic.
type Service struct {
	repo Repository
}

// NewService constructs a catalog Service.
func NewService(repo Repository) *Service { return &Service{repo: repo} }

// ListResult is a page of products plus the total count for that filter.
type ListResult struct {
	Items []store.Product
	Total int64
}

// Create adds a product. A duplicate barcode surfaces as httpx.ErrConflict.
func (s *Service) Create(ctx context.Context, in CreateRequest) (store.Product, error) {
	p, err := s.repo.Create(ctx, store.CreateProductParams{
		Name:         in.Name,
		GenericName:  in.GenericName,
		Form:         in.Form,
		Strength:     in.Strength,
		Barcode:      cleanBarcode(in.Barcode),
		Category:     in.Category,
		Unit:         orDefault(in.Unit, "unit"),
		ReorderLevel: in.ReorderLevel,
	})
	if store.IsUniqueViolation(err) {
		return store.Product{}, fmt.Errorf("barcode already in use: %w", httpx.ErrConflict)
	}
	if err != nil {
		return store.Product{}, fmt.Errorf("catalog: create: %w", err)
	}
	return p, nil
}

// Get returns a product by ID or httpx.ErrNotFound.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (store.Product, error) {
	p, err := s.repo.Get(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Product{}, httpx.ErrNotFound
	}
	if err != nil {
		return store.Product{}, fmt.Errorf("catalog: get: %w", err)
	}
	return p, nil
}

// List returns a page of products, optionally filtered by a search term
// matching name, generic name, or barcode.
func (s *Service) List(ctx context.Context, search *string, p httpx.Pagination) (ListResult, error) {
	search = cleanBarcode(search) // reuse: trims and nils empty
	items, err := s.repo.List(ctx, store.ListProductsParams{
		Search: search,
		Limit:  p.Limit,
		Offset: p.Offset,
	})
	if err != nil {
		return ListResult{}, fmt.Errorf("catalog: list: %w", err)
	}
	total, err := s.repo.Count(ctx, search)
	if err != nil {
		return ListResult{}, fmt.Errorf("catalog: count: %w", err)
	}
	return ListResult{Items: items, Total: total}, nil
}

// Update replaces a product's mutable fields.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateRequest) (store.Product, error) {
	p, err := s.repo.Update(ctx, store.UpdateProductParams{
		ID:           id,
		Name:         in.Name,
		GenericName:  in.GenericName,
		Form:         in.Form,
		Strength:     in.Strength,
		Barcode:      cleanBarcode(in.Barcode),
		Category:     in.Category,
		Unit:         orDefault(in.Unit, "unit"),
		ReorderLevel: in.ReorderLevel,
		Active:       in.Active,
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return store.Product{}, httpx.ErrNotFound
	case store.IsUniqueViolation(err):
		return store.Product{}, fmt.Errorf("barcode already in use: %w", httpx.ErrConflict)
	case err != nil:
		return store.Product{}, fmt.Errorf("catalog: update: %w", err)
	}
	return p, nil
}

// cleanBarcode trims whitespace and treats an empty value as NULL, so blank
// barcodes do not collide on the unique index.
func cleanBarcode(b *string) *string {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*b)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func orDefault(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
