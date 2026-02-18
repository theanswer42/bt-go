package staging

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"bt-go/internal/bt"
)

// filesystemStore is a filesystem-based implementation of stagingStore.
// It stores staged files in a directory structure with a queue file for ordering.
//
// Directory structure:
//
//	<staging_dir>/
//	  queue.json       (ordered list of staged operations)
//	  content/
//	    <checksum>     (staged file content, named by SHA-256)
//
// Concurrency is managed by the caller (stagingArea.mu).
type filesystemStore struct {
	contentDir string
	queueFile  string
}

var _ stagingStore = (*filesystemStore)(nil)

// NewFileSystemStagingArea creates a new filesystem-based staging area.
// maxSize is the maximum total size in bytes; must be positive.
func NewFileSystemStagingArea(fsmgr bt.FilesystemManager, stagingDir string, maxSize int64) (bt.StagingArea, error) {
	contentDir := filepath.Join(stagingDir, "content")
	queueFile := filepath.Join(stagingDir, "queue.json")

	if err := os.MkdirAll(contentDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create staging directory: %w", err)
	}

	return &stagingArea{
		fsmgr: fsmgr,
		store: &filesystemStore{
			contentDir: contentDir,
			queueFile:  queueFile,
		},
		maxSize: maxSize,
	}, nil
}

func (f *filesystemStore) StoreContent(r io.Reader) (string, int64, error) {
	// Create temp file
	tmpFile, err := os.CreateTemp(f.contentDir, ".tmp-*")
	if err != nil {
		return "", 0, fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

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

	// Dedup: if content already exists, discard temp file
	if _, err := os.Stat(destPath); err == nil {
		os.Remove(tmpPath)
		success = true
		return checksum, size, nil
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		return "", 0, fmt.Errorf("renaming temp file: %w", err)
	}

	success = true
	return checksum, size, nil
}

func (f *filesystemStore) RemoveContent(checksum string) {
	os.Remove(filepath.Join(f.contentDir, checksum))
}

func (f *filesystemStore) OpenContent(checksum string) (io.ReadCloser, error) {
	path := filepath.Join(f.contentDir, checksum)
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("content not found: %s", checksum)
		}
		return nil, fmt.Errorf("opening content file: %w", err)
	}
	return file, nil
}

func (f *filesystemStore) ContentSize() (int64, error) {
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

func (f *filesystemStore) Append(op *stagedOperation) error {
	queue, err := f.readQueue()
	if err != nil {
		return err
	}
	queue = append(queue, op)
	return f.writeQueue(queue)
}

func (f *filesystemStore) Peek() (*stagedOperation, error) {
	queue, err := f.readQueue()
	if err != nil {
		return nil, err
	}
	if len(queue) == 0 {
		return nil, nil
	}
	return queue[0], nil
}

func (f *filesystemStore) Pop(directoryID, relativePath, checksum string) (int, error) {
	queue, err := f.readQueue()
	if err != nil {
		return 0, err
	}

	newQueue := make([]*stagedOperation, 0, len(queue))
	removed := false
	checksumCount := 0

	for _, op := range queue {
		if !removed && op.DirectoryID == directoryID &&
			op.RelativePath == relativePath &&
			op.Snapshot.ContentID == checksum {
			removed = true
			continue
		}
		newQueue = append(newQueue, op)
		if op.Snapshot.ContentID == checksum {
			checksumCount++
		}
	}

	if err := f.writeQueue(newQueue); err != nil {
		return 0, err
	}
	return checksumCount, nil
}

func (f *filesystemStore) Len() (int, error) {
	queue, err := f.readQueue()
	if err != nil {
		return 0, err
	}
	return len(queue), nil
}

func (f *filesystemStore) Contains(directoryID, relativePath string) (bool, error) {
	queue, err := f.readQueue()
	if err != nil {
		return false, err
	}
	for _, op := range queue {
		if op.DirectoryID == directoryID && op.RelativePath == relativePath {
			return true, nil
		}
	}
	return false, nil
}

func (f *filesystemStore) readQueue() ([]*stagedOperation, error) {
	data, err := os.ReadFile(f.queueFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []*stagedOperation{}, nil
		}
		return nil, fmt.Errorf("reading queue file: %w", err)
	}

	var queue []*stagedOperation
	if err := json.Unmarshal(data, &queue); err != nil {
		return nil, fmt.Errorf("parsing queue file: %w", err)
	}

	return queue, nil
}

func (f *filesystemStore) writeQueue(queue []*stagedOperation) error {
	data, err := json.MarshalIndent(queue, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling queue: %w", err)
	}

	if err := os.WriteFile(f.queueFile, data, 0644); err != nil {
		return fmt.Errorf("writing queue file: %w", err)
	}

	return nil
}
