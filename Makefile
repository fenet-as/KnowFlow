.PHONY: migrate migrate-down migrate-status migrate-reset test build run up down

DATABASE_URL ?= postgres://knowflow:knowflow@localhost:5432/knowflow?sslmode=disable
MIGRATIONS_DIR ?= migrations

migrate:
	go run ./cmd/migrate -db "$(DATABASE_URL)" -dir "$(MIGRATIONS_DIR)" up

migrate-down:
	go run ./cmd/migrate -db "$(DATABASE_URL)" -dir "$(MIGRATIONS_DIR)" down

migrate-status:
	go run ./cmd/migrate -db "$(DATABASE_URL)" -dir "$(MIGRATIONS_DIR)" status

migrate-reset:
	go run ./cmd/migrate -db "$(DATABASE_URL)" -dir "$(MIGRATIONS_DIR)" reset

test:
	go test -v ./...

build:
	go build ./...

run:
	go run ./cmd/api

up:
	docker compose up -d

down:
	docker compose down

