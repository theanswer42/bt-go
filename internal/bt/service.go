package bt

import (
	"fmt"
	"io"
	"path/filepath"

	"bt-go/internal/database/sqlc"
)

// BTService is the orchestration layer that coordinates across all components
// to perform high-level backup operations needed by the CLI.
type BTService struct {
	database    Database
	stagingArea StagingArea
	vault       Vault
	fsmgr       FilesystemManager
	logger      Logger
	clock       Clock
	idgen       IDGenerator
}

// NewBTService creates a new BTService with the provided dependencies.
// Currently only a single vault is supported; multiple vaults require additional
// implementation work (content seeking, transaction handling across vaults).
func NewBTService(database Database, stagingArea StagingArea, vault Vault, fsmgr FilesystemManager, logger Logger, clock Clock, idgen IDGenerator) *BTService {
	return &BTService{
		database:    database,
		stagingArea: stagingArea,
		vault:       vault,
		fsmgr:       fsmgr,
		logger:      logger,
		clock:       clock,
		idgen:       idgen,
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

	s.logger.Info("directory tracked", "path", path.String())
	return nil
}

// StageFiles stages one or more files for backup.
// If path is a regular file, it stages that single file.
// If path is a directory, it discovers files and stages them all.
// When recursive is true, files in subdirectories are included.
// Returns the number of files staged.
func (s *BTService) StageFiles(path *Path, recursive bool) (int, error) {
	if !path.IsDir() {
		if err := s.stageOneFile(path); err != nil {
			return 0, err
		}
		return 1, nil
	}

	// Find the tracked directory for this path.
	directory, err := s.database.FindDirectoryByPath(path.String())
	if err != nil {
		return 0, fmt.Errorf("finding directory: %w", err)
	}
	if directory == nil {
		directory, err = s.database.SearchDirectoryForPath(path.String())
		if err != nil {
			return 0, fmt.Errorf("searching for directory: %w", err)
		}
	}
	if directory == nil {
		return 0, fmt.Errorf("directory is not tracked: %s", path.String())
	}

	// Discover files on disk.
	files, err := s.fsmgr.FindFiles(path, recursive)
	if err != nil {
		return 0, fmt.Errorf("finding files: %w", err)
	}

	for _, f := range files {
		if err := s.stageOneFile(f); err != nil {
			return 0, err
		}
	}

	return len(files), nil
}

// stageOneFile stages a single file for backup.
func (s *BTService) stageOneFile(path *Path) error {
	directory, err := s.database.SearchDirectoryForPath(path.String())
	if err != nil {
		return fmt.Errorf("searching for directory: %w", err)
	}
	if directory == nil {
		return fmt.Errorf("file is not within a tracked directory: %s", path.String())
	}

	ignored, err := s.fsmgr.IsIgnored(path, directory.Path)
	if err != nil {
		return fmt.Errorf("checking ignore rules: %w", err)
	}
	if ignored {
		return fmt.Errorf("file is ignored: %s", path.String())
	}

	relativePath, err := filepath.Rel(directory.Path, path.String())
	if err != nil {
		return fmt.Errorf("calculating relative path: %w", err)
	}

	if err := s.stagingArea.Stage(directory, relativePath, path); err != nil {
		return fmt.Errorf("staging file: %w", err)
	}

	s.logger.Debug("file staged", "path", path.String())
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

	s.logger.Info("backup complete", "count", count)
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
	} else {
		s.logger.Debug("content deduplicated", "checksum", checksum)
	}

	// Atomically: find/create file, create content record (if needed),
	// compare against current snapshot, and create a new one if anything changed.
	snapshot.ID = s.idgen.New()
	snapshot.CreatedAt = s.clock.Now()
	if err := s.database.CreateFileSnapshotAndContent(directoryID, relativePath, &snapshot); err != nil {
		return fmt.Errorf("recording backup in database: %w", err)
	}

	s.logger.Info("file backed up", "path", relativePath)
	return nil
}
