package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestManager_ReadWrite_RoundTrip(t *testing.T) {
	original := &Config{
		HostID:  "test-host-abc",
		BaseDir: "/home/user/.local/share/bt",
		LogDir:  "/home/user/.local/share/bt/log",
		Vaults: []VaultConfig{
			{Type: "filesystem", Name: "local", FSVaultRoot: "/backup/vault"},
		},
		Encryption: EncryptionConfig{
			PublicKeyPath:  "/home/user/.local/share/bt/keys/bt.pub",
			PrivateKeyPath: "/home/user/.local/share/bt/keys/bt.key",
		},
		Database: DatabaseConfig{Type: "sqlite", DataDir: "/home/user/.local/share/bt/db"},
		Staging:  StagingConfig{Type: "memory", MaxSize: 2048},
		Filesystem: FilesystemConfig{
			Ignore: []string{"*.log", ".git"},
		},
	}

	var buf bytes.Buffer
	m := &Manager{}

	if err := m.Write(&buf, original); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	got, err := m.Read(&buf)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if got.HostID != original.HostID {
		t.Errorf("HostID = %q, want %q", got.HostID, original.HostID)
	}
	if got.BaseDir != original.BaseDir {
		t.Errorf("BaseDir = %q, want %q", got.BaseDir, original.BaseDir)
	}
	if got.LogDir != original.LogDir {
		t.Errorf("LogDir = %q, want %q", got.LogDir, original.LogDir)
	}
	if len(got.Vaults) != 1 {
		t.Fatalf("len(Vaults) = %d, want 1", len(got.Vaults))
	}
	if got.Vaults[0].Type != "filesystem" {
		t.Errorf("Vault.Type = %q, want %q", got.Vaults[0].Type, "filesystem")
	}
	if got.Vaults[0].FSVaultRoot != "/backup/vault" {
		t.Errorf("Vault.FSVaultRoot = %q, want %q", got.Vaults[0].FSVaultRoot, "/backup/vault")
	}
	if got.Encryption.PublicKeyPath != original.Encryption.PublicKeyPath {
		t.Errorf("Encryption.PublicKeyPath = %q, want %q", got.Encryption.PublicKeyPath, original.Encryption.PublicKeyPath)
	}
	if got.Encryption.PrivateKeyPath != original.Encryption.PrivateKeyPath {
		t.Errorf("Encryption.PrivateKeyPath = %q, want %q", got.Encryption.PrivateKeyPath, original.Encryption.PrivateKeyPath)
	}
	if got.Database.Type != "sqlite" {
		t.Errorf("Database.Type = %q, want %q", got.Database.Type, "sqlite")
	}
	if got.Staging.MaxSize != 2048 {
		t.Errorf("Staging.MaxSize = %d, want %d", got.Staging.MaxSize, 2048)
	}
	if len(got.Filesystem.Ignore) != 2 {
		t.Fatalf("len(Filesystem.Ignore) = %d, want 2", len(got.Filesystem.Ignore))
	}
}

func TestNewConfig(t *testing.T) {
	cfg := NewConfig("host-1", "/data/bt")

	if cfg.HostID != "host-1" {
		t.Errorf("HostID = %q, want %q", cfg.HostID, "host-1")
	}
	if cfg.BaseDir != "/data/bt" {
		t.Errorf("BaseDir = %q, want %q", cfg.BaseDir, "/data/bt")
	}
	if cfg.LogDir != "/data/bt/log" {
		t.Errorf("LogDir = %q, want %q", cfg.LogDir, "/data/bt/log")
	}
	if cfg.Encryption.PublicKeyPath != "/data/bt/keys/bt.pub" {
		t.Errorf("Encryption.PublicKeyPath = %q, want %q", cfg.Encryption.PublicKeyPath, "/data/bt/keys/bt.pub")
	}
	if cfg.Encryption.PrivateKeyPath != "/data/bt/keys/bt.key" {
		t.Errorf("Encryption.PrivateKeyPath = %q, want %q", cfg.Encryption.PrivateKeyPath, "/data/bt/keys/bt.key")
	}
}

func TestInit(t *testing.T) {
	t.Run("creates config file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bt.toml")
		cfg := NewConfig("h1", dir)

		if err := Init(path, cfg); err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		if _, err := os.Stat(path); err != nil {
			t.Fatalf("config file not created: %v", err)
		}
	})

	t.Run("fails if file already exists", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bt.toml")
		cfg := NewConfig("h1", dir)

		if err := Init(path, cfg); err != nil {
			t.Fatalf("first Init() error = %v", err)
		}

		err := Init(path, cfg)
		if err == nil {
			t.Fatal("second Init() expected error")
		}
	})
}

func TestReadFromFile(t *testing.T) {
	t.Run("reads valid config", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bt.toml")
		cfg := NewConfig("read-test", dir)
		cfg.Database = DatabaseConfig{Type: "memory"}

		if err := Init(path, cfg); err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		got, err := ReadFromFile(path)
		if err != nil {
			t.Fatalf("ReadFromFile() error = %v", err)
		}
		if got.HostID != "read-test" {
			t.Errorf("HostID = %q, want %q", got.HostID, "read-test")
		}
	})

	t.Run("returns error for missing file", func(t *testing.T) {
		_, err := ReadFromFile("/nonexistent/path/bt.toml")
		if err == nil {
			t.Fatal("ReadFromFile() expected error for missing file")
		}
	})
}
