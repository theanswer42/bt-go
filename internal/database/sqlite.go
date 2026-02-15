package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"bt-go/internal/bt"
	"bt-go/internal/database/sqlc"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// SQLiteDatabase implements the Database interface using SQLite.
type SQLiteDatabase struct {
	db      *sql.DB
	queries *sqlc.Queries
	path    string
}

// NewSQLiteDatabase creates a new SQLite database connection.
// path can be a file path or ":memory:" for in-memory database.
func NewSQLiteDatabase(path string) (*SQLiteDatabase, error) {
	db, err := OpenConnection(path)
	if err != nil {
		return nil, err
	}

	return &SQLiteDatabase{
		db:      db,
		queries: sqlc.New(db),
		path:    path,
	}, nil
}

// OpenConnection opens and configures a SQLite database connection with appropriate PRAGMAs.
// This is exported for use in tools and tests that need a properly configured SQLite connection.
// path can be a file path or ":memory:" for in-memory database.
func OpenConnection(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign key constraints (SQLite default is OFF for backward compatibility)
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Future SQLite optimizations can be added here:
	// - PRAGMA journal_mode = WAL  (Write-Ahead Logging for better concurrency)
	// - PRAGMA busy_timeout = 5000 (Wait up to 5s for locks)
	// - PRAGMA synchronous = NORMAL (Balance between safety and performance)

	return db, nil
}

// Directory operations

func (s *SQLiteDatabase) FindDirectoryByPath(path string) (*sqlc.Directory, error) {
	dir, err := s.queries.GetDirectoryByPath(context.Background(), path)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("finding directory by path: %w", err)
	}
	return &dir, nil
}

func (s *SQLiteDatabase) SearchDirectoryForPath(path string) (*sqlc.Directory, error) {
	// Search for the shortest directory path that is a prefix of the given path.
	// We use shortest (not longest) to be consistent with our consolidation behavior -
	// child directories get merged into parent directories.
	ctx := context.Background()

	// Get all directories
	dirs, err := s.queries.GetDirectoriesByPathPrefix(ctx, "/%")
	if err != nil {
		return nil, fmt.Errorf("searching directories: %w", err)
	}

	var bestMatch *sqlc.Directory

	for i := range dirs {
		dir := &dirs[i]
		// Check if this directory is a prefix of the path
		if path == dir.Path {
			// Exact match - if we're searching for a directory itself, return it
			// But prefer shorter matches if we already have one
			if bestMatch == nil || len(dir.Path) < len(bestMatch.Path) {
				bestMatch = dir
			}
			continue
		}
		// Check if path is inside this directory
		if len(path) > len(dir.Path) && path[:len(dir.Path)] == dir.Path && path[len(dir.Path)] == '/' {
			if bestMatch == nil || len(dir.Path) < len(bestMatch.Path) {
				bestMatch = dir
			}
		}
	}

	return bestMatch, nil
}

func (s *SQLiteDatabase) CreateDirectory(path string) (*sqlc.Directory, error) {
	ctx := context.Background()

	// Start transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := s.queries.WithTx(tx)

	// Find any child directories that need to be consolidated
	childDirs, err := qtx.GetDirectoriesByPathPrefix(ctx, path+"/%")
	if err != nil {
		return nil, fmt.Errorf("finding child directories: %w", err)
	}

	// Create the new directory
	newDir, err := qtx.InsertDirectory(ctx, sqlc.InsertDirectoryParams{
		ID:        uuid.New().String(),
		Path:      path,
		CreatedAt: time.Now(),
	})
	if err != nil {
		return nil, fmt.Errorf("inserting directory: %w", err)
	}

	// Move files from child directories to the new directory
	for _, childDir := range childDirs {
		// Get all files in the child directory
		files, err := qtx.GetFilesByDirectoryID(ctx, childDir.ID)
		if err != nil {
			return nil, fmt.Errorf("getting files from child directory %s: %w", childDir.Path, err)
		}

		// Calculate the relative path prefix for files being moved
		// e.g., if parent is /home/user/docs and child is /home/user/docs/subdir,
		// files need "subdir/" prepended to their names
		relPath, err := filepath.Rel(path, childDir.Path)
		if err != nil {
			return nil, fmt.Errorf("calculating relative path: %w", err)
		}

		// Move each file to the new directory with updated name
		for _, file := range files {
			newName := filepath.Join(relPath, file.Name)
			err := qtx.UpdateFileDirectoryAndName(ctx, sqlc.UpdateFileDirectoryAndNameParams{
				DirectoryID: newDir.ID,
				Name:        newName,
				ID:          file.ID,
			})
			if err != nil {
				return nil, fmt.Errorf("moving file %s: %w", file.Name, err)
			}
		}

		// Delete the child directory
		if err := qtx.DeleteDirectoryByID(ctx, childDir.ID); err != nil {
			return nil, fmt.Errorf("deleting child directory %s: %w", childDir.Path, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return &newDir, nil
}

func (s *SQLiteDatabase) FindDirectoriesByPathPrefix(pathPrefix string) ([]*sqlc.Directory, error) {
	// Append /% to match child directories only (not the prefix itself)
	pattern := pathPrefix + "/%"
	dirs, err := s.queries.GetDirectoriesByPathPrefix(context.Background(), pattern)
	if err != nil {
		return nil, fmt.Errorf("finding directories by path prefix: %w", err)
	}

	// Convert to pointer slice
	result := make([]*sqlc.Directory, len(dirs))
	for i := range dirs {
		result[i] = &dirs[i]
	}
	return result, nil
}

func (s *SQLiteDatabase) DeleteDirectory(directory *sqlc.Directory) error {
	if err := s.queries.DeleteDirectoryByID(context.Background(), directory.ID); err != nil {
		return fmt.Errorf("deleting directory: %w", err)
	}
	return nil
}

// File operations

func (s *SQLiteDatabase) FindFileByPath(directory *sqlc.Directory, relativePath string) (*sqlc.File, error) {
	file, err := s.queries.GetFileByDirectoryAndName(context.Background(), sqlc.GetFileByDirectoryAndNameParams{
		DirectoryID: directory.ID,
		Name:        relativePath,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("finding file by path: %w", err)
	}
	return &file, nil
}

func (s *SQLiteDatabase) FindOrCreateFile(directory *sqlc.Directory, relativePath string) (*sqlc.File, error) {
	// Try to find existing file first
	file, err := s.FindFileByPath(directory, relativePath)
	if err != nil {
		return nil, err
	}
	if file != nil {
		return file, nil
	}

	// Create new file
	newFile, err := s.queries.InsertFile(context.Background(), sqlc.InsertFileParams{
		ID:                uuid.New().String(),
		Name:              relativePath,
		DirectoryID:       directory.ID,
		CurrentSnapshotID: sql.NullString{}, // No snapshot yet
		Deleted:           false,
	})
	if err != nil {
		return nil, fmt.Errorf("creating file: %w", err)
	}
	return &newFile, nil
}

// FileSnapshot operations

func (s *SQLiteDatabase) FindFileSnapshotsForFile(file *sqlc.File) ([]*sqlc.FileSnapshot, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *SQLiteDatabase) FindFileSnapshotByChecksum(file *sqlc.File, checksum string) (*sqlc.FileSnapshot, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *SQLiteDatabase) CreateFileSnapshot(snapshot *sqlc.FileSnapshot) error {
	return fmt.Errorf("not implemented")
}

func (s *SQLiteDatabase) UpdateFileCurrentSnapshot(file *sqlc.File, snapshotID string) error {
	return fmt.Errorf("not implemented")
}

// Content operations

func (s *SQLiteDatabase) CreateContent(checksum string) (*sqlc.Content, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *SQLiteDatabase) FindContentByChecksum(checksum string) (*sqlc.Content, error) {
	return nil, fmt.Errorf("not implemented")
}

// Close closes the database connection.
func (s *SQLiteDatabase) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Compile-time check that SQLiteDatabase implements bt.Database interface
var _ bt.Database = (*SQLiteDatabase)(nil)
