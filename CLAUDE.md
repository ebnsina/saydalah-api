# Saydalah API — working notes

Go 1.26 modular-monolith backend for a multi-branch pharmacy chain. See `README.md` for the feature
list, endpoints, and getting-started. This file is the conventions cheat-sheet.

## Architecture
- Every domain lives in `internal/<module>/` with the **same shape**: `routes.go`, `handler.go`,
  `service.go`, `repository.go`, `dto.go`. Strict direction: **handler → service → repository →
  store (sqlc)**. HTTP concerns never leak into services; SQL never into handlers.
- One shared generated package `internal/store` (sqlc over pgx/v5). **Regenerate after editing
  `db/query/*.sql` or a migration**: `make sqlc`. Never hand-edit `internal/store`.
- Multi-step writes are atomic via `store.Store.Tx(ctx, fn)` + the module's `Repository.Tx`
  (e.g. a sale decrements stock **and** writes the sale **and** appends a movement row, or none).
- Migrations: `internal/migrations/NNNN_*.sql` (goose, embedded, applied at startup). New file = next
  number. A Postgres `ALTER TYPE ... ADD VALUE` needs its **own** migration with `-- NO TRANSACTION`.
- Errors: return sentinels (`httpx.ErrNotFound`, `ErrConflict`, `ErrInvalidInput`,
  `ErrInsufficientStock`, `ErrForbidden`, …) or a typed `httpx.APIError{Status,Message,Err}` with
  `Unwrap`; `httpx.Error` maps them to a status. Handlers never build status codes.
- Money is `shopspring/decimal` — never float.
- **Config is fail-fast**: an unset var uses its default, but a malformed/out-of-range override
  fails startup with a combined error. Add fields in `internal/config`.

## Optional Redis (`REDIS_URL`)
`internal/cache` is **nil-safe** — everything works without Redis. When set it backs:
- **Shared rate limiting** (global + login), so multiple API instances enforce one limit.
- **Report cache** (60s TTL) with **instant invalidation**: reads fold a per-branch version into the
  key; sales create/void/payment, stock adjust/transfer/return, and PO receive call
  `cache.Bump(ctx, branchID…)`. After any report-affecting write, Bump the branch(es).

## Conventions
- **Reads** are open to any authenticated staff; **writes** gated by `middleware.RequireRole(...)`.
- Branch scoping: `id.ResolveBranch(requested)` (admin may pass a branch; others get their own) and
  `id.CanAccessBranch(...)`.
- **List endpoints must populate nested `items`** (the frontend renders them) — batch-load them in
  one query (`WHERE id = ANY($1)`), not per-row.

## Commands
- `make run` (migrations auto-apply, API on :8080), `make test`, `make test-integration`
  (testcontainers — needs Docker), `make sqlc`, `make fmt`, `make lint`, `make db-up`.
- OrbStack/Colima: `export DOCKER_HOST=unix://$HOME/.orbstack/run/docker.sock`.

## Git
- Commit author `ebnsina <ebnsina.me@gmail.com>`. **Do NOT add `Co-Authored-By` or any other
  trailer/identity.**
- Remotes use the `github-es` SSH alias. `docs/` and `data/` are gitignored.
- One feature per commit. Commit/push only when asked; branch off the default branch first.
