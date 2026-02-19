package bt_test

import (
	"testing"
	"time"

	"bt-go/internal/bt"
	"bt-go/internal/testutil"
)

func TestBTService_GetStatus(t *testing.T) {
	// helper to set up a service with a tracked directory and files
	setup := func(t *testing.T) (*bt.BTService, *testutil.MockFilesystemManager, bt.Database, bt.StagingArea) {
		t.Helper()
		db := testutil.NewTestDatabase(t)
		fsmgr := testutil.NewMockFilesystemManager()
		staging := testutil.NewTestStagingArea(fsmgr)
		vault := testutil.NewTestVault()
		svc := bt.NewBTService(db, staging, vault, fsmgr)
		return svc, fsmgr, db, staging
	}

	t.Run("returns error for untracked directory", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr, _, _ := setup(t)
		fsmgr.AddDirectory("/home/user/docs")

		path, _ := fsmgr.Resolve("/home/user/docs")
		_, err := svc.GetStatus(path, false)
		if err == nil {
			t.Fatal("expected error for untracked directory")
		}
	})

	t.Run("returns error for file path", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr, _, _ := setup(t)
		fsmgr.AddFile("/home/user/file.txt", []byte("content"))

		path, _ := fsmgr.Resolve("/home/user/file.txt")
		_, err := svc.GetStatus(path, false)
		if err == nil {
			t.Fatal("expected error for file path")
		}
	})

	t.Run("shows untracked files", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr, _, _ := setup(t)

		fsmgr.AddDirectory("/home/user/docs")
		fsmgr.AddFile("/home/user/docs/file.txt", []byte("content"))

		dirPath, _ := fsmgr.Resolve("/home/user/docs")
		if err := svc.AddDirectory(dirPath); err != nil {
			t.Fatalf("AddDirectory() error = %v", err)
		}

		statuses, err := svc.GetStatus(dirPath, false)
		if err != nil {
			t.Fatalf("GetStatus() error = %v", err)
		}

		if len(statuses) != 1 {
			t.Fatalf("got %d statuses, want 1", len(statuses))
		}
		s := statuses[0]
		if s.RelativePath != "file.txt" {
			t.Errorf("RelativePath = %q, want %q", s.RelativePath, "file.txt")
		}
		if s.IsBackedUp {
			t.Error("expected IsBackedUp = false")
		}
		if s.IsStaged {
			t.Error("expected IsStaged = false")
		}
	})

	t.Run("shows staged files", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr, _, _ := setup(t)

		fsmgr.AddDirectory("/home/user/docs")
		fsmgr.AddFile("/home/user/docs/file.txt", []byte("content"))

		dirPath, _ := fsmgr.Resolve("/home/user/docs")
		svc.AddDirectory(dirPath)

		filePath, _ := fsmgr.Resolve("/home/user/docs/file.txt")
		if _, err := svc.StageFiles(filePath, false); err != nil {
			t.Fatalf("StageFiles() error = %v", err)
		}

		statuses, err := svc.GetStatus(dirPath, false)
		if err != nil {
			t.Fatalf("GetStatus() error = %v", err)
		}

		if len(statuses) != 1 {
			t.Fatalf("got %d statuses, want 1", len(statuses))
		}
		s := statuses[0]
		if !s.IsStaged {
			t.Error("expected IsStaged = true")
		}
		if s.IsBackedUp {
			t.Error("expected IsBackedUp = false")
		}
	})

	t.Run("shows backed up files", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr, _, _ := setup(t)

		fsmgr.AddDirectory("/home/user/docs")
		fsmgr.AddFile("/home/user/docs/file.txt", []byte("content"))

		dirPath, _ := fsmgr.Resolve("/home/user/docs")
		svc.AddDirectory(dirPath)

		filePath, _ := fsmgr.Resolve("/home/user/docs/file.txt")
		svc.StageFiles(filePath, false)

		if _, err := svc.BackupAll(); err != nil {
			t.Fatalf("BackupAll() error = %v", err)
		}

		statuses, err := svc.GetStatus(dirPath, false)
		if err != nil {
			t.Fatalf("GetStatus() error = %v", err)
		}

		if len(statuses) != 1 {
			t.Fatalf("got %d statuses, want 1", len(statuses))
		}
		s := statuses[0]
		if !s.IsBackedUp {
			t.Error("expected IsBackedUp = true")
		}
		if s.IsModifiedSince {
			t.Error("expected IsModifiedSince = false")
		}
	})

	t.Run("shows modified since backup", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr, _, _ := setup(t)

		fsmgr.AddDirectory("/home/user/docs")
		fsmgr.AddFile("/home/user/docs/file.txt", []byte("content"))

		dirPath, _ := fsmgr.Resolve("/home/user/docs")
		svc.AddDirectory(dirPath)

		filePath, _ := fsmgr.Resolve("/home/user/docs/file.txt")
		svc.StageFiles(filePath, false)
		svc.BackupAll()

		// Modify the file (change mtime)
		fsmgr.UpdateFile("/home/user/docs/file.txt", []byte("new content"), time.Now().Add(time.Hour))

		statuses, err := svc.GetStatus(dirPath, false)
		if err != nil {
			t.Fatalf("GetStatus() error = %v", err)
		}

		if len(statuses) != 1 {
			t.Fatalf("got %d statuses, want 1", len(statuses))
		}
		s := statuses[0]
		if !s.IsBackedUp {
			t.Error("expected IsBackedUp = true")
		}
		if !s.IsModifiedSince {
			t.Error("expected IsModifiedSince = true")
		}
	})

	t.Run("non-recursive excludes subdirectory files", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr, _, _ := setup(t)

		fsmgr.AddDirectory("/home/user/docs")
		fsmgr.AddFile("/home/user/docs/top.txt", []byte("top"))
		fsmgr.AddFile("/home/user/docs/sub/nested.txt", []byte("nested"))

		dirPath, _ := fsmgr.Resolve("/home/user/docs")
		svc.AddDirectory(dirPath)

		statuses, err := svc.GetStatus(dirPath, false)
		if err != nil {
			t.Fatalf("GetStatus() error = %v", err)
		}

		if len(statuses) != 1 {
			t.Fatalf("got %d statuses, want 1", len(statuses))
		}
		if statuses[0].RelativePath != "top.txt" {
			t.Errorf("got %q, want %q", statuses[0].RelativePath, "top.txt")
		}
	})

	t.Run("recursive includes subdirectory files", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr, _, _ := setup(t)

		fsmgr.AddDirectory("/home/user/docs")
		fsmgr.AddFile("/home/user/docs/top.txt", []byte("top"))
		fsmgr.AddFile("/home/user/docs/sub/nested.txt", []byte("nested"))

		dirPath, _ := fsmgr.Resolve("/home/user/docs")
		svc.AddDirectory(dirPath)

		statuses, err := svc.GetStatus(dirPath, true)
		if err != nil {
			t.Fatalf("GetStatus() error = %v", err)
		}

		if len(statuses) != 2 {
			t.Fatalf("got %d statuses, want 2", len(statuses))
		}

		paths := make(map[string]bool)
		for _, s := range statuses {
			paths[s.RelativePath] = true
		}
		if !paths["top.txt"] {
			t.Error("expected top.txt in results")
		}
		if !paths["sub/nested.txt"] {
			t.Error("expected sub/nested.txt in results")
		}
	})

	t.Run("works from subdirectory of tracked directory", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr, _, _ := setup(t)

		fsmgr.AddDirectory("/home/user/docs")
		fsmgr.AddDirectory("/home/user/docs/sub")
		fsmgr.AddFile("/home/user/docs/top.txt", []byte("top"))
		fsmgr.AddFile("/home/user/docs/sub/nested.txt", []byte("nested"))

		dirPath, _ := fsmgr.Resolve("/home/user/docs")
		svc.AddDirectory(dirPath)

		subPath, _ := fsmgr.Resolve("/home/user/docs/sub")
		statuses, err := svc.GetStatus(subPath, false)
		if err != nil {
			t.Fatalf("GetStatus() error = %v", err)
		}

		if len(statuses) != 1 {
			t.Fatalf("got %d statuses, want 1", len(statuses))
		}
		if statuses[0].RelativePath != "sub/nested.txt" {
			t.Errorf("got %q, want %q", statuses[0].RelativePath, "sub/nested.txt")
		}
	})
}
