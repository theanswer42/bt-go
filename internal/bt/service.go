package bt

import (
	"fmt"
	"io"
	"path/filepath"
	"time"

	"bt-go/internal/database/sqlc"

	"github.com/google/uuid"
)

// BTService is the orchestration layer that coordinates across all components
// to perform high-level backup operations needed by the CLI.
type BTService struct {
	database    Database
	stagingArea StagingArea
	vault       Vault
	fsmgr       FilesystemManager
}

// NewBTService creates a new BTService with the provided dependencies.
// Currently only a single vault is supported; multiple vaults require additional
// implementation work (content seeking, transaction handling across vaults).
func NewBTService(database Database, stagingArea StagingArea, vault Vault, fsmgr FilesystemManager) *BTService {
	return &BTService{
		database:    database,
		stagingArea: stagingArea,
		vault:       vault,
		fsmgr:       fsmgr,
	}
}

// AddDirectory registers a directory for tracking.
// The path must point to a directory, not a file.
// If the directory is already tracked, this is a no-op.
func (s *BTService) AddDirectory(path *Path) error {
	if !path.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path.String())
	}

	// Check if already tracked
	existing, err := s.database.FindDirectoryByPath(path.String())
	if err != nil {
		return fmt.Errorf("checking for existing directory: %w", err)
	}
	if existing != nil {
		// Already tracked, nothing to do
		return nil
	}

	// Create the directory record
	_, err = s.database.CreateDirectory(path.String())
	if err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	return nil
}

// StageFile stages a file for backup.
// The path must point to a regular file within a tracked directory.
func (s *BTService) StageFile(path *Path) error {
	if path.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", path.String())
	}

	// Find the directory that contains this file
	directory, err := s.database.SearchDirectoryForPath(path.String())
	if err != nil {
		return fmt.Errorf("searching for directory: %w", err)
	}
	if directory == nil {
		return fmt.Errorf("file is not within a tracked directory: %s", path.String())
	}

	// Calculate the relative path within the directory
	relativePath, err := filepath.Rel(directory.Path, path.String())
	if err != nil {
		return fmt.Errorf("calculating relative path: %w", err)
	}

	// Stage the file for backup (no database writes here)
	if err := s.stagingArea.Stage(directory, relativePath, path); err != nil {
		return fmt.Errorf("staging file: %w", err)
	}

	return nil
}

// BackupAll processes all staged files and backs them up to the vault(s).
// Returns the number of files successfully backed up.
func (s *BTService) BackupAll() (int, error) {
	count := 0

	for {
		// Check if there are any staged items left
		queueSize, err := s.stagingArea.Count()
		if err != nil {
			return count, fmt.Errorf("checking staging queue: %w", err)
		}
		if queueSize == 0 {
			break
		}

		// Process the next staged item
		err = s.stagingArea.ProcessNext(func(content io.Reader, snapshot sqlc.FileSnapshot, directoryID string, relativePath string) error {
			return s.backupFile(content, snapshot, directoryID, relativePath)
		})
		if err != nil {
			return count, fmt.Errorf("backing up file: %w", err)
		}

		count++
	}

	return count, nil
}

// backupFile handles the backup of a single file's content and metadata.
//
// Strategy: upload content to the vault first (idempotent), then atomically
// record everything in the database via a single transaction. If the DB
// call fails, the worst outcome is orphaned content in the vault, which is
// harmless. The staging queue will retain the operation for retry.
func (s *BTService) backupFile(content io.Reader, snapshot sqlc.FileSnapshot, directoryID string, relativePath string) error {
	checksum := snapshot.ContentID

	// Check if content already exists in the database (and thus in vault).
	// If so, we can skip the vault upload entirely.
	existingContent, err := s.database.FindContentByChecksum(checksum)
	if err != nil {
		return fmt.Errorf("checking for existing content: %w", err)
	}

	if existingContent == nil {
		// Upload content to vault first — this is idempotent by checksum.
		if err := s.vault.PutContent(checksum, content, snapshot.Size); err != nil {
			return fmt.Errorf("uploading to vault: %w", err)
		}
	}

	// Atomically: find/create file, create content record (if needed),
	// compare against current snapshot, and create a new one if anything changed.
	snapshot.ID = uuid.New().String()
	snapshot.CreatedAt = time.Now()
	if err := s.database.CreateFileSnapshotAndContent(directoryID, relativePath, &snapshot); err != nil {
		return fmt.Errorf("recording backup in database: %w", err)
	}

	return nil
}
