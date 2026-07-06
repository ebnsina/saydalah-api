package sales

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/ebnsina/saydalah-api/internal/auth"
	"github.com/ebnsina/saydalah-api/internal/httpx"
	"github.com/ebnsina/saydalah-api/internal/store"
)

// fakeRepo is an in-memory sales.Repository for unit-testing the FEFO logic
// without a database. Tx snapshots batch state and restores it on error, so
// rollback semantics can be asserted.
type fakeRepo struct {
	batches   []store.StockBatch
	sales     map[uuid.UUID]store.Sale
	items     map[uuid.UUID][]store.SaleItem
	movements []store.RecordStockMovementParams
	nextID    int
}

func newFakeRepo(batches ...store.StockBatch) *fakeRepo {
	return &fakeRepo{
		batches: batches,
		sales:   map[uuid.UUID]store.Sale{},
		items:   map[uuid.UUID][]store.SaleItem{},
	}
}

func (f *fakeRepo) id() uuid.UUID {
	f.nextID++
	return uuid.MustParse("00000000-0000-0000-0000-" + padID(f.nextID))
}

func padID(n int) string {
	s := "000000000000"
	d := []byte(s)
	i := len(d) - 1
	for n > 0 && i >= 0 {
		d[i] = byte('0' + n%10)
		n /= 10
		i--
	}
	return string(d)
}

func (f *fakeRepo) Tx(ctx context.Context, fn func(Repository) error) error {
	snapshot := make([]store.StockBatch, len(f.batches))
	copy(snapshot, f.batches)
	if err := fn(f); err != nil {
		f.batches = snapshot // roll back
		return err
	}
	return nil
}

func (f *fakeRepo) DispensableBatches(_ context.Context, arg store.ListDispensableBatchesParams) ([]store.StockBatch, error) {
	var out []store.StockBatch
	today := time.Now()
	for _, b := range f.batches {
		if b.BranchID == arg.BranchID && b.ProductID == arg.ProductID &&
			b.Quantity > 0 && !b.ExpiryDate.Before(today.Truncate(24*time.Hour)) {
			out = append(out, b)
		}
	}
	// FEFO: earliest expiry first.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].ExpiryDate.Before(out[i].ExpiryDate) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}

func (f *fakeRepo) DecrementBatch(_ context.Context, arg store.DecrementBatchQuantityParams) (store.StockBatch, error) {
	for i := range f.batches {
		if f.batches[i].ID == arg.ID {
			if f.batches[i].Quantity < arg.Qty {
				return store.StockBatch{}, pgx.ErrNoRows
			}
			f.batches[i].Quantity -= arg.Qty
			return f.batches[i], nil
		}
	}
	return store.StockBatch{}, pgx.ErrNoRows
}

func (f *fakeRepo) AdjustBatch(_ context.Context, _ store.AdjustBatchQuantityParams) (store.StockBatch, error) {
	return store.StockBatch{}, nil
}

func (f *fakeRepo) SumReturned(_ context.Context, _ store.SumReturnedForSaleBatchParams) (int64, error) {
	return 0, nil
}

func (f *fakeRepo) MarkVoided(_ context.Context, _ store.MarkSaleVoidedParams) (store.Sale, error) {
	return store.Sale{}, nil
}

func (f *fakeRepo) RecordMovement(_ context.Context, arg store.RecordStockMovementParams) (store.StockMovement, error) {
	f.movements = append(f.movements, arg)
	return store.StockMovement{}, nil
}

func (f *fakeRepo) CreateSale(_ context.Context, arg store.CreateSaleParams) (store.Sale, error) {
	s := store.Sale{
		ID: f.id(), BranchID: arg.BranchID, CashierID: arg.CashierID,
		CustomerID: arg.CustomerID, PrescriptionID: arg.PrescriptionID,
		Subtotal: arg.Subtotal, Discount: arg.Discount, Total: arg.Total,
		Paid: arg.Paid, PaymentMethod: arg.PaymentMethod, CreatedAt: time.Now(),
	}
	f.sales[s.ID] = s
	return s, nil
}

func (f *fakeRepo) AddItem(_ context.Context, arg store.AddSaleItemParams) (store.SaleItem, error) {
	item := store.SaleItem{
		ID: f.id(), SaleID: arg.SaleID, BatchID: arg.BatchID,
		ProductID: arg.ProductID, Qty: arg.Qty, UnitPrice: arg.UnitPrice,
	}
	f.items[arg.SaleID] = append(f.items[arg.SaleID], item)
	return item, nil
}

func (f *fakeRepo) GetSale(_ context.Context, id uuid.UUID) (store.Sale, error) {
	s, ok := f.sales[id]
	if !ok {
		return store.Sale{}, pgx.ErrNoRows
	}
	return s, nil
}

func (f *fakeRepo) ListItems(_ context.Context, saleID uuid.UUID) ([]store.SaleItem, error) {
	return f.items[saleID], nil
}

func (f *fakeRepo) ListSales(_ context.Context, _ store.ListSalesParams) ([]store.Sale, error) {
	return nil, nil
}

func (f *fakeRepo) CountSales(_ context.Context, _ uuid.UUID) (int64, error) { return 0, nil }

// --- test fixtures -----------------------------------------------------------

var (
	testBranch  = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	testProduct = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	cashier     = auth.Identity{UserID: uuid.New(), Role: store.UserRoleCashier, BranchID: &testBranch}
)

func batch(id string, qty int32, price string, expiresInDays int) store.StockBatch {
	return store.StockBatch{
		ID:         uuid.MustParse(id),
		ProductID:  testProduct,
		BranchID:   testBranch,
		Quantity:   qty,
		SalePrice:  decimal.RequireFromString(price),
		ExpiryDate: time.Now().AddDate(0, 0, expiresInDays),
	}
}

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

// TestFEFODispensesEarliestExpiryFirst verifies a sale consumes the earliest-
// expiring batch first and spills into the next, computing the right subtotal.
func TestFEFODispensesEarliestExpiryFirst(t *testing.T) {
	early := batch("aaaaaaaa-0000-0000-0000-000000000001", 10, "2.00", 30)
	late := batch("aaaaaaaa-0000-0000-0000-000000000002", 100, "3.00", 365)
	repo := newFakeRepo(late, early) // intentionally out of order

	svc := NewService(repo)
	resp, err := svc.Create(context.Background(), cashier, CreateRequest{
		PaymentMethod: store.PaymentMethodCash,
		Lines:         []LineInput{{ProductID: testProduct, Qty: 15}},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 allocations, got %d", len(resp.Items))
	}
	if resp.Items[0].BatchID != early.ID || resp.Items[0].Qty != 10 {
		t.Errorf("first allocation should be 10 from early batch, got %+v", resp.Items[0])
	}
	if resp.Items[1].BatchID != late.ID || resp.Items[1].Qty != 5 {
		t.Errorf("second allocation should be 5 from late batch, got %+v", resp.Items[1])
	}
	if !resp.Subtotal.Equal(dec("35")) { // 10*2 + 5*3
		t.Errorf("subtotal = %s, want 35", resp.Subtotal)
	}
	if len(repo.movements) != 2 || repo.movements[0].Qty != -10 || repo.movements[1].Qty != -5 {
		t.Errorf("expected two negative sale movements, got %+v", repo.movements)
	}
}

// TestInsufficientStockRollsBack verifies an unfillable line fails and leaves
// all batch quantities untouched.
func TestInsufficientStockRollsBack(t *testing.T) {
	b := batch("aaaaaaaa-0000-0000-0000-000000000001", 5, "2.00", 30)
	repo := newFakeRepo(b)

	svc := NewService(repo)
	_, err := svc.Create(context.Background(), cashier, CreateRequest{
		PaymentMethod: store.PaymentMethodCash,
		Lines:         []LineInput{{ProductID: testProduct, Qty: 10}},
	})
	if err == nil || !isInsufficient(err) {
		t.Fatalf("expected insufficient stock error, got %v", err)
	}
	if repo.batches[0].Quantity != 5 {
		t.Errorf("batch quantity should be unchanged at 5, got %d", repo.batches[0].Quantity)
	}
	if len(repo.sales) != 0 {
		t.Errorf("no sale should have been created, got %d", len(repo.sales))
	}
}

// TestExpiredStockIsSkipped verifies expired batches are never dispensed.
func TestExpiredStockIsSkipped(t *testing.T) {
	expired := batch("aaaaaaaa-0000-0000-0000-000000000001", 100, "2.00", -1)
	repo := newFakeRepo(expired)

	svc := NewService(repo)
	_, err := svc.Create(context.Background(), cashier, CreateRequest{
		PaymentMethod: store.PaymentMethodCash,
		Lines:         []LineInput{{ProductID: testProduct, Qty: 1}},
	})
	if !isInsufficient(err) {
		t.Fatalf("expected insufficient stock (expired), got %v", err)
	}
}

// TestDiscountCannotExceedSubtotal verifies an over-large discount is rejected.
func TestDiscountCannotExceedSubtotal(t *testing.T) {
	b := batch("aaaaaaaa-0000-0000-0000-000000000001", 10, "2.00", 30)
	repo := newFakeRepo(b)

	svc := NewService(repo)
	_, err := svc.Create(context.Background(), cashier, CreateRequest{
		PaymentMethod: store.PaymentMethodCash,
		Discount:      dec("1000"),
		Lines:         []LineInput{{ProductID: testProduct, Qty: 1}},
	})
	if err == nil {
		t.Fatal("expected error for discount exceeding subtotal")
	}
}

func isInsufficient(err error) bool {
	return errors.Is(err, httpx.ErrInsufficientStock)
}
