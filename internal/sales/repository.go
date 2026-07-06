package sales

import (
	"context"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// Repository is the persistence surface for sales, including the FEFO batch
// reads/writes used during checkout and Tx to run a sale atomically.
type Repository interface {
	DispensableBatches(ctx context.Context, arg store.ListDispensableBatchesParams) ([]store.StockBatch, error)
	DecrementBatch(ctx context.Context, arg store.DecrementBatchQuantityParams) (store.StockBatch, error)
	AdjustBatch(ctx context.Context, arg store.AdjustBatchQuantityParams) (store.StockBatch, error)
	RecordMovement(ctx context.Context, arg store.RecordStockMovementParams) (store.StockMovement, error)
	SumReturned(ctx context.Context, arg store.SumReturnedForSaleBatchParams) (int64, error)
	MarkVoided(ctx context.Context, arg store.MarkSaleVoidedParams) (store.Sale, error)
	CreateSale(ctx context.Context, arg store.CreateSaleParams) (store.Sale, error)
	AddItem(ctx context.Context, arg store.AddSaleItemParams) (store.SaleItem, error)
	GetSale(ctx context.Context, id uuid.UUID) (store.Sale, error)
	ListItems(ctx context.Context, saleID uuid.UUID) ([]store.SaleItem, error)
	ListSales(ctx context.Context, arg store.ListSalesParams) ([]store.Sale, error)
	CountSales(ctx context.Context, branchID uuid.UUID) (int64, error)

	// Tx runs fn against a transaction-scoped Repository, committing on success.
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

func (r *storeRepository) DispensableBatches(ctx context.Context, arg store.ListDispensableBatchesParams) ([]store.StockBatch, error) {
	return r.q.ListDispensableBatches(ctx, arg)
}

func (r *storeRepository) DecrementBatch(ctx context.Context, arg store.DecrementBatchQuantityParams) (store.StockBatch, error) {
	return r.q.DecrementBatchQuantity(ctx, arg)
}

func (r *storeRepository) AdjustBatch(ctx context.Context, arg store.AdjustBatchQuantityParams) (store.StockBatch, error) {
	return r.q.AdjustBatchQuantity(ctx, arg)
}

func (r *storeRepository) RecordMovement(ctx context.Context, arg store.RecordStockMovementParams) (store.StockMovement, error) {
	return r.q.RecordStockMovement(ctx, arg)
}

func (r *storeRepository) SumReturned(ctx context.Context, arg store.SumReturnedForSaleBatchParams) (int64, error) {
	return r.q.SumReturnedForSaleBatch(ctx, arg)
}

func (r *storeRepository) MarkVoided(ctx context.Context, arg store.MarkSaleVoidedParams) (store.Sale, error) {
	return r.q.MarkSaleVoided(ctx, arg)
}

func (r *storeRepository) CreateSale(ctx context.Context, arg store.CreateSaleParams) (store.Sale, error) {
	return r.q.CreateSale(ctx, arg)
}

func (r *storeRepository) AddItem(ctx context.Context, arg store.AddSaleItemParams) (store.SaleItem, error) {
	return r.q.AddSaleItem(ctx, arg)
}

func (r *storeRepository) GetSale(ctx context.Context, id uuid.UUID) (store.Sale, error) {
	return r.q.GetSale(ctx, id)
}

func (r *storeRepository) ListItems(ctx context.Context, saleID uuid.UUID) ([]store.SaleItem, error) {
	return r.q.ListSaleItems(ctx, saleID)
}

func (r *storeRepository) ListSales(ctx context.Context, arg store.ListSalesParams) ([]store.Sale, error) {
	return r.q.ListSales(ctx, arg)
}

func (r *storeRepository) CountSales(ctx context.Context, branchID uuid.UUID) (int64, error) {
	return r.q.CountSales(ctx, branchID)
}
