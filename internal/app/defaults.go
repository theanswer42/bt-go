package app

import (
	"fmt"
	"os"
	"path/filepath"
)

// GetDefaults returns application default paths, checking environment variables first.
// Environment variables:
//   - BT_CONFIG_PATH: config file location (default: ~/.config/bt.toml)
//   - BT_HOME: base directory for bt data (default: ~/.local/share/bt)
func GetDefaults() (map[string]string, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	baseDir, err := getBaseDir()
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"config_path": configPath,
		"base_dir":    baseDir,
		"log_dir":     filepath.Join(baseDir, "log"),
	}, nil
}

// getConfigPath returns the config file path, checking BT_CONFIG_PATH env var first,
// then falling back to the default ~/.config/bt.toml.
func getConfigPath() (string, error) {
	if path := os.Getenv("BT_CONFIG_PATH"); path != "" {
		return path, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "bt.toml"), nil
}

// getBaseDir returns the base directory for bt data, checking BT_HOME env var first,
// then falling back to the XDG default ~/.local/share/bt.
func getBaseDir() (string, error) {
	if path := os.Getenv("BT_HOME"); path != "" {
		return path, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(homeDir, ".local", "share", "bt"), nil
}
