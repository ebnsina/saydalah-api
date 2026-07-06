package prescription

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ebnsina/saydalah-api/internal/auth"
	"github.com/ebnsina/saydalah-api/internal/httpx"
	"github.com/ebnsina/saydalah-api/internal/sales"
	"github.com/ebnsina/saydalah-api/internal/store"
)

// Service holds prescription business logic. Dispensing delegates to the sales
// service so stock is drawn FEFO exactly as at the point of sale.
type Service struct {
	repo  Repository
	sales *sales.Service
}

// NewService constructs a prescription Service. It depends on the sales service
// to fill prescriptions.
func NewService(repo Repository, salesSvc *sales.Service) *Service {
	return &Service{repo: repo, sales: salesSvc}
}

// ListResult is a page of prescriptions plus the total count for a branch.
type ListResult struct {
	Items []Response
	Total int64
}

// Create records a prescription and its items atomically.
func (s *Service) Create(ctx context.Context, id auth.Identity, in CreateRequest) (Response, error) {
	branchID, err := id.ResolveBranch(in.BranchID)
	if err != nil {
		return Response{}, err
	}

	var presc store.Prescription
	var items []store.PrescriptionItem
	err = s.repo.Tx(ctx, func(tx Repository) error {
		var err error
		presc, err = tx.Create(ctx, store.CreatePrescriptionParams{
			CustomerID: in.CustomerID,
			BranchID:   branchID,
			DoctorName: in.DoctorName,
			Notes:      in.Notes,
		})
		if store.IsForeignKeyViolation(err) {
			return fmt.Errorf("customer does not exist: %w", httpx.ErrInvalidInput)
		}
		if err != nil {
			return err
		}
		items = make([]store.PrescriptionItem, 0, len(in.Items))
		for _, it := range in.Items {
			item, err := tx.AddItem(ctx, store.AddPrescriptionItemParams{
				PrescriptionID: presc.ID,
				ProductID:      it.ProductID,
				Qty:            it.Qty,
				Dosage:         it.Dosage,
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
		return Response{}, wrap("create", err)
	}
	return toResponse(presc, items), nil
}

// Get returns a prescription with its items, enforcing branch access.
func (s *Service) Get(ctx context.Context, id auth.Identity, prescriptionID uuid.UUID) (Response, error) {
	presc, err := s.loadAuthorized(ctx, id, prescriptionID)
	if err != nil {
		return Response{}, err
	}
	items, err := s.repo.ListItems(ctx, presc.ID)
	if err != nil {
		return Response{}, fmt.Errorf("prescription: items: %w", err)
	}
	return toResponse(presc, items), nil
}

// List returns a page of prescriptions for the caller's branch.
func (s *Service) List(ctx context.Context, id auth.Identity, requestedBranch *uuid.UUID, p httpx.Pagination) (ListResult, error) {
	branchID, err := id.ResolveBranch(requestedBranch)
	if err != nil {
		return ListResult{}, err
	}
	prescriptions, err := s.repo.List(ctx, store.ListPrescriptionsParams{
		BranchID: branchID,
		Limit:    p.Limit,
		Offset:   p.Offset,
	})
	if err != nil {
		return ListResult{}, fmt.Errorf("prescription: list: %w", err)
	}
	responses := make([]Response, 0, len(prescriptions))
	for _, presc := range prescriptions {
		lineItems, err := s.repo.ListItems(ctx, presc.ID)
		if err != nil {
			return ListResult{}, fmt.Errorf("prescription: list items: %w", err)
		}
		responses = append(responses, toResponse(presc, lineItems))
	}
	total, err := s.repo.Count(ctx, branchID)
	if err != nil {
		return ListResult{}, fmt.Errorf("prescription: count: %w", err)
	}
	return ListResult{Items: responses, Total: total}, nil
}

// Dispense fills a prescription: it rings up a sale for the prescribed items
// (drawn FEFO by the sales service) and then marks the prescription dispensed.
// The sale is created first so that insufficient stock fails cleanly without
// touching the prescription; the dispensed guard prevents a second fill.
func (s *Service) Dispense(ctx context.Context, id auth.Identity, prescriptionID uuid.UUID, in DispenseRequest) (sales.Response, error) {
	presc, err := s.loadAuthorized(ctx, id, prescriptionID)
	if err != nil {
		return sales.Response{}, err
	}
	if presc.DispensedAt != nil {
		return sales.Response{}, fmt.Errorf("prescription already dispensed: %w", httpx.ErrConflict)
	}
	items, err := s.repo.ListItems(ctx, presc.ID)
	if err != nil {
		return sales.Response{}, fmt.Errorf("prescription: items: %w", err)
	}

	lines := make([]sales.LineInput, len(items))
	for i, it := range items {
		lines[i] = sales.LineInput{ProductID: it.ProductID, Qty: it.Qty}
	}
	prescID := presc.ID
	customerID := presc.CustomerID
	sale, err := s.sales.Create(ctx, id, sales.CreateRequest{
		BranchID:       &presc.BranchID,
		CustomerID:     &customerID,
		PrescriptionID: &prescID,
		PaymentMethod:  in.PaymentMethod,
		Discount:       in.Discount,
		Paid:           in.Paid,
		Lines:          lines,
	})
	if err != nil {
		return sales.Response{}, err // already a domain error from the sales service
	}

	if _, err := s.repo.MarkDispensed(ctx, presc.ID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sales.Response{}, fmt.Errorf("prescription already dispensed: %w", httpx.ErrConflict)
		}
		return sales.Response{}, fmt.Errorf("prescription: mark dispensed: %w", err)
	}
	return sale, nil
}

func (s *Service) loadAuthorized(ctx context.Context, id auth.Identity, prescriptionID uuid.UUID) (store.Prescription, error) {
	presc, err := s.repo.Get(ctx, prescriptionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Prescription{}, httpx.ErrNotFound
	}
	if err != nil {
		return store.Prescription{}, fmt.Errorf("prescription: get: %w", err)
	}
	if !id.CanAccessBranch(presc.BranchID) {
		return store.Prescription{}, httpx.ErrForbidden
	}
	return presc, nil
}

func wrap(op string, err error) error {
	if isDomain(err) {
		return err
	}
	return fmt.Errorf("prescription: %s: %w", op, err)
}

func isDomain(err error) bool {
	return errors.Is(err, httpx.ErrInvalidInput) ||
		errors.Is(err, httpx.ErrConflict) ||
		errors.Is(err, httpx.ErrNotFound) ||
		errors.Is(err, httpx.ErrForbidden)
}
