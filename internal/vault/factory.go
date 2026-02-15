package vault

import (
	"fmt"

	"bt-go/internal/bt"
	"bt-go/internal/config"
)

// NewVaultFromConfig creates a Vault implementation based on the vault config type.
func NewVaultFromConfig(cfg config.VaultConfig) (bt.Vault, error) {
	switch cfg.Type {
	case "memory":
		return NewMemoryVault(cfg.Name), nil
	case "s3":
		return nil, fmt.Errorf("s3 vault not yet implemented")
	case "filesystem":
		if cfg.FSVaultRoot == "" {
			return nil, fmt.Errorf("filesystem vault requires fs_vault_root to be set")
		}
		return NewFileSystemVault(cfg.Name, cfg.FSVaultRoot)
	default:
		return nil, fmt.Errorf("unknown vault type: %s", cfg.Type)
	}
}
