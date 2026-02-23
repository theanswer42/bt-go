package staging

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bt-go/internal/bt"
	"bt-go/internal/database/sqlc"
)

// mockFSMgr is a minimal filesystem mock for staging tests.
type mockFSMgr struct {
	files map[string]*mockEntry
}

type mockEntry struct {
	content []byte
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
}

func newMockFSMgr() *mockFSMgr {
	return &mockFSMgr{files: make(map[string]*mockEntry)}
}

func (m *mockFSMgr) addFile(path string, content []byte) {
	now := time.Now()
	m.files[path] = &mockEntry{content: content, mode: 0644, modTime: now}
}

func (m *mockFSMgr) Resolve(rawPath string) (*bt.Path, error) {
	absPath, _ := filepath.Abs(rawPath)
	e, ok := m.files[absPath]
	if !ok {
		return nil, fmt.Errorf("not found: %s", absPath)
	}
	info := &mockFileInfo{name: filepath.Base(absPath), entry: e}
	return bt.NewPath(absPath, e.isDir, info), nil
}

func (m *mockFSMgr) Open(path *bt.Path) (io.ReadCloser, error) {
	e, ok := m.files[path.String()]
	if !ok {
		return nil, fmt.Errorf("not found: %s", path.String())
	}
	return io.NopCloser(bytes.NewReader(e.content)), nil
}

func (m *mockFSMgr) Stat(path *bt.Path) (fs.FileInfo, error) {
	e, ok := m.files[path.String()]
	if !ok {
		return nil, fmt.Errorf("not found: %s", path.String())
	}
	return &mockFileInfo{name: filepath.Base(path.String()), entry: e}, nil
}

func (m *mockFSMgr) ExtractStatData(info fs.FileInfo) (*bt.StatData, error) {
	mfi, ok := info.(*mockFileInfo)
	if !ok {
		return nil, fmt.Errorf("unexpected type")
	}
	return &bt.StatData{
		UID:       1000,
		GID:       1000,
		Atime:     mfi.entry.modTime,
		Ctime:     mfi.entry.modTime,
		BirthTime: sql.NullTime{Valid: false},
	}, nil
}

func (m *mockFSMgr) FindFiles(path *bt.Path, recursive bool) ([]*bt.Path, error) {
	return nil, nil
}

func (m *mockFSMgr) IsIgnored(path *bt.Path, dirRoot string) (bool, error) {
	return false, nil
}

type mockFileInfo struct {
	name  string
	entry *mockEntry
}

func (m *mockFileInfo) Name() string      { return m.name }
func (m *mockFileInfo) Size() int64       { return int64(len(m.entry.content)) }
func (m *mockFileInfo) Mode() fs.FileMode { return m.entry.mode }
func (m *mockFileInfo) ModTime() time.Time { return m.entry.modTime }
func (m *mockFileInfo) IsDir() bool       { return m.entry.isDir }
func (m *mockFileInfo) Sys() any          { return m.entry }

var _ bt.FilesystemManager = (*mockFSMgr)(nil)

// helpers

func newTestSA(t *testing.T) (*stagingArea, *mockFSMgr) {
	t.Helper()
	fsmgr := newMockFSMgr()
	sa := NewMemoryStagingArea(fsmgr, 10*1024*1024).(*stagingArea)
	return sa, fsmgr
}

func stageFile(t *testing.T, sa *stagingArea, fsmgr *mockFSMgr, dir *sqlc.Directory, relPath string, content []byte) {
	t.Helper()
	fullPath := dir.Path + "/" + relPath
	fsmgr.addFile(fullPath, content)
	path, err := fsmgr.Resolve(fullPath)
	if err != nil {
		t.Fatalf("resolve %s: %v", fullPath, err)
	}
	if err := sa.Stage(dir, relPath, path); err != nil {
		t.Fatalf("stage %s: %v", relPath, err)
	}
}

// Tests

func TestStagingArea_Stage(t *testing.T) {
	dir := &sqlc.Directory{ID: "dir-1", Path: "/home/user/docs", CreatedAt: time.Now()}

	t.Run("stages a file and increments count", func(t *testing.T) {
		sa, fsmgr := newTestSA(t)
		stageFile(t, sa, fsmgr, dir, "file.txt", []byte("hello"))

		count, err := sa.Count()
		if err != nil {
			t.Fatalf("Count() error = %v", err)
		}
		if count != 1 {
			t.Errorf("Count() = %d, want 1", count)
		}
	})

	t.Run("size reflects staged content", func(t *testing.T) {
		sa, fsmgr := newTestSA(t)
		stageFile(t, sa, fsmgr, dir, "file.txt", []byte("hello"))

		size, err := sa.Size()
		if err != nil {
			t.Fatalf("Size() error = %v", err)
		}
		if size != 5 {
			t.Errorf("Size() = %d, want 5", size)
		}
	})

	t.Run("deduplicates identical content", func(t *testing.T) {
		sa, fsmgr := newTestSA(t)
		content := []byte("same content")
		stageFile(t, sa, fsmgr, dir, "a.txt", content)
		stageFile(t, sa, fsmgr, dir, "b.txt", content)

		count, _ := sa.Count()
		if count != 2 {
			t.Errorf("Count() = %d, want 2", count)
		}

		size, _ := sa.Size()
		if size != int64(len(content)) {
			t.Errorf("Size() = %d, want %d (deduped)", size, len(content))
		}
	})
}

func TestStagingArea_IsStaged(t *testing.T) {
	dir := &sqlc.Directory{ID: "dir-1", Path: "/home/user/docs", CreatedAt: time.Now()}

	t.Run("returns true for staged file", func(t *testing.T) {
		sa, fsmgr := newTestSA(t)
		stageFile(t, sa, fsmgr, dir, "file.txt", []byte("data"))

		staged, err := sa.IsStaged("dir-1", "file.txt")
		if err != nil {
			t.Fatalf("IsStaged() error = %v", err)
		}
		if !staged {
			t.Error("IsStaged() = false, want true")
		}
	})

	t.Run("returns false for unstaged file", func(t *testing.T) {
		sa, _ := newTestSA(t)

		staged, err := sa.IsStaged("dir-1", "missing.txt")
		if err != nil {
			t.Fatalf("IsStaged() error = %v", err)
		}
		if staged {
			t.Error("IsStaged() = true, want false")
		}
	})
}

func TestStagingArea_ProcessNext(t *testing.T) {
	dir := &sqlc.Directory{ID: "dir-1", Path: "/home/user/docs", CreatedAt: time.Now()}

	t.Run("processes and removes on success", func(t *testing.T) {
		sa, fsmgr := newTestSA(t)
		stageFile(t, sa, fsmgr, dir, "file.txt", []byte("hello"))

		var gotRelPath string
		err := sa.ProcessNext(func(content io.Reader, snapshot sqlc.FileSnapshot, directoryID string, relativePath string) error {
			gotRelPath = relativePath
			return nil
		})
		if err != nil {
			t.Fatalf("ProcessNext() error = %v", err)
		}
		if gotRelPath != "file.txt" {
			t.Errorf("relative path = %q, want %q", gotRelPath, "file.txt")
		}

		count, _ := sa.Count()
		if count != 0 {
			t.Errorf("Count() after process = %d, want 0", count)
		}
	})

	t.Run("retains operation on callback error", func(t *testing.T) {
		sa, fsmgr := newTestSA(t)
		stageFile(t, sa, fsmgr, dir, "file.txt", []byte("hello"))

		err := sa.ProcessNext(func(content io.Reader, snapshot sqlc.FileSnapshot, directoryID string, relativePath string) error {
			return fmt.Errorf("simulated failure")
		})
		if err == nil {
			t.Fatal("ProcessNext() expected error")
		}

		count, _ := sa.Count()
		if count != 1 {
			t.Errorf("Count() after failed process = %d, want 1", count)
		}
	})

	t.Run("empty queue returns no error", func(t *testing.T) {
		sa, _ := newTestSA(t)

		err := sa.ProcessNext(func(content io.Reader, snapshot sqlc.FileSnapshot, directoryID string, relativePath string) error {
			t.Fatal("callback should not be called on empty queue")
			return nil
		})
		if err != nil {
			t.Fatalf("ProcessNext() error = %v", err)
		}
	})

	t.Run("snapshot has correct metadata", func(t *testing.T) {
		sa, fsmgr := newTestSA(t)
		stageFile(t, sa, fsmgr, dir, "file.txt", []byte("hello"))

		err := sa.ProcessNext(func(content io.Reader, snapshot sqlc.FileSnapshot, directoryID string, relativePath string) error {
			if snapshot.Size != 5 {
				t.Errorf("snapshot.Size = %d, want 5", snapshot.Size)
			}
			if snapshot.ContentID == "" {
				t.Error("snapshot.ContentID is empty")
			}
			if snapshot.Permissions != 0644 {
				t.Errorf("snapshot.Permissions = %o, want 644", snapshot.Permissions)
			}
			if directoryID != "dir-1" {
				t.Errorf("directoryID = %q, want %q", directoryID, "dir-1")
			}
			return nil
		})
		if err != nil {
			t.Fatalf("ProcessNext() error = %v", err)
		}
	})
}

func TestStagingArea_SizeLimit(t *testing.T) {
	dir := &sqlc.Directory{ID: "dir-1", Path: "/home/user/docs", CreatedAt: time.Now()}

	fsmgr := newMockFSMgr()
	sa := NewMemoryStagingArea(fsmgr, 10).(*stagingArea) // 10 bytes max

	// Stage a small file that fits
	stageFile(t, sa, fsmgr, dir, "small.txt", []byte("hi"))

	// Stage a file that would exceed the limit
	fsmgr.addFile("/home/user/docs/big.txt", []byte("this is way too big"))
	path, _ := fsmgr.Resolve("/home/user/docs/big.txt")
	err := sa.Stage(dir, "big.txt", path)
	if err == nil {
		t.Fatal("expected error when exceeding size limit")
	}
	if !strings.Contains(err.Error(), "staging area full") {
		t.Errorf("error = %v, want 'staging area full'", err)
	}
}

func TestValidateStatUnchanged(t *testing.T) {
	now := time.Now()

	baseStat := func() *bt.StatData {
		return &bt.StatData{
			UID:       1000,
			GID:       1000,
			Atime:     now,
			Ctime:     now,
			BirthTime: sql.NullTime{Valid: false},
		}
	}
	baseInfo := func() *mockFileInfo {
		return &mockFileInfo{
			name:  "test",
			entry: &mockEntry{content: make([]byte, 100), mode: 0644, modTime: now},
		}
	}

	t.Run("identical stats pass", func(t *testing.T) {
		if err := validateStatUnchanged(baseInfo(), baseInfo(), baseStat(), baseStat()); err != nil {
			t.Errorf("expected nil, got: %v", err)
		}
	})

	t.Run("size change detected", func(t *testing.T) {
		info2 := baseInfo()
		info2.entry = &mockEntry{content: make([]byte, 200), mode: 0644, modTime: now}
		if err := validateStatUnchanged(baseInfo(), info2, baseStat(), baseStat()); err == nil {
			t.Error("expected error for size change")
		}
	})

	t.Run("mode change detected", func(t *testing.T) {
		info2 := baseInfo()
		info2.entry = &mockEntry{content: make([]byte, 100), mode: 0755, modTime: now}
		if err := validateStatUnchanged(baseInfo(), info2, baseStat(), baseStat()); err == nil {
			t.Error("expected error for mode change")
		}
	})

	t.Run("mtime change detected", func(t *testing.T) {
		info2 := baseInfo()
		info2.entry = &mockEntry{content: make([]byte, 100), mode: 0644, modTime: now.Add(time.Hour)}
		if err := validateStatUnchanged(baseInfo(), info2, baseStat(), baseStat()); err == nil {
			t.Error("expected error for mtime change")
		}
	})

	t.Run("ctime change detected", func(t *testing.T) {
		stat2 := baseStat()
		stat2.Ctime = now.Add(time.Second)
		if err := validateStatUnchanged(baseInfo(), baseInfo(), baseStat(), stat2); err == nil {
			t.Error("expected error for ctime change")
		}
	})

	t.Run("uid change detected", func(t *testing.T) {
		stat2 := baseStat()
		stat2.UID = 9999
		if err := validateStatUnchanged(baseInfo(), baseInfo(), baseStat(), stat2); err == nil {
			t.Error("expected error for uid change")
		}
	})

	t.Run("gid change detected", func(t *testing.T) {
		stat2 := baseStat()
		stat2.GID = 9999
		if err := validateStatUnchanged(baseInfo(), baseInfo(), baseStat(), stat2); err == nil {
			t.Error("expected error for gid change")
		}
	})
}
