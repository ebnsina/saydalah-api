package supplier

import (
	"context"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// Repository is the persistence surface the supplier service depends on.
type Repository interface {
	Create(ctx context.Context, arg store.CreateSupplierParams) (store.Supplier, error)
	Get(ctx context.Context, id uuid.UUID) (store.Supplier, error)
	List(ctx context.Context, arg store.ListSuppliersParams) ([]store.Supplier, error)
	Count(ctx context.Context) (int64, error)
	Update(ctx context.Context, arg store.UpdateSupplierParams) (store.Supplier, error)
}

type storeRepository struct{ q *store.Store }

// NewRepository returns a Repository backed by the given store.
func NewRepository(s *store.Store) Repository { return &storeRepository{q: s} }

func (r *storeRepository) Create(ctx context.Context, arg store.CreateSupplierParams) (store.Supplier, error) {
	return r.q.CreateSupplier(ctx, arg)
}

func (r *storeRepository) Get(ctx context.Context, id uuid.UUID) (store.Supplier, error) {
	return r.q.GetSupplier(ctx, id)
}

func (r *storeRepository) List(ctx context.Context, arg store.ListSuppliersParams) ([]store.Supplier, error) {
	return r.q.ListSuppliers(ctx, arg)
}

func (r *storeRepository) Count(ctx context.Context) (int64, error) {
	return r.q.CountSuppliers(ctx)
}

func (r *storeRepository) Update(ctx context.Context, arg store.UpdateSupplierParams) (store.Supplier, error) {
	return r.q.UpdateSupplier(ctx, arg)
}
