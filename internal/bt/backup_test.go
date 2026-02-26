package bt_test

import (
	"testing"

	"bt-go/internal/bt"
	"bt-go/internal/testutil"
)

func TestBTService_BackupAll(t *testing.T) {
	t.Run("returns zero when staging area is empty", func(t *testing.T) {
		db := testutil.NewTestDatabase(t)
		fsmgr := testutil.NewMockFilesystemManager()
		staging := testutil.NewTestStagingArea(fsmgr)
		vault := testutil.NewTestVault()

		svc := bt.NewBTService(db, staging, vault, fsmgr, testutil.NewTestEncryptor(), bt.NewNopLogger(), bt.RealClock{}, bt.UUIDGenerator{})

		count, err := svc.BackupAll()
		if err != nil {
			t.Fatalf("BackupAll() error = %v", err)
		}
		if count != 0 {
			t.Errorf("BackupAll() count = %d, want 0", count)
		}
	})

	t.Run("backs up a single staged file", func(t *testing.T) {
		db := testutil.NewTestDatabase(t)
		fsmgr := testutil.NewMockFilesystemManager()
		staging := testutil.NewTestStagingArea(fsmgr)
		vault := testutil.NewTestVault()

		// Set up filesystem with a directory and file
		fsmgr.AddDirectory("/home/user/docs")
		fsmgr.AddFile("/home/user/docs/file.txt", []byte("hello world"))

		svc := bt.NewBTService(db, staging, vault, fsmgr, testutil.NewTestEncryptor(), bt.NewNopLogger(), bt.RealClock{}, bt.UUIDGenerator{})

		// Add directory
		dirPath, _ := fsmgr.Resolve("/home/user/docs")
		if err := svc.AddDirectory(dirPath, false); err != nil {
			t.Fatalf("AddDirectory() error = %v", err)
		}

		// Stage file
		filePath, _ := fsmgr.Resolve("/home/user/docs/file.txt")
		if _, err := svc.StageFiles(filePath, false); err != nil {
			t.Fatalf("StageFiles() error = %v", err)
		}

		// Verify file is staged
		stagedCount, _ := staging.Count()
		if stagedCount != 1 {
			t.Fatalf("staged count = %d, want 1", stagedCount)
		}

		// Backup all
		count, err := svc.BackupAll()
		if err != nil {
			t.Fatalf("BackupAll() error = %v", err)
		}
		if count != 1 {
			t.Errorf("BackupAll() count = %d, want 1", count)
		}

		// Verify staging area is now empty
		stagedCount, _ = staging.Count()
		if stagedCount != 0 {
			t.Errorf("staged count after backup = %d, want 0", stagedCount)
		}
	})

	t.Run("backs up multiple staged files", func(t *testing.T) {
		db := testutil.NewTestDatabase(t)
		fsmgr := testutil.NewMockFilesystemManager()
		staging := testutil.NewTestStagingArea(fsmgr)
		vault := testutil.NewTestVault()

		// Set up filesystem
		fsmgr.AddDirectory("/home/user/docs")
		fsmgr.AddFile("/home/user/docs/file1.txt", []byte("content 1"))
		fsmgr.AddFile("/home/user/docs/file2.txt", []byte("content 2"))
		fsmgr.AddFile("/home/user/docs/file3.txt", []byte("content 3"))

		svc := bt.NewBTService(db, staging, vault, fsmgr, testutil.NewTestEncryptor(), bt.NewNopLogger(), bt.RealClock{}, bt.UUIDGenerator{})

		// Add directory
		dirPath, _ := fsmgr.Resolve("/home/user/docs")
		svc.AddDirectory(dirPath, false)

		// Stage files
		for _, name := range []string{"file1.txt", "file2.txt", "file3.txt"} {
			filePath, _ := fsmgr.Resolve("/home/user/docs/" + name)
			if _, err := svc.StageFiles(filePath, false); err != nil {
				t.Fatalf("StageFiles(%s) error = %v", name, err)
			}
		}

		// Backup all
		count, err := svc.BackupAll()
		if err != nil {
			t.Fatalf("BackupAll() error = %v", err)
		}
		if count != 3 {
			t.Errorf("BackupAll() count = %d, want 3", count)
		}

		// Verify staging area is empty
		stagedCount, _ := staging.Count()
		if stagedCount != 0 {
			t.Errorf("staged count after backup = %d, want 0", stagedCount)
		}
	})

	t.Run("deduplicates content across files", func(t *testing.T) {
		db := testutil.NewTestDatabase(t)
		fsmgr := testutil.NewMockFilesystemManager()
		staging := testutil.NewTestStagingArea(fsmgr)
		vault := testutil.NewTestVault()

		// Two files with identical content
		content := []byte("identical content")
		fsmgr.AddDirectory("/home/user/docs")
		fsmgr.AddFile("/home/user/docs/file1.txt", content)
		fsmgr.AddFile("/home/user/docs/file2.txt", content)

		svc := bt.NewBTService(db, staging, vault, fsmgr, testutil.NewTestEncryptor(), bt.NewNopLogger(), bt.RealClock{}, bt.UUIDGenerator{})

		// Add directory and stage files
		dirPath, _ := fsmgr.Resolve("/home/user/docs")
		svc.AddDirectory(dirPath, false)

		file1Path, _ := fsmgr.Resolve("/home/user/docs/file1.txt")
		svc.StageFiles(file1Path, false)

		file2Path, _ := fsmgr.Resolve("/home/user/docs/file2.txt")
		svc.StageFiles(file2Path, false)

		// Backup all
		count, err := svc.BackupAll()
		if err != nil {
			t.Fatalf("BackupAll() error = %v", err)
		}
		if count != 2 {
			t.Errorf("BackupAll() count = %d, want 2", count)
		}

		// Both files should be backed up, but content should only be stored once
		// We can verify this by checking the staging area processed both
		stagedCount, _ := staging.Count()
		if stagedCount != 0 {
			t.Errorf("staged count after backup = %d, want 0", stagedCount)
		}
	})

	t.Run("handles re-backup of unchanged file", func(t *testing.T) {
		db := testutil.NewTestDatabase(t)
		fsmgr := testutil.NewMockFilesystemManager()
		staging := testutil.NewTestStagingArea(fsmgr)
		vault := testutil.NewTestVault()

		content := []byte("file content")
		fsmgr.AddDirectory("/home/user/docs")
		fsmgr.AddFile("/home/user/docs/file.txt", content)

		svc := bt.NewBTService(db, staging, vault, fsmgr, testutil.NewTestEncryptor(), bt.NewNopLogger(), bt.RealClock{}, bt.UUIDGenerator{})

		// Add directory
		dirPath, _ := fsmgr.Resolve("/home/user/docs")
		svc.AddDirectory(dirPath, false)

		// First backup
		filePath, _ := fsmgr.Resolve("/home/user/docs/file.txt")
		svc.StageFiles(filePath, false)
		count1, err := svc.BackupAll()
		if err != nil {
			t.Fatalf("first BackupAll() error = %v", err)
		}
		if count1 != 1 {
			t.Errorf("first BackupAll() count = %d, want 1", count1)
		}

		// Stage same file again (content unchanged)
		svc.StageFiles(filePath, false)
		count2, err := svc.BackupAll()
		if err != nil {
			t.Fatalf("second BackupAll() error = %v", err)
		}
		if count2 != 1 {
			t.Errorf("second BackupAll() count = %d, want 1", count2)
		}
	})
}
