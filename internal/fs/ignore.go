package fs

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// defaultIgnorePatterns are always applied regardless of config or .btignore.
var defaultIgnorePatterns = []string{".btignore"}

// ignorePattern is a parsed ignore pattern with its matching strategy.
type ignorePattern struct {
	pattern   string
	matchPath bool // true = match against relative path; false = match against basename only
}

// IgnoreMatcher checks file paths against a set of ignore patterns.
// Patterns without '/' match against the file's basename only.
// Patterns with '/' match against the full relative path from the directory root.
type IgnoreMatcher struct {
	patterns []ignorePattern
}

// NewIgnoreMatcher creates an IgnoreMatcher from raw pattern strings.
// Blank lines and lines starting with '#' are skipped.
func NewIgnoreMatcher(rawPatterns []string) *IgnoreMatcher {
	var patterns []ignorePattern
	for _, raw := range rawPatterns {
		raw = strings.TrimSpace(raw)
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		patterns = append(patterns, ignorePattern{
			pattern:   raw,
			matchPath: strings.Contains(raw, "/"),
		})
	}
	return &IgnoreMatcher{patterns: patterns}
}

// Match reports whether the given relative path should be ignored.
// relativePath should use filepath separators and be relative to the directory root.
func (m *IgnoreMatcher) Match(relativePath string) bool {
	if len(m.patterns) == 0 {
		return false
	}

	// Normalize to forward slashes for consistent matching.
	normalized := filepath.ToSlash(relativePath)
	basename := filepath.Base(relativePath)

	for _, p := range m.patterns {
		var matched bool
		var err error
		if p.matchPath {
			matched, err = filepath.Match(p.pattern, normalized)
		} else {
			matched, err = filepath.Match(p.pattern, basename)
		}
		if err != nil {
			// Bad pattern — skip rather than crash.
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

// ParseIgnoreFile reads a .btignore file and returns the raw pattern strings.
// Returns nil and no error if the file does not exist.
func ParseIgnoreFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("opening ignore file: %w", err)
	}
	defer f.Close()

	var patterns []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		patterns = append(patterns, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading ignore file: %w", err)
	}
	return patterns, nil
}
