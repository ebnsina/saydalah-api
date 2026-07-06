//go:build integration

// Package integration drives the feature services end-to-end against a real
// PostgreSQL instance started with testcontainers. Run with:
//
//	go test -tags=integration ./internal/integration/...
//
// It requires a working Docker daemon.
package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/ebnsina/saydalah-api/internal/database"
	"github.com/ebnsina/saydalah-api/internal/migrations"
	"github.com/ebnsina/saydalah-api/internal/store"
)

// pool is the shared connection pool to the containerized database, set up once
// in TestMain and reused by every test.
var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("saydalah"),
		tcpostgres.WithUsername("saydalah"),
		tcpostgres.WithPassword("saydalah"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "start postgres container:", err)
		os.Exit(1)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintln(os.Stderr, "connection string:", err)
		os.Exit(1)
	}

	pool, err = database.Connect(ctx, connStr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect pool:", err)
		os.Exit(1)
	}
	if err := migrations.Up(ctx, pool); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}

	code := m.Run()

	pool.Close()
	_ = container.Terminate(ctx)
	os.Exit(code)
}

// newStore returns a Store backed by the shared containerized pool.
func newStore() *store.Store { return store.NewStore(pool) }
