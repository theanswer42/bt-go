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
	baseMatcher *IgnoreMatcher // hard-coded + config patterns
}

// NewOSFilesystemManager creates a new filesystem manager that operates on the real filesystem.
// configPatterns are ignore patterns from the user's config file.
func NewOSFilesystemManager(configPatterns []string) *OSFilesystemManager {
	allBase := append(defaultIgnorePatterns, configPatterns...)
	return &OSFilesystemManager{
		baseMatcher: NewIgnoreMatcher(allBase),
	}
}

// combinedMatcher builds an IgnoreMatcher that includes the base patterns
// plus any .btignore patterns found in the given directory.
func (m *OSFilesystemManager) combinedMatcher(dirPath string) *IgnoreMatcher {
	btignorePath := filepath.Join(dirPath, ".btignore")
	btignorePatterns, err := ParseIgnoreFile(btignorePath)
	if err != nil || len(btignorePatterns) == 0 {
		return m.baseMatcher
	}
	allPatterns := make([]string, 0, len(defaultIgnorePatterns)+len(btignorePatterns))
	for _, p := range m.baseMatcher.patterns {
		allPatterns = append(allPatterns, p.pattern)
	}
	allPatterns = append(allPatterns, btignorePatterns...)
	return NewIgnoreMatcher(allPatterns)
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
// Ignored files are excluded based on hard-coded, config, and .btignore patterns.
func (m *OSFilesystemManager) FindFiles(path *bt.Path, recursive bool) ([]*bt.Path, error) {
	if !path.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", path.String())
	}

	matcher := m.combinedMatcher(path.String())
	dirRoot := path.String()
	var paths []*bt.Path

	if recursive {
		err := filepath.WalkDir(dirRoot, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !d.Type().IsRegular() {
				return nil
			}
			rel, err := filepath.Rel(dirRoot, p)
			if err != nil {
				return fmt.Errorf("computing relative path: %w", err)
			}
			if matcher.Match(rel) {
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
		entries, err := os.ReadDir(dirRoot)
		if err != nil {
			return nil, fmt.Errorf("reading directory: %w", err)
		}
		for _, entry := range entries {
			if !entry.Type().IsRegular() {
				continue
			}
			if matcher.Match(entry.Name()) {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				return nil, fmt.Errorf("stat %s: %w", entry.Name(), err)
			}
			fullPath := filepath.Join(dirRoot, entry.Name())
			paths = append(paths, bt.NewPath(fullPath, false, info))
		}
	}

	return paths, nil
}

// IsIgnored checks whether a file path should be ignored based on
// ignore rules (hard-coded patterns, config patterns, and .btignore in dirRoot).
func (m *OSFilesystemManager) IsIgnored(path *bt.Path, dirRoot string) (bool, error) {
	rel, err := filepath.Rel(dirRoot, path.String())
	if err != nil {
		return false, fmt.Errorf("computing relative path: %w", err)
	}
	matcher := m.combinedMatcher(dirRoot)
	return matcher.Match(rel), nil
}

// Compile-time check that OSFilesystemManager implements bt.FilesystemManager interface
var _ bt.FilesystemManager = (*OSFilesystemManager)(nil)
