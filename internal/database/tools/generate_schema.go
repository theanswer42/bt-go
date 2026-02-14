// generate_schema generates schema.sql from migration files.
//
// This tool applies all migrations to an in-memory database and extracts
// the resulting schema to internal/database/sqlc/schema.sql.
//
// Usage (must run from project root):
//
//	go run internal/database/tools/generate_schema.go
//
// Or use the Makefile:
//
//	make generate-schema
package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"bt-go/internal/database"
	"bt-go/internal/database/migrations"
)

func main() {
	// Verify we're running from project root
	if _, err := os.Stat("go.mod"); err != nil {
		fmt.Fprintln(os.Stderr, "Error: must run from project root (where go.mod is)")
		fmt.Fprintln(os.Stderr, "Usage: go run internal/database/tools/generate_schema.go")
		os.Exit(1)
	}

	// Create in-memory database with proper SQLite configuration
	db, err := database.OpenConnection(":memory:")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Apply all migrations
	if err := migrations.MigrateUp(db); err != nil {
		fmt.Fprintf(os.Stderr, "Migration failed: %v\n", err)
		os.Exit(1)
	}

	// Extract schema from the migrated database
	schema, err := extractSchema(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to extract schema: %v\n", err)
		os.Exit(1)
	}

	// Output path (relative to project root)
	outPath := filepath.Join("internal", "database", "sqlc", "schema.sql")

	// Ensure output directory exists
	outDir := filepath.Dir(outPath)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output directory: %v\n", err)
		os.Exit(1)
	}

	// Write schema to file
	if err := os.WriteFile(outPath, []byte(schema), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write schema file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Generated %s from migrations\n", outPath)
}

// extractSchema extracts the SQL schema from the database.
// It queries sqlite_master for all CREATE statements, excluding:
// - SQLite internal tables (sqlite_*)
// - Migration tracking table (schema_migrations)
func extractSchema(db *sql.DB) (string, error) {
	query := `
		SELECT sql || ';'
		FROM sqlite_master
		WHERE type IN ('table', 'index')
		  AND name NOT LIKE 'sqlite_%'
		  AND name != 'schema_migrations'
		  AND tbl_name != 'schema_migrations'
		ORDER BY
		  CASE type
		    WHEN 'table' THEN 1
		    WHEN 'index' THEN 2
		  END,
		  name
	`

	rows, err := db.Query(query)
	if err != nil {
		return "", fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var schema string
	for rows.Next() {
		var sql string
		if err := rows.Scan(&sql); err != nil {
			return "", fmt.Errorf("scan failed: %w", err)
		}
		schema += sql + "\n\n"
	}

	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("rows error: %w", err)
	}

	// Add header comment
	header := `-- This file is auto-generated from migration files.
-- DO NOT EDIT MANUALLY. Run 'make generate-schema' to regenerate.
-- Source: internal/database/migrations/files/*.sql

`
	return header + schema, nil
}
