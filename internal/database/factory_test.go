package database

import (
	"os"
	"path/filepath"
	"strings"
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

func TestCreateFromConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     func(dir string) config.DatabaseConfig
		setup   func(t *testing.T, dir string)
		wantErr string // substring; empty means success
		wantDB  bool
	}{
		{
			name:   "sqlite creates migrated database",
			cfg:    func(dir string) config.DatabaseConfig { return config.DatabaseConfig{Type: "sqlite", DataDir: dir} },
			wantDB: true,
		},
		{
			name: "memory is a no-op",
			cfg:  func(string) config.DatabaseConfig { return config.DatabaseConfig{Type: "memory"} },
		},
		{
			name:    "sqlite without data_dir",
			cfg:     func(string) config.DatabaseConfig { return config.DatabaseConfig{Type: "sqlite"} },
			wantErr: "data_dir required",
		},
		{
			name:    "unknown type",
			cfg:     func(string) config.DatabaseConfig { return config.DatabaseConfig{Type: "postgres"} },
			wantErr: "unknown database type",
		},
		{
			name: "refuses existing database file",
			cfg:  func(dir string) config.DatabaseConfig { return config.DatabaseConfig{Type: "sqlite", DataDir: dir} },
			setup: func(t *testing.T, dir string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, "host-1.db"), []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			},
			wantErr: "already exists",
		},
		{
			name: "missing data_dir directory fails and leaves nothing behind",
			cfg: func(dir string) config.DatabaseConfig {
				return config.DatabaseConfig{Type: "sqlite", DataDir: filepath.Join(dir, "missing")}
			},
			wantErr: "creating database",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.setup != nil {
				tt.setup(t, dir)
			}
			cfg := tt.cfg(dir)

			err := CreateFromConfig(cfg, "host-1")
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("CreateFromConfig() error = %v, want containing %q", err, tt.wantErr)
				}
				if tt.setup != nil {
					data, err := os.ReadFile(filepath.Join(dir, "host-1.db"))
					if err != nil || string(data) != "keep" {
						t.Errorf("existing database was changed (data=%q, err=%v)", data, err)
					}
				} else if cfg.DataDir != "" {
					if _, err := os.Stat(filepath.Join(cfg.DataDir, "host-1.db")); !os.IsNotExist(err) {
						t.Errorf("database file left behind (stat err = %v)", err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("CreateFromConfig() error = %v", err)
			}
			if !tt.wantDB {
				return
			}

			db, err := NewDatabaseFromConfig(cfg, "host-1")
			if err != nil {
				t.Fatalf("opening created database: %v", err)
			}
			defer db.Close()
			if err := db.CheckMigrations(); err != nil {
				t.Errorf("CheckMigrations() on created database = %v, want nil", err)
			}
		})
	}
}
