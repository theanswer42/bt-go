package bt

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"bt-go/internal/database/sqlc"
)

// Restore restores files from the vault.
// If absPath matches a tracked directory exactly, all files in that directory are restored.
// Providing a checksum with a directory path is an error.
// Otherwise, absPath is treated as a file path and the specified (or current) version is restored.
// Returns the list of output file paths written.
func (s *BTService) Restore(absPath string, checksum string) ([]string, error) {
	s.logger.Info("restore started", "path", absPath)

	// Check if absPath matches a tracked directory exactly.
	dir, err := s.database.FindDirectoryByPath(absPath)
	if err != nil {
		return nil, fmt.Errorf("checking directory: %w", err)
	}

	if dir != nil {
		if checksum != "" {
			return nil, fmt.Errorf("cannot restore a directory with a specific checksum")
		}
		return s.restoreDirectory(dir)
	}

	// Treat as a file path.
	outPath, err := s.restoreFile(absPath, checksum)
	if err != nil {
		return nil, err
	}
	return []string{outPath}, nil
}

// restoreFile restores a single file from the vault.
func (s *BTService) restoreFile(absPath string, checksum string) (string, error) {
	directory, err := s.database.SearchDirectoryForPath(absPath)
	if err != nil {
		return "", fmt.Errorf("searching for directory: %w", err)
	}
	if directory == nil {
		return "", fmt.Errorf("file is not within a tracked directory: %s", absPath)
	}

	relativePath, err := filepath.Rel(directory.Path, absPath)
	if err != nil {
		return "", fmt.Errorf("calculating relative path: %w", err)
	}

	file, err := s.database.FindFileByPath(directory, relativePath)
	if err != nil {
		return "", fmt.Errorf("finding file: %w", err)
	}
	if file == nil {
		return "", fmt.Errorf("file has no backup history: %s", absPath)
	}

	snapshot, err := s.resolveSnapshot(file, checksum)
	if err != nil {
		return "", err
	}

	return s.restoreOneFile(directory, relativePath, snapshot)
}

// resolveSnapshot finds the appropriate snapshot for restore.
// If checksum is provided, looks up the specific version.
// Otherwise, uses the file's current snapshot.
func (s *BTService) resolveSnapshot(file *sqlc.File, checksum string) (*sqlc.FileSnapshot, error) {
	if checksum != "" {
		snap, err := s.database.FindFileSnapshotByChecksum(file, checksum)
		if err != nil {
			return nil, fmt.Errorf("finding snapshot by checksum: %w", err)
		}
		if snap == nil {
			return nil, fmt.Errorf("no snapshot found with checksum %s", checksum)
		}
		return snap, nil
	}

	if !file.CurrentSnapshotID.Valid {
		return nil, fmt.Errorf("file has no current snapshot")
	}

	snapshots, err := s.database.FindFileSnapshotsForFile(file)
	if err != nil {
		return nil, fmt.Errorf("finding snapshots: %w", err)
	}

	for _, snap := range snapshots {
		if snap.ID == file.CurrentSnapshotID.String {
			return snap, nil
		}
	}

	return nil, fmt.Errorf("current snapshot not found in database")
}

// restoreDirectory restores all files in a tracked directory.
func (s *BTService) restoreDirectory(dir *sqlc.Directory) ([]string, error) {
	files, err := s.database.FindFilesByDirectory(dir)
	if err != nil {
		return nil, fmt.Errorf("finding files: %w", err)
	}

	var restored []string
	for _, file := range files {
		if file.Deleted || !file.CurrentSnapshotID.Valid {
			continue
		}

		snapshot, err := s.resolveSnapshot(file, "")
		if err != nil {
			return restored, fmt.Errorf("resolving snapshot for %s: %w", file.Name, err)
		}

		outPath, err := s.restoreOneFile(dir, file.Name, snapshot)
		if err != nil {
			return restored, fmt.Errorf("restoring %s: %w", file.Name, err)
		}
		restored = append(restored, outPath)
	}

	return restored, nil
}

// restoreOneFile writes a single file from the vault to disk.
// The output path is {dir}/{basename}.{checksum[:12]}.btrestored.
func (s *BTService) restoreOneFile(dir *sqlc.Directory, relativePath string, snapshot *sqlc.FileSnapshot) (string, error) {
	outPath := buildRestorePath(dir.Path, relativePath, snapshot.ContentID)

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return "", fmt.Errorf("creating parent directory: %w", err)
	}

	// Fail if file already exists.
	if _, err := os.Stat(outPath); err == nil {
		return "", fmt.Errorf("output file already exists: %s", outPath)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("creating output file: %w", err)
	}
	defer f.Close()

	if err := s.vault.GetContent(snapshot.ContentID, f); err != nil {
		os.Remove(outPath)
		return "", fmt.Errorf("retrieving content from vault: %w", err)
	}

	// Restore metadata.
	if err := os.Chmod(outPath, fs.FileMode(snapshot.Permissions)); err != nil {
		return "", fmt.Errorf("setting permissions: %w", err)
	}
	if err := os.Chtimes(outPath, snapshot.AccessedAt, snapshot.ModifiedAt); err != nil {
		return "", fmt.Errorf("setting file times: %w", err)
	}

	s.logger.Info("file restored", "path", outPath)
	return outPath, nil
}

// buildRestorePath constructs the output path for a restored file.
// Format: {dir}/{basename}.{checksum[:12]}.btrestored
func buildRestorePath(dirPath string, relativePath string, contentID string) string {
	fullPath := filepath.Join(dirPath, relativePath)
	dir := filepath.Dir(fullPath)
	base := filepath.Base(fullPath)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]

	shortChecksum := contentID
	if len(shortChecksum) > 12 {
		shortChecksum = shortChecksum[:12]
	}

	restored := fmt.Sprintf("%s.%s.btrestored", name+ext, shortChecksum)
	return filepath.Join(dir, restored)
}
