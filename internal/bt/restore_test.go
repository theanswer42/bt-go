package bt_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bt-go/internal/bt"
	"bt-go/internal/testutil"
)

// setupRestore creates a service backed by a real temp directory so
// that restore can write files to disk.
func setupRestore(t *testing.T) (*bt.BTService, *testutil.MockFilesystemManager, string) {
	t.Helper()
	db := testutil.NewTestDatabase(t)
	fsmgr := testutil.NewMockFilesystemManager()
	staging := testutil.NewTestStagingArea(fsmgr)
	vault := testutil.NewTestVault()
	svc := bt.NewBTService(db, staging, vault, fsmgr, bt.NewNopLogger(), bt.RealClock{}, bt.UUIDGenerator{})

	dir := t.TempDir()
	return svc, fsmgr, dir
}

// backupOneFile is a helper that adds a directory, adds a file, stages, and backs it up.
func backupOneFile(t *testing.T, svc *bt.BTService, fsmgr *testutil.MockFilesystemManager, dirPath string, relPath string, content []byte) {
	t.Helper()

	fsmgr.AddDirectory(dirPath)
	fullPath := filepath.Join(dirPath, relPath)
	// Ensure parent dir exists in mock if relPath has subdirs
	fsmgr.AddFile(fullPath, content)

	dirP, err := fsmgr.Resolve(dirPath)
	if err != nil {
		t.Fatalf("resolve dir: %v", err)
	}
	if err := svc.AddDirectory(dirP); err != nil {
		t.Fatalf("add directory: %v", err)
	}

	fileP, err := fsmgr.Resolve(fullPath)
	if err != nil {
		t.Fatalf("resolve file: %v", err)
	}
	if _, err := svc.StageFiles(fileP, false); err != nil {
		t.Fatalf("stage: %v", err)
	}
	if _, err := svc.BackupAll(); err != nil {
		t.Fatalf("backup: %v", err)
	}
}

func TestBTService_Restore(t *testing.T) {
	t.Run("restore file without checksum uses current snapshot", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr, dir := setupRestore(t)

		content := []byte("hello world")
		backupOneFile(t, svc, fsmgr, dir, "file.txt", content)

		paths, err := svc.Restore(filepath.Join(dir, "file.txt"), "")
		if err != nil {
			t.Fatalf("Restore() error = %v", err)
		}
		if len(paths) != 1 {
			t.Fatalf("got %d paths, want 1", len(paths))
		}

		// Verify output file name pattern
		if !strings.HasSuffix(paths[0], ".btrestored") {
			t.Errorf("output path %s does not end with .btrestored", paths[0])
		}

		// Verify content
		got, err := os.ReadFile(paths[0])
		if err != nil {
			t.Fatalf("reading restored file: %v", err)
		}
		if string(got) != string(content) {
			t.Errorf("content = %q, want %q", string(got), string(content))
		}
	})

	t.Run("restore file with checksum restores specific version", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr, dir := setupRestore(t)

		v1 := []byte("version one")
		backupOneFile(t, svc, fsmgr, dir, "file.txt", v1)

		// Get the checksum of v1 via file history
		filePath, _ := fsmgr.Resolve(filepath.Join(dir, "file.txt"))
		entries, err := svc.GetFileHistory(filePath)
		if err != nil {
			t.Fatalf("GetFileHistory() error = %v", err)
		}
		v1Checksum := entries[0].ContentChecksum

		// Backup v2
		fsmgr.UpdateFile(filepath.Join(dir, "file.txt"), []byte("version two"), time.Now().Add(time.Hour))
		filePath, _ = fsmgr.Resolve(filepath.Join(dir, "file.txt"))
		svc.StageFiles(filePath, false)
		svc.BackupAll()

		// Restore v1 by checksum
		paths, err := svc.Restore(filepath.Join(dir, "file.txt"), v1Checksum)
		if err != nil {
			t.Fatalf("Restore() error = %v", err)
		}

		got, err := os.ReadFile(paths[0])
		if err != nil {
			t.Fatalf("reading restored file: %v", err)
		}
		if string(got) != string(v1) {
			t.Errorf("content = %q, want %q", string(got), string(v1))
		}

		// Verify checksum is in the filename
		if !strings.Contains(paths[0], v1Checksum[:12]) {
			t.Errorf("output path %s does not contain checksum prefix %s", paths[0], v1Checksum[:12])
		}
	})

	t.Run("restore untracked file returns error", func(t *testing.T) {
		t.Parallel()
		svc, _, dir := setupRestore(t)

		_, err := svc.Restore(filepath.Join(dir, "nope.txt"), "")
		if err == nil {
			t.Fatal("expected error for untracked file")
		}
	})

	t.Run("restore file with no backup returns error", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr, dir := setupRestore(t)

		fsmgr.AddDirectory(dir)
		dirP, _ := fsmgr.Resolve(dir)
		svc.AddDirectory(dirP)

		// File is tracked in dir but never backed up
		_, err := svc.Restore(filepath.Join(dir, "missing.txt"), "")
		if err == nil {
			t.Fatal("expected error for file with no backup")
		}
	})

	t.Run("restore directory restores all files", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr, dir := setupRestore(t)

		backupOneFile(t, svc, fsmgr, dir, "a.txt", []byte("aaa"))

		// Add and backup a second file
		fsmgr.AddFile(filepath.Join(dir, "b.txt"), []byte("bbb"))
		fileP, _ := fsmgr.Resolve(filepath.Join(dir, "b.txt"))
		svc.StageFiles(fileP, false)
		svc.BackupAll()

		paths, err := svc.Restore(dir, "")
		if err != nil {
			t.Fatalf("Restore() error = %v", err)
		}
		if len(paths) != 2 {
			t.Fatalf("got %d paths, want 2", len(paths))
		}

		// All outputs should end with .btrestored
		for _, p := range paths {
			if !strings.HasSuffix(p, ".btrestored") {
				t.Errorf("path %s does not end with .btrestored", p)
			}
			if _, err := os.Stat(p); err != nil {
				t.Errorf("restored file does not exist: %s", p)
			}
		}
	})

	t.Run("restore directory with checksum returns error", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr, dir := setupRestore(t)

		backupOneFile(t, svc, fsmgr, dir, "file.txt", []byte("data"))

		_, err := svc.Restore(dir, "somechecksum")
		if err == nil {
			t.Fatal("expected error for directory + checksum")
		}
	})

	t.Run("restore fails if output file already exists", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr, dir := setupRestore(t)

		backupOneFile(t, svc, fsmgr, dir, "file.txt", []byte("data"))

		// First restore succeeds
		paths, err := svc.Restore(filepath.Join(dir, "file.txt"), "")
		if err != nil {
			t.Fatalf("first Restore() error = %v", err)
		}

		// Verify the file exists
		if _, err := os.Stat(paths[0]); err != nil {
			t.Fatalf("restored file missing: %v", err)
		}

		// Second restore of same file+version should fail
		_, err = svc.Restore(filepath.Join(dir, "file.txt"), "")
		if err == nil {
			t.Fatal("expected error when output file already exists")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("error = %v, want 'already exists'", err)
		}
	})

	t.Run("restore file with bad checksum returns error", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr, dir := setupRestore(t)

		backupOneFile(t, svc, fsmgr, dir, "file.txt", []byte("data"))

		_, err := svc.Restore(filepath.Join(dir, "file.txt"), "nonexistentchecksum")
		if err == nil {
			t.Fatal("expected error for bad checksum")
		}
	})
}
