package reporting

import (
	"context"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// Repository is the read-only aggregate persistence surface.
type Repository interface {
	SalesSummary(ctx context.Context, arg store.SalesSummaryParams) (store.SalesSummaryRow, error)
	SalesDaily(ctx context.Context, arg store.SalesDailyParams) ([]store.SalesDailyRow, error)
	SalesByPayment(ctx context.Context, arg store.SalesByPaymentParams) ([]store.SalesByPaymentRow, error)
	InventoryValuation(ctx context.Context, branchID uuid.UUID) (store.InventoryValuationRow, error)
	TopSelling(ctx context.Context, arg store.TopSellingProductsParams) ([]store.TopSellingProductsRow, error)
}

type storeRepository struct{ q *store.Store }

// NewRepository returns a Repository backed by the given store.
func NewRepository(s *store.Store) Repository { return &storeRepository{q: s} }

func (r *storeRepository) SalesSummary(ctx context.Context, arg store.SalesSummaryParams) (store.SalesSummaryRow, error) {
	return r.q.SalesSummary(ctx, arg)
}

func (r *storeRepository) SalesDaily(ctx context.Context, arg store.SalesDailyParams) ([]store.SalesDailyRow, error) {
	return r.q.SalesDaily(ctx, arg)
}

func (r *storeRepository) SalesByPayment(ctx context.Context, arg store.SalesByPaymentParams) ([]store.SalesByPaymentRow, error) {
	return r.q.SalesByPayment(ctx, arg)
}

func (r *storeRepository) InventoryValuation(ctx context.Context, branchID uuid.UUID) (store.InventoryValuationRow, error) {
	return r.q.InventoryValuation(ctx, branchID)
}

func (r *storeRepository) TopSelling(ctx context.Context, arg store.TopSellingProductsParams) ([]store.TopSellingProductsRow, error) {
	return r.q.TopSellingProducts(ctx, arg)
}
