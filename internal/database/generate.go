package database

// This file documents code generation for the database package.
//
// To regenerate schema and sqlc code:
//   go generate ./internal/database
//
// Or use the Makefile:
//   make generate

//go:generate go run tools/generate_schema.go
//go:generate sqlc generate -f sqlc/sqlc.yaml
