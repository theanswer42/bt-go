package bt

import "bt-go/internal/database/sqlc"

// Database provides an interface for metadata storage operations.
// All methods should be implemented with appropriate transaction handling.
type Database interface {
	// Directory operations

	// FindDirectoryByPath returns a directory with an exact path match.
	FindDirectoryByPath(path string) (*sqlc.Directory, error)

	// SearchDirectoryForPath finds the directory that contains the given path.
	// For example, if "/home/user/docs" is tracked and path is "/home/user/docs/file.txt",
	// this returns the "/home/user/docs" directory.
	SearchDirectoryForPath(path string) (*sqlc.Directory, error)

	// CreateDirectory creates a new tracked directory.
	// If there are existing child directories, moves their files and deletes them.
	CreateDirectory(path string) (*sqlc.Directory, error)

	// FindDirectoriesByPathPrefix returns all directories whose path starts with the given prefix.
	FindDirectoriesByPathPrefix(pathPrefix string) ([]*sqlc.Directory, error)

	// DeleteDirectory deletes a directory from tracking.
	DeleteDirectory(directory *sqlc.Directory) error

	// File operations

	// FindFileByPath returns a file within a directory by its relative path.
	FindFileByPath(directory *sqlc.Directory, relativePath string) (*sqlc.File, error)

	// FindOrCreateFile finds an existing file or creates a new one.
	FindOrCreateFile(directory *sqlc.Directory, relativePath string) (*sqlc.File, error)

	// MoveFiles moves all files from source directory to destination directory.
	MoveFiles(sourceDir, destDir *sqlc.Directory) error

	// FileSnapshot operations

	// FindFileSnapshotsForFile returns all snapshots for a given file, ordered by creation time.
	FindFileSnapshotsForFile(file *sqlc.File) ([]*sqlc.FileSnapshot, error)

	// FindFileSnapshotByChecksum returns a snapshot for a file with a specific content checksum.
	FindFileSnapshotByChecksum(file *sqlc.File, checksum string) (*sqlc.FileSnapshot, error)

	// CreateFileSnapshot creates a new snapshot for a file.
	CreateFileSnapshot(snapshot *sqlc.FileSnapshot) error

	// UpdateFileCurrentSnapshot updates the current snapshot pointer for a file.
	UpdateFileCurrentSnapshot(file *sqlc.File, snapshotID string) error

	// Content operations

	// CreateContent records that content with the given checksum exists in the vault.
	CreateContent(checksum string) (*sqlc.Content, error)

	// FindContentByChecksum returns content metadata by checksum.
	FindContentByChecksum(checksum string) (*sqlc.Content, error)

	// Close closes the database connection.
	Close() error
}
