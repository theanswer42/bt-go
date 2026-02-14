.PHONY: help generate-schema sqlc generate build test clean

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

generate-schema: ## Generate schema.sql from migration files
	@echo "Generating schema.sql from migrations..."
	@go run internal/database/tools/generate_schema.go

sqlc: generate-schema ## Generate sqlc code from schema.sql
	@echo "Generating sqlc code..."
	@sqlc generate -f internal/database/sqlc/sqlc.yaml

generate: sqlc ## Generate all code (schema + sqlc)

build: ## Build the bt binary
	@echo "Building bt..."
	@go build -o bin/bt ./cmd/bt

test: ## Run all tests
	@echo "Running tests..."
	@go test ./...

test-verbose: ## Run all tests with verbose output
	@echo "Running tests (verbose)..."
	@go test -v ./...

test-race: ## Run all tests with race detector
	@echo "Running tests with race detector..."
	@go test -race ./...

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf bin/
	@echo "Done."
