.PHONY: all build run test test-integration test-coverage vet lint migrate migrate-up migrate-down migrate-status migrate-reset benchmark eval up down

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || echo "unknown")

LDFLAGS = -X 'github.com/fenet-as/KnowFlow/internal/platform.Version=$(VERSION)' \
          -X 'github.com/fenet-as/KnowFlow/internal/platform.Commit=$(COMMIT)' \
          -X 'github.com/fenet-as/KnowFlow/internal/platform.BuildTime=$(BUILD_TIME)'

DATABASE_URL ?= postgres://knowflow:knowflow@localhost:5433/knowflow?sslmode=disable
MIGRATIONS_DIR ?= migrations

all: lint test build

build:
	go build -ldflags="$(LDFLAGS)" -o bin/api ./cmd/api

run:
	go run -ldflags="$(LDFLAGS)" ./cmd/api

test:
	go test -v -p 1 -cover ./...

test-integration:
	go test -v -p 1 ./test/integration/...

test-coverage:
	go test -v -p 1 -coverprofile=coverage.out ./... && go tool cover -func=coverage.out

vet:
	go vet ./...

lint:
	@which golangci-lint > /dev/null 2>&1 && golangci-lint run ./... || go vet ./...

migrate:
	go run ./cmd/migrate -db "$(DATABASE_URL)" -dir "$(MIGRATIONS_DIR)" up

migrate-up: migrate

migrate-down:
	go run ./cmd/migrate -db "$(DATABASE_URL)" -dir "$(MIGRATIONS_DIR)" down

migrate-status:
	go run ./cmd/migrate -db "$(DATABASE_URL)" -dir "$(MIGRATIONS_DIR)" status

migrate-reset:
	go run ./cmd/migrate -db "$(DATABASE_URL)" -dir "$(MIGRATIONS_DIR)" reset

benchmark:
	go run ./cmd/benchmark -db "$(DATABASE_URL)"

eval:
	go run ./cmd/eval -corpus "demo/corpus" -k 6 -threshold 0.65

up:
	docker compose up -d

down:
	docker compose down
