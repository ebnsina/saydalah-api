package stock

import (
	"context"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// Repository is the persistence surface for manual stock operations, with Tx to
// pair a batch write with its movement-ledger row atomically.
type Repository interface {
	GetBatch(ctx context.Context, id uuid.UUID) (store.StockBatch, error)
	AdjustBatch(ctx context.Context, arg store.AdjustBatchQuantityParams) (store.StockBatch, error)
	RecordMovement(ctx context.Context, arg store.RecordStockMovementParams) (store.StockMovement, error)
	ListMovements(ctx context.Context, arg store.ListStockMovementsParams) ([]store.ListStockMovementsRow, error)
	CountMovements(ctx context.Context, arg store.CountStockMovementsParams) (int64, error)

	Tx(ctx context.Context, fn func(Repository) error) error
}

type storeRepository struct {
	store *store.Store
	q     store.Querier
}

// NewRepository returns a Repository backed by the given store.
func NewRepository(s *store.Store) Repository {
	return &storeRepository{store: s, q: s.Queries}
}

func (r *storeRepository) Tx(ctx context.Context, fn func(Repository) error) error {
	return r.store.Tx(ctx, func(q *store.Queries) error {
		return fn(&storeRepository{store: r.store, q: q})
	})
}

func (r *storeRepository) GetBatch(ctx context.Context, id uuid.UUID) (store.StockBatch, error) {
	return r.q.GetStockBatch(ctx, id)
}

func (r *storeRepository) AdjustBatch(ctx context.Context, arg store.AdjustBatchQuantityParams) (store.StockBatch, error) {
	return r.q.AdjustBatchQuantity(ctx, arg)
}

func (r *storeRepository) RecordMovement(ctx context.Context, arg store.RecordStockMovementParams) (store.StockMovement, error) {
	return r.q.RecordStockMovement(ctx, arg)
}

func (r *storeRepository) ListMovements(ctx context.Context, arg store.ListStockMovementsParams) ([]store.ListStockMovementsRow, error) {
	return r.q.ListStockMovements(ctx, arg)
}

func (r *storeRepository) CountMovements(ctx context.Context, arg store.CountStockMovementsParams) (int64, error) {
	return r.q.CountStockMovements(ctx, arg)
}
