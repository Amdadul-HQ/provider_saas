.PHONY: up down backend-dev worker-dev frontend-dev gen test lint migrate seed

up:            ## start postgres + redis
	docker compose up -d
down:
	docker compose down
backend-dev:   ## run API (auto-migrates)
	cd backend && go run ./cmd/api
worker-dev:    ## run background worker
	cd backend && go run ./cmd/worker
frontend-dev:
	cd frontend && pnpm dev
gen:           ## regenerate gqlgen + graphql-codegen
	cd backend && go run github.com/99designs/gqlgen generate
	cd frontend && pnpm codegen
migrate:
	cd backend && go run ./cmd/api migrate
seed:
	cd backend && go run ./cmd/api seed
test:
	cd backend && go vet ./... && go test ./...
	cd frontend && pnpm typecheck && pnpm lint
lint:
	cd backend && go vet ./...
	cd frontend && pnpm lint
