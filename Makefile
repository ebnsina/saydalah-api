.DEFAULT_GOAL := help
.PHONY: help db-up db-down sqlc generate run build test tidy lint fmt migrate-status

# Load .env if present so `make run` picks up local config.
ifneq (,$(wildcard .env))
include .env
export
endif

GO      ?= go
SQLC    ?= go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest
GOOSE   ?= go run github.com/pressly/goose/v3/cmd/goose@latest

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

db-up: ## Start local Postgres
	docker compose up -d db

db-down: ## Stop local Postgres
	docker compose down

sqlc: ## Regenerate type-safe DB code from SQL
	$(SQLC) generate

generate: sqlc ## Run all code generation

run: ## Run the API (migrations apply automatically at startup)
	$(GO) run ./cmd/api

build: ## Compile the API binary to ./bin/api
	$(GO) build -o bin/api ./cmd/api

test: ## Run all tests
	$(GO) test ./... -race -count=1

tidy: ## Tidy go.mod / go.sum
	$(GO) mod tidy

fmt: ## Format code
	$(GO) fmt ./...

lint: ## Run golangci-lint (install: https://golangci-lint.run)
	golangci-lint run ./...

migrate-status: ## Show migration status (requires DATABASE_URL)
	$(GOOSE) -dir internal/migrations postgres "$(DATABASE_URL)" status
