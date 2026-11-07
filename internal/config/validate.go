package config

import "fmt"

// Validate reports the first problem that would stop bt from building its
// components from c. Errors name the offending section and key, e.g.
// "[staging] staging_dir is required"; Load prefixes them with the file path.
func (c *Config) Validate() error {
	if c.HostID == "" {
		return fmt.Errorf("host_id is required")
	}

	switch c.Database.Type {
	case "sqlite":
		if c.Database.DataDir == "" {
			return fmt.Errorf("[database] data_dir is required when type = %q", c.Database.Type)
		}
	case "memory":
	default:
		return fmt.Errorf("[database] unknown type %q (want \"sqlite\" or \"memory\")", c.Database.Type)
	}

	switch c.Staging.Type {
	case "filesystem":
		if c.Staging.StagingDir == "" {
			return fmt.Errorf("[staging] staging_dir is required when type = %q", c.Staging.Type)
		}
	case "memory":
	default:
		return fmt.Errorf("[staging] unknown type %q (want \"filesystem\" or \"memory\")", c.Staging.Type)
	}

	switch c.Encryption.Type {
	case "age", "", "test":
	default:
		return fmt.Errorf("[encryption] unknown type %q (want \"age\")", c.Encryption.Type)
	}

	if len(c.Vaults) == 0 {
		return fmt.Errorf("no vaults configured: add a [[vaults]] entry (see the examples at the end of the file)")
	}
	for i, v := range c.Vaults {
		switch v.Type {
		case "filesystem":
			if v.FSVaultRoot == "" {
				return fmt.Errorf("[[vaults]] #%d (%q): fs_vault_root is required when type = \"filesystem\"", i+1, v.Name)
			}
		case "s3", "memory":
		default:
			return fmt.Errorf("[[vaults]] #%d (%q): unknown type %q (want \"filesystem\" or \"s3\")", i+1, v.Name, v.Type)
		}
	}
	return nil
}
