package catalog

import (
	"context"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// Repository is the persistence surface the catalog service depends on.
type Repository interface {
	Create(ctx context.Context, arg store.CreateProductParams) (store.Product, error)
	Get(ctx context.Context, id uuid.UUID) (store.Product, error)
	GetByBarcode(ctx context.Context, code string) (store.Product, error)
	List(ctx context.Context, arg store.ListProductsParams) ([]store.Product, error)
	Count(ctx context.Context, search *string) (int64, error)
	Update(ctx context.Context, arg store.UpdateProductParams) (store.Product, error)
}

type storeRepository struct{ q *store.Store }

// NewRepository returns a Repository backed by the given store.
func NewRepository(s *store.Store) Repository { return &storeRepository{q: s} }

func (r *storeRepository) Create(ctx context.Context, arg store.CreateProductParams) (store.Product, error) {
	return r.q.CreateProduct(ctx, arg)
}

func (r *storeRepository) Get(ctx context.Context, id uuid.UUID) (store.Product, error) {
	return r.q.GetProduct(ctx, id)
}

func (r *storeRepository) GetByBarcode(ctx context.Context, code string) (store.Product, error) {
	return r.q.GetProductByBarcode(ctx, &code)
}

func (r *storeRepository) List(ctx context.Context, arg store.ListProductsParams) ([]store.Product, error) {
	return r.q.ListProducts(ctx, arg)
}

func (r *storeRepository) Count(ctx context.Context, search *string) (int64, error) {
	return r.q.CountProducts(ctx, search)
}

func (r *storeRepository) Update(ctx context.Context, arg store.UpdateProductParams) (store.Product, error) {
	return r.q.UpdateProduct(ctx, arg)
}
