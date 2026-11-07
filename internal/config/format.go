package config

import (
	"fmt"
	"io"
)

// redacted is shown in place of secrets by Format.
const redacted = "********"

// Format writes a human-readable summary of the effective config to w.
// Secrets (s3_secret_access_key) are never written: they are shown as
// "********" when set and "(not set)" otherwise.
func Format(w io.Writer, cfg *Config) error {
	p := &printer{w: w}
	p.line("Host ID:   %s", orNotSet(cfg.HostID))
	p.line("Base Dir:  %s", orNotSet(cfg.BaseDir))
	p.line("Log Dir:   %s", orNotSet(cfg.LogDir))
	p.line("")
	p.line("Database:")
	p.line("  type:        %s", orNotSet(cfg.Database.Type))
	p.line("  data_dir:    %s", orNotSet(cfg.Database.DataDir))
	p.line("")
	p.line("Staging:")
	p.line("  type:        %s", orNotSet(cfg.Staging.Type))
	p.line("  staging_dir: %s", orNotSet(cfg.Staging.StagingDir))
	p.line("  max_size:    %d", cfg.Staging.MaxSize)
	p.line("")
	p.line("Encryption:")
	p.line("  type:        %s", orNotSet(cfg.Encryption.Type))
	p.line("  public key:  %s", orNotSet(cfg.Encryption.PublicKeyPath))
	p.line("  private key: %s", orNotSet(cfg.Encryption.PrivateKeyPath))
	p.line("")
	if len(cfg.Vaults) == 0 {
		p.line("Vaults: (none configured)")
	} else {
		p.line("Vaults:")
	}
	for i, v := range cfg.Vaults {
		p.line("  [%d] name: %s", i+1, orNotSet(v.Name))
		p.line("      type: %s", orNotSet(v.Type))
		switch v.Type {
		case "filesystem":
			p.line("      fs_vault_root:        %s", orNotSet(v.FSVaultRoot))
		case "s3":
			p.line("      s3_bucket:            %s", orNotSet(v.S3Bucket))
			p.line("      s3_region:            %s", orNotSet(v.S3Region))
			p.line("      s3_content_prefix:    %s", orNotSet(v.S3ContentPrefix))
			p.line("      s3_metadata_prefix:   %s", orNotSet(v.S3MetadataPrefix))
			p.line("      s3_access_key_id:     %s", orNotSet(v.S3AccessKeyID))
			p.line("      s3_secret_access_key: %s", redact(v.S3SecretAccessKey))
		}
	}
	if len(cfg.Filesystem.Ignore) > 0 {
		p.line("")
		p.line("Ignore patterns:")
		for _, pat := range cfg.Filesystem.Ignore {
			p.line("  %s", pat)
		}
	}
	return p.err
}

// printer writes formatted lines, remembering the first write error.
type printer struct {
	w   io.Writer
	err error
}

func (p *printer) line(format string, args ...any) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprintf(p.w, format+"\n", args...)
}

func orNotSet(s string) string {
	if s == "" {
		return "(not set)"
	}
	return s
}

func redact(s string) string {
	if s == "" {
		return "(not set)"
	}
	return redacted
}
