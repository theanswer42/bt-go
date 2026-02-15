package staging

import (
	"fmt"
	"os"
	"path/filepath"

	"bt-go/internal/bt"
)

// FileSystemStagingArea is a filesystem-based implementation of the StagingArea interface.
// It stores staged files in a directory structure with a queue file for ordering.
//
// Directory structure:
//
//	<staging_dir>/
//	  queue.json    (ordered list of staged files)
//	  files/
//	    <file_id>/
//	      <snapshot_id>    (staged file content)
type FileSystemStagingArea struct {
	stagingDir string
	filesDir   string
	maxSize    int64
	// TODO: Add queue management during implementation
}

// NewFileSystemStagingArea creates a new filesystem-based staging area.
// maxSize is the maximum total size in bytes; must be positive.
func NewFileSystemStagingArea(stagingDir string, maxSize int64) (*FileSystemStagingArea, error) {
	filesDir := filepath.Join(stagingDir, "files")

	// Create directory structure
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create staging directory: %w", err)
	}

	return &FileSystemStagingArea{
		stagingDir: stagingDir,
		filesDir:   filesDir,
		maxSize:    maxSize,
	}, nil
}

// Compile-time check that FileSystemStagingArea implements bt.StagingArea interface
var _ bt.StagingArea = (*FileSystemStagingArea)(nil)
