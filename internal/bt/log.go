package bt

import (
	"fmt"
	"path/filepath"
	"time"
)

// FileHistoryEntry represents a single backed-up version of a file.
type FileHistoryEntry struct {
	ContentChecksum string
	BackedUpAt      time.Time
	Size            int64
	ModifiedAt      time.Time
	IsCurrent       bool
}

// GetFileHistory returns the backup history for a file, newest first.
func (s *BTService) GetFileHistory(path *Path) ([]*FileHistoryEntry, error) {
	s.logger.Debug("fetching file history", "path", path.String())

	if path.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a file: %s", path.String())
	}

	directory, err := s.database.SearchDirectoryForPath(path.String())
	if err != nil {
		return nil, fmt.Errorf("searching for directory: %w", err)
	}
	if directory == nil {
		return nil, fmt.Errorf("file is not within a tracked directory: %s", path.String())
	}

	relativePath, err := filepath.Rel(directory.Path, path.String())
	if err != nil {
		return nil, fmt.Errorf("calculating relative path: %w", err)
	}

	file, err := s.database.FindFileByPath(directory, relativePath)
	if err != nil {
		return nil, fmt.Errorf("finding file: %w", err)
	}
	if file == nil {
		return nil, fmt.Errorf("file has no backup history: %s", path.String())
	}

	snapshots, err := s.database.FindFileSnapshotsForFile(file)
	if err != nil {
		return nil, fmt.Errorf("finding snapshots: %w", err)
	}

	entries := make([]*FileHistoryEntry, len(snapshots))
	for i, snap := range snapshots {
		entries[i] = &FileHistoryEntry{
			ContentChecksum: snap.ContentID,
			BackedUpAt:      snap.CreatedAt,
			Size:            snap.Size,
			ModifiedAt:      snap.ModifiedAt,
			IsCurrent:       file.CurrentSnapshotID.Valid && file.CurrentSnapshotID.String == snap.ID,
		}
	}

	// Reverse to newest first
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	return entries, nil
}
