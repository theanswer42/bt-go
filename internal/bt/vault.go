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

	// PutMetadata stores metadata (typically a SQLite database) for a specific host.
	// size is the number of bytes that will be read from r.
	PutMetadata(hostID string, r io.Reader, size int64) error

	// GetMetadata retrieves metadata for a specific host and writes it to w.
	GetMetadata(hostID string, w io.Writer) error

	// ValidateSetup verifies that the vault is accessible and properly configured.
	ValidateSetup() error
}
