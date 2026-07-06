//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/ebnsina/saydalah-api/internal/auth"
	"github.com/ebnsina/saydalah-api/internal/branch"
	"github.com/ebnsina/saydalah-api/internal/catalog"
	"github.com/ebnsina/saydalah-api/internal/httpx"
	"github.com/ebnsina/saydalah-api/internal/inventory"
	"github.com/ebnsina/saydalah-api/internal/purchasing"
	"github.com/ebnsina/saydalah-api/internal/sales"
	"github.com/ebnsina/saydalah-api/internal/stock"
	"github.com/ebnsina/saydalah-api/internal/store"
	"github.com/ebnsina/saydalah-api/internal/supplier"
	"github.com/ebnsina/saydalah-api/internal/user"
)

// env bundles the services under test wired to the shared store.
type env struct {
	st        *store.Store
	branch    *branch.Service
	catalog   *catalog.Service
	supplier  *supplier.Service
	purchase  *purchasing.Service
	sales     *sales.Service
	inventory *inventory.Service
	stock     *stock.Service
	admin     auth.Identity // an admin identity backed by a real users row
}

func newEnv(t *testing.T) *env {
	t.Helper()
	st := newStore()
	e := &env{
		st:        st,
		branch:    branch.NewService(branch.NewRepository(st)),
		catalog:   catalog.NewService(catalog.NewRepository(st)),
		supplier:  supplier.NewService(supplier.NewRepository(st)),
		purchase:  purchasing.NewService(purchasing.NewRepository(st)),
		sales:     sales.NewService(sales.NewRepository(st)),
		inventory: inventory.NewService(inventory.NewRepository(st)),
		stock:     stock.NewService(stock.NewRepository(st)),
	}
	// A sale's cashier_id references users(id), so the acting identity must be a
	// real user.
	u, err := user.NewService(user.NewRepository(st)).Create(context.Background(), user.CreateRequest{
		Email:    "admin+" + uuid.NewString() + "@test.local",
		Password: "password123",
		Role:     store.UserRoleAdmin,
	})
	if err != nil {
		t.Fatalf("seed admin user: %v", err)
	}
	e.admin = auth.Identity{UserID: u.ID, Role: store.UserRoleAdmin}
	return e
}

// seedBranchAndProduct creates an isolated branch, product, and supplier so each
// test's stock math is independent.
func (e *env) seedBranchAndProduct(t *testing.T) (branchID, productID, supplierID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	b, err := e.branch.Create(ctx, branch.CreateRequest{Name: "Branch " + uuid.NewString()})
	if err != nil {
		t.Fatalf("create branch: %v", err)
	}
	p, err := e.catalog.Create(ctx, catalog.CreateRequest{Name: "Drug " + uuid.NewString(), Unit: "tablet"})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	s, err := e.supplier.Create(ctx, supplier.CreateRequest{Name: "Supplier " + uuid.NewString()})
	if err != nil {
		t.Fatalf("create supplier: %v", err)
	}
	return b.ID, p.ID, s.ID
}

func money(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func (e *env) onHand(t *testing.T, branchID, productID uuid.UUID) int64 {
	t.Helper()
	res, err := e.inventory.OnHand(context.Background(), e.admin, &branchID, productID)
	if err != nil {
		t.Fatalf("on-hand: %v", err)
	}
	return res.OnHand
}

// receiveTwoBatches places a PO and receives an early (small) and a late (large)
// batch so FEFO behavior can be exercised.
func (e *env) receiveTwoBatches(t *testing.T, branchID, productID, supplierID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	po, err := e.purchase.Create(ctx, e.admin, purchasing.CreateRequest{
		BranchID:   &branchID,
		SupplierID: supplierID,
		Items:      []purchasing.ItemInput{{ProductID: productID, Qty: 110, UnitCost: money("1")}},
	})
	if err != nil {
		t.Fatalf("create PO: %v", err)
	}
	_, err = e.purchase.Receive(ctx, e.admin, po.ID, purchasing.ReceiveRequest{
		Lines: []purchasing.ReceiveLine{
			{ProductID: productID, BatchNo: "EARLY", Quantity: 10, CostPrice: money("1"), SalePrice: money("2"), ExpiryDate: time.Now().AddDate(0, 0, 30)},
			{ProductID: productID, BatchNo: "LATE", Quantity: 100, CostPrice: money("1"), SalePrice: money("3"), ExpiryDate: time.Now().AddDate(0, 0, 365)},
		},
	})
	if err != nil {
		t.Fatalf("receive PO: %v", err)
	}
}

func TestReceiptThenFEFOSale(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	branchID, productID, supplierID := e.seedBranchAndProduct(t)

	e.receiveTwoBatches(t, branchID, productID, supplierID)
	if got := e.onHand(t, branchID, productID); got != 110 {
		t.Fatalf("on-hand after receipt = %d, want 110", got)
	}

	sale, err := e.sales.Create(ctx, e.admin, sales.CreateRequest{
		BranchID:      &branchID,
		PaymentMethod: store.PaymentMethodCash,
		Lines:         []sales.LineInput{{ProductID: productID, Qty: 15}},
	})
	if err != nil {
		t.Fatalf("sale: %v", err)
	}
	// FEFO fully consumes the earliest-expiring EARLY batch (10 @ 2.00) before
	// drawing 5 from LATE (@ 3.00). Line items are ordered by batch expiry, so
	// EARLY comes first deterministically.
	if len(sale.Items) != 2 {
		t.Fatalf("expected 2 line items, got %d: %+v", len(sale.Items), sale.Items)
	}
	if !sale.Items[0].UnitPrice.Equal(money("2")) || sale.Items[0].Qty != 10 {
		t.Errorf("first line should be 10 @ 2.00 (EARLY), got %+v", sale.Items[0])
	}
	if !sale.Items[1].UnitPrice.Equal(money("3")) || sale.Items[1].Qty != 5 {
		t.Errorf("second line should be 5 @ 3.00 (LATE), got %+v", sale.Items[1])
	}
	if !sale.Subtotal.Equal(money("35")) {
		t.Errorf("subtotal = %s, want 35", sale.Subtotal)
	}
	if got := e.onHand(t, branchID, productID); got != 95 {
		t.Errorf("on-hand after sale = %d, want 95", got)
	}
}

func TestSaleInsufficientStockRollsBack(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	branchID, productID, supplierID := e.seedBranchAndProduct(t)
	e.receiveTwoBatches(t, branchID, productID, supplierID)

	_, err := e.sales.Create(ctx, e.admin, sales.CreateRequest{
		BranchID:      &branchID,
		PaymentMethod: store.PaymentMethodCash,
		Lines:         []sales.LineInput{{ProductID: productID, Qty: 100_000}},
	})
	if !errors.Is(err, httpx.ErrInsufficientStock) {
		t.Fatalf("expected insufficient stock, got %v", err)
	}
	if got := e.onHand(t, branchID, productID); got != 110 {
		t.Errorf("on-hand should be unchanged at 110 after failed sale, got %d", got)
	}
}

func TestTransferBetweenBranches(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	src, productID, supplierID := e.seedBranchAndProduct(t)
	e.receiveTwoBatches(t, src, productID, supplierID)

	dst, err := e.branch.Create(ctx, branch.CreateRequest{Name: "Dst " + uuid.NewString()})
	if err != nil {
		t.Fatalf("create dst branch: %v", err)
	}

	// Pick the LATE batch (the one with stock and a far expiry).
	batches, err := e.st.ListDispensableBatches(ctx, store.ListDispensableBatchesParams{BranchID: src, ProductID: productID})
	if err != nil || len(batches) == 0 {
		t.Fatalf("list batches: %v", err)
	}
	late := batches[len(batches)-1]

	if _, err := e.stock.Transfer(ctx, e.admin, stock.TransferRequest{
		BatchID: late.ID, ToBranchID: dst.ID, Qty: 20,
	}); err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if got := e.onHand(t, dst.ID, productID); got != 20 {
		t.Errorf("destination on-hand = %d, want 20", got)
	}
	if got := e.onHand(t, src, productID); got != 90 {
		t.Errorf("source on-hand = %d, want 90", got)
	}
}

func TestStockTakeReconciliation(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	branchID, productID, supplierID := e.seedBranchAndProduct(t)
	e.receiveTwoBatches(t, branchID, productID, supplierID) // 110 on hand

	// Pick the LATE batch (has 100 units) and physically count it as 90.
	batches, err := e.st.ListDispensableBatches(ctx, store.ListDispensableBatchesParams{BranchID: branchID, ProductID: productID})
	if err != nil || len(batches) == 0 {
		t.Fatalf("list batches: %v", err)
	}
	late := batches[len(batches)-1]

	res, err := e.stock.StockTake(ctx, e.admin, stock.StockTakeRequest{
		BranchID: &branchID,
		Lines:    []stock.StockTakeLine{{BatchID: late.ID, CountedQty: 90}},
	})
	if err != nil {
		t.Fatalf("stock-take: %v", err)
	}
	if len(res.Lines) != 1 || res.Lines[0].PreviousQty != 100 || res.Lines[0].CountedQty != 90 || res.Lines[0].Delta != -10 {
		t.Fatalf("unexpected reconciliation: %+v", res.Lines)
	}
	// 110 - 10 counted-down = 100 on hand.
	if got := e.onHand(t, branchID, productID); got != 100 {
		t.Errorf("on-hand after stock-take = %d, want 100", got)
	}

	// The stock-take movement must be attributed to the acting user (audit).
	moves, err := e.stock.Movements(ctx, e.admin, &branchID, &productID, httpx.Pagination{Limit: 10})
	if err != nil {
		t.Fatalf("movements: %v", err)
	}
	var found bool
	for _, m := range moves.Items {
		if m.RefType == "stock_take" {
			found = true
			if m.CreatedBy == nil || *m.CreatedBy != e.admin.UserID {
				t.Errorf("stock_take movement created_by = %v, want %s", m.CreatedBy, e.admin.UserID)
			}
		}
	}
	if !found {
		t.Error("no stock_take movement found in ledger")
	}
}

func TestSaleLinkedReturnCap(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	branchID, productID, supplierID := e.seedBranchAndProduct(t)
	e.receiveTwoBatches(t, branchID, productID, supplierID)

	sale, err := e.sales.Create(ctx, e.admin, sales.CreateRequest{
		BranchID:      &branchID,
		PaymentMethod: store.PaymentMethodCash,
		Lines:         []sales.LineInput{{ProductID: productID, Qty: 4}},
	})
	if err != nil {
		t.Fatalf("sale: %v", err)
	}
	batchID := sale.Items[0].BatchID

	// Returning 3 of 4 is allowed.
	if _, err := e.stock.Return(ctx, e.admin, stock.ReturnRequest{BatchID: batchID, Qty: 3, SaleID: &sale.ID}); err != nil {
		t.Fatalf("valid return: %v", err)
	}
	// A further 2 would exceed the 4 sold.
	if _, err := e.stock.Return(ctx, e.admin, stock.ReturnRequest{BatchID: batchID, Qty: 2, SaleID: &sale.ID}); !errors.Is(err, httpx.ErrInvalidInput) {
		t.Fatalf("over-return should be rejected, got %v", err)
	}
}
