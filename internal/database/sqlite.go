package database

import (
	"database/sql"
	"fmt"

	"bt-go/internal/bt"
	"bt-go/internal/model"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// SQLiteDatabase implements the Database interface using SQLite.
type SQLiteDatabase struct {
	db   *sql.DB
	path string
}

// NewSQLiteDatabase creates a new SQLite database connection.
// path can be a file path or ":memory:" for in-memory database.
func NewSQLiteDatabase(path string) (*SQLiteDatabase, error) {
	db, err := OpenConnection(path)
	if err != nil {
		return nil, err
	}

	return &SQLiteDatabase{
		db:   db,
		path: path,
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

func (s *SQLiteDatabase) FindDirectoryByPath(path string) (*model.Directory, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *SQLiteDatabase) SearchDirectoryForPath(path string) (*model.Directory, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *SQLiteDatabase) CreateDirectory(path string) (*model.Directory, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *SQLiteDatabase) FindDirectoriesByPathPrefix(pathPrefix string) ([]*model.Directory, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *SQLiteDatabase) DeleteDirectory(directory *model.Directory) error {
	return fmt.Errorf("not implemented")
}

// File operations

func (s *SQLiteDatabase) FindFileByPath(directory *model.Directory, relativePath string) (*model.File, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *SQLiteDatabase) FindOrCreateFile(directory *model.Directory, relativePath string) (*model.File, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *SQLiteDatabase) MoveFiles(sourceDir, destDir *model.Directory) error {
	return fmt.Errorf("not implemented")
}

// FileSnapshot operations

func (s *SQLiteDatabase) FindFileSnapshotsForFile(file *model.File) ([]*model.FileSnapshot, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *SQLiteDatabase) FindFileSnapshotByChecksum(file *model.File, checksum string) (*model.FileSnapshot, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *SQLiteDatabase) CreateFileSnapshot(snapshot *model.FileSnapshot) error {
	return fmt.Errorf("not implemented")
}

func (s *SQLiteDatabase) UpdateFileCurrentSnapshot(file *model.File, snapshotID string) error {
	return fmt.Errorf("not implemented")
}

// Content operations

func (s *SQLiteDatabase) CreateContent(checksum string) (*model.Content, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *SQLiteDatabase) FindContentByChecksum(checksum string) (*model.Content, error) {
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
