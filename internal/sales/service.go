package sales

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/ebnsina/saydalah-api/internal/auth"
	"github.com/ebnsina/saydalah-api/internal/httpx"
	"github.com/ebnsina/saydalah-api/internal/store"
)

// Service holds point-of-sale business logic, notably FEFO dispensing.
type Service struct {
	repo Repository
}

// NewService constructs a sales Service.
func NewService(repo Repository) *Service { return &Service{repo: repo} }

// ListResult is a page of sales plus the total count for a branch.
type ListResult struct {
	Items []store.Sale
	Total int64
}

// allocation is a decided batch draw computed during dispensing.
type allocation struct {
	batchID   uuid.UUID
	productID uuid.UUID
	qty       int32
	unitPrice decimal.Decimal
}

// Create rings up a sale. Within one transaction it dispenses every line FEFO,
// decrements the drawn batches, records the invoice, line items, and sale
// movements. If any product lacks enough non-expired stock the whole sale rolls
// back with httpx.ErrInsufficientStock.
func (s *Service) Create(ctx context.Context, id auth.Identity, in CreateRequest) (Response, error) {
	branchID, err := id.ResolveBranch(in.BranchID)
	if err != nil {
		return Response{}, err
	}
	discount := in.Discount
	if discount.IsNegative() {
		return Response{}, fmt.Errorf("discount must not be negative: %w", httpx.ErrInvalidInput)
	}

	var sale store.Sale
	var items []store.SaleItem
	err = s.repo.Tx(ctx, func(tx Repository) error {
		allocs, subtotal, err := dispense(ctx, tx, branchID, in.Lines)
		if err != nil {
			return err
		}

		total := subtotal.Sub(discount)
		if total.IsNegative() {
			return fmt.Errorf("discount exceeds subtotal: %w", httpx.ErrInvalidInput)
		}

		sale, err = tx.CreateSale(ctx, store.CreateSaleParams{
			BranchID:       branchID,
			CashierID:      id.UserID,
			CustomerID:     in.CustomerID,
			PrescriptionID: in.PrescriptionID,
			Subtotal:       subtotal,
			Discount:       discount,
			Total:          total,
			Paid:           in.Paid,
			PaymentMethod:  in.PaymentMethod,
		})
		if store.IsForeignKeyViolation(err) {
			return fmt.Errorf("customer or prescription does not exist: %w", httpx.ErrInvalidInput)
		}
		if err != nil {
			return err
		}

		for _, a := range allocs {
			if _, err := tx.AddItem(ctx, store.AddSaleItemParams{
				SaleID:    sale.ID,
				BatchID:   a.batchID,
				ProductID: a.productID,
				Qty:       a.qty,
				UnitPrice: a.unitPrice,
			}); err != nil {
				return err
			}
			batchID := a.batchID
			saleRef := sale.ID
			if _, err := tx.RecordMovement(ctx, store.RecordStockMovementParams{
				ProductID: a.productID,
				BranchID:  branchID,
				BatchID:   &batchID,
				Type:      store.MovementTypeSale,
				Qty:       -a.qty, // outbound
				RefType:   "sale",
				RefID:     &saleRef,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Response{}, wrap("create", err)
	}

	items, err = s.repo.ListItems(ctx, sale.ID)
	if err != nil {
		return Response{}, fmt.Errorf("sales: items: %w", err)
	}
	return toResponse(sale, items), nil
}

// dispense allocates each requested line across available batches earliest-
// expiry first, decrementing stock as it goes, and returns the allocations and
// the running subtotal. It errors with ErrInsufficientStock if a line cannot be
// filled from non-expired stock.
func dispense(ctx context.Context, tx Repository, branchID uuid.UUID, lines []LineInput) ([]allocation, decimal.Decimal, error) {
	subtotal := decimal.Zero
	var allocs []allocation

	for _, line := range lines {
		batches, err := tx.DispensableBatches(ctx, store.ListDispensableBatchesParams{
			BranchID:  branchID,
			ProductID: line.ProductID,
		})
		if err != nil {
			return nil, decimal.Zero, err
		}

		remaining := line.Qty
		for _, b := range batches {
			if remaining == 0 {
				break
			}
			take := min(remaining, b.Quantity)
			if _, err := tx.DecrementBatch(ctx, store.DecrementBatchQuantityParams{Qty: take, ID: b.ID}); err != nil {
				return nil, decimal.Zero, err
			}
			allocs = append(allocs, allocation{
				batchID:   b.ID,
				productID: line.ProductID,
				qty:       take,
				unitPrice: b.SalePrice,
			})
			subtotal = subtotal.Add(b.SalePrice.Mul(decimal.NewFromInt32(take)))
			remaining -= take
		}
		if remaining > 0 {
			return nil, decimal.Zero, fmt.Errorf("product %s: %w", line.ProductID, httpx.ErrInsufficientStock)
		}
	}
	return allocs, subtotal, nil
}

// Get returns a sale with its items, enforcing branch access.
func (s *Service) Get(ctx context.Context, id auth.Identity, saleID uuid.UUID) (Response, error) {
	sale, err := s.repo.GetSale(ctx, saleID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Response{}, httpx.ErrNotFound
	}
	if err != nil {
		return Response{}, fmt.Errorf("sales: get: %w", err)
	}
	if !id.CanAccessBranch(sale.BranchID) {
		return Response{}, httpx.ErrForbidden
	}
	items, err := s.repo.ListItems(ctx, sale.ID)
	if err != nil {
		return Response{}, fmt.Errorf("sales: items: %w", err)
	}
	return toResponse(sale, items), nil
}

// List returns a page of sales for the caller's branch.
func (s *Service) List(ctx context.Context, id auth.Identity, requestedBranch *uuid.UUID, p httpx.Pagination) (ListResult, error) {
	branchID, err := id.ResolveBranch(requestedBranch)
	if err != nil {
		return ListResult{}, err
	}
	items, err := s.repo.ListSales(ctx, store.ListSalesParams{
		BranchID: branchID,
		Limit:    p.Limit,
		Offset:   p.Offset,
	})
	if err != nil {
		return ListResult{}, fmt.Errorf("sales: list: %w", err)
	}
	total, err := s.repo.CountSales(ctx, branchID)
	if err != nil {
		return ListResult{}, fmt.Errorf("sales: count: %w", err)
	}
	return ListResult{Items: items, Total: total}, nil
}

func wrap(op string, err error) error {
	if isDomain(err) {
		return err
	}
	return fmt.Errorf("sales: %s: %w", op, err)
}

func isDomain(err error) bool {
	return errors.Is(err, httpx.ErrInvalidInput) ||
		errors.Is(err, httpx.ErrConflict) ||
		errors.Is(err, httpx.ErrNotFound) ||
		errors.Is(err, httpx.ErrForbidden) ||
		errors.Is(err, httpx.ErrInsufficientStock)
}
