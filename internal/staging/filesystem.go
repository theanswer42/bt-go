package staging

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"bt-go/internal/bt"
	"bt-go/internal/database/sqlc"
)

// FileSystemStagingArea is a filesystem-based implementation of the StagingArea interface.
// It stores staged files in a directory structure with a queue file for ordering.
//
// Directory structure:
//
//	<staging_dir>/
//	  queue.json       (ordered list of staged operations)
//	  content/
//	    <checksum>     (staged file content, named by SHA-256)
type FileSystemStagingArea struct {
	fsmgr      bt.FilesystemManager
	stagingDir string
	contentDir string
	queueFile  string
	maxSize    int64
	mu         sync.Mutex // protects queue file access
}

// NewFileSystemStagingArea creates a new filesystem-based staging area.
// maxSize is the maximum total size in bytes; must be positive.
func NewFileSystemStagingArea(fsmgr bt.FilesystemManager, stagingDir string, maxSize int64) (*FileSystemStagingArea, error) {
	contentDir := filepath.Join(stagingDir, "content")
	queueFile := filepath.Join(stagingDir, "queue.json")

	// Create directory structure
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create staging directory: %w", err)
	}

	return &FileSystemStagingArea{
		fsmgr:      fsmgr,
		stagingDir: stagingDir,
		contentDir: contentDir,
		queueFile:  queueFile,
		maxSize:    maxSize,
	}, nil
}

// Stage stages a file for backup.
func (f *FileSystemStagingArea) Stage(directory *sqlc.Directory, file *sqlc.File, path *bt.Path) error {
	// 1. Get initial stat from the path
	info1 := path.Info()
	stat1, err := extractStatData(info1)
	if err != nil {
		return fmt.Errorf("extracting stat data: %w", err)
	}

	// 2. Open the source file
	reader, err := f.fsmgr.Open(path)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer reader.Close()

	// 3. Copy to temp file while computing checksum
	checksum, size, err := f.copyToStaging(reader)
	if err != nil {
		return fmt.Errorf("copying to staging: %w", err)
	}

	// 4. Re-stat to validate file hasn't changed
	info2, err := f.fsmgr.Stat(path)
	if err != nil {
		f.removeContent(checksum)
		return fmt.Errorf("re-stat file: %w", err)
	}
	stat2, err := extractStatData(info2)
	if err != nil {
		f.removeContent(checksum)
		return fmt.Errorf("extracting re-stat data: %w", err)
	}

	if err := validateStatUnchanged(info1, info2, stat1, stat2); err != nil {
		f.removeContent(checksum)
		return fmt.Errorf("file changed during staging: %w", err)
	}

	// 5. Check size limit
	currentSize, err := f.Size()
	if err != nil {
		f.removeContent(checksum)
		return fmt.Errorf("getting current size: %w", err)
	}
	if currentSize > f.maxSize {
		f.removeContent(checksum)
		return fmt.Errorf("staging area full: would exceed max size of %d bytes", f.maxSize)
	}

	// 6. Add operation to queue
	op := &bt.StagedOperation{
		DirectoryID: directory.ID,
		Snapshot: sqlc.FileSnapshot{
			FileID:      file.ID,
			ContentID:   checksum,
			Size:        size,
			Permissions: int64(info1.Mode().Perm()),
			Uid:         stat1.UID,
			Gid:         stat1.GID,
			AccessedAt:  stat1.Atime,
			ModifiedAt:  info1.ModTime(),
			ChangedAt:   stat1.Ctime,
			BornAt:      stat1.BirthTime,
		},
	}

	if err := f.appendToQueue(op); err != nil {
		f.removeContent(checksum)
		return fmt.Errorf("adding to queue: %w", err)
	}

	return nil
}

// Next returns the next staged operation from the queue.
func (f *FileSystemStagingArea) Next() (*bt.StagedOperation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	queue, err := f.readQueue()
	if err != nil {
		return nil, err
	}

	if len(queue) == 0 {
		return nil, nil
	}
	return queue[0], nil
}

// GetContent returns a reader for the staged content by checksum.
func (f *FileSystemStagingArea) GetContent(checksum string) (io.ReadCloser, error) {
	contentPath := filepath.Join(f.contentDir, checksum)
	file, err := os.Open(contentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("content not found: %s", checksum)
		}
		return nil, fmt.Errorf("opening content file: %w", err)
	}
	return file, nil
}

// Remove removes a staged operation after successful backup.
func (f *FileSystemStagingArea) Remove(op *bt.StagedOperation) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	queue, err := f.readQueue()
	if err != nil {
		return err
	}

	// Find and remove from queue
	found := false
	checksum := op.Snapshot.ContentID
	checksumCount := 0
	newQueue := make([]*bt.StagedOperation, 0, len(queue))

	for _, queued := range queue {
		if !found && queued.Snapshot.FileID == op.Snapshot.FileID &&
			queued.Snapshot.ContentID == op.Snapshot.ContentID {
			found = true
			continue // skip this one
		}
		newQueue = append(newQueue, queued)
		if queued.Snapshot.ContentID == checksum {
			checksumCount++
		}
	}

	if !found {
		return fmt.Errorf("operation not found in queue")
	}

	// Write updated queue
	if err := f.writeQueue(newQueue); err != nil {
		return err
	}

	// Remove content if no more references
	if checksumCount == 0 {
		f.removeContent(checksum)
	}

	return nil
}

// Count returns the number of staged operations in the queue.
func (f *FileSystemStagingArea) Count() (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	queue, err := f.readQueue()
	if err != nil {
		return 0, err
	}
	return len(queue), nil
}

// Size returns the total size of staged content in bytes.
func (f *FileSystemStagingArea) Size() (int64, error) {
	var totalSize int64

	entries, err := os.ReadDir(f.contentDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("reading content directory: %w", err)
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		totalSize += info.Size()
	}

	return totalSize, nil
}

// copyToStaging copies content to staging area while computing checksum.
// Returns the checksum and size. If content already exists (dedup), skips the copy.
func (f *FileSystemStagingArea) copyToStaging(r io.Reader) (string, int64, error) {
	// Create temp file
	tmpFile, err := os.CreateTemp(f.contentDir, ".tmp-*")
	if err != nil {
		return "", 0, fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Clean up on failure
	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

	// Copy while computing hash
	hash := sha256.New()
	writer := io.MultiWriter(hash, tmpFile)
	size, err := io.Copy(writer, r)
	if err != nil {
		tmpFile.Close()
		return "", 0, fmt.Errorf("copying content: %w", err)
	}
	tmpFile.Close()

	checksum := hex.EncodeToString(hash.Sum(nil))
	destPath := filepath.Join(f.contentDir, checksum)

	// Check if content already exists (dedup)
	if _, err := os.Stat(destPath); err == nil {
		os.Remove(tmpPath)
		success = true
		return checksum, size, nil
	}

	// Rename temp file to final name
	if err := os.Rename(tmpPath, destPath); err != nil {
		return "", 0, fmt.Errorf("renaming temp file: %w", err)
	}

	success = true
	return checksum, size, nil
}

// removeContent removes a content file by checksum.
func (f *FileSystemStagingArea) removeContent(checksum string) {
	contentPath := filepath.Join(f.contentDir, checksum)
	os.Remove(contentPath)
}

// readQueue reads the queue from disk.
func (f *FileSystemStagingArea) readQueue() ([]*bt.StagedOperation, error) {
	data, err := os.ReadFile(f.queueFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []*bt.StagedOperation{}, nil
		}
		return nil, fmt.Errorf("reading queue file: %w", err)
	}

	var queue []*bt.StagedOperation
	if err := json.Unmarshal(data, &queue); err != nil {
		return nil, fmt.Errorf("parsing queue file: %w", err)
	}

	return queue, nil
}

// writeQueue writes the queue to disk.
func (f *FileSystemStagingArea) writeQueue(queue []*bt.StagedOperation) error {
	data, err := json.MarshalIndent(queue, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling queue: %w", err)
	}

	if err := os.WriteFile(f.queueFile, data, 0644); err != nil {
		return fmt.Errorf("writing queue file: %w", err)
	}

	return nil
}

// appendToQueue adds an operation to the queue.
func (f *FileSystemStagingArea) appendToQueue(op *bt.StagedOperation) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	queue, err := f.readQueue()
	if err != nil {
		return err
	}

	queue = append(queue, op)
	return f.writeQueue(queue)
}

// Compile-time check that FileSystemStagingArea implements bt.StagingArea interface
var _ bt.StagingArea = (*FileSystemStagingArea)(nil)
