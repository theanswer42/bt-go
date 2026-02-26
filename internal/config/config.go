package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config represents the main configuration for bt.
type Config struct {
	HostID     string           `toml:"host_id"`
	BaseDir    string           `toml:"base_dir"`
	LogDir     string           `toml:"log_dir"`
	Vaults     []VaultConfig    `toml:"vaults"`
	Encryption EncryptionConfig `toml:"encryption"`
	Database   DatabaseConfig   `toml:"database"`
	Staging    StagingConfig    `toml:"staging"`
	Filesystem FilesystemConfig `toml:"filesystem"`
}

// EncryptionConfig holds paths to the age key pair used for encryption.
type EncryptionConfig struct {
	Type           string `toml:"type"`             // "age" (default) or "test"
	PublicKeyPath  string `toml:"public_key_path"`
	PrivateKeyPath string `toml:"private_key_path"`
}

// FilesystemConfig holds filesystem-related settings.
type FilesystemConfig struct {
	Ignore []string `toml:"ignore"`
}

// VaultConfig represents configuration for a vault backend.
// This uses a tagged union pattern - the Type field determines which other fields are relevant.
type VaultConfig struct {
	Type string `toml:"type"` // "memory", "s3", or "filesystem"
	Name string `toml:"name"`

	// S3-specific fields (only used when Type == "s3")
	S3Bucket string `toml:"s3_bucket,omitempty"`
	S3Prefix string `toml:"s3_prefix,omitempty"`
	S3Region string `toml:"s3_region,omitempty"`

	// FileSystem-specific fields (only used when Type == "filesystem")
	FSVaultRoot string `toml:"fs_vault_root,omitempty"`
}

// DatabaseConfig represents configuration for the metadata database.
// This uses a tagged union pattern - the Type field determines which other fields are relevant.
type DatabaseConfig struct {
	Type    string `toml:"type"`               // "sqlite" or "memory"
	DataDir string `toml:"data_dir,omitempty"` // only used for type=sqlite
}

// StagingConfig represents configuration for the staging area.
// This uses a tagged union pattern - the Type field determines which other fields are relevant.
type StagingConfig struct {
	Type       string `toml:"type"`                  // "memory" or "filesystem"
	StagingDir string `toml:"staging_dir,omitempty"` // only used for type=filesystem
	MaxSize    int64  `toml:"max_size"`              // max total size in bytes; must be positive, defaults to 1MB
}

// NewConfig creates a new Config with the provided values and default key paths.
func NewConfig(hostID, baseDir string) *Config {
	return &Config{
		HostID:  hostID,
		BaseDir: baseDir,
		LogDir:  filepath.Join(baseDir, "log"),
		Encryption: EncryptionConfig{
			PublicKeyPath:  filepath.Join(baseDir, "keys", "bt.pub"),
			PrivateKeyPath: filepath.Join(baseDir, "keys", "bt.key"),
		},
	}
}

// Manager handles reading and writing configuration.
type Manager struct{}

// Read decodes a Config from the provided reader.
func (m *Manager) Read(r io.Reader) (*Config, error) {
	var cfg Config
	if _, err := toml.NewDecoder(r).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}
	return &cfg, nil
}

// Write encodes a Config to the provided writer.
func (m *Manager) Write(w io.Writer, cfg *Config) error {
	if err := toml.NewEncoder(w).Encode(cfg); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}
	return nil
}

// ReadFromFile reads a Config from the specified file path.
func ReadFromFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer f.Close()

	m := &Manager{}
	cfg, err := m.Read(f)
	if err != nil {
		return nil, fmt.Errorf("reading config from %s: %w", path, err)
	}
	return cfg, nil
}

// writeToFile writes a Config to the specified file path.
// This is an internal helper and should not be exported.
func writeToFile(path string, cfg *Config) error {
	// Ensure the directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer f.Close()

	m := &Manager{}
	if err := m.Write(f, cfg); err != nil {
		return fmt.Errorf("writing config to %s: %w", path, err)
	}
	return nil
}

// Init initializes a new config file at the specified path with the provided Config.
func Init(path string, cfg *Config) error {
	// Check if config already exists
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config file already exists at %s", path)
	}

	if err := writeToFile(path, cfg); err != nil {
		return fmt.Errorf("initializing config: %w", err)
	}
	return nil
}
