package bt

import (
	"database/sql"
	"io"
	"io/fs"
	"time"
)

// StatData holds platform-specific file metadata that isn't available
// through the standard fs.FileInfo interface.
type StatData struct {
	UID       int64
	GID       int64
	Atime     time.Time
	Ctime     time.Time
	BirthTime sql.NullTime
}

// FilesystemManager provides an interface for filesystem operations.
// It abstracts file access to enable testing without touching the real filesystem.
type FilesystemManager interface {
	// Resolve validates a raw path and returns a Path object.
	// It resolves the path to an absolute path, stats it, and validates
	// it's a regular file or directory (not a symlink, device, etc.).
	Resolve(rawPath string) (*Path, error)

	// Open opens a file for reading.
	Open(path *Path) (io.ReadCloser, error)

	// Stat returns fresh file info for a path.
	// Unlike path.Info() which returns cached info from when the path was resolved,
	// this always fetches current info from the filesystem.
	Stat(path *Path) (fs.FileInfo, error)

	// ExtractStatData extracts platform-specific metadata from a FileInfo.
	// This includes uid, gid, atime, ctime, and birthtime where available.
	ExtractStatData(info fs.FileInfo) (*StatData, error)

	// FindFiles discovers regular files under the given directory path.
	// If recursive is false, only files directly in the directory are returned.
	// If recursive is true, files in all subdirectories are included.
	// Symlinks, devices, and other special files are skipped.
	FindFiles(path *Path, recursive bool) ([]*Path, error)
}
