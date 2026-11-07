package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"HostID", cfg.HostID, "host-1"},
		{"BaseDir", cfg.BaseDir, "/data/bt"},
		{"LogDir", cfg.LogDir, "/data/bt/log"},
		{"Encryption.Type", cfg.Encryption.Type, "age"},
		{"Encryption.PublicKeyPath", cfg.Encryption.PublicKeyPath, "/data/bt/keys/bt.pub"},
		{"Encryption.PrivateKeyPath", cfg.Encryption.PrivateKeyPath, "/data/bt/keys/bt.key"},
		{"Database.Type", cfg.Database.Type, "sqlite"},
		{"Database.DataDir", cfg.Database.DataDir, "/data/bt/data"},
		{"Staging.Type", cfg.Staging.Type, "filesystem"},
		{"Staging.StagingDir", cfg.Staging.StagingDir, "/data/bt/staging"},
		{"Staging.MaxSize", cfg.Staging.MaxSize, int64(1 << 20)},
		{"len(Vaults)", len(cfg.Vaults), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestNewFilesystemVaultConfig(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		root     string
		wantRoot string
	}{
		{name: "absolute path kept", root: "/backup/vault", wantRoot: "/backup/vault"},
		{name: "relative path made absolute", root: "vault", wantRoot: filepath.Join(wd, "vault")},
		{name: "path cleaned", root: "/backup//vault/", wantRoot: "/backup/vault"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewFilesystemVaultConfig(tt.root)
			if err != nil {
				t.Fatalf("NewFilesystemVaultConfig() error = %v", err)
			}
			want := VaultConfig{Type: "filesystem", Name: DefaultFSVaultName, FSVaultRoot: tt.wantRoot}
			if got != want {
				t.Errorf("NewFilesystemVaultConfig() = %+v, want %+v", got, want)
			}
		})
	}
}

func TestInit(t *testing.T) {
	t.Run("creates config file with mode 0600", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bt.toml")
		cfg := NewConfig("h1", dir)

		if err := Init(path, cfg); err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("config file not created: %v", err)
		}
		if got := info.Mode().Perm(); got != 0600 {
			t.Errorf("config file mode = %o, want 600", got)
		}
	})

	t.Run("creates missing parent directory with mode 0700", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "conf")
		path := filepath.Join(dir, "bt.toml")

		if err := Init(path, NewConfig("h1", dir)); err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("config dir not created: %v", err)
		}
		if got := info.Mode().Perm(); got != 0700 {
			t.Errorf("config dir mode = %o, want 700", got)
		}
	})

	t.Run("written config round-trips and appends vault examples", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bt.toml")
		cfg := NewConfig("h1", dir)
		cfg.Vaults = []VaultConfig{{Type: "filesystem", Name: "local", FSVaultRoot: "/v"}}

		if err := Init(path, cfg); err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if !strings.Contains(string(data), "# Example vault configurations") {
			t.Error("Init() did not append vault examples to config file")
		}
		got, err := Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if got.Staging.StagingDir != cfg.Staging.StagingDir || got.Vaults[0] != cfg.Vaults[0] {
			t.Errorf("Load() = %+v, want %+v", got, cfg)
		}
	})

	t.Run("refuses an existing file and leaves it untouched", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bt.toml")
		if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
			t.Fatal(err)
		}

		err := Init(path, NewConfig("h1", dir))
		if err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("Init() error = %v, want 'already exists'", err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "original" {
			t.Errorf("existing file was modified: %q", data)
		}
	})
}

func TestReadFromFile(t *testing.T) {
	t.Run("reads config without validating", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bt.toml")
		cfg := NewConfig("read-test", dir) // no vaults: invalid, but readable

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

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string // substring; empty means success
	}{
		{
			name: "valid config",
			content: `host_id = "h"
[database]
type = "memory"
[staging]
type = "memory"
[[vaults]]
type = "memory"
name = "m"
`,
		},
		{
			name: "invalid config names the file and key",
			content: `host_id = "h"
[database]
type = "memory"
[staging]
type = "filesystem"
[[vaults]]
type = "memory"
`,
			wantErr: "[staging] staging_dir is required",
		},
		{
			name:    "malformed toml",
			content: "host_id = ",
			wantErr: "decode",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "bt.toml")
			if err := os.WriteFile(path, []byte(tt.content), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(path)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Load() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Load() error = nil, want %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) || !strings.Contains(err.Error(), path) {
				t.Errorf("Load() error = %q, want it to contain %q and %q", err, tt.wantErr, path)
			}
		})
	}
}
