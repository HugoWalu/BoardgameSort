# Board Game Night Planner — developer entry points.
# Run `make` (or `make help`) to list the targets.

SHELL   := /bin/bash
GO      ?= go
GOOSE   ?= goose
SQLC    ?= sqlc
BIN     ?= bin/boardgamesort
MIGRATIONS_DIR ?= internal/database/migrations

# The app reads .env.local itself (godotenv). goose does not, so pull the
# direct-connection URL out of the same file for migrations and dumps.
DIRECT_URL ?= $(shell sed -n 's/^DIRECT_URL=//p' .env.local 2>/dev/null | tr -d '"' | tail -n1)

.DEFAULT_GOAL := help
.PHONY: help dev build test test-race fmt fmt-check vet check tidy generate generate-check \
        migrate migrate-status migrate-down migrate-create seed clean

help: ## Show this help
	@awk 'BEGIN {FS = ":.*## "; printf "\nUsage: make <target>\n\n"} \
	/^[a-zA-Z0-9_.-]+:.*## / {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}' \
	$(MAKEFILE_LIST)

dev: ## Run the server on :8080 (loads .env.local)
	$(GO) run .

build: ## Build a static binary into bin/
	@mkdir -p $(dir $(BIN))
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o $(BIN) .

test: ## Run the test suite
	$(GO) test ./...

test-race: ## Run the test suite with the race detector
	$(GO) test -race ./...

fmt: ## Format all Go files
	$(GO) fmt ./...

fmt-check: ## Fail if any Go file is not gofmt'd
	@files="$$(gofmt -l .)"; \
	if [ -n "$$files" ]; then \
		echo "gofmt required for:"; echo "$$files"; exit 1; \
	fi

vet: ## Run go vet
	$(GO) vet ./...

check: fmt-check vet generate-check test build ## The gate: fmt + vet + sqlc + tests + build

tidy: ## Sync go.mod/go.sum with the imports
	$(GO) mod tidy

generate: ## Regenerate sqlc code from internal/database/queries
	$(SQLC) generate

generate-check: ## Fail if generated sqlc code is out of date
	$(SQLC) diff

migrate: ## Apply all pending migrations (uses DIRECT_URL)
	@test -n "$(DIRECT_URL)" || { echo "DIRECT_URL is not set in .env.local"; exit 1; }
	@mkdir -p $(MIGRATIONS_DIR)
	$(GOOSE) -dir $(MIGRATIONS_DIR) postgres "$(DIRECT_URL)" up

migrate-status: ## Show migration status
	@test -n "$(DIRECT_URL)" || { echo "DIRECT_URL is not set in .env.local"; exit 1; }
	$(GOOSE) -dir $(MIGRATIONS_DIR) postgres "$(DIRECT_URL)" status

migrate-down: ## Roll back the most recent migration
	@test -n "$(DIRECT_URL)" || { echo "DIRECT_URL is not set in .env.local"; exit 1; }
	$(GOOSE) -dir $(MIGRATIONS_DIR) postgres "$(DIRECT_URL)" down

migrate-create: ## Create a migration: make migrate-create name=add_games
	@test -n "$(name)" || { echo "usage: make migrate-create name=add_games"; exit 1; }
	@mkdir -p $(MIGRATIONS_DIR)
	$(GOOSE) -dir $(MIGRATIONS_DIR) create $(name) sql

seed: ## Create the dev group + host + invite link (cmd/seed)
	$(GO) run ./cmd/seed

clean: ## Remove build artifacts
	rm -rf bin
