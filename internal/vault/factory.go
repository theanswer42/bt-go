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
		if cfg.S3Bucket == "" {
			return nil, fmt.Errorf("s3 vault requires s3_bucket")
		}
		if cfg.S3Region == "" {
			return nil, fmt.Errorf("s3 vault requires s3_region")
		}
		if cfg.S3AccessKeyID == "" || cfg.S3SecretAccessKey == "" {
			return nil, fmt.Errorf("s3 vault requires s3_access_key_id and s3_secret_access_key")
		}
		return NewS3Vault(cfg.Name, cfg.S3Bucket, cfg.S3ContentPrefix, cfg.S3MetadataPrefix,
			cfg.S3Region, cfg.S3AccessKeyID, cfg.S3SecretAccessKey)
	case "filesystem":
		if cfg.FSVaultRoot == "" {
			return nil, fmt.Errorf("filesystem vault requires fs_vault_root to be set")
		}
		return NewFileSystemVault(cfg.Name, cfg.FSVaultRoot)
	default:
		return nil, fmt.Errorf("unknown vault type: %s", cfg.Type)
	}
}
