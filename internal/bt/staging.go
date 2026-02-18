package bt

import (
	"io"

	"bt-go/internal/database/sqlc"
)

// BackupFunc is called by ProcessNext with the staged content and metadata.
// directoryID and relativePath identify the file within a tracked directory.
// The snapshot contains file metadata captured at staging time (no FileID set).
// If it returns nil, the staged operation is removed (committed).
// If it returns an error, the operation stays in queue for retry.
type BackupFunc func(content io.Reader, snapshot sqlc.FileSnapshot, directoryID string, relativePath string) error

// StagingArea provides an interface for staging files before backup.
// Files are staged in a queue and processed during backup operations.
// The staging area enforces a maximum size to prevent filling up the filesystem.
//
// Staging does not write to the metadata store. File identity is tracked
// by directory ID + relative path; database File records are created
// at backup time.
type StagingArea interface {
	// Stage stages a file for backup.
	// It stats the source file, copies content to staging (computing checksum),
	// re-stats to validate the file hasn't changed, and adds to the queue.
	// If the same checksum already exists in staging, content is deduplicated.
	Stage(directory *sqlc.Directory, relativePath string, path *Path) error

	// ProcessNext gets the next staged operation and calls fn with its data.
	// If fn returns nil, the staged operation is removed (committed).
	// If fn returns an error, the operation stays in queue for retry.
	// Returns nil with no error if the queue is empty.
	ProcessNext(fn BackupFunc) error

	// Count returns the number of staged operations in the queue.
	Count() (int, error)

	// Size returns the total size of staged content in bytes.
	Size() (int64, error)

	// IsStaged reports whether a file is currently in the staging queue.
	IsStaged(directoryID string, relativePath string) (bool, error)
}
