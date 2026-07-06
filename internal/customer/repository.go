package customer

import (
	"context"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// Repository is the persistence surface the customer service depends on.
type Repository interface {
	Create(ctx context.Context, arg store.CreateCustomerParams) (store.Customer, error)
	Get(ctx context.Context, id uuid.UUID) (store.Customer, error)
	List(ctx context.Context, arg store.ListCustomersParams) ([]store.Customer, error)
	Count(ctx context.Context, search *string) (int64, error)
	Update(ctx context.Context, arg store.UpdateCustomerParams) (store.Customer, error)
}

type storeRepository struct{ q *store.Store }

// NewRepository returns a Repository backed by the given store.
func NewRepository(s *store.Store) Repository { return &storeRepository{q: s} }

func (r *storeRepository) Create(ctx context.Context, arg store.CreateCustomerParams) (store.Customer, error) {
	return r.q.CreateCustomer(ctx, arg)
}

func (r *storeRepository) Get(ctx context.Context, id uuid.UUID) (store.Customer, error) {
	return r.q.GetCustomer(ctx, id)
}

func (r *storeRepository) List(ctx context.Context, arg store.ListCustomersParams) ([]store.Customer, error) {
	return r.q.ListCustomers(ctx, arg)
}

func (r *storeRepository) Count(ctx context.Context, search *string) (int64, error) {
	return r.q.CountCustomers(ctx, search)
}

func (r *storeRepository) Update(ctx context.Context, arg store.UpdateCustomerParams) (store.Customer, error) {
	return r.q.UpdateCustomer(ctx, arg)
}
