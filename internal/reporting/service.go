package reporting

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/auth"
	"github.com/ebnsina/saydalah-api/internal/store"
)

// Service holds reporting logic. All methods are branch-scoped via the caller's
// identity (admins pass an explicit branch).
type Service struct {
	repo Repository
}

// NewService constructs a reporting Service.
func NewService(repo Repository) *Service { return &Service{repo: repo} }

// SalesSummary totals sales for a branch over [from, to).
func (s *Service) SalesSummary(ctx context.Context, id auth.Identity, branch *uuid.UUID, rng DateRange) (SalesSummaryResponse, error) {
	branchID, err := id.ResolveBranch(branch)
	if err != nil {
		return SalesSummaryResponse{}, err
	}
	row, err := s.repo.SalesSummary(ctx, store.SalesSummaryParams{
		BranchID: branchID,
		From:     rng.From,
		To:       rng.To,
	})
	if err != nil {
		return SalesSummaryResponse{}, fmt.Errorf("reporting: sales summary: %w", err)
	}
	return SalesSummaryResponse{
		DateRange:     rng,
		SaleCount:     row.SaleCount,
		Revenue:       row.Revenue,
		DiscountTotal: row.DiscountTotal,
	}, nil
}

// SalesDaily returns per-day sales totals for a branch over [from, to).
func (s *Service) SalesDaily(ctx context.Context, id auth.Identity, branch *uuid.UUID, rng DateRange) ([]DailySales, error) {
	branchID, err := id.ResolveBranch(branch)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.SalesDaily(ctx, store.SalesDailyParams{
		BranchID: branchID,
		From:     rng.From,
		To:       rng.To,
	})
	if err != nil {
		return nil, fmt.Errorf("reporting: sales daily: %w", err)
	}
	out := make([]DailySales, len(rows))
	for i, row := range rows {
		out[i] = dailyFromRow(row)
	}
	return out, nil
}

// InventoryValuation values a branch's current stock at cost and retail.
func (s *Service) InventoryValuation(ctx context.Context, id auth.Identity, branch *uuid.UUID) (InventoryValuationResponse, error) {
	branchID, err := id.ResolveBranch(branch)
	if err != nil {
		return InventoryValuationResponse{}, err
	}
	row, err := s.repo.InventoryValuation(ctx, branchID)
	if err != nil {
		return InventoryValuationResponse{}, fmt.Errorf("reporting: valuation: %w", err)
	}
	return InventoryValuationResponse{
		TotalUnits:  row.TotalUnits,
		CostValue:   row.CostValue,
		RetailValue: row.RetailValue,
	}, nil
}

// TopProducts returns the best-selling products for a branch over [from, to).
func (s *Service) TopProducts(ctx context.Context, id auth.Identity, branch *uuid.UUID, rng DateRange, limit int32) ([]TopProduct, error) {
	branchID, err := id.ResolveBranch(branch)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	rows, err := s.repo.TopSelling(ctx, store.TopSellingProductsParams{
		BranchID: branchID,
		From:     rng.From,
		To:       rng.To,
		Limit:    limit,
	})
	if err != nil {
		return nil, fmt.Errorf("reporting: top products: %w", err)
	}
	out := make([]TopProduct, len(rows))
	for i, row := range rows {
		out[i] = topFromRow(row)
	}
	return out, nil
}
