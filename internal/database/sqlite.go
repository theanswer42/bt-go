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
	"bt-go/internal/database/migrations"
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

// NewSQLiteDatabaseFromDB wraps an existing database connection.
// The caller is responsible for ensuring the connection is properly configured.
func NewSQLiteDatabaseFromDB(db *sql.DB) *SQLiteDatabase {
	return &SQLiteDatabase{
		db:      db,
		queries: sqlc.New(db),
		path:    "",
	}
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

func (s *SQLiteDatabase) FindFilesByDirectory(directory *sqlc.Directory) ([]*sqlc.File, error) {
	files, err := s.queries.GetFilesByDirectoryID(context.Background(), directory.ID)
	if err != nil {
		return nil, fmt.Errorf("finding files by directory: %w", err)
	}

	result := make([]*sqlc.File, len(files))
	for i := range files {
		result[i] = &files[i]
	}
	return result, nil
}

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
	snapshots, err := s.queries.GetFileSnapshotsByFileID(context.Background(), file.ID)
	if err != nil {
		return nil, fmt.Errorf("finding file snapshots: %w", err)
	}

	result := make([]*sqlc.FileSnapshot, len(snapshots))
	for i := range snapshots {
		result[i] = &snapshots[i]
	}
	return result, nil
}

func (s *SQLiteDatabase) FindFileSnapshotByChecksum(file *sqlc.File, checksum string) (*sqlc.FileSnapshot, error) {
	snapshot, err := s.queries.GetFileSnapshotByFileAndContent(context.Background(), sqlc.GetFileSnapshotByFileAndContentParams{
		FileID:    file.ID,
		ContentID: checksum,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("finding file snapshot by checksum: %w", err)
	}
	return &snapshot, nil
}

func (s *SQLiteDatabase) CreateFileSnapshot(snapshot *sqlc.FileSnapshot) error {
	_, err := s.queries.InsertFileSnapshot(context.Background(), sqlc.InsertFileSnapshotParams{
		ID:          snapshot.ID,
		FileID:      snapshot.FileID,
		ContentID:   snapshot.ContentID,
		CreatedAt:   snapshot.CreatedAt,
		Size:        snapshot.Size,
		Permissions: snapshot.Permissions,
		Uid:         snapshot.Uid,
		Gid:         snapshot.Gid,
		AccessedAt:  snapshot.AccessedAt,
		ModifiedAt:  snapshot.ModifiedAt,
		ChangedAt:   snapshot.ChangedAt,
		BornAt:      snapshot.BornAt,
	})
	if err != nil {
		return fmt.Errorf("creating file snapshot: %w", err)
	}
	return nil
}

func (s *SQLiteDatabase) UpdateFileCurrentSnapshot(file *sqlc.File, snapshotID string) error {
	err := s.queries.UpdateFileCurrentSnapshot(context.Background(), sqlc.UpdateFileCurrentSnapshotParams{
		CurrentSnapshotID: sql.NullString{String: snapshotID, Valid: true},
		ID:                file.ID,
	})
	if err != nil {
		return fmt.Errorf("updating file current snapshot: %w", err)
	}
	return nil
}

// CreateFileSnapshotAndContent atomically records a backup in a single transaction:
// 1. Finds or creates the file record for the given directory + relative path.
// 2. Creates the content record if it doesn't already exist.
// 3. Compares against the file's current snapshot — if all relevant fields match,
//    this is a no-op (the file hasn't changed).
// 4. Otherwise creates a new snapshot and updates the file's current snapshot pointer.
func (s *SQLiteDatabase) CreateFileSnapshotAndContent(directoryID string, relativePath string, snapshot *sqlc.FileSnapshot) error {
	ctx := context.Background()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := s.queries.WithTx(tx)

	// 1. Find or create the file record.
	file, err := qtx.GetFileByDirectoryAndName(ctx, sqlc.GetFileByDirectoryAndNameParams{
		DirectoryID: directoryID,
		Name:        relativePath,
	})
	if errors.Is(err, sql.ErrNoRows) {
		file, err = qtx.InsertFile(ctx, sqlc.InsertFileParams{
			ID:                uuid.New().String(),
			Name:              relativePath,
			DirectoryID:       directoryID,
			CurrentSnapshotID: sql.NullString{},
			Deleted:           false,
		})
		if err != nil {
			return fmt.Errorf("creating file: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("finding file: %w", err)
	}

	// 2. Create content record if it doesn't exist.
	_, err = qtx.GetContentByID(ctx, snapshot.ContentID)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = qtx.InsertContent(ctx, sqlc.InsertContentParams{
			ID:        snapshot.ContentID,
			CreatedAt: time.Now(),
		})
		if err != nil {
			return fmt.Errorf("creating content: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("checking for existing content: %w", err)
	}

	// 3. Check the file's current snapshot. If it matches, nothing changed — skip.
	if file.CurrentSnapshotID.Valid {
		current, err := qtx.GetFileSnapshotByID(ctx, file.CurrentSnapshotID.String)
		if err != nil {
			return fmt.Errorf("loading current snapshot: %w", err)
		}
		if snapshotsEqual(&current, snapshot) {
			// Nothing changed — commit the content record (if new) and return.
			if err := tx.Commit(); err != nil {
				return fmt.Errorf("committing transaction: %w", err)
			}
			return nil
		}
	}

	// 4. Create new snapshot and update the file's current pointer.
	snapshot.FileID = file.ID
	created, err := qtx.InsertFileSnapshot(ctx, sqlc.InsertFileSnapshotParams{
		ID:          snapshot.ID,
		FileID:      file.ID,
		ContentID:   snapshot.ContentID,
		CreatedAt:   snapshot.CreatedAt,
		Size:        snapshot.Size,
		Permissions: snapshot.Permissions,
		Uid:         snapshot.Uid,
		Gid:         snapshot.Gid,
		AccessedAt:  snapshot.AccessedAt,
		ModifiedAt:  snapshot.ModifiedAt,
		ChangedAt:   snapshot.ChangedAt,
		BornAt:      snapshot.BornAt,
	})
	if err != nil {
		return fmt.Errorf("creating file snapshot: %w", err)
	}

	err = qtx.UpdateFileCurrentSnapshot(ctx, sqlc.UpdateFileCurrentSnapshotParams{
		CurrentSnapshotID: sql.NullString{String: created.ID, Valid: true},
		ID:                file.ID,
	})
	if err != nil {
		return fmt.Errorf("updating file current snapshot: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// snapshotsEqual compares all relevant fields of two file snapshots.
// ID and CreatedAt are excluded — they're identity/metadata, not file state.
func snapshotsEqual(a, b *sqlc.FileSnapshot) bool {
	return a.ContentID == b.ContentID &&
		a.Size == b.Size &&
		a.Permissions == b.Permissions &&
		a.Uid == b.Uid &&
		a.Gid == b.Gid &&
		a.AccessedAt.Equal(b.AccessedAt) &&
		a.ModifiedAt.Equal(b.ModifiedAt) &&
		a.ChangedAt.Equal(b.ChangedAt) &&
		a.BornAt == b.BornAt
}

// Backup operation tracking

func (s *SQLiteDatabase) CreateBackupOperation(operation string, parameters string) (*sqlc.BackupOperation, error) {
	op, err := s.queries.InsertBackupOperation(context.Background(), sqlc.InsertBackupOperationParams{
		StartedAt:  time.Now(),
		Operation:  operation,
		Parameters: parameters,
	})
	if err != nil {
		return nil, fmt.Errorf("creating backup operation: %w", err)
	}
	return &op, nil
}

func (s *SQLiteDatabase) FinishBackupOperation(id int64, status string) error {
	err := s.queries.UpdateBackupOperationFinished(context.Background(), sqlc.UpdateBackupOperationFinishedParams{
		FinishedAt: sql.NullTime{Time: time.Now(), Valid: true},
		Status:     status,
		ID:         id,
	})
	if err != nil {
		return fmt.Errorf("finishing backup operation: %w", err)
	}
	return nil
}

func (s *SQLiteDatabase) ListBackupOperations(limit int) ([]*sqlc.BackupOperation, error) {
	ops, err := s.queries.GetBackupOperations(context.Background(), int64(limit))
	if err != nil {
		return nil, fmt.Errorf("listing backup operations: %w", err)
	}

	result := make([]*sqlc.BackupOperation, len(ops))
	for i := range ops {
		result[i] = &ops[i]
	}
	return result, nil
}

func (s *SQLiteDatabase) MaxBackupOperationID() (int64, error) {
	id, err := s.queries.GetMaxBackupOperationID(context.Background())
	if err != nil {
		return 0, fmt.Errorf("getting max backup operation ID: %w", err)
	}
	return id, nil
}

// Content operations

func (s *SQLiteDatabase) CreateContent(checksum string) (*sqlc.Content, error) {
	content, err := s.queries.InsertContent(context.Background(), sqlc.InsertContentParams{
		ID:        checksum,
		CreatedAt: time.Now(),
	})
	if err != nil {
		return nil, fmt.Errorf("creating content: %w", err)
	}
	return &content, nil
}

func (s *SQLiteDatabase) FindContentByChecksum(checksum string) (*sqlc.Content, error) {
	content, err := s.queries.GetContentByID(context.Background(), checksum)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("finding content by checksum: %w", err)
	}
	return &content, nil
}

// Path returns the database file path (or ":memory:" for in-memory databases).
func (s *SQLiteDatabase) Path() string {
	return s.path
}

// CheckMigrations verifies the database schema is up-to-date.
func (s *SQLiteDatabase) CheckMigrations() error {
	return migrations.CheckDBMigrationStatus(s.db)
}

// BackupTo creates a complete copy of the database at destPath using VACUUM INTO.
func (s *SQLiteDatabase) BackupTo(destPath string) error {
	_, err := s.db.Exec("VACUUM INTO ?", destPath)
	if err != nil {
		return fmt.Errorf("backing up database: %w", err)
	}
	return nil
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
