package stock

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ebnsina/saydalah-api/internal/auth"
	"github.com/ebnsina/saydalah-api/internal/httpx"
	"github.com/ebnsina/saydalah-api/internal/store"
)

type fakeRepo struct {
	batch     store.StockBatch
	movements []store.RecordStockMovementParams
	created   []store.StockBatch

	// Sale-linked return fixtures.
	sale         store.Sale
	saleItems    []store.SaleItem
	alreadyRet   int64
	saleNotFound bool
}

func (f *fakeRepo) Tx(ctx context.Context, fn func(Repository) error) error { return fn(f) }

func (f *fakeRepo) GetBatch(context.Context, uuid.UUID) (store.StockBatch, error) {
	return f.batch, nil
}

func (f *fakeRepo) AdjustBatch(_ context.Context, arg store.AdjustBatchQuantityParams) (store.StockBatch, error) {
	if f.batch.Quantity+arg.Delta < 0 {
		return store.StockBatch{}, pgx.ErrNoRows // matches the SQL guard
	}
	f.batch.Quantity += arg.Delta
	return f.batch, nil
}

func (f *fakeRepo) CreateBatch(_ context.Context, arg store.CreateStockBatchParams) (store.StockBatch, error) {
	b := store.StockBatch{
		ID: uuid.New(), ProductID: arg.ProductID, BranchID: arg.BranchID,
		BatchNo: arg.BatchNo, Quantity: arg.Quantity, ExpiryDate: arg.ExpiryDate,
	}
	f.created = append(f.created, b)
	return b, nil
}

func (f *fakeRepo) RecordMovement(_ context.Context, arg store.RecordStockMovementParams) (store.StockMovement, error) {
	f.movements = append(f.movements, arg)
	return store.StockMovement{}, nil
}

func (f *fakeRepo) ListMovements(context.Context, store.ListStockMovementsParams) ([]store.ListStockMovementsRow, error) {
	return nil, nil
}
func (f *fakeRepo) CountMovements(context.Context, store.CountStockMovementsParams) (int64, error) {
	return 0, nil
}

func (f *fakeRepo) GetSale(context.Context, uuid.UUID) (store.Sale, error) {
	if f.saleNotFound {
		return store.Sale{}, pgx.ErrNoRows
	}
	return f.sale, nil
}

func (f *fakeRepo) ListSaleItems(context.Context, uuid.UUID) ([]store.SaleItem, error) {
	return f.saleItems, nil
}

func (f *fakeRepo) SumReturnedForSaleBatch(context.Context, store.SumReturnedForSaleBatchParams) (int64, error) {
	return f.alreadyRet, nil
}

var (
	branchID = uuid.New()
	manager  = auth.Identity{UserID: uuid.New(), Role: store.UserRoleManager, BranchID: &branchID}
)

func newBatch(qty int32) store.StockBatch {
	return store.StockBatch{ID: uuid.New(), ProductID: uuid.New(), BranchID: branchID, Quantity: qty}
}

func TestAdjustDown(t *testing.T) {
	repo := &fakeRepo{batch: newBatch(10)}
	res, err := NewService(repo).Adjust(context.Background(), manager, AdjustRequest{BatchID: repo.batch.ID, Delta: -3})
	if err != nil {
		t.Fatalf("Adjust: %v", err)
	}
	if res.Quantity != 7 {
		t.Errorf("quantity = %d, want 7", res.Quantity)
	}
	if len(repo.movements) != 1 || repo.movements[0].Type != store.MovementTypeAdjustment || repo.movements[0].Qty != -3 {
		t.Errorf("expected one adjustment movement of -3, got %+v", repo.movements)
	}
}

func TestAdjustRejectsNegativeResult(t *testing.T) {
	repo := &fakeRepo{batch: newBatch(2)}
	_, err := NewService(repo).Adjust(context.Background(), manager, AdjustRequest{BatchID: repo.batch.ID, Delta: -5})
	if !errors.Is(err, httpx.ErrInvalidInput) {
		t.Fatalf("expected invalid-input error, got %v", err)
	}
	if repo.batch.Quantity != 2 {
		t.Errorf("quantity should stay 2, got %d", repo.batch.Quantity)
	}
	if len(repo.movements) != 0 {
		t.Errorf("no movement should be recorded on failure, got %d", len(repo.movements))
	}
}

func TestReturnIncrementsAndRecordsReturn(t *testing.T) {
	repo := &fakeRepo{batch: newBatch(10)}
	res, err := NewService(repo).Return(context.Background(), manager, ReturnRequest{BatchID: repo.batch.ID, Qty: 4})
	if err != nil {
		t.Fatalf("Return: %v", err)
	}
	if res.Quantity != 14 {
		t.Errorf("quantity = %d, want 14", res.Quantity)
	}
	if repo.movements[0].Type != store.MovementTypeReturn || repo.movements[0].Qty != 4 {
		t.Errorf("expected a return movement of +4, got %+v", repo.movements[0])
	}
}

func TestTransferMovesStockBetweenBranches(t *testing.T) {
	repo := &fakeRepo{batch: newBatch(10)}
	dst := uuid.New()
	res, err := NewService(repo).Transfer(context.Background(), manager, TransferRequest{
		BatchID: repo.batch.ID, ToBranchID: dst, Qty: 4,
	})
	if err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	if res.Source.Quantity != 6 {
		t.Errorf("source quantity = %d, want 6", res.Source.Quantity)
	}
	if res.Destination.Quantity != 4 || res.Destination.BranchID != dst {
		t.Errorf("destination batch = %+v, want qty 4 at %s", res.Destination, dst)
	}
	if len(repo.movements) != 2 ||
		repo.movements[0].Type != store.MovementTypeTransferOut || repo.movements[0].Qty != -4 ||
		repo.movements[1].Type != store.MovementTypeTransferIn || repo.movements[1].Qty != 4 {
		t.Errorf("expected transfer_out(-4) then transfer_in(+4), got %+v", repo.movements)
	}
}

func TestTransferInsufficientStock(t *testing.T) {
	repo := &fakeRepo{batch: newBatch(3)}
	_, err := NewService(repo).Transfer(context.Background(), manager, TransferRequest{
		BatchID: repo.batch.ID, ToBranchID: uuid.New(), Qty: 5,
	})
	if !errors.Is(err, httpx.ErrInsufficientStock) {
		t.Fatalf("expected insufficient stock, got %v", err)
	}
	if repo.batch.Quantity != 3 {
		t.Errorf("source should be unchanged at 3, got %d", repo.batch.Quantity)
	}
}

func TestTransferRejectsSameBranch(t *testing.T) {
	repo := &fakeRepo{batch: newBatch(10)}
	_, err := NewService(repo).Transfer(context.Background(), manager, TransferRequest{
		BatchID: repo.batch.ID, ToBranchID: branchID, Qty: 1, // same as source branch
	})
	if !errors.Is(err, httpx.ErrInvalidInput) {
		t.Fatalf("expected invalid input for same-branch transfer, got %v", err)
	}
}

func TestSaleLinkedReturnWithinSoldQuantity(t *testing.T) {
	b := newBatch(0)
	saleID := uuid.New()
	repo := &fakeRepo{
		batch:      b,
		sale:       store.Sale{ID: saleID, BranchID: branchID},
		saleItems:  []store.SaleItem{{BatchID: b.ID, Qty: 5}},
		alreadyRet: 2, // 2 of 5 already returned; 3 remain
	}
	res, err := NewService(repo).Return(context.Background(), manager, ReturnRequest{
		BatchID: b.ID, Qty: 3, SaleID: &saleID,
	})
	if err != nil {
		t.Fatalf("Return: %v", err)
	}
	if res.Quantity != 3 {
		t.Errorf("quantity = %d, want 3", res.Quantity)
	}
	if repo.movements[0].RefType != "sale" || repo.movements[0].RefID == nil {
		t.Errorf("sale-linked return should reference the sale, got %+v", repo.movements[0])
	}
}

func TestSaleLinkedReturnExceedingSoldIsRejected(t *testing.T) {
	b := newBatch(0)
	saleID := uuid.New()
	repo := &fakeRepo{
		batch:      b,
		sale:       store.Sale{ID: saleID, BranchID: branchID},
		saleItems:  []store.SaleItem{{BatchID: b.ID, Qty: 5}},
		alreadyRet: 4, // only 1 remains
	}
	_, err := NewService(repo).Return(context.Background(), manager, ReturnRequest{
		BatchID: b.ID, Qty: 3, SaleID: &saleID,
	})
	if !errors.Is(err, httpx.ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
	if len(repo.movements) != 0 {
		t.Errorf("no movement should be recorded on rejection")
	}
}

func TestSaleLinkedReturnBatchNotInSale(t *testing.T) {
	b := newBatch(0)
	saleID := uuid.New()
	repo := &fakeRepo{
		batch:     b,
		sale:      store.Sale{ID: saleID, BranchID: branchID},
		saleItems: []store.SaleItem{{BatchID: uuid.New(), Qty: 5}}, // different batch
	}
	_, err := NewService(repo).Return(context.Background(), manager, ReturnRequest{
		BatchID: b.ID, Qty: 1, SaleID: &saleID,
	})
	if !errors.Is(err, httpx.ErrInvalidInput) {
		t.Fatalf("expected invalid input for batch not in sale, got %v", err)
	}
}

func TestForbiddenOtherBranch(t *testing.T) {
	repo := &fakeRepo{batch: newBatch(10)}
	other := uuid.New()
	staff := auth.Identity{UserID: uuid.New(), Role: store.UserRoleCashier, BranchID: &other}
	_, err := NewService(repo).Adjust(context.Background(), staff, AdjustRequest{BatchID: repo.batch.ID, Delta: -1})
	if !errors.Is(err, httpx.ErrForbidden) {
		t.Fatalf("expected forbidden for other branch, got %v", err)
	}
}
