package auth

import (
	"context"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// Repository is the minimal persistence surface the auth service needs. Keeping
// it here (rather than importing the user package) avoids an import cycle, since
// the user package depends on auth for password hashing.
type Repository interface {
	GetByEmail(ctx context.Context, email string) (store.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (store.User, error)
}

type storeRepository struct{ q *store.Store }

// NewRepository returns a Repository backed by the given store.
func NewRepository(s *store.Store) Repository { return &storeRepository{q: s} }

func (r *storeRepository) GetByEmail(ctx context.Context, email string) (store.User, error) {
	return r.q.GetUserByEmail(ctx, email)
}

func (r *storeRepository) GetByID(ctx context.Context, id uuid.UUID) (store.User, error) {
	return r.q.GetUserByID(ctx, id)
}
