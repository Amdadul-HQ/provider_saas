.PHONY: up down db-setup backend-dev worker-dev frontend-dev gen test test-integration lint migrate seed

# Dev defaults (QA BUG-01): one committed dev VAULT_KEY for seed, API and worker. A local
# .env (copied from .env.example) overrides any of these. Without .env the defaults target
# Homebrew Postgres with the psaas_dev database (make db-setup).
-include .env
DEV_VAULT_KEY := 6465762d6f6e6c792d7661756c742d6b65792d646f2d6e6f742d7573652d2121
APP_ENV ?= development
VAULT_KEY ?= $(DEV_VAULT_KEY)
JWT_SECRET ?= change-me-32-bytes-minimum-secret-key
DATABASE_URL ?= postgres://psaas_app:psaas_app@localhost:5432/psaas_dev?sslmode=disable
MIGRATE_DATABASE_URL ?= postgres://$(USER)@localhost:5432/psaas_dev?sslmode=disable
export APP_ENV VAULT_KEY JWT_SECRET DATABASE_URL MIGRATE_DATABASE_URL

up:            ## start postgres + redis (docker; creates psaas_app on first init)
	docker compose up -d
down:
	docker compose down
db-setup:      ## Homebrew Postgres: create psaas_dev + the non-superuser API role psaas_app, then migrate
	createdb psaas_dev 2>/dev/null || true
	psql -v ON_ERROR_STOP=1 -d psaas_dev -f backend/scripts/dev-db-role.sql
	$(MAKE) migrate
backend-dev:   ## run API (auto-migrates as MIGRATE_DATABASE_URL in development)
	cd backend && go run ./cmd/api
worker-dev:    ## run background worker
	cd backend && go run ./cmd/worker
frontend-dev:
	cd frontend && pnpm dev
gen:           ## regenerate gqlgen + graphql-codegen
	cd backend && go run github.com/99designs/gqlgen generate
	cd frontend && pnpm codegen
migrate:       ## apply migrations as the table owner
	cd backend && go run ./cmd/api migrate
seed:          ## migrate + demo tenant (idempotent)
	cd backend && go run ./cmd/api seed
test:
	cd backend && go vet ./... && go test ./...
	cd frontend && pnpm typecheck && pnpm lint
test-integration: ## integration tests: migrate as owner, run as psaas_app
	cd backend && TEST_DATABASE_URL="$(MIGRATE_DATABASE_URL)" TEST_APP_DATABASE_URL="$(DATABASE_URL)" go test -tags integration -count=1 ./...
lint:
	cd backend && go vet ./...
	cd frontend && pnpm lint
