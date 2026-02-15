package fs

import (
	"bt-go/internal/bt"
)

// OSFilesystemManager is the real filesystem implementation of FilesystemManager.
// It performs actual filesystem operations using the os package.
type OSFilesystemManager struct {
	// TODO: Add fields as needed during implementation (e.g., ignore patterns)
}

// NewOSFilesystemManager creates a new filesystem manager that operates on the real filesystem.
func NewOSFilesystemManager() *OSFilesystemManager {
	return &OSFilesystemManager{}
}

// Compile-time check that OSFilesystemManager implements bt.FilesystemManager interface
var _ bt.FilesystemManager = (*OSFilesystemManager)(nil)
