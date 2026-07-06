package prescription

import (
	"context"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// Repository is the persistence surface for prescriptions, with Tx to create a
// prescription and its items atomically.
type Repository interface {
	Create(ctx context.Context, arg store.CreatePrescriptionParams) (store.Prescription, error)
	AddItem(ctx context.Context, arg store.AddPrescriptionItemParams) (store.PrescriptionItem, error)
	Get(ctx context.Context, id uuid.UUID) (store.Prescription, error)
	ListItems(ctx context.Context, prescriptionID uuid.UUID) ([]store.PrescriptionItem, error)
	List(ctx context.Context, arg store.ListPrescriptionsParams) ([]store.Prescription, error)
	Count(ctx context.Context, branchID uuid.UUID) (int64, error)
	MarkDispensed(ctx context.Context, id uuid.UUID) (store.Prescription, error)

	Tx(ctx context.Context, fn func(Repository) error) error
}

type storeRepository struct {
	store *store.Store
	q     store.Querier
}

// NewRepository returns a Repository backed by the given store.
func NewRepository(s *store.Store) Repository {
	return &storeRepository{store: s, q: s.Queries}
}

func (r *storeRepository) Tx(ctx context.Context, fn func(Repository) error) error {
	return r.store.Tx(ctx, func(q *store.Queries) error {
		return fn(&storeRepository{store: r.store, q: q})
	})
}

func (r *storeRepository) Create(ctx context.Context, arg store.CreatePrescriptionParams) (store.Prescription, error) {
	return r.q.CreatePrescription(ctx, arg)
}

func (r *storeRepository) AddItem(ctx context.Context, arg store.AddPrescriptionItemParams) (store.PrescriptionItem, error) {
	return r.q.AddPrescriptionItem(ctx, arg)
}

func (r *storeRepository) Get(ctx context.Context, id uuid.UUID) (store.Prescription, error) {
	return r.q.GetPrescription(ctx, id)
}

func (r *storeRepository) ListItems(ctx context.Context, prescriptionID uuid.UUID) ([]store.PrescriptionItem, error) {
	return r.q.ListPrescriptionItems(ctx, prescriptionID)
}

func (r *storeRepository) List(ctx context.Context, arg store.ListPrescriptionsParams) ([]store.Prescription, error) {
	return r.q.ListPrescriptions(ctx, arg)
}

func (r *storeRepository) Count(ctx context.Context, branchID uuid.UUID) (int64, error) {
	return r.q.CountPrescriptions(ctx, branchID)
}

func (r *storeRepository) MarkDispensed(ctx context.Context, id uuid.UUID) (store.Prescription, error) {
	return r.q.MarkPrescriptionDispensed(ctx, id)
}
