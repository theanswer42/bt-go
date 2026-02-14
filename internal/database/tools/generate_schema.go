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

	// Determine output path (relative to project root)
	outPath := filepath.Join("internal", "database", "sqlc", "schema.sql")

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
