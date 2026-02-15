package bt

import (
	"io"

	"bt-go/internal/database/sqlc"
)

// StagingArea provides an interface for staging files before backup.
// Files are staged in a queue and processed during backup operations.
// The staging area enforces a maximum size to prevent filling up the filesystem.
type StagingArea interface {
	// Stage stages a file for backup.
	// It stats the source file, copies content to staging (computing checksum),
	// re-stats to validate the file hasn't changed, and adds to the queue.
	// If the same checksum already exists in staging, content is deduplicated.
	Stage(directory *sqlc.Directory, file *sqlc.File, path *Path) error

	// Next returns the next staged operation from the queue.
	// Returns nil if the queue is empty.
	Next() (*StagedOperation, error)

	// GetContent returns a reader for the staged content by checksum.
	GetContent(checksum string) (io.ReadCloser, error)

	// Remove removes a staged operation after successful backup.
	// It removes the operation from the queue and deletes the content
	// if no other operations reference the same checksum.
	Remove(op *StagedOperation) error

	// Count returns the number of staged operations in the queue.
	Count() (int, error)

	// Size returns the total size of staged content in bytes.
	Size() (int64, error)
}
