package bt_test

import (
	"testing"

	"bt-go/internal/bt"
	"bt-go/internal/testutil"
)

func TestBTService_AddDirectory(t *testing.T) {
	t.Run("adds new directory", func(t *testing.T) {
		db := testutil.NewTestDatabase(t)
		fsmgr := testutil.NewMockFilesystemManager()
		fsmgr.AddDirectory("/home/user/docs")

		svc := bt.NewBTService(db, nil, nil, fsmgr)

		path, _ := fsmgr.Resolve("/home/user/docs")
		err := svc.AddDirectory(path)
		if err != nil {
			t.Fatalf("AddDirectory() error = %v", err)
		}

		// Verify directory was created by finding it in the database
		dir, err := db.FindDirectoryByPath("/home/user/docs")
		if err != nil {
			t.Fatalf("FindDirectoryByPath() error = %v", err)
		}
		if dir == nil {
			t.Error("directory was not created in database")
		}
	})

	t.Run("returns error for non-directory path", func(t *testing.T) {
		db := testutil.NewTestDatabase(t)
		fsmgr := testutil.NewMockFilesystemManager()
		fsmgr.AddFile("/home/user/file.txt", []byte("content"))

		svc := bt.NewBTService(db, nil, nil, fsmgr)

		path, _ := fsmgr.Resolve("/home/user/file.txt")
		err := svc.AddDirectory(path)
		if err == nil {
			t.Error("AddDirectory() expected error for non-directory path")
		}
	})

	t.Run("is idempotent for existing directory", func(t *testing.T) {
		db := testutil.NewTestDatabase(t)
		fsmgr := testutil.NewMockFilesystemManager()
		fsmgr.AddDirectory("/home/user/docs")

		svc := bt.NewBTService(db, nil, nil, fsmgr)

		path, _ := fsmgr.Resolve("/home/user/docs")

		// Add directory first time
		if err := svc.AddDirectory(path); err != nil {
			t.Fatalf("first AddDirectory() error = %v", err)
		}

		// Add directory second time - should succeed (idempotent)
		if err := svc.AddDirectory(path); err != nil {
			t.Fatalf("second AddDirectory() error = %v", err)
		}
	})
}

func TestBTService_StageFiles(t *testing.T) {
	setup := func(t *testing.T) (*bt.BTService, *testutil.MockFilesystemManager) {
		t.Helper()
		db := testutil.NewTestDatabase(t)
		fsmgr := testutil.NewMockFilesystemManager()
		staging := testutil.NewTestStagingArea(fsmgr)
		svc := bt.NewBTService(db, staging, nil, fsmgr)
		return svc, fsmgr
	}

	t.Run("stages a single file", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr := setup(t)

		fsmgr.AddDirectory("/home/user/docs")
		fsmgr.AddFile("/home/user/docs/file.txt", []byte("content"))

		dirPath, _ := fsmgr.Resolve("/home/user/docs")
		svc.AddDirectory(dirPath)

		filePath, _ := fsmgr.Resolve("/home/user/docs/file.txt")
		count, err := svc.StageFiles(filePath, false)
		if err != nil {
			t.Fatalf("StageFiles() error = %v", err)
		}
		if count != 1 {
			t.Errorf("StageFiles() count = %d, want 1", count)
		}
	})

	t.Run("stages all top-level files in directory", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr := setup(t)

		fsmgr.AddDirectory("/home/user/docs")
		fsmgr.AddFile("/home/user/docs/a.txt", []byte("aaa"))
		fsmgr.AddFile("/home/user/docs/b.txt", []byte("bbb"))
		fsmgr.AddFile("/home/user/docs/sub/c.txt", []byte("ccc"))

		dirPath, _ := fsmgr.Resolve("/home/user/docs")
		svc.AddDirectory(dirPath)

		count, err := svc.StageFiles(dirPath, false)
		if err != nil {
			t.Fatalf("StageFiles() error = %v", err)
		}
		if count != 2 {
			t.Errorf("StageFiles() count = %d, want 2", count)
		}
	})

	t.Run("recursive stages files in subdirectories", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr := setup(t)

		fsmgr.AddDirectory("/home/user/docs")
		fsmgr.AddFile("/home/user/docs/a.txt", []byte("aaa"))
		fsmgr.AddFile("/home/user/docs/sub/b.txt", []byte("bbb"))
		fsmgr.AddFile("/home/user/docs/sub/deep/c.txt", []byte("ccc"))

		dirPath, _ := fsmgr.Resolve("/home/user/docs")
		svc.AddDirectory(dirPath)

		count, err := svc.StageFiles(dirPath, true)
		if err != nil {
			t.Fatalf("StageFiles() error = %v", err)
		}
		if count != 3 {
			t.Errorf("StageFiles() count = %d, want 3", count)
		}
	})

	t.Run("returns error for untracked directory", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr := setup(t)

		fsmgr.AddDirectory("/home/user/docs")
		fsmgr.AddFile("/home/user/docs/file.txt", []byte("content"))

		dirPath, _ := fsmgr.Resolve("/home/user/docs")
		_, err := svc.StageFiles(dirPath, false)
		if err == nil {
			t.Fatal("expected error for untracked directory")
		}
	})

	t.Run("empty directory stages zero files", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr := setup(t)

		fsmgr.AddDirectory("/home/user/docs")

		dirPath, _ := fsmgr.Resolve("/home/user/docs")
		svc.AddDirectory(dirPath)

		count, err := svc.StageFiles(dirPath, false)
		if err != nil {
			t.Fatalf("StageFiles() error = %v", err)
		}
		if count != 0 {
			t.Errorf("StageFiles() count = %d, want 0", count)
		}
	})

	t.Run("returns error for file not in tracked directory", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr := setup(t)

		fsmgr.AddFile("/home/user/untracked/file.txt", []byte("content"))

		filePath, _ := fsmgr.Resolve("/home/user/untracked/file.txt")
		_, err := svc.StageFiles(filePath, false)
		if err == nil {
			t.Fatal("expected error for file not in tracked directory")
		}
	})

	t.Run("returns error when staging an ignored file directly", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr := setup(t)
		fsmgr.SetIgnorePatterns([]string{"*.log"})

		fsmgr.AddDirectory("/home/user/docs")
		fsmgr.AddFile("/home/user/docs/app.log", []byte("log data"))

		dirPath, _ := fsmgr.Resolve("/home/user/docs")
		svc.AddDirectory(dirPath)

		filePath, _ := fsmgr.Resolve("/home/user/docs/app.log")
		_, err := svc.StageFiles(filePath, false)
		if err == nil {
			t.Fatal("expected error when staging ignored file")
		}
	})

	t.Run("directory staging excludes ignored files", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr := setup(t)
		fsmgr.SetIgnorePatterns([]string{"*.log"})

		fsmgr.AddDirectory("/home/user/docs")
		fsmgr.AddFile("/home/user/docs/readme.txt", []byte("hello"))
		fsmgr.AddFile("/home/user/docs/app.log", []byte("log data"))
		fsmgr.AddFile("/home/user/docs/error.log", []byte("errors"))

		dirPath, _ := fsmgr.Resolve("/home/user/docs")
		svc.AddDirectory(dirPath)

		count, err := svc.StageFiles(dirPath, false)
		if err != nil {
			t.Fatalf("StageFiles() error = %v", err)
		}
		if count != 1 {
			t.Errorf("StageFiles() count = %d, want 1 (only readme.txt)", count)
		}
	})

	t.Run("non-ignored file stages successfully with ignore patterns set", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr := setup(t)
		fsmgr.SetIgnorePatterns([]string{"*.log"})

		fsmgr.AddDirectory("/home/user/docs")
		fsmgr.AddFile("/home/user/docs/readme.txt", []byte("hello"))

		dirPath, _ := fsmgr.Resolve("/home/user/docs")
		svc.AddDirectory(dirPath)

		filePath, _ := fsmgr.Resolve("/home/user/docs/readme.txt")
		count, err := svc.StageFiles(filePath, false)
		if err != nil {
			t.Fatalf("StageFiles() error = %v", err)
		}
		if count != 1 {
			t.Errorf("StageFiles() count = %d, want 1", count)
		}
	})
}
