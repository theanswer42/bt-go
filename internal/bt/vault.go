package bt

import "io"

// Vault provides an interface for backup storage backends.
// All operations use io.Reader/io.Writer for streaming to support large files
// without loading them entirely into memory.
type Vault interface {
	// PutContent stores content identified by its checksum.
	// The operation is idempotent: storing the same checksum multiple times is safe.
	// size is the number of bytes that will be read from r.
	PutContent(checksum string, r io.Reader, size int64) error

	// GetContent retrieves content by checksum and writes it to w.
	GetContent(checksum string, w io.Writer) error

	// PutMetadata stores a named metadata item for a specific host.
	// size is the number of bytes that will be read from r.
	// version is stored alongside the metadata for consistency checks.
	// Known names: "db" (SQLite database), "public_key", "private_key".
	PutMetadata(hostID string, name string, r io.Reader, size int64, version int64) error

	// GetMetadata retrieves a named metadata item for a specific host and writes it to w.
	GetMetadata(hostID string, name string, w io.Writer) error

	// GetMetadataVersion returns the metadata version for a named item on a host.
	// Returns 0 if no metadata has been stored for this host/name.
	GetMetadataVersion(hostID string, name string) (int64, error)

	// ValidateSetup verifies that the vault is accessible and properly configured.
	ValidateSetup() error
}
