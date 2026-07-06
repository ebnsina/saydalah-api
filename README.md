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
Set `ADMIN_EMAIL` and `ADMIN_PASSWORD` to bootstrap the first admin on an empty database.

## Modules & endpoints

Mounted under `/api/v1`, all behind a JWT except `POST /auth/login`:

- `auth` — `POST /auth/login`, `GET /auth/me`
- `branches`, `users` — chain administration (manager/admin)
- `products`, `suppliers` — master catalog (read: all staff; write: manager/admin)
- `purchase-orders` — ordering + `POST /{id}/receive` (creates stock batches)
- `inventory` — `batches`, `near-expiry`, `low-stock`, `on-hand/{productID}`
- `stock` — `POST /stock/adjustments`, `POST /stock/returns`, `POST /stock/transfers` (inter-branch), `POST /stock/stock-takes` (physical count), `GET /stock/movements` (audit ledger)
- `sales` — `POST /sales` FEFO checkout, list/get
- `customers`, `prescriptions` — `POST /prescriptions/{id}/dispense` reuses FEFO
- `reports` — `sales-summary`, `sales-daily`, `inventory-valuation`, `top-products` (manager/admin)

The full contract is in [`api/openapi.yaml`](api/openapi.yaml).

## Quality

`make test` runs unit tests (FEFO dispensing, auth/JWT, tenant isolation, pagination).
`make test-integration` runs `-tags=integration` tests that spin up **real Postgres via
testcontainers** and drive the services end-to-end (goods receipt → FEFO sale + rollback →
inter-branch transfer → sale-linked return); it needs a running Docker daemon. CI
(`.github/workflows/ci.yml`) builds, vets, checks that `internal/store` is regenerated, runs both
unit and integration tests, and lints with golangci-lint (`.golangci.yml`).

> Using OrbStack/Colima instead of Docker Desktop? Point testcontainers at the socket, e.g.
> `export DOCKER_HOST=unix://$HOME/.orbstack/run/docker.sock`.
