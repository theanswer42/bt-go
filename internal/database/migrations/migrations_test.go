package migrations

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestMigrateUp_FreshDatabase(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Migrate up
	err := MigrateUp(db)
	if err != nil {
		t.Fatalf("MigrateUp() failed: %v", err)
	}

	// Verify tables were created
	tables := []string{"contents", "directories", "files", "file_snapshots", "schema_migrations"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("Table %s was not created: %v", table, err)
		}
	}
}

func TestCheckDBMigrationStatus_FreshDatabase(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Fresh database should need migration
	err := CheckDBMigrationStatus(db)
	if err == nil {
		t.Error("CheckDBMigrationStatus() expected error for fresh database, got nil")
	}

	// Error should mention needing migration
	if err.Error() != "database has no schema version (needs migration)" {
		t.Errorf("CheckDBMigrationStatus() error = %q, want error about needing migration", err.Error())
	}
}

func TestCheckDBMigrationStatus_AfterMigration(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Migrate up
	if err := MigrateUp(db); err != nil {
		t.Fatalf("MigrateUp() failed: %v", err)
	}

	// Status should be OK now
	err := CheckDBMigrationStatus(db)
	if err != nil {
		t.Errorf("CheckDBMigrationStatus() after migration returned error: %v", err)
	}
}

func TestMigrateUp_Idempotent(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Run migration twice
	if err := MigrateUp(db); err != nil {
		t.Fatalf("First MigrateUp() failed: %v", err)
	}

	if err := MigrateUp(db); err != nil {
		t.Errorf("Second MigrateUp() failed: %v (should be idempotent)", err)
	}

	// Status should still be OK
	if err := CheckDBMigrationStatus(db); err != nil {
		t.Errorf("CheckDBMigrationStatus() after double migration returned error: %v", err)
	}
}

func TestForeignKeyConstraints(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	// Migrate
	if err := MigrateUp(db); err != nil {
		t.Fatalf("MigrateUp() failed: %v", err)
	}

	// Try to insert a file with non-existent directory (should fail due to FK constraint)
	_, err := db.Exec(`
		INSERT INTO files (id, name, directory_id, deleted)
		VALUES ('file-1', 'test.txt', 'non-existent-dir', 0)
	`)

	if err == nil {
		t.Error("Expected foreign key constraint violation, but insert succeeded")
	}
}

func TestSchema_Contents(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := MigrateUp(db); err != nil {
		t.Fatalf("MigrateUp() failed: %v", err)
	}

	// Test inserting a content record
	checksum := "abc123def456"
	_, err := db.Exec("INSERT INTO contents (id, created_at) VALUES (?, datetime('now'))", checksum)
	if err != nil {
		t.Fatalf("Failed to insert content: %v", err)
	}

	// Verify it was inserted
	var id string
	err = db.QueryRow("SELECT id FROM contents WHERE id = ?", checksum).Scan(&id)
	if err != nil {
		t.Errorf("Failed to retrieve content: %v", err)
	}

	if id != checksum {
		t.Errorf("Retrieved content id = %q, want %q", id, checksum)
	}
}

func TestSchema_DirectoryPathUnique(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := MigrateUp(db); err != nil {
		t.Fatalf("MigrateUp() failed: %v", err)
	}

	// Insert first directory
	_, err := db.Exec("INSERT INTO directories (id, path, created_at) VALUES ('dir-1', '/test/path', datetime('now'))")
	if err != nil {
		t.Fatalf("Failed to insert first directory: %v", err)
	}

	// Try to insert duplicate path (should fail due to UNIQUE constraint)
	_, err = db.Exec("INSERT INTO directories (id, path, created_at) VALUES ('dir-2', '/test/path', datetime('now'))")
	if err == nil {
		t.Error("Expected unique constraint violation for duplicate path, but insert succeeded")
	}
}

// openTestDB opens an in-memory SQLite database for testing.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	return db
}
