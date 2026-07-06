package inventory

import (
	"context"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// Repository is the read-only persistence surface for inventory views.
type Repository interface {
	ListBatches(ctx context.Context, arg store.ListBranchBatchesParams) ([]store.ListBranchBatchesRow, error)
	CountBatches(ctx context.Context, branchID uuid.UUID) (int64, error)
	NearExpiry(ctx context.Context, arg store.ListNearExpiryBatchesParams) ([]store.ListNearExpiryBatchesRow, error)
	LowStock(ctx context.Context, branchID uuid.UUID) ([]store.ListLowStockRow, error)
	OnHand(ctx context.Context, arg store.StockOnHandParams) (int64, error)
}

type storeRepository struct{ q *store.Store }

// NewRepository returns a Repository backed by the given store.
func NewRepository(s *store.Store) Repository { return &storeRepository{q: s} }

func (r *storeRepository) ListBatches(ctx context.Context, arg store.ListBranchBatchesParams) ([]store.ListBranchBatchesRow, error) {
	return r.q.ListBranchBatches(ctx, arg)
}

func (r *storeRepository) CountBatches(ctx context.Context, branchID uuid.UUID) (int64, error) {
	return r.q.CountBranchBatches(ctx, branchID)
}

func (r *storeRepository) NearExpiry(ctx context.Context, arg store.ListNearExpiryBatchesParams) ([]store.ListNearExpiryBatchesRow, error) {
	return r.q.ListNearExpiryBatches(ctx, arg)
}

func (r *storeRepository) LowStock(ctx context.Context, branchID uuid.UUID) ([]store.ListLowStockRow, error) {
	return r.q.ListLowStock(ctx, branchID)
}

func (r *storeRepository) OnHand(ctx context.Context, arg store.StockOnHandParams) (int64, error) {
	return r.q.StockOnHand(ctx, arg)
}
