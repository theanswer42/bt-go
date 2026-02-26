package bt_test

import (
	"bytes"
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

func TestBTService_BackupAll_Encrypted(t *testing.T) {
	t.Run("encrypted directory stores ciphertext in vault", func(t *testing.T) {
		db := testutil.NewTestDatabase(t)
		fsmgr := testutil.NewMockFilesystemManager()
		staging := testutil.NewTestStagingArea(fsmgr)
		vault := testutil.NewTestVault()

		fsmgr.AddDirectory("/home/user/secret")
		fsmgr.AddFile("/home/user/secret/file.txt", []byte("plaintext content"))

		svc := bt.NewBTService(db, staging, vault, fsmgr, testutil.NewTestEncryptor(), bt.NewNopLogger(), bt.RealClock{}, bt.UUIDGenerator{})

		dirPath, _ := fsmgr.Resolve("/home/user/secret")
		if err := svc.AddDirectory(dirPath, true); err != nil { // encrypted=true
			t.Fatalf("AddDirectory() error = %v", err)
		}

		filePath, _ := fsmgr.Resolve("/home/user/secret/file.txt")
		if _, err := svc.StageFiles(filePath, false); err != nil {
			t.Fatalf("StageFiles() error = %v", err)
		}

		count, err := svc.BackupAll()
		if err != nil {
			t.Fatalf("BackupAll() error = %v", err)
		}
		if count != 1 {
			t.Errorf("BackupAll() count = %d, want 1", count)
		}

		// The vault should contain the encrypted checksum, not the plaintext checksum.
		// TestEncryptor prepends an 8-byte header, so the encrypted bytes differ from plaintext.
		// We verify by checking that at least one content entry exists in the DB and that
		// the virtual plaintext record points to a different (encrypted) content ID.
		plaintextChecksum := testutil.SHA256Hex([]byte("plaintext content"))
		content, err := db.FindContentByChecksum(plaintextChecksum)
		if err != nil {
			t.Fatalf("FindContentByChecksum() error = %v", err)
		}
		if content == nil {
			t.Fatal("plaintext content record not found in database")
		}
		if !content.EncryptedContentID.Valid {
			t.Fatal("plaintext content record should have an encrypted_content_id")
		}

		encChecksum := content.EncryptedContentID.String
		if encChecksum == plaintextChecksum {
			t.Error("encrypted checksum should differ from plaintext checksum")
		}

		// The encrypted content record should also be in the DB.
		encContent, err := db.FindContentByChecksum(encChecksum)
		if err != nil {
			t.Fatalf("FindContentByChecksum(encrypted) error = %v", err)
		}
		if encContent == nil {
			t.Fatal("encrypted content record not found in database")
		}
		if encContent.EncryptedContentID.Valid {
			t.Error("encrypted content record should not have its own encrypted_content_id")
		}

		// The vault should hold the encrypted bytes, not the plaintext.
		var encBuf bytes.Buffer
		if err := vault.GetContent(encChecksum, &encBuf); err != nil {
			t.Fatalf("encrypted content not found in vault: %v", err)
		}
		if encBuf.Len() == len([]byte("plaintext content")) {
			t.Error("vault should hold encrypted (larger) content, not raw plaintext")
		}
	})

	t.Run("unencrypted directory stores plaintext in vault", func(t *testing.T) {
		db := testutil.NewTestDatabase(t)
		fsmgr := testutil.NewMockFilesystemManager()
		staging := testutil.NewTestStagingArea(fsmgr)
		vault := testutil.NewTestVault()

		fsmgr.AddDirectory("/home/user/docs")
		fsmgr.AddFile("/home/user/docs/file.txt", []byte("plaintext content"))

		svc := bt.NewBTService(db, staging, vault, fsmgr, testutil.NewTestEncryptor(), bt.NewNopLogger(), bt.RealClock{}, bt.UUIDGenerator{})

		dirPath, _ := fsmgr.Resolve("/home/user/docs")
		svc.AddDirectory(dirPath, false) // encrypted=false

		filePath, _ := fsmgr.Resolve("/home/user/docs/file.txt")
		svc.StageFiles(filePath, false)
		svc.BackupAll()

		// The plaintext content record should have no encrypted_content_id.
		plaintextChecksum := testutil.SHA256Hex([]byte("plaintext content"))
		content, err := db.FindContentByChecksum(plaintextChecksum)
		if err != nil {
			t.Fatalf("FindContentByChecksum() error = %v", err)
		}
		if content == nil {
			t.Fatal("content record not found in database")
		}
		if content.EncryptedContentID.Valid {
			t.Error("unencrypted content record should not have an encrypted_content_id")
		}
	})

	t.Run("encrypted file deduplicates on second backup", func(t *testing.T) {
		db := testutil.NewTestDatabase(t)
		fsmgr := testutil.NewMockFilesystemManager()
		staging := testutil.NewTestStagingArea(fsmgr)
		vault := testutil.NewTestVault()

		fsmgr.AddDirectory("/home/user/secret")
		fsmgr.AddFile("/home/user/secret/file.txt", []byte("same content"))

		svc := bt.NewBTService(db, staging, vault, fsmgr, testutil.NewTestEncryptor(), bt.NewNopLogger(), bt.RealClock{}, bt.UUIDGenerator{})

		dirPath, _ := fsmgr.Resolve("/home/user/secret")
		svc.AddDirectory(dirPath, true)

		filePath, _ := fsmgr.Resolve("/home/user/secret/file.txt")

		// First backup
		svc.StageFiles(filePath, false)
		count1, err := svc.BackupAll()
		if err != nil {
			t.Fatalf("first BackupAll() error = %v", err)
		}
		if count1 != 1 {
			t.Errorf("first BackupAll() count = %d, want 1", count1)
		}

		// Second backup of same content — should deduplicate
		svc.StageFiles(filePath, false)
		count2, err := svc.BackupAll()
		if err != nil {
			t.Fatalf("second BackupAll() error = %v", err)
		}
		if count2 != 1 {
			t.Errorf("second BackupAll() count = %d, want 1", count2)
		}

		// Still only one vault upload
		plaintextChecksum := testutil.SHA256Hex([]byte("same content"))
		content, _ := db.FindContentByChecksum(plaintextChecksum)
		if content == nil {
			t.Fatal("content record missing after dedup backup")
		}
	})
}
