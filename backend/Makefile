# Backend tasks. Run from backend/.
#   make sqlc            regenerate type-safe query code (after editing sql/)
#   make seed-dev        load test stories + leaderboard users into the docker-compose DB
#   make assets-upload   upload the WebP images in frontend/public to the Supabase bucket
#   make migrate-new name=add_cases
#   make migrate-up | migrate-down | migrate-status

-include ../.env
export

MIGRATIONS := sql/migrations
GOOSE      ?= goose
SQLC       ?= sqlc

.DEFAULT_GOAL := help
.PHONY: help run-api run-worker emailctl-stats emailctl-list emailctl-requeue emailctl-purge docker-up docker-down docker-logs build test check sqlc sqlc-vet sqlc-diff migrate-new migrate-up migrate-down migrate-status seed-dev assets-upload

help: ## Show this help
	@awk -F':.*## ' '/^[a-zA-Z_-]+:.*## /{printf "  %-16s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

run-api: ## Run the API server
	go run ./cmd/api

run-worker: ## Run the email worker (needs Redis and the BREVO_* variables)
	go run ./cmd/worker

emailctl-stats: ## Email queue sizes (pending / dead-letter)
	go run ./cmd/emailctl stats

emailctl-list: ## List dead-lettered emails (codes masked)
	go run ./cmd/emailctl list

emailctl-requeue: ## Move dead-lettered emails back to the queue
	go run ./cmd/emailctl requeue

emailctl-purge: ## Delete all dead-lettered emails
	go run ./cmd/emailctl purge

docker-up: ## Build and start api + email-worker (+ infra); scale: WORKERS=3
	cd .. && docker compose --profile app up -d --build --scale email-worker=$(or $(WORKERS),1)

docker-down: ## Stop everything started by docker-up
	cd .. && docker compose --profile app down

docker-logs: ## Follow api and worker logs
	cd .. && docker compose --profile app logs -f api email-worker

build: ## Compile everything
	go build ./...

test: ## Run tests with the race detector
	go test -race ./...

check: sqlc-diff build ## What CI should run: generated code is current, build, vet, test
	go vet ./...
	go test ./...

sqlc-vet: ## Validate queries against the migrations
	$(SQLC) vet

sqlc: sqlc-vet ## Generate Go code from sql/queries
	$(SQLC) generate

sqlc-diff: ## Fail if generated code is out of date
	$(SQLC) diff

migrate-new: ## Create a migration: make migrate-new name=add_cases
	@test -n "$(name)" || (echo "usage: make migrate-new name=<snake_case>"; exit 1)
	$(GOOSE) -dir $(MIGRATIONS) -s create $(name) sql

migrate-up: ## Apply all pending migrations
	$(GOOSE) -dir $(MIGRATIONS) postgres "$(POSTGRES_URI)" up

migrate-down: ## Roll back the latest migration
	$(GOOSE) -dir $(MIGRATIONS) postgres "$(POSTGRES_URI)" down

migrate-status: ## Show applied / pending migrations
	$(GOOSE) -dir $(MIGRATIONS) postgres "$(POSTGRES_URI)" status

seed-dev: ## Load test stories and leaderboard users into the docker-compose DB (dev only)
	./sql/seed/seed-dev.sh

assets-upload: ## Upload WebP images in src= (default ../frontend/public) to the Supabase bucket, same paths
	go run ./cmd/assetctl upload-dir $(or $(src),../frontend/public)
