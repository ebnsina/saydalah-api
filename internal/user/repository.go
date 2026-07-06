package user

import (
	"context"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// Repository is the persistence surface the user service depends on.
type Repository interface {
	Create(ctx context.Context, arg store.CreateUserParams) (store.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (store.User, error)
	GetByEmail(ctx context.Context, email string) (store.User, error)
	List(ctx context.Context, arg store.ListUsersParams) ([]store.User, error)
	Count(ctx context.Context) (int64, error)
	Update(ctx context.Context, arg store.UpdateUserParams) (store.User, error)
	SetPassword(ctx context.Context, id uuid.UUID, hash string) error
}

type storeRepository struct{ q *store.Store }

// NewRepository returns a Repository backed by the given store.
func NewRepository(s *store.Store) Repository { return &storeRepository{q: s} }

func (r *storeRepository) Create(ctx context.Context, arg store.CreateUserParams) (store.User, error) {
	return r.q.CreateUser(ctx, arg)
}

func (r *storeRepository) GetByID(ctx context.Context, id uuid.UUID) (store.User, error) {
	return r.q.GetUserByID(ctx, id)
}

func (r *storeRepository) GetByEmail(ctx context.Context, email string) (store.User, error) {
	return r.q.GetUserByEmail(ctx, email)
}

func (r *storeRepository) List(ctx context.Context, arg store.ListUsersParams) ([]store.User, error) {
	return r.q.ListUsers(ctx, arg)
}

func (r *storeRepository) Count(ctx context.Context) (int64, error) {
	return r.q.CountUsers(ctx)
}

func (r *storeRepository) SetPassword(ctx context.Context, id uuid.UUID, hash string) error {
	return r.q.SetUserPassword(ctx, store.SetUserPasswordParams{ID: id, PasswordHash: hash})
}

func (r *storeRepository) Update(ctx context.Context, arg store.UpdateUserParams) (store.User, error) {
	return r.q.UpdateUser(ctx, arg)
}
