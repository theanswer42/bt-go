package bt

import (
	"io"
	"io/fs"
)

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
}
