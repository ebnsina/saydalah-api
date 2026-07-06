// Package migrations embeds the SQL migration files and applies them at
// startup so a freshly deployed binary always converges the schema without a
// separate migration step. The same *.sql files are the schema source for sqlc.
package migrations

import (
	"context"
	"embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var files embed.FS

// FS exposes the embedded migration files (used by tests and tooling).
func FS() embed.FS { return files }

// Up applies all pending migrations against the given pool. It opens a
// database/sql handle over the same pgx pool (goose speaks database/sql) so we
// do not open a second physical connection pool.
func Up(ctx context.Context, pool *pgxpool.Pool) error {
	goose.SetBaseFS(files)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("migrations: set dialect: %w", err)
	}

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("migrations: up: %w", err)
	}
	return nil
}
