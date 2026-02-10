SHELL := /bin/bash
ROOT_DIR := $(shell pwd)
BIN_DIR := $(ROOT_DIR)/bin

export PATH := $(BIN_DIR):$(PATH)

OAPI_CODEGEN_VERSION := v2.4.1
SQLC_VERSION := v1.27.0
GOOSE_VERSION := v3.22.1
OPENAPI_TS_VERSION := 7.6.1

.PHONY: tools gen gen-api gen-client test lint db-up db-down

tools:
	@mkdir -p $(BIN_DIR)
	GOBIN=$(BIN_DIR) go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION)
	GOBIN=$(BIN_DIR) go install github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION)
	GOBIN=$(BIN_DIR) go install github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION)

gen: tools gen-api gen-client

gen-api:
	cd apps/api && oapi-codegen --config oapi-types.cfg.yaml openapi.yaml
	cd apps/api && oapi-codegen --config oapi-server.cfg.yaml openapi.yaml
	cd apps/api && sqlc generate -f sqlc/sqlc.yaml

gen-client:
	cd packages/client && npm install
	cd packages/client && npx openapi-typescript@$(OPENAPI_TS_VERSION) ../../apps/api/openapi.yaml --output src/types.ts

test:
	cd apps/api && go test ./...

lint:
	cd apps/api && gofmt -w $$(find . -name '*.go' -not -path './internal/gen/*')

# Starts full local stack.
db-up:
	docker compose up --build

db-down:
	docker compose down -v
