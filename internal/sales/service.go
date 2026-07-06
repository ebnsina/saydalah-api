package sales

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/ebnsina/saydalah-api/internal/auth"
	"github.com/ebnsina/saydalah-api/internal/httpx"
	"github.com/ebnsina/saydalah-api/internal/store"
)

// Service holds point-of-sale business logic, notably FEFO dispensing.
type Service struct {
	repo    Repository
	taxRate decimal.Decimal // fraction applied to (subtotal - discount)
}

// NewService constructs a sales Service with the given tax rate (0 = tax-free).
func NewService(repo Repository, taxRate decimal.Decimal) *Service {
	return &Service{repo: repo, taxRate: taxRate}
}

// actor returns a pointer to the acting user's ID for stamping created_by on
// movement-ledger rows.
func actor(id auth.Identity) *uuid.UUID { u := id.UserID; return &u }

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

		taxable := subtotal.Sub(discount)
		if taxable.IsNegative() {
			return fmt.Errorf("discount exceeds subtotal: %w", httpx.ErrInvalidInput)
		}
		tax := taxable.Mul(s.taxRate).Round(2)
		total := taxable.Add(tax)

		sale, err = tx.CreateSale(ctx, store.CreateSaleParams{
			BranchID:       branchID,
			CashierID:      id.UserID,
			CustomerID:     in.CustomerID,
			PrescriptionID: in.PrescriptionID,
			Subtotal:       subtotal,
			Discount:       discount,
			Tax:            tax,
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
				CreatedBy: actor(id),
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
			// Actionable message: name the product and the available vs requested
			// quantity so the cashier can adjust the cart.
			available := line.Qty - remaining
			name := line.ProductID.String()
			if p, err := tx.GetProduct(ctx, line.ProductID); err == nil && p.Name != "" {
				name = p.Name
			}
			return nil, decimal.Zero, &httpx.APIError{
				Status:  http.StatusConflict,
				Message: fmt.Sprintf("Not enough stock for %s — %d in stock, %d requested.", name, available, line.Qty),
				Err:     httpx.ErrInsufficientStock,
			}
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

// Void reverses a sale: it restores each line's outstanding quantity (what was
// sold minus what has already been returned) to its batch, records sale_void
// return movements, and marks the sale voided. It is idempotent-guarded — a
// second void is a conflict — and refuses a sale that already has a partial
// return reconciled, since those are handled per-line by stock returns.
func (s *Service) Void(ctx context.Context, id auth.Identity, saleID uuid.UUID) (Response, error) {
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
	if sale.VoidedAt != nil {
		return Response{}, fmt.Errorf("sale already voided: %w", httpx.ErrConflict)
	}

	items, err := s.repo.ListItems(ctx, sale.ID)
	if err != nil {
		return Response{}, fmt.Errorf("sales: items: %w", err)
	}

	var updated store.Sale
	err = s.repo.Tx(ctx, func(tx Repository) error {
		updated, err = tx.MarkVoided(ctx, store.MarkSaleVoidedParams{ID: sale.ID, VoidedBy: actor(id)})
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("sale already voided: %w", httpx.ErrConflict)
		}
		if err != nil {
			return err
		}

		for _, it := range items {
			batchID := it.BatchID
			returned, err := tx.SumReturned(ctx, store.SumReturnedForSaleBatchParams{RefID: &sale.ID, BatchID: &batchID})
			if err != nil {
				return err
			}
			outstanding := int64(it.Qty) - returned
			if outstanding <= 0 {
				continue // fully returned already
			}
			if _, err := tx.AdjustBatch(ctx, store.AdjustBatchQuantityParams{ID: batchID, Delta: int32(outstanding)}); err != nil {
				return err
			}
			saleRef := sale.ID
			if _, err := tx.RecordMovement(ctx, store.RecordStockMovementParams{
				ProductID: it.ProductID,
				BranchID:  sale.BranchID,
				BatchID:   &batchID,
				Type:      store.MovementTypeReturn,
				Qty:       int32(outstanding),
				RefType:   "sale_void",
				RefID:     &saleRef,
				CreatedBy: actor(id),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Response{}, wrap("void", err)
	}
	return toResponse(updated, items), nil
}

// List returns a page of sales for the caller's branch, optionally filtered to
// one customer.
func (s *Service) List(ctx context.Context, id auth.Identity, requestedBranch *uuid.UUID, customerID *uuid.UUID, p httpx.Pagination) (ListResult, error) {
	branchID, err := id.ResolveBranch(requestedBranch)
	if err != nil {
		return ListResult{}, err
	}
	items, err := s.repo.ListSales(ctx, store.ListSalesParams{
		BranchID:   branchID,
		CustomerID: customerID,
		Limit:      p.Limit,
		Offset:     p.Offset,
	})
	if err != nil {
		return ListResult{}, fmt.Errorf("sales: list: %w", err)
	}
	total, err := s.repo.CountSales(ctx, store.CountSalesParams{BranchID: branchID, CustomerID: customerID})
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
	var apiErr *httpx.APIError
	if errors.As(err, &apiErr) {
		return true
	}
	return errors.Is(err, httpx.ErrInvalidInput) ||
		errors.Is(err, httpx.ErrConflict) ||
		errors.Is(err, httpx.ErrNotFound) ||
		errors.Is(err, httpx.ErrForbidden) ||
		errors.Is(err, httpx.ErrInsufficientStock)
}
