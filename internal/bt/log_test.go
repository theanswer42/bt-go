package bt_test

import (
	"testing"
	"time"

	"bt-go/internal/bt"
	"bt-go/internal/testutil"
)

func TestBTService_GetFileHistory(t *testing.T) {
	setup := func(t *testing.T) (*bt.BTService, *testutil.MockFilesystemManager) {
		t.Helper()
		db := testutil.NewTestDatabase(t)
		fsmgr := testutil.NewMockFilesystemManager()
		staging := testutil.NewTestStagingArea(fsmgr)
		vault := testutil.NewTestVault()
		svc := bt.NewBTService(db, staging, vault, fsmgr, testutil.NewTestEncryptor(), bt.NewNopLogger(), bt.RealClock{}, bt.UUIDGenerator{})
		return svc, fsmgr
	}

	t.Run("file with multiple snapshots returns newest first", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr := setup(t)

		fsmgr.AddDirectory("/home/user/docs")
		fsmgr.AddFile("/home/user/docs/file.txt", []byte("version1"))

		dirPath, _ := fsmgr.Resolve("/home/user/docs")
		svc.AddDirectory(dirPath, false)

		// First backup
		filePath, _ := fsmgr.Resolve("/home/user/docs/file.txt")
		svc.StageFiles(filePath, false)
		svc.BackupAll()

		// Modify and backup again
		fsmgr.UpdateFile("/home/user/docs/file.txt", []byte("version2"), time.Now().Add(time.Hour))
		filePath, _ = fsmgr.Resolve("/home/user/docs/file.txt")
		svc.StageFiles(filePath, false)
		svc.BackupAll()

		entries, err := svc.GetFileHistory(filePath)
		if err != nil {
			t.Fatalf("GetFileHistory() error = %v", err)
		}

		if len(entries) != 2 {
			t.Fatalf("got %d entries, want 2", len(entries))
		}

		// Newest first
		if !entries[0].BackedUpAt.After(entries[1].BackedUpAt) {
			t.Error("expected newest entry first")
		}

		// Only the first entry should be current
		if !entries[0].IsCurrent {
			t.Error("expected first (newest) entry to be current")
		}
		if entries[1].IsCurrent {
			t.Error("expected second (older) entry to not be current")
		}

		// Different content checksums
		if entries[0].ContentChecksum == entries[1].ContentChecksum {
			t.Error("expected different checksums for different versions")
		}
	})

	t.Run("file not in tracked directory returns error", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr := setup(t)

		fsmgr.AddFile("/home/user/untracked/file.txt", []byte("data"))

		filePath, _ := fsmgr.Resolve("/home/user/untracked/file.txt")
		_, err := svc.GetFileHistory(filePath)
		if err == nil {
			t.Fatal("expected error for file not in tracked directory")
		}
	})

	t.Run("file not backed up returns error", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr := setup(t)

		fsmgr.AddDirectory("/home/user/docs")
		fsmgr.AddFile("/home/user/docs/new.txt", []byte("data"))

		dirPath, _ := fsmgr.Resolve("/home/user/docs")
		svc.AddDirectory(dirPath, false)

		filePath, _ := fsmgr.Resolve("/home/user/docs/new.txt")
		_, err := svc.GetFileHistory(filePath)
		if err == nil {
			t.Fatal("expected error for file with no backup history")
		}
	})

	t.Run("directory path returns error", func(t *testing.T) {
		t.Parallel()
		svc, fsmgr := setup(t)

		fsmgr.AddDirectory("/home/user/docs")

		dirPath, _ := fsmgr.Resolve("/home/user/docs")
		_, err := svc.GetFileHistory(dirPath)
		if err == nil {
			t.Fatal("expected error for directory path")
		}
	})
}
