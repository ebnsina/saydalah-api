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

// Transfer moves qty units of a batch to another branch. In one transaction it
// decrements the source batch, records a transfer_out movement, creates a
// matching batch at the destination (same product, batch number, expiry, and
// prices), and records a transfer_in movement. Insufficient source stock or an
// unknown destination branch fails the whole transfer.
func (s *Service) Transfer(ctx context.Context, id auth.Identity, in TransferRequest) (TransferResponse, error) {
	src, err := s.repo.GetBatch(ctx, in.BatchID)
	if errors.Is(err, pgx.ErrNoRows) {
		return TransferResponse{}, httpx.ErrNotFound
	}
	if err != nil {
		return TransferResponse{}, fmt.Errorf("stock: get batch: %w", err)
	}
	if !id.CanAccessBranch(src.BranchID) {
		return TransferResponse{}, httpx.ErrForbidden
	}
	if in.ToBranchID == src.BranchID {
		return TransferResponse{}, fmt.Errorf("destination must differ from source branch: %w", httpx.ErrInvalidInput)
	}

	var depleted, created store.StockBatch
	err = s.repo.Tx(ctx, func(tx Repository) error {
		depleted, err = tx.AdjustBatch(ctx, store.AdjustBatchQuantityParams{ID: src.ID, Delta: -in.Qty})
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("source batch %s: %w", src.ID, httpx.ErrInsufficientStock)
		}
		if err != nil {
			return err
		}
		srcRef := src.ID
		if _, err := tx.RecordMovement(ctx, store.RecordStockMovementParams{
			ProductID: src.ProductID, BranchID: src.BranchID, BatchID: &srcRef,
			Type: store.MovementTypeTransferOut, Qty: -in.Qty,
			RefType: "transfer", Note: in.Note,
		}); err != nil {
			return err
		}

		created, err = tx.CreateBatch(ctx, store.CreateStockBatchParams{
			ProductID:  src.ProductID,
			BranchID:   in.ToBranchID,
			BatchNo:    src.BatchNo,
			Quantity:   in.Qty,
			CostPrice:  src.CostPrice,
			SalePrice:  src.SalePrice,
			ExpiryDate: src.ExpiryDate,
		})
		if store.IsForeignKeyViolation(err) {
			return fmt.Errorf("destination branch does not exist: %w", httpx.ErrInvalidInput)
		}
		if err != nil {
			return err
		}
		dstRef := created.ID
		if _, err := tx.RecordMovement(ctx, store.RecordStockMovementParams{
			ProductID: src.ProductID, BranchID: in.ToBranchID, BatchID: &dstRef,
			Type: store.MovementTypeTransferIn, Qty: in.Qty,
			RefType: "transfer", Note: in.Note,
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if isDomain(err) {
			return TransferResponse{}, err
		}
		return TransferResponse{}, fmt.Errorf("stock: transfer: %w", err)
	}
	return TransferResponse{Source: batchResponse(depleted), Destination: batchResponse(created)}, nil
}

// isDomain reports whether err is one of the client-facing sentinels httpx maps.
func isDomain(err error) bool {
	return errors.Is(err, httpx.ErrInvalidInput) ||
		errors.Is(err, httpx.ErrInsufficientStock) ||
		errors.Is(err, httpx.ErrConflict) ||
		errors.Is(err, httpx.ErrNotFound) ||
		errors.Is(err, httpx.ErrForbidden)
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
