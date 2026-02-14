package model

import "time"

// Content represents content-addressable data in the vault.
// The ID is the SHA-256 checksum of the content itself.
type Content struct {
	ID        string    // SHA-256 checksum (not a UUID)
	CreatedAt time.Time
}

// Directory represents a tracked directory on the local host.
type Directory struct {
	ID        string // UUID
	Path      string // Absolute path on host
	CreatedAt time.Time
}

// File represents a file within a tracked directory.
type File struct {
	ID                string // UUID
	Name              string // Relative path within directory
	DirectoryID       string // Foreign key to Directory
	CurrentSnapshotID string // Foreign key to current FileSnapshot
	Deleted           bool   // Whether file has been deleted
}

// FileSnapshot represents the state of a file at a specific point in time.
type FileSnapshot struct {
	ID         string    // UUID
	FileID     string    // Foreign key to File
	ContentID  string    // Checksum (foreign key to Content)
	CreatedAt  time.Time // When snapshot was created
	Size       int64     // File size in bytes
	Permissions uint32   // File mode/permissions
	UID        int       // User ID
	GID        int       // Group ID
	AccessedAt time.Time // File access time (atime)
	ModifiedAt time.Time // File modification time (mtime)
	ChangedAt  time.Time // Metadata change time (ctime)
	BornAt     *time.Time // File creation time (birthtime, if available)
}
