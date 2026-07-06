package stock

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ebnsina/saydalah-api/internal/auth"
	"github.com/ebnsina/saydalah-api/internal/cache"
	"github.com/ebnsina/saydalah-api/internal/httpx"
	"github.com/ebnsina/saydalah-api/internal/store"
)

// Service holds manual stock-operation logic (adjustments and returns) and the
// movement-ledger read.
// Service carries the report-cache invalidator so stock changes refresh the
// inventory-valuation report immediately.
type Service struct {
	repo  Repository
	cache *cache.Cache
}

// NewService constructs a stock Service.
func NewService(repo Repository, c *cache.Cache) *Service { return &Service{repo: repo, cache: c} }

// actor returns a pointer to the acting user's ID for stamping created_by on
// movement-ledger rows.
func actor(id auth.Identity) *uuid.UUID { u := id.UserID; return &u }

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
// 'return' movement. When linked to a sale, it is reconciled against that sale:
// the batch must have been dispensed in it and the cumulative returned quantity
// may not exceed what was sold from that batch.
func (s *Service) Return(ctx context.Context, id auth.Identity, in ReturnRequest) (BatchResponse, error) {
	refType := "return"
	if in.SaleID != nil {
		refType = "sale"
		if err := s.validateSaleReturn(ctx, id, in); err != nil {
			return BatchResponse{}, err
		}
	}
	return s.apply(ctx, id, in.BatchID, in.Qty, store.MovementTypeReturn, refType, in.SaleID, in.Note)
}

// validateSaleReturn enforces that a sale-linked return is legitimate: the sale
// exists in a branch the caller can access, the batch was actually sold in it,
// and qty + already-returned does not exceed the quantity dispensed from that
// batch.
func (s *Service) validateSaleReturn(ctx context.Context, id auth.Identity, in ReturnRequest) error {
	sale, err := s.repo.GetSale(ctx, *in.SaleID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("sale not found: %w", httpx.ErrInvalidInput)
	}
	if err != nil {
		return fmt.Errorf("stock: get sale: %w", err)
	}
	if !id.CanAccessBranch(sale.BranchID) {
		return httpx.ErrForbidden
	}
	if sale.VoidedAt != nil {
		return fmt.Errorf("sale has been voided: %w", httpx.ErrConflict)
	}

	items, err := s.repo.ListSaleItems(ctx, sale.ID)
	if err != nil {
		return fmt.Errorf("stock: sale items: %w", err)
	}
	var sold int64
	for _, it := range items {
		if it.BatchID == in.BatchID {
			sold += int64(it.Qty)
		}
	}
	if sold == 0 {
		return fmt.Errorf("batch was not dispensed in this sale: %w", httpx.ErrInvalidInput)
	}

	returned, err := s.repo.SumReturnedForSaleBatch(ctx, store.SumReturnedForSaleBatchParams{
		RefID:   in.SaleID,
		BatchID: &in.BatchID,
	})
	if err != nil {
		return fmt.Errorf("stock: sum returned: %w", err)
	}
	if returned+int64(in.Qty) > sold {
		return fmt.Errorf("return exceeds quantity sold (%d sold, %d already returned): %w",
			sold, returned, httpx.ErrInvalidInput)
	}
	return nil
}

// apply loads and authorizes the batch, then adjusts its quantity and records
// the movement in one transaction.
// PurchaseReturn removes quantity from a batch (returning it to the supplier),
// recording a purchase_return movement.
func (s *Service) PurchaseReturn(ctx context.Context, id auth.Identity, in PurchaseReturnRequest) (BatchResponse, error) {
	return s.apply(ctx, id, in.BatchID, -in.Qty, store.MovementTypePurchaseReturn, "purchase_return", nil, in.Note)
}

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
			CreatedBy: actor(id),
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
	s.cache.Bump(ctx, batch.BranchID) // stock changed → refresh valuation
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
			RefType: "transfer", Note: in.Note, CreatedBy: actor(id),
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
			RefType: "transfer", Note: in.Note, CreatedBy: actor(id),
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
	s.cache.Bump(ctx, src.BranchID, in.ToBranchID) // both branches' valuations changed
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

// StockTake reconciles physically counted quantities against the system. In one
// transaction, each batch (which must belong to the caller's branch) is set to
// its counted quantity and any difference is recorded as an adjustment movement
// tagged 'stock_take'. Batches counted equal to the system value produce no
// movement.
func (s *Service) StockTake(ctx context.Context, id auth.Identity, in StockTakeRequest) (StockTakeResponse, error) {
	branchID, err := id.ResolveBranch(in.BranchID)
	if err != nil {
		return StockTakeResponse{}, err
	}

	out := StockTakeResponse{Lines: make([]StockTakeResult, 0, len(in.Lines))}
	err = s.repo.Tx(ctx, func(tx Repository) error {
		for _, line := range in.Lines {
			batch, err := tx.GetBatch(ctx, line.BatchID)
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("batch %s: %w", line.BatchID, httpx.ErrNotFound)
			}
			if err != nil {
				return err
			}
			if batch.BranchID != branchID {
				return fmt.Errorf("batch %s is not in this branch: %w", line.BatchID, httpx.ErrInvalidInput)
			}

			row, err := tx.SetBatchQuantity(ctx, store.SetBatchQuantityParams{
				ID:       line.BatchID,
				Quantity: line.CountedQty,
			})
			if err != nil {
				return err
			}
			delta := row.Quantity - row.PreviousQuantity
			if delta != 0 {
				bID := line.BatchID
				if _, err := tx.RecordMovement(ctx, store.RecordStockMovementParams{
					ProductID: batch.ProductID,
					BranchID:  branchID,
					BatchID:   &bID,
					Type:      store.MovementTypeAdjustment,
					Qty:       delta,
					RefType:   "stock_take",
					Note:      "physical count",
					CreatedBy: actor(id),
				}); err != nil {
					return err
				}
			}
			out.Lines = append(out.Lines, StockTakeResult{
				BatchID:     line.BatchID,
				PreviousQty: row.PreviousQuantity,
				CountedQty:  row.Quantity,
				Delta:       delta,
			})
			out.TotalDelta += int64(delta)
		}
		return nil
	})
	if err != nil {
		if isDomain(err) {
			return StockTakeResponse{}, err
		}
		return StockTakeResponse{}, fmt.Errorf("stock: stock-take: %w", err)
	}
	return out, nil
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
