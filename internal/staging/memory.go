package staging

import (
	"bt-go/internal/bt"
)

// MemoryStagingArea is an in-memory implementation of the StagingArea interface.
// It stores staged files in memory, making it useful for testing.
// This implementation is safe for concurrent use.
type MemoryStagingArea struct {
	maxSize     int64
	currentSize int64
	// TODO: Add queue data structure during implementation
}

// NewMemoryStagingArea creates a new in-memory staging area.
// maxSize is the maximum total size in bytes; must be positive.
func NewMemoryStagingArea(maxSize int64) *MemoryStagingArea {
	return &MemoryStagingArea{
		maxSize: maxSize,
	}
}

// Compile-time check that MemoryStagingArea implements bt.StagingArea interface
var _ bt.StagingArea = (*MemoryStagingArea)(nil)
