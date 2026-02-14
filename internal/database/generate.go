package database

// This file documents code generation for the database package.
//
// To regenerate schema and sqlc code:
//   go generate ./internal/database
//
// Or use the Makefile:
//   make generate

//go:generate sh -c "cd ../.. && go run internal/database/tools/generate_schema.go"
//go:generate sh -c "cd ../.. && sqlc generate -f internal/database/sqlc/sqlc.yaml"
