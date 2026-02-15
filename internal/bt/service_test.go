package bt

import (
	"fmt"
	"io/fs"
	"testing"
	"time"

	"bt-go/internal/database/sqlc"
)

// mockFileInfo implements fs.FileInfo for testing
type mockFileInfo struct {
	name  string
	isDir bool
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return 0 }
func (m mockFileInfo) Mode() fs.FileMode  { return 0 }
func (m mockFileInfo) ModTime() time.Time { return time.Time{} }
func (m mockFileInfo) IsDir() bool        { return m.isDir }
func (m mockFileInfo) Sys() any           { return nil }

// mockDatabase implements Database for testing
type mockDatabase struct {
	directories map[string]*sqlc.Directory
	createErr   error
	findErr     error
}

func newMockDatabase() *mockDatabase {
	return &mockDatabase{
		directories: make(map[string]*sqlc.Directory),
	}
}

func (m *mockDatabase) FindDirectoryByPath(path string) (*sqlc.Directory, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	dir, ok := m.directories[path]
	if !ok {
		return nil, nil
	}
	return dir, nil
}

func (m *mockDatabase) CreateDirectory(path string) (*sqlc.Directory, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	dir := &sqlc.Directory{
		ID:        "test-id",
		Path:      path,
		CreatedAt: time.Now(),
	}
	m.directories[path] = dir
	return dir, nil
}

// Stub implementations for other interface methods
func (m *mockDatabase) SearchDirectoryForPath(path string) (*sqlc.Directory, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockDatabase) FindDirectoriesByPathPrefix(pathPrefix string) ([]*sqlc.Directory, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockDatabase) DeleteDirectory(directory *sqlc.Directory) error {
	return fmt.Errorf("not implemented")
}
func (m *mockDatabase) FindFileByPath(directory *sqlc.Directory, relativePath string) (*sqlc.File, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockDatabase) FindOrCreateFile(directory *sqlc.Directory, relativePath string) (*sqlc.File, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockDatabase) FindFileSnapshotsForFile(file *sqlc.File) ([]*sqlc.FileSnapshot, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockDatabase) FindFileSnapshotByChecksum(file *sqlc.File, checksum string) (*sqlc.FileSnapshot, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockDatabase) CreateFileSnapshot(snapshot *sqlc.FileSnapshot) error {
	return fmt.Errorf("not implemented")
}
func (m *mockDatabase) UpdateFileCurrentSnapshot(file *sqlc.File, snapshotID string) error {
	return fmt.Errorf("not implemented")
}
func (m *mockDatabase) CreateContent(checksum string) (*sqlc.Content, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockDatabase) FindContentByChecksum(checksum string) (*sqlc.Content, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockDatabase) Close() error { return nil }

func TestBTService_AddDirectory(t *testing.T) {
	t.Run("adds new directory", func(t *testing.T) {
		db := newMockDatabase()
		svc := NewBTService(db, nil, nil, nil)

		path := NewPath("/home/user/docs", true, mockFileInfo{name: "docs", isDir: true})
		err := svc.AddDirectory(path)
		if err != nil {
			t.Fatalf("AddDirectory() error = %v", err)
		}

		// Verify directory was created
		if _, ok := db.directories["/home/user/docs"]; !ok {
			t.Error("directory was not created in database")
		}
	})

	t.Run("returns error for non-directory path", func(t *testing.T) {
		db := newMockDatabase()
		svc := NewBTService(db, nil, nil, nil)

		path := NewPath("/home/user/file.txt", false, mockFileInfo{name: "file.txt", isDir: false})
		err := svc.AddDirectory(path)
		if err == nil {
			t.Error("AddDirectory() expected error for non-directory path")
		}
	})

	t.Run("is idempotent for existing directory", func(t *testing.T) {
		db := newMockDatabase()
		svc := NewBTService(db, nil, nil, nil)

		path := NewPath("/home/user/docs", true, mockFileInfo{name: "docs", isDir: true})

		// Add directory first time
		err := svc.AddDirectory(path)
		if err != nil {
			t.Fatalf("first AddDirectory() error = %v", err)
		}

		// Add directory second time - should succeed (idempotent)
		err = svc.AddDirectory(path)
		if err != nil {
			t.Fatalf("second AddDirectory() error = %v", err)
		}
	})

	t.Run("propagates database find error", func(t *testing.T) {
		db := newMockDatabase()
		db.findErr = fmt.Errorf("database connection lost")
		svc := NewBTService(db, nil, nil, nil)

		path := NewPath("/home/user/docs", true, mockFileInfo{name: "docs", isDir: true})
		err := svc.AddDirectory(path)
		if err == nil {
			t.Error("AddDirectory() expected error when database fails")
		}
	})

	t.Run("propagates database create error", func(t *testing.T) {
		db := newMockDatabase()
		db.createErr = fmt.Errorf("database write failed")
		svc := NewBTService(db, nil, nil, nil)

		path := NewPath("/home/user/docs", true, mockFileInfo{name: "docs", isDir: true})
		err := svc.AddDirectory(path)
		if err == nil {
			t.Error("AddDirectory() expected error when database create fails")
		}
	})
}
