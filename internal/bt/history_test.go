package bt_test

import (
	"testing"

	"bt-go/internal/bt"
	"bt-go/internal/testutil"
)

func TestBTService_GetHistory(t *testing.T) {
	setup := func(t *testing.T) (*bt.BTService, bt.Database) {
		t.Helper()
		db := testutil.NewTestDatabase(t)
		fsmgr := testutil.NewMockFilesystemManager()
		staging := testutil.NewTestStagingArea(fsmgr)
		vault := testutil.NewTestVault()
		svc := bt.NewBTService(db, staging, vault, fsmgr)
		return svc, db
	}

	t.Run("returns operations in newest-first order with limit", func(t *testing.T) {
		t.Parallel()
		svc, db := setup(t)

		// Create some operations
		db.CreateBackupOperation("AddDirectory", "/home/user/docs")
		db.CreateBackupOperation("BackupAll", "")
		db.CreateBackupOperation("AddDirectory", "/home/user/photos")

		ops, err := svc.GetHistory(2)
		if err != nil {
			t.Fatalf("GetHistory() error = %v", err)
		}

		if len(ops) != 2 {
			t.Fatalf("got %d ops, want 2", len(ops))
		}

		// Newest first (higher ID first)
		if ops[0].ID <= ops[1].ID {
			t.Errorf("expected newest first: got IDs %d, %d", ops[0].ID, ops[1].ID)
		}
	})

	t.Run("empty history returns empty slice", func(t *testing.T) {
		t.Parallel()
		svc, _ := setup(t)

		ops, err := svc.GetHistory(50)
		if err != nil {
			t.Fatalf("GetHistory() error = %v", err)
		}

		if len(ops) != 0 {
			t.Fatalf("got %d ops, want 0", len(ops))
		}
	})

	t.Run("limit larger than total returns all", func(t *testing.T) {
		t.Parallel()
		svc, db := setup(t)

		db.CreateBackupOperation("BackupAll", "")

		ops, err := svc.GetHistory(50)
		if err != nil {
			t.Fatalf("GetHistory() error = %v", err)
		}

		if len(ops) != 1 {
			t.Fatalf("got %d ops, want 1", len(ops))
		}
	})
}
