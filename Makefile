SHELL := /bin/bash

.PHONY: help lint format deps test build install setup db-up db-down db-reset migrate migrate-alembic dev

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

# --- Lint & Format ---

lint: ## Run all linters (ruff + golangci-lint)
	./scripts/lint.sh

lint-go: ## Run golangci-lint only
	cd cli/src && golangci-lint run ./...

format: ## Auto-format all code (ruff + gofmt)
	./scripts/format.sh

# --- Dependencies ---

deps: ## Install all dependencies (python + go)
	./scripts/deps.sh

# --- Docs ---

docs-schema: ## Generate schema docs from SQLAlchemy models
	cd server && PYTHONPATH=./src uv run python ../scripts/generate_schema_docs.py

changelog: ## Generate changelog from conventional commits
	git-cliff -o docs/CHANGELOG.md

# --- Testing ---

test: ## Run all tests (server + cli)
	cd server && uv run pytest -q
	cd cli/src && go test ./... -count=1

test-server: ## Run server tests only
	cd server && uv run pytest -q

test-cli: ## Run CLI tests only
	cd cli/src && go test ./... -count=1

# --- Build ---

build: ## Build CLI binary
	cd cli/src && go build -o build/nebula ./cmd/nebula

# --- Database ---

db-up: ## Start postgres container
	docker compose up -d postgres

db-down: ## Stop all containers
	docker compose down

db-reset: ## Full database reset (destroy + recreate)
	docker compose down
	rm -rf database/data
	docker compose up -d postgres
	@echo "waiting for postgres..."
	@sleep 3
	@echo "db reset complete"

migrate: ## Run database migrations
	./scripts/migrate.sh

migrate-alembic: ## Run alembic migrations
	cd server && PYTHONPATH=./src uv run alembic upgrade head

# --- Dev ---

dev: ## Start API server with reload
	cd server && PYTHONPATH=./src .venv/bin/uvicorn nebula_api.app:app --host 127.0.0.1 --port 8765 --reload

# --- Setup ---

install: ## Install git hooks + deps
	./scripts/deps.sh
	cp scripts/pre-commit .git/hooks/pre-commit
	chmod +x .git/hooks/pre-commit
	@echo "hooks installed"

setup: install db-up migrate ## Full setup from fresh clone
	@echo "setup complete"
