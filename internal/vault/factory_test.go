package vault

import (
	"testing"

	"bt-go/internal/config"
)

func TestNewVaultFromConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.VaultConfig
		wantErr bool
		wantNil bool
	}{
		{
			name: "memory vault",
			cfg: config.VaultConfig{
				Type: "memory",
				Name: "test-memory",
			},
			wantErr: false,
			wantNil: false,
		},
		{
			name: "s3 vault - not yet implemented",
			cfg: config.VaultConfig{
				Type:     "s3",
				Name:     "test-s3",
				S3Bucket: "my-bucket",
			},
			wantErr: true,
			wantNil: true,
		},
		{
			name: "filesystem vault - not yet implemented",
			cfg: config.VaultConfig{
				Type:        "filesystem",
				Name:        "test-fs",
				FSVaultRoot: "/tmp/vault",
			},
			wantErr: true,
			wantNil: true,
		},
		{
			name: "unknown vault type",
			cfg: config.VaultConfig{
				Type: "unknown",
				Name: "test-unknown",
			},
			wantErr: true,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewVaultFromConfig(tt.cfg)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewVaultFromConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if (got == nil) != tt.wantNil {
				t.Errorf("NewVaultFromConfig() returned nil = %v, wantNil %v", got == nil, tt.wantNil)
			}

			// For successful cases, verify the vault works
			if !tt.wantErr && got != nil {
				if err := got.ValidateSetup(); err != nil {
					t.Errorf("ValidateSetup() error = %v", err)
				}
			}
		})
	}
}
