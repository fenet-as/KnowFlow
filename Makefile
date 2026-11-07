.PHONY: all build run test vet lint migrate migrate-down migrate-status migrate-reset up down

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
	go test -v ./...

vet:
	go vet ./...

lint:
	@which golangci-lint > /dev/null 2>&1 && golangci-lint run ./... || go vet ./...

migrate:
	go run ./cmd/migrate -db "$(DATABASE_URL)" -dir "$(MIGRATIONS_DIR)" up

migrate-down:
	go run ./cmd/migrate -db "$(DATABASE_URL)" -dir "$(MIGRATIONS_DIR)" down

migrate-status:
	go run ./cmd/migrate -db "$(DATABASE_URL)" -dir "$(MIGRATIONS_DIR)" status

migrate-reset:
	go run ./cmd/migrate -db "$(DATABASE_URL)" -dir "$(MIGRATIONS_DIR)" reset

up:
	docker compose up -d

down:
	docker compose down
