SHELL := /bin/bash

.PHONY: run migrate-up migrate-down seed

run:
	go run ./cmd/server

migrate-up:
	goose -dir ./migrations postgres "$$DATABASE_URL" up

migrate-down:
	goose -dir ./migrations postgres "$$DATABASE_URL" down

seed:
	go run ./cmd/seed
