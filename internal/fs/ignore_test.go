package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewIgnoreMatcher(t *testing.T) {
	t.Run("skips blank lines and comments", func(t *testing.T) {
		t.Parallel()
		m := NewIgnoreMatcher([]string{"", "  ", "# comment", "*.log"})
		if len(m.patterns) != 1 {
			t.Fatalf("expected 1 pattern, got %d", len(m.patterns))
		}
		if m.patterns[0].pattern != "*.log" {
			t.Errorf("expected *.log, got %s", m.patterns[0].pattern)
		}
	})

	t.Run("classifies path vs basename patterns", func(t *testing.T) {
		t.Parallel()
		m := NewIgnoreMatcher([]string{"*.log", "build/output"})
		if m.patterns[0].matchPath {
			t.Error("*.log should not be a path pattern")
		}
		if !m.patterns[1].matchPath {
			t.Error("build/output should be a path pattern")
		}
	})
}

func TestIgnoreMatcher_Match(t *testing.T) {
	tests := []struct {
		name         string
		patterns     []string
		relativePath string
		want         bool
	}{
		{
			name:         "basename glob matches file in root",
			patterns:     []string{"*.log"},
			relativePath: "app.log",
			want:         true,
		},
		{
			name:         "basename glob matches file in subdirectory",
			patterns:     []string{"*.log"},
			relativePath: filepath.Join("sub", "app.log"),
			want:         true,
		},
		{
			name:         "basename glob does not match different extension",
			patterns:     []string{"*.log"},
			relativePath: "app.txt",
			want:         false,
		},
		{
			name:         "exact basename match",
			patterns:     []string{".btignore"},
			relativePath: ".btignore",
			want:         true,
		},
		{
			name:         "exact basename matches in subdirectory",
			patterns:     []string{".DS_Store"},
			relativePath: filepath.Join("sub", ".DS_Store"),
			want:         true,
		},
		{
			name:         "path pattern matches exact relative path",
			patterns:     []string{"build/output"},
			relativePath: filepath.Join("build", "output"),
			want:         true,
		},
		{
			name:         "path pattern does not match wrong path",
			patterns:     []string{"build/output"},
			relativePath: filepath.Join("src", "output"),
			want:         false,
		},
		{
			name:         "path pattern with glob",
			patterns:     []string{"build/*.o"},
			relativePath: filepath.Join("build", "main.o"),
			want:         true,
		},
		{
			name:         "question mark wildcard",
			patterns:     []string{"?.txt"},
			relativePath: "a.txt",
			want:         true,
		},
		{
			name:         "question mark does not match multiple chars",
			patterns:     []string{"?.txt"},
			relativePath: "ab.txt",
			want:         false,
		},
		{
			name:         "character class",
			patterns:     []string{"*.[oa]"},
			relativePath: "main.o",
			want:         true,
		},
		{
			name:         "no patterns matches nothing",
			patterns:     nil,
			relativePath: "anything.txt",
			want:         false,
		},
		{
			name:         "empty string path",
			patterns:     []string{"*.log"},
			relativePath: "",
			want:         false,
		},
		{
			name:         "multiple patterns first matches",
			patterns:     []string{"*.log", "*.tmp"},
			relativePath: "debug.log",
			want:         true,
		},
		{
			name:         "multiple patterns second matches",
			patterns:     []string{"*.log", "*.tmp"},
			relativePath: "data.tmp",
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := NewIgnoreMatcher(tt.patterns)
			got := m.Match(tt.relativePath)
			if got != tt.want {
				t.Errorf("Match(%q) = %v, want %v", tt.relativePath, got, tt.want)
			}
		})
	}
}

func TestParseIgnoreFile(t *testing.T) {
	t.Run("reads patterns from file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, ".btignore")
		content := "*.log\n# comment\n\n*.tmp\nbuild/output\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("writing test file: %v", err)
		}

		patterns, err := ParseIgnoreFile(path)
		if err != nil {
			t.Fatalf("ParseIgnoreFile() error = %v", err)
		}
		if len(patterns) != 5 { // includes blank and comment lines — filtering is NewIgnoreMatcher's job
			t.Fatalf("expected 5 raw lines, got %d", len(patterns))
		}

		// Verify the matcher filters correctly
		m := NewIgnoreMatcher(patterns)
		if len(m.patterns) != 3 {
			t.Errorf("expected 3 parsed patterns, got %d", len(m.patterns))
		}
	})

	t.Run("returns nil for missing file", func(t *testing.T) {
		t.Parallel()
		patterns, err := ParseIgnoreFile("/nonexistent/.btignore")
		if err != nil {
			t.Fatalf("ParseIgnoreFile() error = %v", err)
		}
		if patterns != nil {
			t.Errorf("expected nil patterns, got %v", patterns)
		}
	})
}
