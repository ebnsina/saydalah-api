package inventory

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/auth"
	"github.com/ebnsina/saydalah-api/internal/httpx"
	"github.com/ebnsina/saydalah-api/internal/store"
)

// Service holds inventory read logic. All views are branch-scoped: the branch is
// resolved from the caller's identity (admins pass an explicit branch).
type Service struct {
	repo Repository
}

// NewService constructs an inventory Service.
func NewService(repo Repository) *Service { return &Service{repo: repo} }

// BatchesResult is a page of in-stock batches plus the total count.
type BatchesResult struct {
	Items []BatchResponse
	Total int64
}

// Batches returns in-stock batches for the caller's branch, earliest expiry
// first.
func (s *Service) Batches(ctx context.Context, id auth.Identity, requestedBranch *uuid.UUID, p httpx.Pagination) (BatchesResult, error) {
	branchID, err := id.ResolveBranch(requestedBranch)
	if err != nil {
		return BatchesResult{}, err
	}
	rows, err := s.repo.ListBatches(ctx, store.ListBranchBatchesParams{
		BranchID: branchID,
		Limit:    p.Limit,
		Offset:   p.Offset,
	})
	if err != nil {
		return BatchesResult{}, fmt.Errorf("inventory: batches: %w", err)
	}
	total, err := s.repo.CountBatches(ctx, branchID)
	if err != nil {
		return BatchesResult{}, fmt.Errorf("inventory: count batches: %w", err)
	}
	items := make([]BatchResponse, len(rows))
	for i, row := range rows {
		items[i] = batchFromBranchRow(row)
	}
	return BatchesResult{Items: items, Total: total}, nil
}

// NearExpiry returns in-stock batches expiring within the given number of days.
func (s *Service) NearExpiry(ctx context.Context, id auth.Identity, requestedBranch *uuid.UUID, withinDays int32) ([]BatchResponse, error) {
	branchID, err := id.ResolveBranch(requestedBranch)
	if err != nil {
		return nil, err
	}
	if withinDays <= 0 {
		withinDays = 30
	}
	rows, err := s.repo.NearExpiry(ctx, store.ListNearExpiryBatchesParams{
		BranchID:   branchID,
		WithinDays: withinDays,
	})
	if err != nil {
		return nil, fmt.Errorf("inventory: near expiry: %w", err)
	}
	out := make([]BatchResponse, len(rows))
	for i, row := range rows {
		out[i] = batchFromExpiryRow(row)
	}
	return out, nil
}

// LowStock returns products at or below their reorder level for the branch.
func (s *Service) LowStock(ctx context.Context, id auth.Identity, requestedBranch *uuid.UUID) ([]LowStockResponse, error) {
	branchID, err := id.ResolveBranch(requestedBranch)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.LowStock(ctx, branchID)
	if err != nil {
		return nil, fmt.Errorf("inventory: low stock: %w", err)
	}
	out := make([]LowStockResponse, len(rows))
	for i, row := range rows {
		out[i] = lowStockFromRow(row)
	}
	return out, nil
}

// OnHand returns the total on-hand quantity of a product in the branch.
func (s *Service) OnHand(ctx context.Context, id auth.Identity, requestedBranch *uuid.UUID, productID uuid.UUID) (OnHandResponse, error) {
	branchID, err := id.ResolveBranch(requestedBranch)
	if err != nil {
		return OnHandResponse{}, err
	}
	qty, err := s.repo.OnHand(ctx, store.StockOnHandParams{BranchID: branchID, ProductID: productID})
	if err != nil {
		return OnHandResponse{}, fmt.Errorf("inventory: on hand: %w", err)
	}
	return OnHandResponse{ProductID: productID, BranchID: branchID, OnHand: qty}, nil
}
