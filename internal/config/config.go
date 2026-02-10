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
	HostID  string `toml:"host_id"`
	BaseDir string `toml:"base_dir"`
	LogDir  string `toml:"log_dir"`
}

// NewConfig creates a new Config with the provided values.
func NewConfig(hostID, baseDir string) *Config {
	return &Config{
		HostID:  hostID,
		BaseDir: baseDir,
		LogDir:  filepath.Join(baseDir, "log"),
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
