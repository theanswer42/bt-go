package testutil

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"time"

	"bt-go/internal/bt"
)

// MockFile represents a file in the mock filesystem.
type MockFile struct {
	Content     []byte
	Permissions fs.FileMode
	ModTime     time.Time
	IsDirectory bool
	// Stat data - set once when file is created
	Atime time.Time
	Ctime time.Time
}

// MockFilesystemManager is an in-memory filesystem for testing.
type MockFilesystemManager struct {
	files map[string]*MockFile
}

// NewMockFilesystemManager creates a new mock filesystem.
func NewMockFilesystemManager() *MockFilesystemManager {
	return &MockFilesystemManager{
		files: make(map[string]*MockFile),
	}
}

// AddFile adds a file to the mock filesystem.
func (m *MockFilesystemManager) AddFile(path string, content []byte) {
	now := time.Now()
	m.files[path] = &MockFile{
		Content:     content,
		Permissions: 0644,
		ModTime:     now,
		IsDirectory: false,
		Atime:       now,
		Ctime:       now,
	}
}

// AddDirectory adds a directory to the mock filesystem.
func (m *MockFilesystemManager) AddDirectory(path string) {
	now := time.Now()
	m.files[path] = &MockFile{
		Content:     nil,
		Permissions: 0755,
		ModTime:     now,
		IsDirectory: true,
		Atime:       now,
		Ctime:       now,
	}
}

func (m *MockFilesystemManager) Resolve(rawPath string) (*bt.Path, error) {
	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		return nil, err
	}

	file, ok := m.files[absPath]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", absPath)
	}

	info := &mockFileInfo{
		name:     filepath.Base(absPath),
		size:     int64(len(file.Content)),
		mode:     file.Permissions,
		modTime:  file.ModTime,
		isDir:    file.IsDirectory,
		mockFile: file,
	}

	return bt.NewPath(absPath, file.IsDirectory, info), nil
}

func (m *MockFilesystemManager) Open(path *bt.Path) (io.ReadCloser, error) {
	file, ok := m.files[path.String()]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", path.String())
	}
	if file.IsDirectory {
		return nil, fmt.Errorf("cannot open directory: %s", path.String())
	}
	return io.NopCloser(bytes.NewReader(file.Content)), nil
}

func (m *MockFilesystemManager) Stat(path *bt.Path) (fs.FileInfo, error) {
	file, ok := m.files[path.String()]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", path.String())
	}

	return &mockFileInfo{
		name:     filepath.Base(path.String()),
		size:     int64(len(file.Content)),
		mode:     file.Permissions,
		modTime:  file.ModTime,
		isDir:    file.IsDirectory,
		mockFile: file,
	}, nil
}

func (m *MockFilesystemManager) ExtractStatData(info fs.FileInfo) (*bt.StatData, error) {
	// Get the MockFile from Sys() to return consistent stat data
	mockFile, ok := info.Sys().(*MockFile)
	if !ok {
		return nil, fmt.Errorf("cannot extract stat data: expected *MockFile, got %T", info.Sys())
	}

	return &bt.StatData{
		UID:       1000,
		GID:       1000,
		Atime:     mockFile.Atime,
		Ctime:     mockFile.Ctime,
		BirthTime: sql.NullTime{Valid: false},
	}, nil
}

// mockFileInfo implements fs.FileInfo
type mockFileInfo struct {
	name     string
	size     int64
	mode     fs.FileMode
	modTime  time.Time
	isDir    bool
	mockFile *MockFile // reference to get stat data
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() fs.FileMode  { return m.mode }
func (m *mockFileInfo) ModTime() time.Time { return m.modTime }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() any           { return m.mockFile }

// Compile-time check
var _ bt.FilesystemManager = (*MockFilesystemManager)(nil)
