package config

import (
	"strings"
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	valid := func() *Config {
		cfg := NewConfig("host-1", "/data/bt")
		cfg.Vaults = []VaultConfig{{Type: "filesystem", Name: "local", FSVaultRoot: "/vault"}}
		return cfg
	}

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string // substring; empty means valid
	}{
		{name: "defaults plus filesystem vault", mutate: func(*Config) {}},
		{name: "memory components", mutate: func(c *Config) {
			c.Database = DatabaseConfig{Type: "memory"}
			c.Staging = StagingConfig{Type: "memory"}
			c.Vaults = []VaultConfig{{Type: "memory", Name: "m"}}
		}},
		{name: "s3 vault", mutate: func(c *Config) {
			c.Vaults = []VaultConfig{{Type: "s3", Name: "r", S3Bucket: "b"}}
		}},
		{name: "empty encryption type means age", mutate: func(c *Config) { c.Encryption.Type = "" }},
		{name: "missing host_id", mutate: func(c *Config) { c.HostID = "" }, wantErr: "host_id is required"},
		{name: "sqlite without data_dir", mutate: func(c *Config) { c.Database.DataDir = "" }, wantErr: "[database] data_dir is required"},
		{name: "unknown database type", mutate: func(c *Config) { c.Database.Type = "postgres" }, wantErr: `[database] unknown type "postgres"`},
		{name: "filesystem staging without staging_dir", mutate: func(c *Config) { c.Staging.StagingDir = "" }, wantErr: "[staging] staging_dir is required"},
		{name: "unknown staging type", mutate: func(c *Config) { c.Staging.Type = "disk" }, wantErr: `[staging] unknown type "disk"`},
		{name: "unknown encryption type", mutate: func(c *Config) { c.Encryption.Type = "gpg" }, wantErr: `[encryption] unknown type "gpg"`},
		{name: "no vaults", mutate: func(c *Config) { c.Vaults = nil }, wantErr: "no vaults configured"},
		{name: "filesystem vault without root", mutate: func(c *Config) { c.Vaults[0].FSVaultRoot = "" }, wantErr: `[[vaults]] #1 ("local"): fs_vault_root is required`},
		{name: "unknown vault type", mutate: func(c *Config) {
			c.Vaults = append(c.Vaults, VaultConfig{Type: "ftp", Name: "x"})
		}, wantErr: `[[vaults]] #2 ("x"): unknown type "ftp"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := valid()
			tt.mutate(cfg)
			err := cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}
