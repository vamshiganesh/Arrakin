.PHONY: help build run test sqlc migrate-up migrate-down migrate-create seed docker-up docker-down docker-logs tidy fmt vet lint swagger

APP_NAME := arrakin
CMD_DIR := ./cmd/arrakin
BIN_DIR := ./bin
DATABASE_URL ?= postgres://arrakin:arrakin@localhost:5432/arrakin?sslmode=disable
MIGRATIONS_DIR := file://migrations
SEED_FILE := seeds/001_demo_data.sql

help:
	@echo "Arrakin development targets:"
	@echo "  make docker-up     Start Postgres and Redis"
	@echo "  make docker-down   Stop infrastructure containers"
	@echo "  make migrate-up    Apply database migrations"
	@echo "  make migrate-down  Roll back last migration"
	@echo "  make seed          Load idempotent demo seed data"
	@echo "  make sqlc          Generate sqlc store code"
	@echo "  make build         Build the API binary"
	@echo "  make run           Run the API locally"
	@echo "  make test          Run unit tests (skips DB integration)"
	@echo "  make test-integration  Run Docker Postgres integration tests"
	@echo "  make tidy          go mod tidy"

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(APP_NAME) $(CMD_DIR)

run:
	go run $(CMD_DIR)

test:
	go test ./...

test-integration:
	go test ./internal/integration/... -v -count=1 -timeout 120s

sqlc:
	sqlc generate

swagger:
	swag init -g cmd/arrakin/main.go -o docs --parseDependency --parseInternal

migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1

seed:
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f $(SEED_FILE)

migrate-create:
	@read -p "Migration name: " name; \
	migrate create -ext sql -dir migrations -seq $$name

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

tidy:
	go mod tidy

fmt:
	go fmt ./...

vet:
	go vet ./...

lint: fmt vet
