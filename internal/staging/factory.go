package staging

import (
	"fmt"

	"bt-go/internal/bt"
	"bt-go/internal/config"
)

// DefaultMaxSize is the default maximum staging area size (1MB).
const DefaultMaxSize int64 = 1024 * 1024

// NewStagingAreaFromConfig creates a StagingArea implementation based on the config type.
func NewStagingAreaFromConfig(cfg config.StagingConfig, fsmgr bt.FilesystemManager) (bt.StagingArea, error) {
	maxSize := cfg.MaxSize
	if maxSize <= 0 {
		maxSize = DefaultMaxSize
	}

	switch cfg.Type {
	case "memory":
		return NewMemoryStagingArea(fsmgr, maxSize), nil
	case "filesystem":
		if cfg.StagingDir == "" {
			return nil, fmt.Errorf("filesystem staging area requires staging_dir to be set")
		}
		return NewFileSystemStagingArea(fsmgr, cfg.StagingDir, maxSize)
	default:
		return nil, fmt.Errorf("unknown staging area type: %s", cfg.Type)
	}
}
