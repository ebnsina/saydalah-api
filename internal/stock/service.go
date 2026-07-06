package stock

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ebnsina/saydalah-api/internal/auth"
	"github.com/ebnsina/saydalah-api/internal/httpx"
	"github.com/ebnsina/saydalah-api/internal/store"
)

// Service holds manual stock-operation logic (adjustments and returns) and the
// movement-ledger read.
type Service struct {
	repo Repository
}

// NewService constructs a stock Service.
func NewService(repo Repository) *Service { return &Service{repo: repo} }

// MovementsResult is a page of ledger rows plus the total count.
type MovementsResult struct {
	Items []MovementResponse
	Total int64
}

// Adjust applies a signed correction to a batch and records an 'adjustment'
// movement. The batch must belong to a branch the caller can access; the result
// may not go below zero.
func (s *Service) Adjust(ctx context.Context, id auth.Identity, in AdjustRequest) (BatchResponse, error) {
	return s.apply(ctx, id, in.BatchID, in.Delta, store.MovementTypeAdjustment, "adjustment", nil, in.Note)
}

// Return puts quantity back into a batch (a customer return) and records a
// 'return' movement, optionally linked to the originating sale.
func (s *Service) Return(ctx context.Context, id auth.Identity, in ReturnRequest) (BatchResponse, error) {
	refType := "return"
	if in.SaleID != nil {
		refType = "sale"
	}
	return s.apply(ctx, id, in.BatchID, in.Qty, store.MovementTypeReturn, refType, in.SaleID, in.Note)
}

// apply loads and authorizes the batch, then adjusts its quantity and records
// the movement in one transaction.
func (s *Service) apply(
	ctx context.Context,
	id auth.Identity,
	batchID uuid.UUID,
	delta int32,
	mvType store.MovementType,
	refType string,
	refID *uuid.UUID,
	note string,
) (BatchResponse, error) {
	batch, err := s.repo.GetBatch(ctx, batchID)
	if errors.Is(err, pgx.ErrNoRows) {
		return BatchResponse{}, httpx.ErrNotFound
	}
	if err != nil {
		return BatchResponse{}, fmt.Errorf("stock: get batch: %w", err)
	}
	if !id.CanAccessBranch(batch.BranchID) {
		return BatchResponse{}, httpx.ErrForbidden
	}

	var updated store.StockBatch
	err = s.repo.Tx(ctx, func(tx Repository) error {
		var err error
		updated, err = tx.AdjustBatch(ctx, store.AdjustBatchQuantityParams{ID: batchID, Delta: delta})
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("adjustment would make stock negative: %w", httpx.ErrInvalidInput)
		}
		if err != nil {
			return err
		}
		bID := batchID
		if _, err := tx.RecordMovement(ctx, store.RecordStockMovementParams{
			ProductID: batch.ProductID,
			BranchID:  batch.BranchID,
			BatchID:   &bID,
			Type:      mvType,
			Qty:       delta,
			RefType:   refType,
			RefID:     refID,
			Note:      note,
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, httpx.ErrInvalidInput) {
			return BatchResponse{}, err
		}
		return BatchResponse{}, fmt.Errorf("stock: apply: %w", err)
	}
	return batchResponse(updated), nil
}

// Movements returns a page of the movement ledger for the caller's branch,
// optionally filtered to a single product.
func (s *Service) Movements(ctx context.Context, id auth.Identity, branch, product *uuid.UUID, p httpx.Pagination) (MovementsResult, error) {
	branchID, err := id.ResolveBranch(branch)
	if err != nil {
		return MovementsResult{}, err
	}
	rows, err := s.repo.ListMovements(ctx, store.ListStockMovementsParams{
		BranchID:  branchID,
		ProductID: product,
		Limit:     p.Limit,
		Offset:    p.Offset,
	})
	if err != nil {
		return MovementsResult{}, fmt.Errorf("stock: movements: %w", err)
	}
	total, err := s.repo.CountMovements(ctx, store.CountStockMovementsParams{
		BranchID:  branchID,
		ProductID: product,
	})
	if err != nil {
		return MovementsResult{}, fmt.Errorf("stock: count movements: %w", err)
	}
	items := make([]MovementResponse, len(rows))
	for i, row := range rows {
		items[i] = movementResponse(row)
	}
	return MovementsResult{Items: items, Total: total}, nil
}
