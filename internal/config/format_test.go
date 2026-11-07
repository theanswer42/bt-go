package config

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestFormat(t *testing.T) {
	const secret = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"

	tests := []struct {
		name        string
		vaults      []VaultConfig
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:   "no vaults",
			vaults: nil,
			wantContain: []string{
				"Host ID:   host-1",
				"data_dir:    /data/bt/data",
				"staging_dir: /data/bt/staging",
				"public key:  /data/bt/keys/bt.pub",
				"private key: /data/bt/keys/bt.key",
				"Vaults: (none configured)",
			},
		},
		{
			name:        "filesystem vault",
			vaults:      []VaultConfig{{Type: "filesystem", Name: "local", FSVaultRoot: "/vault"}},
			wantContain: []string{"name: local", "type: filesystem", "fs_vault_root:        /vault"},
		},
		{
			name: "s3 secret is redacted",
			vaults: []VaultConfig{{
				Type: "s3", Name: "remote", S3Bucket: "my-bucket", S3Region: "us-east-1",
				S3AccessKeyID: "AKIAEXAMPLE", S3SecretAccessKey: secret,
			}},
			wantContain: []string{"s3_bucket:            my-bucket", "s3_access_key_id:     AKIAEXAMPLE", "s3_secret_access_key: ********"},
			wantAbsent:  []string{secret},
		},
		{
			name:        "unset s3 secret shows not set",
			vaults:      []VaultConfig{{Type: "s3", Name: "remote", S3Bucket: "b"}},
			wantContain: []string{"s3_secret_access_key: (not set)"},
			wantAbsent:  []string{"********"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig("host-1", "/data/bt")
			cfg.Vaults = tt.vaults

			var buf bytes.Buffer
			if err := Format(&buf, cfg); err != nil {
				t.Fatalf("Format() error = %v", err)
			}
			out := buf.String()
			for _, s := range tt.wantContain {
				if !strings.Contains(out, s) {
					t.Errorf("output missing %q:\n%s", s, out)
				}
			}
			for _, s := range tt.wantAbsent {
				if strings.Contains(out, s) {
					t.Errorf("output must not contain %q:\n%s", s, out)
				}
			}
		})
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestFormat_WriteError(t *testing.T) {
	err := Format(failingWriter{}, NewConfig("h", "/b"))
	if err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("Format() error = %v, want write error", err)
	}
}
