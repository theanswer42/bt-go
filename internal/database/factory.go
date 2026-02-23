package database

import (
	"fmt"
	"path/filepath"

	"bt-go/internal/bt"
	"bt-go/internal/config"
)

// NewDatabaseFromConfig creates a Database implementation based on the database config type.
func NewDatabaseFromConfig(cfg config.DatabaseConfig, hostID string) (bt.Database, error) {
	switch cfg.Type {
	case "sqlite":
		if cfg.DataDir == "" {
			return nil, fmt.Errorf("data_dir required for sqlite database")
		}
		dbPath := filepath.Join(cfg.DataDir, hostID+".db")
		return NewSQLiteDatabase(dbPath, nil, nil)
	case "memory":
		return NewSQLiteDatabase(":memory:", nil, nil)
	default:
		return nil, fmt.Errorf("unknown database type: %s", cfg.Type)
	}
}
