# Saydalah — Pharmacy Management API

A modular Go backend for a **multi-branch pharmacy chain**, covering inventory & batch/expiry
tracking, sales/POS, purchasing & suppliers, and prescriptions & customers. A SvelteKit frontend
consumes it over a clean JSON REST boundary (`/api/v1`).

## Stack

| Concern | Choice |
|---|---|
| Language | Go 1.26 (`log/slog`, `net/http`, `context`) |
| Router | `go-chi/chi/v5` |
| Database | PostgreSQL + `jackc/pgx/v5` (pooled) |
| Queries | `sqlc` → type-safe Go from SQL (`pgx/v5`) |
| Migrations | `pressly/goose/v3`, embedded, applied at startup |
| Auth | `golang-jwt/jwt/v5` + bcrypt, role-based |
| Validation | `go-playground/validator/v10` |
| Money | `shopspring/decimal` |

## Architecture

Modular monolith. Each domain under `internal/<module>/` has the **same shape** —
`routes.go`, `handler.go`, `service.go`, `repository.go`, `dto.go` — with a strict dependency
direction: `handler → service → repository → store (sqlc)`. HTTP concerns never leak into business
logic; SQL never leaks into handlers.

```
cmd/api/            composition root (wires config, pool, migrations, modules, server)
internal/
  config/           typed env config
  database/         pgx pool
  migrations/       embedded goose SQL, applied on boot
  store/            sqlc-generated queries + Tx helper (NewStore/Tx)
  server/           chi router, global middleware, /healthz + /readyz, /api/v1 group
  middleware/       request ID, structured logging, panic recovery, (auth/RBAC)
  httpx/            JSON encode/decode, error→status mapping, validation, pagination
  <modules>/        auth, user, branch, catalog, purchasing, inventory, sales, prescription
db/query/           *.sql query files consumed by sqlc
```

Transactions use `store.Store.Tx(ctx, func(q *store.Queries) error)` so multi-step operations
(e.g. a sale that decrements stock **and** writes an invoice **and** appends a movement ledger row)
commit or roll back atomically.

## Getting started

```bash
cp .env.example .env          # adjust as needed
make db-up                    # start local Postgres (docker compose)
make sqlc                     # (re)generate internal/store from SQL — needed after schema/query edits
make run                      # migrations apply at startup; API on :8080
```

Verify:

```bash
curl localhost:8080/healthz   # {"status":"ok"}
curl localhost:8080/readyz    # {"status":"ready"} once the DB is reachable
```

## Make targets

`make help` lists them: `db-up`, `db-down`, `sqlc`, `run`, `build`, `test`, `tidy`, `fmt`, `lint`,
`migrate-status`.

## Configuration

All config comes from the environment (see `.env.example`): `DATABASE_URL` and `JWT_SECRET` are
required; `APP_ENV`, `HTTP_ADDR`, `JWT_TTL`, `CORS_ORIGINS`, `SHUTDOWN_TIMEOUT` have defaults.
