package auth

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// Repository is the persistence surface the auth service needs: user lookups and
// server-side refresh-token storage. Keeping it here (rather than importing the
// user package) avoids an import cycle, since the user package depends on auth
// for password hashing.
type Repository interface {
	GetByEmail(ctx context.Context, email string) (store.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (store.User, error)

	CreateRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) (store.RefreshToken, error)
	GetRefreshToken(ctx context.Context, tokenHash string) (store.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id uuid.UUID) error
	RevokeUserRefreshTokens(ctx context.Context, userID uuid.UUID) error

	// Tx runs fn against a transaction-scoped Repository, committing on success.
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

func (r *storeRepository) GetByEmail(ctx context.Context, email string) (store.User, error) {
	return r.q.GetUserByEmail(ctx, email)
}

func (r *storeRepository) GetByID(ctx context.Context, id uuid.UUID) (store.User, error) {
	return r.q.GetUserByID(ctx, id)
}

func (r *storeRepository) CreateRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) (store.RefreshToken, error) {
	return r.q.CreateRefreshToken(ctx, store.CreateRefreshTokenParams{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	})
}

func (r *storeRepository) GetRefreshToken(ctx context.Context, tokenHash string) (store.RefreshToken, error) {
	return r.q.GetRefreshToken(ctx, tokenHash)
}

func (r *storeRepository) RevokeRefreshToken(ctx context.Context, id uuid.UUID) error {
	return r.q.RevokeRefreshToken(ctx, id)
}

func (r *storeRepository) RevokeUserRefreshTokens(ctx context.Context, userID uuid.UUID) error {
	return r.q.RevokeUserRefreshTokens(ctx, userID)
}
