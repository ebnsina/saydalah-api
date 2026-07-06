package purchasing

import (
	"context"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// Repository is the persistence surface for purchasing. It includes the stock
// batch/movement writes used during goods receipt, plus Tx to run a receive
// atomically. The methods delegate to a store.Querier, which is satisfied by
// both the pool-bound queries and a transaction-bound one.
type Repository interface {
	CreateOrder(ctx context.Context, arg store.CreatePurchaseOrderParams) (store.PurchaseOrder, error)
	AddItem(ctx context.Context, arg store.AddPurchaseOrderItemParams) (store.PurchaseOrderItem, error)
	GetOrder(ctx context.Context, id uuid.UUID) (store.PurchaseOrder, error)
	ListItems(ctx context.Context, poID uuid.UUID) ([]store.PurchaseOrderItem, error)
	ListOrders(ctx context.Context, arg store.ListPurchaseOrdersParams) ([]store.PurchaseOrder, error)
	CountOrders(ctx context.Context, branchID uuid.UUID) (int64, error)
	MarkReceived(ctx context.Context, id uuid.UUID) (store.PurchaseOrder, error)
	CreateBatch(ctx context.Context, arg store.CreateStockBatchParams) (store.StockBatch, error)
	RecordMovement(ctx context.Context, arg store.RecordStockMovementParams) (store.StockMovement, error)

	// Tx runs fn against a transaction-scoped Repository, committing on success.
	Tx(ctx context.Context, fn func(Repository) error) error
}

// storeRepository holds the store (for beginning transactions) and the active
// query executor q (pool-bound normally, tx-bound inside Tx).
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

func (r *storeRepository) CreateOrder(ctx context.Context, arg store.CreatePurchaseOrderParams) (store.PurchaseOrder, error) {
	return r.q.CreatePurchaseOrder(ctx, arg)
}

func (r *storeRepository) AddItem(ctx context.Context, arg store.AddPurchaseOrderItemParams) (store.PurchaseOrderItem, error) {
	return r.q.AddPurchaseOrderItem(ctx, arg)
}

func (r *storeRepository) GetOrder(ctx context.Context, id uuid.UUID) (store.PurchaseOrder, error) {
	return r.q.GetPurchaseOrder(ctx, id)
}

func (r *storeRepository) ListItems(ctx context.Context, poID uuid.UUID) ([]store.PurchaseOrderItem, error) {
	return r.q.ListPurchaseOrderItems(ctx, poID)
}

func (r *storeRepository) ListOrders(ctx context.Context, arg store.ListPurchaseOrdersParams) ([]store.PurchaseOrder, error) {
	return r.q.ListPurchaseOrders(ctx, arg)
}

func (r *storeRepository) CountOrders(ctx context.Context, branchID uuid.UUID) (int64, error) {
	return r.q.CountPurchaseOrders(ctx, branchID)
}

func (r *storeRepository) MarkReceived(ctx context.Context, id uuid.UUID) (store.PurchaseOrder, error) {
	return r.q.MarkPurchaseOrderReceived(ctx, id)
}

func (r *storeRepository) CreateBatch(ctx context.Context, arg store.CreateStockBatchParams) (store.StockBatch, error) {
	return r.q.CreateStockBatch(ctx, arg)
}

func (r *storeRepository) RecordMovement(ctx context.Context, arg store.RecordStockMovementParams) (store.StockMovement, error) {
	return r.q.RecordStockMovement(ctx, arg)
}
