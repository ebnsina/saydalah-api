package purchasing

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

// Service holds purchasing business logic, including the transactional goods-
// receipt flow that turns an order into stock.
type Service struct {
	repo Repository
}

// NewService constructs a purchasing Service.
func NewService(repo Repository) *Service { return &Service{repo: repo} }

// ListResult is a page of purchase orders plus the total count for a branch.
type ListResult struct {
	Items []store.PurchaseOrder
	Total int64
}

// Create places a purchase order and its items atomically, within the branch the
// caller is authorized for.
func (s *Service) Create(ctx context.Context, id auth.Identity, in CreateRequest) (OrderResponse, error) {
	branchID, err := id.ResolveBranch(in.BranchID)
	if err != nil {
		return OrderResponse{}, err
	}
	for _, it := range in.Items {
		if it.UnitCost.IsNegative() {
			return OrderResponse{}, fmt.Errorf("unit_cost must not be negative: %w", httpx.ErrInvalidInput)
		}
	}

	var po store.PurchaseOrder
	var items []store.PurchaseOrderItem
	err = s.repo.Tx(ctx, func(tx Repository) error {
		var err error
		po, err = tx.CreateOrder(ctx, store.CreatePurchaseOrderParams{
			BranchID:   branchID,
			SupplierID: in.SupplierID,
			Reference:  in.Reference,
		})
		if store.IsForeignKeyViolation(err) {
			return fmt.Errorf("supplier does not exist: %w", httpx.ErrInvalidInput)
		}
		if err != nil {
			return err
		}
		items = make([]store.PurchaseOrderItem, 0, len(in.Items))
		for _, it := range in.Items {
			item, err := tx.AddItem(ctx, store.AddPurchaseOrderItemParams{
				PoID:      po.ID,
				ProductID: it.ProductID,
				Qty:       it.Qty,
				UnitCost:  it.UnitCost,
			})
			if store.IsForeignKeyViolation(err) {
				return fmt.Errorf("product does not exist: %w", httpx.ErrInvalidInput)
			}
			if err != nil {
				return err
			}
			items = append(items, item)
		}
		return nil
	})
	if err != nil {
		return OrderResponse{}, wrap("create", err)
	}
	return toResponse(po, items), nil
}

// Get returns a purchase order with its items, enforcing branch access.
func (s *Service) Get(ctx context.Context, id auth.Identity, poID uuid.UUID) (OrderResponse, error) {
	po, err := s.loadAuthorized(ctx, id, poID)
	if err != nil {
		return OrderResponse{}, err
	}
	items, err := s.repo.ListItems(ctx, po.ID)
	if err != nil {
		return OrderResponse{}, fmt.Errorf("purchasing: items: %w", err)
	}
	return toResponse(po, items), nil
}

// List returns a page of purchase orders for the caller's branch.
func (s *Service) List(ctx context.Context, id auth.Identity, requestedBranch *uuid.UUID, p httpx.Pagination) (ListResult, error) {
	branchID, err := id.ResolveBranch(requestedBranch)
	if err != nil {
		return ListResult{}, err
	}
	items, err := s.repo.ListOrders(ctx, store.ListPurchaseOrdersParams{
		BranchID: branchID,
		Limit:    p.Limit,
		Offset:   p.Offset,
	})
	if err != nil {
		return ListResult{}, fmt.Errorf("purchasing: list: %w", err)
	}
	total, err := s.repo.CountOrders(ctx, branchID)
	if err != nil {
		return ListResult{}, fmt.Errorf("purchasing: count: %w", err)
	}
	return ListResult{Items: items, Total: total}, nil
}

// Receive records goods received against an order: it marks the order received
// and, for each line, creates a stock batch and a purchase movement — all in one
// transaction. Receiving an already-received order is a conflict.
func (s *Service) Receive(ctx context.Context, id auth.Identity, poID uuid.UUID, in ReceiveRequest) (OrderResponse, error) {
	po, err := s.loadAuthorized(ctx, id, poID)
	if err != nil {
		return OrderResponse{}, err
	}
	for _, l := range in.Lines {
		if l.CostPrice.IsNegative() || l.SalePrice.IsNegative() {
			return OrderResponse{}, fmt.Errorf("prices must not be negative: %w", httpx.ErrInvalidInput)
		}
	}

	var updated store.PurchaseOrder
	err = s.repo.Tx(ctx, func(tx Repository) error {
		var err error
		updated, err = tx.MarkReceived(ctx, po.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("order already received: %w", httpx.ErrConflict)
		}
		if err != nil {
			return err
		}

		for _, l := range in.Lines {
			batch, err := tx.CreateBatch(ctx, store.CreateStockBatchParams{
				ProductID:  l.ProductID,
				BranchID:   po.BranchID,
				BatchNo:    l.BatchNo,
				Quantity:   l.Quantity,
				CostPrice:  l.CostPrice,
				SalePrice:  l.SalePrice,
				ExpiryDate: l.ExpiryDate,
			})
			if store.IsForeignKeyViolation(err) {
				return fmt.Errorf("product does not exist: %w", httpx.ErrInvalidInput)
			}
			if err != nil {
				return err
			}
			poRef := po.ID
			if _, err := tx.RecordMovement(ctx, store.RecordStockMovementParams{
				ProductID: l.ProductID,
				BranchID:  po.BranchID,
				BatchID:   &batch.ID,
				Type:      store.MovementTypePurchase,
				Qty:       l.Quantity,
				RefType:   "purchase_order",
				RefID:     &poRef,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return OrderResponse{}, wrap("receive", err)
	}

	items, err := s.repo.ListItems(ctx, po.ID)
	if err != nil {
		return OrderResponse{}, fmt.Errorf("purchasing: items: %w", err)
	}
	return toResponse(updated, items), nil
}

// loadAuthorized fetches a purchase order and verifies the caller may access its
// branch.
func (s *Service) loadAuthorized(ctx context.Context, id auth.Identity, poID uuid.UUID) (store.PurchaseOrder, error) {
	po, err := s.repo.GetOrder(ctx, poID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.PurchaseOrder{}, httpx.ErrNotFound
	}
	if err != nil {
		return store.PurchaseOrder{}, fmt.Errorf("purchasing: get: %w", err)
	}
	if !id.CanAccessBranch(po.BranchID) {
		return store.PurchaseOrder{}, httpx.ErrForbidden
	}
	return po, nil
}

// wrap preserves domain sentinels (so httpx maps them) while adding context to
// unexpected errors.
func wrap(op string, err error) error {
	if isDomain(err) {
		return err
	}
	return fmt.Errorf("purchasing: %s: %w", op, err)
}

func isDomain(err error) bool {
	return errors.Is(err, httpx.ErrInvalidInput) ||
		errors.Is(err, httpx.ErrConflict) ||
		errors.Is(err, httpx.ErrNotFound) ||
		errors.Is(err, httpx.ErrForbidden)
}
