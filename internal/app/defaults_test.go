package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetDefaults(t *testing.T) {
	t.Run("uses env vars when set", func(t *testing.T) {
		t.Setenv("BT_CONFIG_PATH", "/custom/config.toml")
		t.Setenv("BT_HOME", "/custom/bt")

		defaults, err := GetDefaults()
		if err != nil {
			t.Fatalf("GetDefaults() error = %v", err)
		}

		if defaults["config_path"] != "/custom/config.toml" {
			t.Errorf("config_path = %q, want %q", defaults["config_path"], "/custom/config.toml")
		}
		if defaults["base_dir"] != "/custom/bt" {
			t.Errorf("base_dir = %q, want %q", defaults["base_dir"], "/custom/bt")
		}
		if defaults["log_dir"] != "/custom/bt/log" {
			t.Errorf("log_dir = %q, want %q", defaults["log_dir"], "/custom/bt/log")
		}
	})

	t.Run("falls back to home dir defaults", func(t *testing.T) {
		t.Setenv("BT_CONFIG_PATH", "")
		t.Setenv("BT_HOME", "")

		defaults, err := GetDefaults()
		if err != nil {
			t.Fatalf("GetDefaults() error = %v", err)
		}

		homeDir, _ := os.UserHomeDir()

		wantConfig := filepath.Join(homeDir, ".config", "bt.toml")
		if defaults["config_path"] != wantConfig {
			t.Errorf("config_path = %q, want %q", defaults["config_path"], wantConfig)
		}

		wantBase := filepath.Join(homeDir, ".local", "share", "bt")
		if defaults["base_dir"] != wantBase {
			t.Errorf("base_dir = %q, want %q", defaults["base_dir"], wantBase)
		}

		wantLog := filepath.Join(wantBase, "log")
		if defaults["log_dir"] != wantLog {
			t.Errorf("log_dir = %q, want %q", defaults["log_dir"], wantLog)
		}
	})
}
