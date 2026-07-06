package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store is the hand-written entry point to the generated query layer. It embeds
// *Queries so callers can run single queries directly (store.GetBranch(...)),
// and adds Tx for running several queries atomically. This file is NOT
// generated, so `sqlc generate` never overwrites it.
type Store struct {
	*Queries
	pool *pgxpool.Pool
}

// NewStore wraps a pgx pool in a Store. The pool satisfies the generated DBTX
// interface, so non-transactional queries run directly against it.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{Queries: New(pool), pool: pool}
}

// Tx runs fn within a single database transaction, passing it a *Queries bound
// to that transaction. It commits if fn returns nil and rolls back otherwise,
// so a partially-applied operation (e.g. a sale that decrements stock but fails
// to write the invoice) never persists.
func (s *Store) Tx(ctx context.Context, fn func(q *Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op once committed

	if err := fn(s.Queries.WithTx(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: commit tx: %w", err)
	}
	return nil
}
