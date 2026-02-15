package bt

import "fmt"

// BTService is the orchestration layer that coordinates across all components
// to perform high-level backup operations needed by the CLI.
type BTService struct {
	database    Database
	stagingArea StagingArea
	vaults      []Vault
	fsmgr       FilesystemManager
}

// NewBTService creates a new BTService with the provided dependencies.
func NewBTService(database Database, stagingArea StagingArea, vaults []Vault, fsmgr FilesystemManager) *BTService {
	return &BTService{
		database:    database,
		stagingArea: stagingArea,
		vaults:      vaults,
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
