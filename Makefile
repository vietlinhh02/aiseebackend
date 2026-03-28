.PHONY: dev stop db-up db-down migrate migrate-down generate test lint build clean logs

# === Development ===

dev: db-up migrate ## Start everything for development
	@echo "Starting backend server..."
	@source .env 2>/dev/null; \
	DATABASE_URL=$${DATABASE_URL:-postgres://aisee:aisee@localhost:5432/aisee?sslmode=disable} \
	JWT_SECRET=$${JWT_SECRET:-your-jwt-secret-min-32-chars-long} \
	INTERNAL_API_KEY=$${INTERNAL_API_KEY:-your-internal-api-key} \
	LIVEKIT_URL=$${LIVEKIT_URL:-ws://localhost:7880} \
	LIVEKIT_API_KEY=$${LIVEKIT_API_KEY:-devkey} \
	LIVEKIT_API_SECRET=$${LIVEKIT_API_SECRET:-devsecret} \
	PORT=$${PORT:-8080} \
	go run ./cmd/server/

stop: ## Stop all Docker services
	docker compose down

# === Docker ===

db-up: ## Start PostgreSQL, Redis, LiveKit
	docker compose up -d postgres redis livekit
	@echo "Waiting for PostgreSQL..."
	@until docker compose exec -T postgres pg_isready -U aisee > /dev/null 2>&1; do sleep 1; done
	@echo "PostgreSQL ready."

db-down: ## Stop Docker services
	docker compose down

# === Database ===

migrate: ## Run database migrations
	@command -v migrate > /dev/null 2>&1 || { echo "Install: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest"; exit 1; }
	migrate -path db/migrations -database "postgres://aisee:aisee@localhost:5432/aisee?sslmode=disable" up

migrate-down: ## Rollback all migrations
	migrate -path db/migrations -database "postgres://aisee:aisee@localhost:5432/aisee?sslmode=disable" down -all

migrate-create: ## Create new migration (usage: make migrate-create name=add_something)
	migrate create -ext sql -dir db/migrations -seq $(name)

# === Code Generation ===

generate: ## Run sqlc codegen
	@command -v sqlc > /dev/null 2>&1 || { echo "Install: go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest"; exit 1; }
	sqlc generate
	@echo "Generated internal/dbgen/"

# === Testing ===

test: ## Run all tests
	go test ./... -v

test-short: ## Run tests (short output)
	go test ./...

test-cover: ## Run tests with coverage
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out
	@rm coverage.out

# === Build ===

build: ## Build binary
	CGO_ENABLED=0 go build -o bin/server ./cmd/server/

build-docker: ## Build all Docker images
	docker compose build

# === Quality ===

lint: ## Run linter (requires golangci-lint)
	@command -v golangci-lint > /dev/null 2>&1 || { echo "Install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; exit 1; }
	golangci-lint run ./...

fmt: ## Format code
	gofmt -s -w .

# === Agent Worker ===

agent-setup: ## Setup Python agent worker
	cd agent_worker && uv venv && uv pip install -e .

agent-dev: ## Run agent worker in dev mode
	cd agent_worker && \
	LIVEKIT_URL=$${LIVEKIT_URL:-ws://localhost:7880} \
	LIVEKIT_API_KEY=$${LIVEKIT_API_KEY:-devkey} \
	LIVEKIT_API_SECRET=$${LIVEKIT_API_SECRET:-devsecret} \
	GO_BACKEND_URL=$${GO_BACKEND_URL:-http://localhost:8080/api/internal} \
	INTERNAL_API_KEY=$${INTERNAL_API_KEY:-your-internal-api-key} \
	GEMINI_API_KEY=$${GEMINI_API_KEY} \
	python main.py dev

# === Full Stack ===

up: ## Start everything with Docker Compose (all 5 services)
	docker compose up --build -d

down: ## Stop everything
	docker compose down

logs: ## Tail logs from all services
	docker compose logs -f

# === Utilities ===

psql: ## Connect to PostgreSQL
	docker compose exec postgres psql -U aisee

redis-cli: ## Connect to Redis
	docker compose exec redis redis-cli

clean: ## Remove build artifacts
	rm -rf bin/ coverage.out

# === Help ===

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
