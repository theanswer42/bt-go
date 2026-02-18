package fs

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"bt-go/internal/bt"
)

// OSFilesystemManager is the real filesystem implementation of FilesystemManager.
// It performs actual filesystem operations using the os package.
type OSFilesystemManager struct {
	// TODO: Add fields as needed during implementation (e.g., ignore patterns)
}

// NewOSFilesystemManager creates a new filesystem manager that operates on the real filesystem.
func NewOSFilesystemManager() *OSFilesystemManager {
	return &OSFilesystemManager{}
}

// Resolve validates a raw path and returns a Path object.
func (m *OSFilesystemManager) Resolve(rawPath string) (*bt.Path, error) {
	// Convert to absolute path
	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		return nil, fmt.Errorf("resolving absolute path: %w", err)
	}

	// Stat the path
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("stat path: %w", err)
	}

	// Check for special file types we don't support
	mode := info.Mode()
	if mode&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("symlinks not supported: %s", absPath)
	}
	if mode&os.ModeDevice != 0 {
		return nil, fmt.Errorf("device files not supported: %s", absPath)
	}
	if mode&os.ModeNamedPipe != 0 {
		return nil, fmt.Errorf("named pipes not supported: %s", absPath)
	}
	if mode&os.ModeSocket != 0 {
		return nil, fmt.Errorf("sockets not supported: %s", absPath)
	}

	return bt.NewPath(absPath, info.IsDir(), info), nil
}

// Open opens a file for reading.
func (m *OSFilesystemManager) Open(path *bt.Path) (io.ReadCloser, error) {
	if path.IsDir() {
		return nil, fmt.Errorf("cannot open directory as file: %s", path.String())
	}
	return os.Open(path.String())
}

// Stat returns fresh file info for a path.
func (m *OSFilesystemManager) Stat(path *bt.Path) (fs.FileInfo, error) {
	return os.Stat(path.String())
}

// FindFiles discovers regular files under the given directory path.
func (m *OSFilesystemManager) FindFiles(path *bt.Path, recursive bool) ([]*bt.Path, error) {
	if !path.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", path.String())
	}

	var paths []*bt.Path

	if recursive {
		err := filepath.WalkDir(path.String(), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !d.Type().IsRegular() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return fmt.Errorf("stat %s: %w", p, err)
			}
			paths = append(paths, bt.NewPath(p, false, info))
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walking directory: %w", err)
		}
	} else {
		entries, err := os.ReadDir(path.String())
		if err != nil {
			return nil, fmt.Errorf("reading directory: %w", err)
		}
		for _, entry := range entries {
			if !entry.Type().IsRegular() {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				return nil, fmt.Errorf("stat %s: %w", entry.Name(), err)
			}
			fullPath := filepath.Join(path.String(), entry.Name())
			paths = append(paths, bt.NewPath(fullPath, false, info))
		}
	}

	return paths, nil
}

// Compile-time check that OSFilesystemManager implements bt.FilesystemManager interface
var _ bt.FilesystemManager = (*OSFilesystemManager)(nil)
