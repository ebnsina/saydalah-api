package branch

import (
	"context"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// Repository is the persistence surface the branch service depends on. It is an
// interface (not the concrete store) so the service can be unit-tested with a
// fake and so the module only exposes the queries it actually needs.
type Repository interface {
	Create(ctx context.Context, arg store.CreateBranchParams) (store.Branch, error)
	Get(ctx context.Context, id uuid.UUID) (store.Branch, error)
	List(ctx context.Context, arg store.ListBranchesParams) ([]store.Branch, error)
	Count(ctx context.Context) (int64, error)
	Update(ctx context.Context, arg store.UpdateBranchParams) (store.Branch, error)
}

// storeRepository adapts the generated *store.Store to Repository.
type storeRepository struct{ q *store.Store }

// NewRepository returns a Repository backed by the given store.
func NewRepository(s *store.Store) Repository { return &storeRepository{q: s} }

func (r *storeRepository) Create(ctx context.Context, arg store.CreateBranchParams) (store.Branch, error) {
	return r.q.CreateBranch(ctx, arg)
}

func (r *storeRepository) Get(ctx context.Context, id uuid.UUID) (store.Branch, error) {
	return r.q.GetBranch(ctx, id)
}

func (r *storeRepository) List(ctx context.Context, arg store.ListBranchesParams) ([]store.Branch, error) {
	return r.q.ListBranches(ctx, arg)
}

func (r *storeRepository) Count(ctx context.Context) (int64, error) {
	return r.q.CountBranches(ctx)
}

func (r *storeRepository) Update(ctx context.Context, arg store.UpdateBranchParams) (store.Branch, error) {
	return r.q.UpdateBranch(ctx, arg)
}
