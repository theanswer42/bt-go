package testutil

import (
	"bt-go/internal/bt"
	"bt-go/internal/staging"
)

const (
	// DefaultStagingMaxSize is the default max size for test staging areas (10MB).
	DefaultStagingMaxSize = 10 * 1024 * 1024
)

// NewTestStagingArea creates a new in-memory staging area for testing.
func NewTestStagingArea(fsmgr *MockFilesystemManager) bt.StagingArea {
	return staging.NewMemoryStagingArea(fsmgr, DefaultStagingMaxSize)
}

// NewTestStagingAreaWithSize creates a new in-memory staging area with a custom max size.
func NewTestStagingAreaWithSize(fsmgr *MockFilesystemManager, maxSize int64) bt.StagingArea {
	return staging.NewMemoryStagingArea(fsmgr, maxSize)
}
