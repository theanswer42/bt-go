package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"bt-go/internal/database/sqlc"
)

// newTestDB creates a new in-memory database with schema applied.
func newTestDB(t *testing.T) *SQLiteDatabase {
	t.Helper()

	db, err := NewSQLiteDatabase(":memory:", nil, nil)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	if _, err := db.db.Exec(Schema); err != nil {
		db.Close()
		t.Fatalf("failed to apply schema: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

func TestSQLiteDatabase_FindDirectoryByPath(t *testing.T) {
	t.Run("returns nil when directory not found", func(t *testing.T) {
		db := newTestDB(t)

		dir, err := db.FindDirectoryByPath("/nonexistent/path")
		if err != nil {
			t.Fatalf("FindDirectoryByPath() error = %v", err)
		}
		if dir != nil {
			t.Errorf("FindDirectoryByPath() = %v, want nil", dir)
		}
	})

	t.Run("finds existing directory", func(t *testing.T) {
		db := newTestDB(t)

		// Create a directory first
		created, err := db.CreateDirectory("/home/user/docs")
		if err != nil {
			t.Fatalf("CreateDirectory() error = %v", err)
		}

		// Now find it
		found, err := db.FindDirectoryByPath("/home/user/docs")
		if err != nil {
			t.Fatalf("FindDirectoryByPath() error = %v", err)
		}
		if found == nil {
			t.Fatal("FindDirectoryByPath() returned nil, want directory")
		}
		if found.ID != created.ID {
			t.Errorf("ID = %v, want %v", found.ID, created.ID)
		}
		if found.Path != "/home/user/docs" {
			t.Errorf("Path = %v, want /home/user/docs", found.Path)
		}
	})
}

func TestSQLiteDatabase_CreateDirectory(t *testing.T) {
	t.Run("creates directory successfully", func(t *testing.T) {
		db := newTestDB(t)

		dir, err := db.CreateDirectory("/home/user/docs")
		if err != nil {
			t.Fatalf("CreateDirectory() error = %v", err)
		}

		if dir.ID == "" {
			t.Error("ID is empty")
		}
		if dir.Path != "/home/user/docs" {
			t.Errorf("Path = %v, want /home/user/docs", dir.Path)
		}
		if dir.CreatedAt.IsZero() {
			t.Error("CreatedAt is zero")
		}
	})

	t.Run("fails on duplicate path", func(t *testing.T) {
		db := newTestDB(t)

		_, err := db.CreateDirectory("/home/user/docs")
		if err != nil {
			t.Fatalf("first CreateDirectory() error = %v", err)
		}

		_, err = db.CreateDirectory("/home/user/docs")
		if err == nil {
			t.Error("second CreateDirectory() expected error for duplicate path")
		}
	})

	t.Run("creates multiple directories with different paths", func(t *testing.T) {
		db := newTestDB(t)

		dir1, err := db.CreateDirectory("/home/user/docs")
		if err != nil {
			t.Fatalf("CreateDirectory(/home/user/docs) error = %v", err)
		}

		dir2, err := db.CreateDirectory("/home/user/photos")
		if err != nil {
			t.Fatalf("CreateDirectory(/home/user/photos) error = %v", err)
		}

		if dir1.ID == dir2.ID {
			t.Error("directories have same ID")
		}
	})

	t.Run("consolidates child directories", func(t *testing.T) {
		db := newTestDB(t)

		// Create child directory first
		childDir, err := db.CreateDirectory("/home/user/docs/subdir")
		if err != nil {
			t.Fatalf("CreateDirectory(child) error = %v", err)
		}

		// Add a file to the child directory
		createTestFile(t, db, childDir.ID, "file.txt")

		// Now create the parent directory - should consolidate
		parentDir, err := db.CreateDirectory("/home/user/docs")
		if err != nil {
			t.Fatalf("CreateDirectory(parent) error = %v", err)
		}

		// Child directory should no longer exist
		childFound, err := db.FindDirectoryByPath("/home/user/docs/subdir")
		if err != nil {
			t.Fatalf("FindDirectoryByPath(child) error = %v", err)
		}
		if childFound != nil {
			t.Error("child directory should have been deleted")
		}

		// File should have been moved to parent with updated name
		files, err := db.queries.GetFilesByDirectoryID(context.Background(), parentDir.ID)
		if err != nil {
			t.Fatalf("GetFilesByDirectoryID() error = %v", err)
		}
		if len(files) != 1 {
			t.Fatalf("expected 1 file, got %d", len(files))
		}
		if files[0].Name != "subdir/file.txt" {
			t.Errorf("file name = %q, want %q", files[0].Name, "subdir/file.txt")
		}
	})

	t.Run("consolidates multiple nested child directories", func(t *testing.T) {
		db := newTestDB(t)

		// Create nested child directories
		child1, err := db.CreateDirectory("/home/user/docs/a")
		if err != nil {
			t.Fatalf("CreateDirectory(a) error = %v", err)
		}
		child2, err := db.CreateDirectory("/home/user/docs/b")
		if err != nil {
			t.Fatalf("CreateDirectory(b) error = %v", err)
		}

		// Add files to each
		createTestFile(t, db, child1.ID, "file1.txt")
		createTestFile(t, db, child2.ID, "file2.txt")

		// Create parent
		parentDir, err := db.CreateDirectory("/home/user/docs")
		if err != nil {
			t.Fatalf("CreateDirectory(parent) error = %v", err)
		}

		// Both child directories should be deleted
		for _, path := range []string{"/home/user/docs/a", "/home/user/docs/b"} {
			found, _ := db.FindDirectoryByPath(path)
			if found != nil {
				t.Errorf("child directory %s should have been deleted", path)
			}
		}

		// Both files should be in parent
		files, err := db.queries.GetFilesByDirectoryID(context.Background(), parentDir.ID)
		if err != nil {
			t.Fatalf("GetFilesByDirectoryID() error = %v", err)
		}
		if len(files) != 2 {
			t.Fatalf("expected 2 files, got %d", len(files))
		}

		// Check file names
		names := make(map[string]bool)
		for _, f := range files {
			names[f.Name] = true
		}
		if !names["a/file1.txt"] {
			t.Error("expected file a/file1.txt")
		}
		if !names["b/file2.txt"] {
			t.Error("expected file b/file2.txt")
		}
	})
}

func TestSQLiteDatabase_FindDirectoriesByPathPrefix(t *testing.T) {
	t.Run("finds child directories", func(t *testing.T) {
		db := newTestDB(t)

		// Create parent and children
		db.CreateDirectory("/home/user/docs")
		db.CreateDirectory("/home/user/docs/a")
		db.CreateDirectory("/home/user/docs/b")
		db.CreateDirectory("/home/user/photos") // not a child

		dirs, err := db.FindDirectoriesByPathPrefix("/home/user/docs")
		if err != nil {
			t.Fatalf("FindDirectoriesByPathPrefix() error = %v", err)
		}

		if len(dirs) != 2 {
			t.Fatalf("expected 2 directories, got %d", len(dirs))
		}

		paths := make(map[string]bool)
		for _, d := range dirs {
			paths[d.Path] = true
		}
		if !paths["/home/user/docs/a"] {
			t.Error("expected /home/user/docs/a")
		}
		if !paths["/home/user/docs/b"] {
			t.Error("expected /home/user/docs/b")
		}
	})

	t.Run("returns empty for no children", func(t *testing.T) {
		db := newTestDB(t)

		db.CreateDirectory("/home/user/docs")

		dirs, err := db.FindDirectoriesByPathPrefix("/home/user/docs")
		if err != nil {
			t.Fatalf("FindDirectoriesByPathPrefix() error = %v", err)
		}
		if len(dirs) != 0 {
			t.Errorf("expected 0 directories, got %d", len(dirs))
		}
	})
}

func TestSQLiteDatabase_DeleteDirectory(t *testing.T) {
	t.Run("deletes directory", func(t *testing.T) {
		db := newTestDB(t)

		dir, err := db.CreateDirectory("/home/user/docs")
		if err != nil {
			t.Fatalf("CreateDirectory() error = %v", err)
		}

		err = db.DeleteDirectory(dir)
		if err != nil {
			t.Fatalf("DeleteDirectory() error = %v", err)
		}

		found, err := db.FindDirectoryByPath("/home/user/docs")
		if err != nil {
			t.Fatalf("FindDirectoryByPath() error = %v", err)
		}
		if found != nil {
			t.Error("directory should have been deleted")
		}
	})
}

// createTestFile is a helper to create a file in a directory for testing.
func createTestFile(t *testing.T, db *SQLiteDatabase, directoryID, name string) {
	t.Helper()

	_, err := db.queries.InsertFile(context.Background(), sqlc.InsertFileParams{
		ID:                uuid.New().String(),
		Name:              name,
		DirectoryID:       directoryID,
		CurrentSnapshotID: sql.NullString{},
		Deleted:           false,
	})
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
}

func TestSQLiteDatabase_SearchDirectoryForPath(t *testing.T) {
	t.Run("finds directory containing path", func(t *testing.T) {
		db := newTestDB(t)

		dir, err := db.CreateDirectory("/home/user/docs")
		if err != nil {
			t.Fatalf("CreateDirectory() error = %v", err)
		}

		found, err := db.SearchDirectoryForPath("/home/user/docs/file.txt")
		if err != nil {
			t.Fatalf("SearchDirectoryForPath() error = %v", err)
		}
		if found == nil {
			t.Fatal("expected to find directory")
		}
		if found.ID != dir.ID {
			t.Errorf("found wrong directory: got %s, want %s", found.ID, dir.ID)
		}
	})

	t.Run("returns nil for path not in any directory", func(t *testing.T) {
		db := newTestDB(t)

		db.CreateDirectory("/home/user/docs")

		found, err := db.SearchDirectoryForPath("/home/user/photos/image.jpg")
		if err != nil {
			t.Fatalf("SearchDirectoryForPath() error = %v", err)
		}
		if found != nil {
			t.Errorf("expected nil, got %v", found)
		}
	})

	t.Run("finds shortest matching prefix with nested directories", func(t *testing.T) {
		db := newTestDB(t)

		// Create parent first, then child (unusual but possible)
		parent, _ := db.CreateDirectory("/home/user/docs")
		// Manually create child to simulate unusual state
		db.queries.InsertDirectory(context.Background(), sqlc.InsertDirectoryParams{
			ID:        "child-id",
			Path:      "/home/user/docs/subdir",
			CreatedAt: time.Now(),
			Encrypted: 0,
		})

		found, err := db.SearchDirectoryForPath("/home/user/docs/subdir/file.txt")
		if err != nil {
			t.Fatalf("SearchDirectoryForPath() error = %v", err)
		}
		if found == nil {
			t.Fatal("expected to find directory")
		}
		// Should return the parent (shortest prefix) not the child
		if found.ID != parent.ID {
			t.Errorf("expected parent directory, got %s", found.Path)
		}
	})
}

func TestSQLiteDatabase_FindFileByPath(t *testing.T) {
	t.Run("finds existing file", func(t *testing.T) {
		db := newTestDB(t)

		dir, _ := db.CreateDirectory("/home/user/docs")
		createTestFile(t, db, dir.ID, "file.txt")

		file, err := db.FindFileByPath(dir, "file.txt")
		if err != nil {
			t.Fatalf("FindFileByPath() error = %v", err)
		}
		if file == nil {
			t.Fatal("expected to find file")
		}
		if file.Name != "file.txt" {
			t.Errorf("Name = %s, want file.txt", file.Name)
		}
	})

	t.Run("returns nil for non-existent file", func(t *testing.T) {
		db := newTestDB(t)

		dir, _ := db.CreateDirectory("/home/user/docs")

		file, err := db.FindFileByPath(dir, "nonexistent.txt")
		if err != nil {
			t.Fatalf("FindFileByPath() error = %v", err)
		}
		if file != nil {
			t.Errorf("expected nil, got %v", file)
		}
	})
}

func TestSQLiteDatabase_FindOrCreateFile(t *testing.T) {
	t.Run("creates new file", func(t *testing.T) {
		db := newTestDB(t)

		dir, _ := db.CreateDirectory("/home/user/docs")

		file, err := db.FindOrCreateFile(dir, "newfile.txt")
		if err != nil {
			t.Fatalf("FindOrCreateFile() error = %v", err)
		}
		if file == nil {
			t.Fatal("expected file to be created")
		}
		if file.Name != "newfile.txt" {
			t.Errorf("Name = %s, want newfile.txt", file.Name)
		}
		if file.DirectoryID != dir.ID {
			t.Errorf("DirectoryID = %s, want %s", file.DirectoryID, dir.ID)
		}
	})

	t.Run("finds existing file", func(t *testing.T) {
		db := newTestDB(t)

		dir, _ := db.CreateDirectory("/home/user/docs")
		createTestFile(t, db, dir.ID, "existing.txt")

		file1, _ := db.FindFileByPath(dir, "existing.txt")
		file2, err := db.FindOrCreateFile(dir, "existing.txt")
		if err != nil {
			t.Fatalf("FindOrCreateFile() error = %v", err)
		}
		if file2.ID != file1.ID {
			t.Errorf("expected same file ID, got %s and %s", file1.ID, file2.ID)
		}
	})
}

func TestSQLiteDatabase_CreateFileSnapshotAndContent(t *testing.T) {
	makeSnapshot := func(contentID string) *sqlc.FileSnapshot {
		return &sqlc.FileSnapshot{
			ID:          uuid.New().String(),
			ContentID:   contentID,
			CreatedAt:   time.Now(),
			Size:        42,
			Permissions: 0644,
			Uid:         1000,
			Gid:         1000,
			AccessedAt:  time.Now(),
			ModifiedAt:  time.Now(),
			ChangedAt:   time.Now(),
		}
	}

	t.Run("creates file, content, and snapshot for new file", func(t *testing.T) {
		db := newTestDB(t)
		dir, _ := db.CreateDirectory("/home/user/docs")

		snap := makeSnapshot("abc123checksum")
		err := db.CreateFileSnapshotAndContent(dir.ID, "newfile.txt", snap)
		if err != nil {
			t.Fatalf("CreateFileSnapshotAndContent() error = %v", err)
		}

		// Verify file was created
		file, err := db.FindFileByPath(dir, "newfile.txt")
		if err != nil {
			t.Fatalf("FindFileByPath() error = %v", err)
		}
		if file == nil {
			t.Fatal("file was not created")
		}
		if !file.CurrentSnapshotID.Valid {
			t.Error("file.CurrentSnapshotID should be set")
		}

		// Verify content was created
		content, err := db.FindContentByChecksum("abc123checksum")
		if err != nil {
			t.Fatalf("FindContentByChecksum() error = %v", err)
		}
		if content == nil {
			t.Error("content was not created")
		}
	})

	t.Run("skips when snapshot unchanged", func(t *testing.T) {
		db := newTestDB(t)
		dir, _ := db.CreateDirectory("/home/user/docs")

		snap1 := makeSnapshot("checksum1")
		if err := db.CreateFileSnapshotAndContent(dir.ID, "file.txt", snap1); err != nil {
			t.Fatalf("first call error = %v", err)
		}

		file, _ := db.FindFileByPath(dir, "file.txt")
		firstSnapshotID := file.CurrentSnapshotID.String

		// Call again with identical metadata
		snap2 := makeSnapshot("checksum1")
		snap2.Size = snap1.Size
		snap2.Permissions = snap1.Permissions
		snap2.Uid = snap1.Uid
		snap2.Gid = snap1.Gid
		snap2.AccessedAt = snap1.AccessedAt
		snap2.ModifiedAt = snap1.ModifiedAt
		snap2.ChangedAt = snap1.ChangedAt
		snap2.BornAt = snap1.BornAt

		if err := db.CreateFileSnapshotAndContent(dir.ID, "file.txt", snap2); err != nil {
			t.Fatalf("second call error = %v", err)
		}

		// Current snapshot should not have changed
		file, _ = db.FindFileByPath(dir, "file.txt")
		if file.CurrentSnapshotID.String != firstSnapshotID {
			t.Errorf("snapshot pointer changed: %s -> %s", firstSnapshotID, file.CurrentSnapshotID.String)
		}
	})

	t.Run("creates new snapshot when content changes", func(t *testing.T) {
		db := newTestDB(t)
		dir, _ := db.CreateDirectory("/home/user/docs")

		snap1 := makeSnapshot("checksum-v1")
		db.CreateFileSnapshotAndContent(dir.ID, "file.txt", snap1)

		file, _ := db.FindFileByPath(dir, "file.txt")
		firstSnapshotID := file.CurrentSnapshotID.String

		snap2 := makeSnapshot("checksum-v2")
		db.CreateFileSnapshotAndContent(dir.ID, "file.txt", snap2)

		file, _ = db.FindFileByPath(dir, "file.txt")
		if file.CurrentSnapshotID.String == firstSnapshotID {
			t.Error("snapshot pointer should have changed for new content")
		}
	})
}

func TestSQLiteDatabase_BackupOperations(t *testing.T) {
	t.Run("create and list operations", func(t *testing.T) {
		db := newTestDB(t)

		op1, err := db.CreateBackupOperation("AddDirectory", "/docs")
		if err != nil {
			t.Fatalf("CreateBackupOperation() error = %v", err)
		}
		if op1.ID == 0 {
			t.Error("operation ID should be non-zero")
		}
		if op1.Operation != "AddDirectory" {
			t.Errorf("Operation = %q, want %q", op1.Operation, "AddDirectory")
		}

		op2, err := db.CreateBackupOperation("BackupAll", "")
		if err != nil {
			t.Fatalf("CreateBackupOperation() error = %v", err)
		}

		ops, err := db.ListBackupOperations(10)
		if err != nil {
			t.Fatalf("ListBackupOperations() error = %v", err)
		}
		if len(ops) != 2 {
			t.Fatalf("got %d operations, want 2", len(ops))
		}

		// Newest first
		if ops[0].ID != op2.ID {
			t.Errorf("expected newest first: got ID %d, want %d", ops[0].ID, op2.ID)
		}
	})

	t.Run("finish operation sets status and time", func(t *testing.T) {
		db := newTestDB(t)

		op, _ := db.CreateBackupOperation("BackupAll", "")
		err := db.FinishBackupOperation(op.ID, "success")
		if err != nil {
			t.Fatalf("FinishBackupOperation() error = %v", err)
		}

		ops, _ := db.ListBackupOperations(1)
		if ops[0].Status != "success" {
			t.Errorf("Status = %q, want %q", ops[0].Status, "success")
		}
		if !ops[0].FinishedAt.Valid {
			t.Error("FinishedAt should be set")
		}
	})

	t.Run("max operation ID", func(t *testing.T) {
		db := newTestDB(t)

		// Empty DB should return 0
		maxID, err := db.MaxBackupOperationID()
		if err != nil {
			t.Fatalf("MaxBackupOperationID() error = %v", err)
		}
		if maxID != 0 {
			t.Errorf("MaxBackupOperationID() = %d, want 0", maxID)
		}

		db.CreateBackupOperation("op1", "")
		op2, _ := db.CreateBackupOperation("op2", "")

		maxID, err = db.MaxBackupOperationID()
		if err != nil {
			t.Fatalf("MaxBackupOperationID() error = %v", err)
		}
		if maxID != op2.ID {
			t.Errorf("MaxBackupOperationID() = %d, want %d", maxID, op2.ID)
		}
	})
}

func TestSQLiteDatabase_BackupTo(t *testing.T) {
	db := newTestDB(t)
	db.CreateDirectory("/home/user/docs")

	destPath := filepath.Join(t.TempDir(), "backup.db")
	if err := db.BackupTo(destPath); err != nil {
		t.Fatalf("BackupTo() error = %v", err)
	}

	// Open the backup and verify it has the data
	backup, err := NewSQLiteDatabase(destPath, nil, nil)
	if err != nil {
		t.Fatalf("opening backup: %v", err)
	}
	defer backup.Close()

	dir, err := backup.FindDirectoryByPath("/home/user/docs")
	if err != nil {
		t.Fatalf("FindDirectoryByPath() error = %v", err)
	}
	if dir == nil {
		t.Error("backup does not contain the directory")
	}
}

func TestSQLiteDatabase_CheckMigrations(t *testing.T) {
	t.Run("fails on DB without migrations applied", func(t *testing.T) {
		db, err := NewSQLiteDatabase(":memory:", nil, nil)
		if err != nil {
			t.Fatalf("NewSQLiteDatabase() error = %v", err)
		}
		defer db.Close()

		// DB has no schema at all — should fail
		if err := db.CheckMigrations(); err == nil {
			t.Error("CheckMigrations() expected error for missing schema")
		}
	})
}
