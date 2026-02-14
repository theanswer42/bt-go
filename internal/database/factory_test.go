package database

import (
	"testing"

	"bt-go/internal/config"
)

func TestNewDatabaseFromConfig(t *testing.T) {
	t.Run("memory database", func(t *testing.T) {
		cfg := config.DatabaseConfig{Type: "memory"}
		got, err := NewDatabaseFromConfig(cfg, "test-host-123")

		if err != nil {
			t.Errorf("NewDatabaseFromConfig() unexpected error: %v", err)
			return
		}

		if got == nil {
			t.Error("NewDatabaseFromConfig() returned nil")
		}

		if got != nil {
			got.Close()
		}
	})

	t.Run("sqlite database", func(t *testing.T) {
		cfg := config.DatabaseConfig{
			Type:    "sqlite",
			DataDir: t.TempDir(),
		}
		got, err := NewDatabaseFromConfig(cfg, "test-host-123")

		if err != nil {
			t.Errorf("NewDatabaseFromConfig() unexpected error: %v", err)
			return
		}

		if got == nil {
			t.Error("NewDatabaseFromConfig() returned nil")
		}

		if got != nil {
			got.Close()
		}
	})

	t.Run("sqlite database without data_dir", func(t *testing.T) {
		cfg := config.DatabaseConfig{Type: "sqlite"}
		got, err := NewDatabaseFromConfig(cfg, "test-host-123")

		if err == nil {
			t.Error("NewDatabaseFromConfig() expected error for missing data_dir, got nil")
		}

		if got != nil {
			t.Error("NewDatabaseFromConfig() should return nil on error")
			got.Close()
		}
	})

	t.Run("unknown database type", func(t *testing.T) {
		cfg := config.DatabaseConfig{Type: "unknown"}
		got, err := NewDatabaseFromConfig(cfg, "test-host-123")

		if err == nil {
			t.Error("NewDatabaseFromConfig() expected error for unknown type, got nil")
		}

		if got != nil {
			t.Error("NewDatabaseFromConfig() should return nil on error")
			got.Close()
		}
	})
}
