# Saydalah — Pharmacy Management API

A modular Go backend for a **multi-branch pharmacy chain**, covering inventory & batch/expiry
tracking, sales/POS, purchasing & suppliers, and prescriptions & customers. A SvelteKit frontend
consumes it over a clean JSON REST boundary (`/api/v1`).

## Features

- **Multi-branch chain** — a shared product catalog with per-branch stock; every inventory, sales,
  and purchasing query is branch-scoped, and admins can act across branches.
- **Auth & RBAC** — JWT access tokens with **rotating refresh tokens** (reuse/theft detection),
  bcrypt password hashing, role-based access (`admin`/`manager`/`pharmacist`/`cashier`), plus
  self-service password change and admin password reset.
- **Catalog & suppliers** — product master data (with barcode lookup for POS) and supplier records.
- **Purchasing** — purchase orders and **goods receipt**, which creates expiry-dated stock batches.
- **Inventory** — per-branch stock, batch/expiry tracking, and **near-expiry** & **low-stock**
  (reorder) alerts.
- **Sales / POS** — **FEFO** (first-expiry-first-out) checkout in a single transaction, with
  **tax/VAT**, discounts, **partial payment / on-account credit**, invoices, and **void**
  (refund + restore stock). Per-branch **on-hand** is returned on the product list so the POS can
  show availability before checkout. Settle outstanding balances with `POST /sales/{id}/payment`.
- **Prescriptions & customers** — customer records and prescription **dispensing** (reuses FEFO).
- **Manual stock operations** — adjustments, returns, **supplier (purchase) returns**,
  **inter-branch transfers**, and physical **stock-takes**.
- **Audit ledger** — every stock change writes an append-only `stock_movements` row (type, qty,
  reference, and the acting user) in the same transaction.
- **Reporting** — sales summary, daily sales, **sales by payment method**, inventory valuation, and
  top products.
- **Operational** — fail-fast typed config, per-IP rate limiting (stricter on login), structured
  request-scoped logging, graceful shutdown, and `/healthz` + `/readyz` probes.
- **Optional Redis** (`REDIS_URL`) — when set, rate limiting is **shared across instances** and
  report responses are **cached** (60s) with **instant per-branch invalidation** on any
  report-affecting write. Nil-safe: the API runs fully without it.

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
  httpx/            JSON encode/decode, error→status mapping, validation, pagination, ETag caching
  cache/            optional Redis read-cache + rate-limit backend (nil-safe)
  <modules>/        auth, user, branch, catalog, supplier, purchasing, inventory, stock,
                    sales, customer, prescription, reporting
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

## Docker / deployment

The service is packaged as a **26 MB distroless** image (static binary, non-root). Migrations are
embedded and applied at startup, so the container is fully self-contained.

```bash
make docker-build                 # build the image (saydalah-api:latest)

# Run the whole stack (API + Postgres) in containers — the API talks to the db
# service over the compose network, so it is unaffected by anything on host :5432.
JWT_SECRET=... ADMIN_EMAIL=... ADMIN_PASSWORD=... make up
curl localhost:8080/readyz
make down
```

In production, supply `DATABASE_URL`, `JWT_SECRET`, and the other env vars from your platform's
secret store; point liveness at `/healthz` and readiness at `/readyz`.

## Make targets

`make help` lists them: `db-up`, `db-down`, `sqlc`, `run`, `build`, `test`, `tidy`, `fmt`, `lint`,
`migrate-status`.

## Configuration

All config comes from the environment (see `.env.example`): `DATABASE_URL` and `JWT_SECRET` are
required; `APP_ENV`, `HTTP_ADDR`, `JWT_TTL`, `CORS_ORIGINS`, `SHUTDOWN_TIMEOUT`,
`RATE_LIMIT_RPS`/`RATE_LIMIT_BURST`, and `LOGIN_RATE_RPS`/`LOGIN_RATE_BURST` have defaults.
Set `ADMIN_EMAIL` and `ADMIN_PASSWORD` to bootstrap the first admin on an empty database.
`TAX_RATE` sets the sales VAT fraction (e.g. `0.15`). `REDIS_URL` is optional — when set it enables
shared (cross-instance) rate limiting and the report cache; when unset or unreachable the API logs a
warning and runs degraded (in-memory limiter, no cache) rather than failing to start.

Config loading is **fail-fast**: an unset variable uses its default, but a variable set to a
malformed or out-of-range value makes startup fail with a combined error — a bad override never
silently reverts to a default.

Requests are rate-limited per client IP (429 + `Retry-After`), with a stricter limiter on
`POST /auth/login`.

## Modules & endpoints

Mounted under `/api/v1`, all behind a JWT except `POST /auth/login`:

- `auth` — `POST /auth/login`, `POST /auth/refresh` (rotating), `POST /auth/logout`, `GET /auth/me`
- `branches`, `users` — chain administration (manager/admin)
- `products`, `suppliers` — master catalog (read: all staff; write: manager/admin);
  `GET /products/barcode/{code}` for POS scan lookup; `GET /products?branch_id=…` includes per-branch `on_hand`
- `purchase-orders` — ordering + `POST /{id}/receive` (creates stock batches)
- `inventory` — `batches`, `near-expiry`, `low-stock`, `on-hand/{productID}`
- `stock` — `POST /stock/adjustments`, `POST /stock/returns`, `POST /stock/purchase-returns` (to supplier), `POST /stock/transfers` (inter-branch), `POST /stock/stock-takes` (physical count), `GET /stock/movements` (audit ledger)
- `sales` — `POST /sales` FEFO checkout (tax, discount, on-account), list/get, `POST /sales/{id}/payment` (settle balance), `POST /sales/{id}/void` (refund + restore stock)
- `customers`, `prescriptions` — `POST /prescriptions/{id}/dispense` reuses FEFO
- `reports` — `sales-summary`, `sales-daily`, `sales-by-payment`, `inventory-valuation`, `top-products` (manager/admin); cached when Redis is configured

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
