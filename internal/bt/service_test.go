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
