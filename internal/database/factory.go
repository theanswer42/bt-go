package database

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"bt-go/internal/bt"
	"bt-go/internal/config"
	"bt-go/internal/database/migrations"
)

// NewDatabaseFromConfig creates a Database implementation based on the database config type.
func NewDatabaseFromConfig(cfg config.DatabaseConfig, hostID string) (bt.Database, error) {
	switch cfg.Type {
	case "sqlite":
		if cfg.DataDir == "" {
			return nil, fmt.Errorf("data_dir required for sqlite database")
		}
		return NewSQLiteDatabase(sqlitePath(cfg.DataDir, hostID), nil, nil)
	case "memory":
		return NewSQLiteDatabase(":memory:", nil, nil)
	default:
		return nil, fmt.Errorf("unknown database type: %s", cfg.Type)
	}
}

// sqlitePath returns the location of the host's SQLite database in dataDir.
func sqlitePath(dataDir, hostID string) string {
	return filepath.Join(dataDir, hostID+".db")
}

// CreateFromConfig creates a new, fully migrated database for hostID. It is
// called once by `bt config init`. For sqlite it refuses to touch an existing
// database file and, if migration fails, removes the files it created. For
// memory it does nothing, since a memory database lives only for one run.
func CreateFromConfig(cfg config.DatabaseConfig, hostID string) (err error) {
	switch cfg.Type {
	case "memory":
		return nil
	case "sqlite":
	default:
		return fmt.Errorf("unknown database type: %s", cfg.Type)
	}
	if cfg.DataDir == "" {
		return fmt.Errorf("data_dir required for sqlite database")
	}

	path := sqlitePath(cfg.DataDir, hostID)
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("database %s already exists", path)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("checking database %s: %w", path, err)
	}

	defer func() {
		if err == nil {
			return
		}
		// Best effort: remove the database and any SQLite side files so a
		// later init starts clean. The original error is what matters.
		for _, suffix := range []string{"", "-journal", "-wal", "-shm"} {
			_ = os.Remove(path + suffix)
		}
	}()

	db, err := OpenConnection(path)
	if err != nil {
		return fmt.Errorf("creating database %s: %w", path, err)
	}
	if err := migrations.MigrateUp(db); err != nil {
		db.Close()
		return fmt.Errorf("migrating database %s: %w", path, err)
	}
	if err := db.Close(); err != nil {
		return fmt.Errorf("closing database %s: %w", path, err)
	}
	return nil
}
