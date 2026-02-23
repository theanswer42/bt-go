package testutil

import (
	"testing"

	"bt-go/internal/bt"
	"bt-go/internal/database"
)

// NewTestDatabase creates a new in-memory SQLite database with schema applied.
// The database is automatically closed when the test completes.
func NewTestDatabase(t *testing.T) bt.Database {
	t.Helper()

	sqlDB, err := database.OpenConnection(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	if _, err := sqlDB.Exec(database.Schema); err != nil {
		sqlDB.Close()
		t.Fatalf("failed to apply schema: %v", err)
	}

	db := database.NewSQLiteDatabaseFromDB(sqlDB, nil, nil)

	t.Cleanup(func() {
		db.Close()
	})

	return db
}
