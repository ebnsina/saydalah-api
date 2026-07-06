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

// GetByBarcode returns the product with the given barcode (POS scan lookup), or
// httpx.ErrNotFound if none matches.
func (s *Service) GetByBarcode(ctx context.Context, code string) (store.Product, error) {
	p, err := s.repo.GetByBarcode(ctx, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Product{}, httpx.ErrNotFound
	}
	if err != nil {
		return store.Product{}, fmt.Errorf("catalog: get by barcode: %w", err)
	}
	return p, nil
}

// Filter holds the optional product-list filters. A nil field means "no filter"
// on that dimension.
type Filter struct {
	Search   *string
	Category *string
	Active   *bool
}

// List returns a page of products matching the filter (search over name/generic/
// barcode, plus optional category and active-status filters).
func (s *Service) List(ctx context.Context, f Filter, p httpx.Pagination) (ListResult, error) {
	search := cleanBarcode(f.Search) // reuse: trims and nils empty
	items, err := s.repo.List(ctx, store.ListProductsParams{
		Search:   search,
		Category: f.Category,
		Active:   f.Active,
		Limit:    p.Limit,
		Offset:   p.Offset,
	})
	if err != nil {
		return ListResult{}, fmt.Errorf("catalog: list: %w", err)
	}
	total, err := s.repo.Count(ctx, store.CountProductsParams{
		Search:   search,
		Category: f.Category,
		Active:   f.Active,
	})
	if err != nil {
		return ListResult{}, fmt.Errorf("catalog: count: %w", err)
	}
	return ListResult{Items: items, Total: total}, nil
}

// Categories returns the distinct non-empty product categories, for filter UIs.
func (s *Service) Categories(ctx context.Context) ([]string, error) {
	cats, err := s.repo.Categories(ctx)
	if err != nil {
		return nil, fmt.Errorf("catalog: categories: %w", err)
	}
	return cats, nil
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
