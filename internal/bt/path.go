package bt

import "io/fs"

// Path represents a validated filesystem path with cached metadata.
// Path objects are created by FilesystemManager.Resolve() which validates
// the path exists, resolves it to an absolute path, and caches stat info.
type Path struct {
	absPath string
	isDir   bool
	info    fs.FileInfo
}

// NewPath creates a Path from its components.
// This is primarily for use by FilesystemManager implementations.
func NewPath(absPath string, isDir bool, info fs.FileInfo) *Path {
	return &Path{
		absPath: absPath,
		isDir:   isDir,
		info:    info,
	}
}

// String returns the absolute path as a string.
func (p *Path) String() string {
	return p.absPath
}

// IsDir returns true if this path points to a directory.
func (p *Path) IsDir() bool {
	return p.isDir
}

// Info returns the cached file info from when the path was resolved.
func (p *Path) Info() fs.FileInfo {
	return p.info
}
