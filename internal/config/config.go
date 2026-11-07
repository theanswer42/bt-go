package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
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
	Type           string `toml:"type"` // "age" (default) or "test"
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
	S3Bucket          string `toml:"s3_bucket,omitempty"`
	S3Region          string `toml:"s3_region,omitempty"`
	S3ContentPrefix   string `toml:"s3_content_prefix,omitempty"`
	S3MetadataPrefix  string `toml:"s3_metadata_prefix,omitempty"`
	S3AccessKeyID     string `toml:"s3_access_key_id,omitempty"`
	S3SecretAccessKey string `toml:"s3_secret_access_key,omitempty"`

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

// vaultExamples is appended as a comment block to newly initialized config files
// to show users how to configure each vault type.
const vaultExamples = `
# Example vault configurations (uncomment and fill in to use):
#
# [[vaults]]
# type = "filesystem"
# name = "local"
# fs_vault_root = "/path/to/vault"
#
# [[vaults]]
# type = "s3"
# name = "remote"
# s3_bucket = "my-bucket"
# s3_region = "us-east-1"
# s3_content_prefix = "bt/content"
# s3_metadata_prefix = "bt/metadata"
# s3_access_key_id = "AKIAIOSFODNN7EXAMPLE"
# s3_secret_access_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
`

// DefaultFSVaultName is the name given to the filesystem vault created by
// NewFilesystemVaultConfig.
const DefaultFSVaultName = "local"

// NewConfig creates a new Config whose paths are all derived from baseDir:
// logs, the SQLite data directory, the staging directory and the key pair.
// No vault is configured; see NewFilesystemVaultConfig.
func NewConfig(hostID, baseDir string) *Config {
	return &Config{
		HostID:  hostID,
		BaseDir: baseDir,
		LogDir:  filepath.Join(baseDir, "log"),
		Encryption: EncryptionConfig{
			Type:           "age",
			PublicKeyPath:  filepath.Join(baseDir, "keys", "bt.pub"),
			PrivateKeyPath: filepath.Join(baseDir, "keys", "bt.key"),
		},
		Database: DatabaseConfig{
			Type:    "sqlite",
			DataDir: filepath.Join(baseDir, "data"),
		},
		Staging: StagingConfig{
			Type:       "filesystem",
			StagingDir: filepath.Join(baseDir, "staging"),
			MaxSize:    1 << 20, // 1 MB
		},
	}
}

// NewFilesystemVaultConfig returns a filesystem vault entry named
// DefaultFSVaultName rooted at root, which is made absolute.
func NewFilesystemVaultConfig(root string) (VaultConfig, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return VaultConfig{}, fmt.Errorf("resolving vault root %q: %w", root, err)
	}
	return VaultConfig{Type: "filesystem", Name: DefaultFSVaultName, FSVaultRoot: abs}, nil
}

// VaultExamples returns the commented vault examples that Init appends to a
// new config file.
func VaultExamples() string {
	return vaultExamples
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

// ReadFromFile reads a Config from the specified file path without validating
// it. Use Load when the config is about to be used to build components.
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

// Load reads a Config from path and validates it. Validation errors name the
// config file, e.g. "config /home/u/.config/bt.toml: [staging] staging_dir is required".
func Load(path string) (*Config, error) {
	cfg, err := ReadFromFile(path)
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	return cfg, nil
}

// Init creates a new config file at path containing cfg followed by the
// commented vault examples. The file is created with mode 0600 and its parent
// directory, if missing, with mode 0700. Init fails if the file already exists;
// if writing fails part-way, the partially written file is removed.
func Init(path string, cfg *Config) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("config file already exists at %s", path)
	}
	if err != nil {
		return fmt.Errorf("creating config file: %w", err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("closing config file: %w", cerr)
		}
		if err != nil {
			// Best effort: the original error is what the caller needs to see.
			_ = os.Remove(path)
		}
	}()

	m := &Manager{}
	if err := m.Write(f, cfg); err != nil {
		return fmt.Errorf("writing config to %s: %w", path, err)
	}
	if _, err := f.WriteString(vaultExamples); err != nil {
		return fmt.Errorf("appending vault examples: %w", err)
	}
	return nil
}
