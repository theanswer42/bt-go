package bt

import (
	"fmt"
	"path/filepath"

	"bt-go/internal/database/sqlc"
)

// FileStatus represents the backup state of a single file.
type FileStatus struct {
	RelativePath    string
	IsBackedUp      bool
	IsStaged        bool
	IsModifiedSince bool
}

// GetStatus returns the backup status of files under the given path.
// If recursive is true, files in subdirectories are included.
func (s *BTService) GetStatus(path *Path, recursive bool) ([]*FileStatus, error) {
	s.logger.Debug("computing status", "path", path.String())

	if !path.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", path.String())
	}

	// Find the tracked directory for this path.
	directory, err := s.database.FindDirectoryByPath(path.String())
	if err != nil {
		return nil, fmt.Errorf("finding directory: %w", err)
	}
	if directory == nil {
		// The cwd might be inside a tracked directory.
		directory, err = s.database.SearchDirectoryForPath(path.String())
		if err != nil {
			return nil, fmt.Errorf("searching for directory: %w", err)
		}
	}
	if directory == nil {
		return nil, fmt.Errorf("directory is not tracked: %s", path.String())
	}

	// Discover files on disk.
	diskFiles, err := s.fsmgr.FindFiles(path, recursive)
	if err != nil {
		return nil, fmt.Errorf("finding files: %w", err)
	}

	// Build a set of relative paths we've seen on disk.
	seen := make(map[string]bool, len(diskFiles))
	var statuses []*FileStatus

	for _, f := range diskFiles {
		relPath, err := filepath.Rel(directory.Path, f.String())
		if err != nil {
			return nil, fmt.Errorf("computing relative path: %w", err)
		}
		seen[relPath] = true

		status, err := s.getFileStatus(directory, relPath, f)
		if err != nil {
			return nil, fmt.Errorf("getting status for %s: %w", relPath, err)
		}
		statuses = append(statuses, status)
	}

	// Also check for backed-up files that no longer exist on disk.
	dbFiles, err := s.database.FindFilesByDirectory(directory)
	if err != nil {
		return nil, fmt.Errorf("finding database files: %w", err)
	}

	// Determine the relative prefix of the queried path within the directory.
	// For example, if dir is /home/user/docs and path is /home/user/docs/sub,
	// we only want files under "sub/".
	queryPrefix := ""
	if path.String() != directory.Path {
		rel, err := filepath.Rel(directory.Path, path.String())
		if err != nil {
			return nil, fmt.Errorf("computing query prefix: %w", err)
		}
		queryPrefix = rel + "/"
	}

	for _, dbFile := range dbFiles {
		// Filter to the queried subtree.
		if queryPrefix != "" {
			if len(dbFile.Name) <= len(queryPrefix) || dbFile.Name[:len(queryPrefix)] != queryPrefix {
				continue
			}
		}
		// Filter by recursion depth.
		if !recursive && queryPrefix != "" {
			sub := dbFile.Name[len(queryPrefix):]
			if filepath.Dir(sub) != "." {
				continue
			}
		} else if !recursive && queryPrefix == "" {
			if filepath.Dir(dbFile.Name) != "." {
				continue
			}
		}

		if seen[dbFile.Name] {
			continue
		}

		// File exists in DB but not on disk — still show as backed up.
		if dbFile.CurrentSnapshotID.Valid {
			statuses = append(statuses, &FileStatus{
				RelativePath:    dbFile.Name,
				IsBackedUp:      true,
				IsModifiedSince: true, // missing from disk counts as modified
			})
		}
	}

	return statuses, nil
}

// getFileStatus computes the status for a single file on disk.
func (s *BTService) getFileStatus(directory *sqlc.Directory, relativePath string, filePath *Path) (*FileStatus, error) {
	status := &FileStatus{
		RelativePath: relativePath,
	}

	// Check if the file is staged.
	staged, err := s.stagingArea.IsStaged(directory.ID, relativePath)
	if err != nil {
		return nil, fmt.Errorf("checking staged: %w", err)
	}
	status.IsStaged = staged

	// Check if the file has been backed up.
	file, err := s.database.FindFileByPath(directory, relativePath)
	if err != nil {
		return nil, fmt.Errorf("finding file: %w", err)
	}
	if file == nil || !file.CurrentSnapshotID.Valid {
		// Not backed up.
		return status, nil
	}

	status.IsBackedUp = true

	// Check if modified since last backup using the mtime+size heuristic.
	snapshots, err := s.database.FindFileSnapshotsForFile(file)
	if err != nil {
		return nil, fmt.Errorf("finding snapshots: %w", err)
	}

	// Find the current snapshot.
	var currentSnapshot *sqlc.FileSnapshot
	for _, snap := range snapshots {
		if snap.ID == file.CurrentSnapshotID.String {
			currentSnapshot = snap
			break
		}
	}
	if currentSnapshot == nil {
		// Snapshot referenced but not found — treat as modified.
		status.IsModifiedSince = true
		return status, nil
	}

	// Compare mtime and size against current file.
	info := filePath.Info()
	if info.Size() != currentSnapshot.Size || !info.ModTime().Equal(currentSnapshot.ModifiedAt) {
		status.IsModifiedSince = true
	}

	return status, nil
}
