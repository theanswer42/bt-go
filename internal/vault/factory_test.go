package vault

import (
	"testing"

	"bt-go/internal/config"
)

func TestNewVaultFromConfig(t *testing.T) {
	t.Run("memory vault", func(t *testing.T) {
		cfg := config.VaultConfig{
			Type: "memory",
			Name: "test-memory",
		}
		got, err := NewVaultFromConfig(cfg)
		if err != nil {
			t.Errorf("NewVaultFromConfig() error = %v", err)
			return
		}
		if got == nil {
			t.Error("NewVaultFromConfig() returned nil")
			return
		}
		if err := got.ValidateSetup(); err != nil {
			t.Errorf("ValidateSetup() error = %v", err)
		}
	})

	t.Run("filesystem vault", func(t *testing.T) {
		cfg := config.VaultConfig{
			Type:        "filesystem",
			Name:        "test-fs",
			FSVaultRoot: t.TempDir(),
		}
		got, err := NewVaultFromConfig(cfg)
		if err != nil {
			t.Errorf("NewVaultFromConfig() error = %v", err)
			return
		}
		if got == nil {
			t.Error("NewVaultFromConfig() returned nil")
			return
		}
		if err := got.ValidateSetup(); err != nil {
			t.Errorf("ValidateSetup() error = %v", err)
		}
	})

	t.Run("filesystem vault missing root", func(t *testing.T) {
		cfg := config.VaultConfig{
			Type: "filesystem",
			Name: "test-fs",
			// FSVaultRoot not set
		}
		_, err := NewVaultFromConfig(cfg)
		if err == nil {
			t.Error("NewVaultFromConfig() expected error for missing fs_vault_root")
		}
	})

	t.Run("s3 vault - not yet implemented", func(t *testing.T) {
		cfg := config.VaultConfig{
			Type:     "s3",
			Name:     "test-s3",
			S3Bucket: "my-bucket",
		}
		_, err := NewVaultFromConfig(cfg)
		if err == nil {
			t.Error("NewVaultFromConfig() expected error for unimplemented s3")
		}
	})

	t.Run("unknown vault type", func(t *testing.T) {
		cfg := config.VaultConfig{
			Type: "unknown",
			Name: "test-unknown",
		}
		_, err := NewVaultFromConfig(cfg)
		if err == nil {
			t.Error("NewVaultFromConfig() expected error for unknown type")
		}
	})
}
