package staging

import (
	"fmt"
	"sync"

	"bt-go/internal/bt"
	"bt-go/internal/database/sqlc"
)

// stagingArea implements bt.StagingArea using a pluggable stagingStore
// for the storage mechanics. All shared algorithm logic lives here.
type stagingArea struct {
	fsmgr   bt.FilesystemManager
	store   stagingStore
	maxSize int64
	mu      sync.Mutex
}

var _ bt.StagingArea = (*stagingArea)(nil)

// Stage stages a file for backup.
func (s *stagingArea) Stage(directory *sqlc.Directory, relativePath string, path *bt.Path) error {
	// 1. Get initial stat from the path
	info1 := path.Info()
	stat1, err := s.fsmgr.ExtractStatData(info1)
	if err != nil {
		return fmt.Errorf("extracting stat data: %w", err)
	}

	// 2. Open the source file
	reader, err := s.fsmgr.Open(path)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}

	// 3. Store content (hash + store), then close reader
	s.mu.Lock()
	checksum, size, err := s.store.StoreContent(reader)
	s.mu.Unlock()
	reader.Close()
	if err != nil {
		return fmt.Errorf("storing content: %w", err)
	}

	// 4. Re-stat to validate file hasn't changed
	info2, err := s.fsmgr.Stat(path)
	if err != nil {
		s.mu.Lock()
		s.store.RemoveContent(checksum)
		s.mu.Unlock()
		return fmt.Errorf("re-stat file: %w", err)
	}
	stat2, err := s.fsmgr.ExtractStatData(info2)
	if err != nil {
		s.mu.Lock()
		s.store.RemoveContent(checksum)
		s.mu.Unlock()
		return fmt.Errorf("extracting re-stat data: %w", err)
	}

	if err := validateStatUnchanged(info1, info2, stat1, stat2); err != nil {
		s.mu.Lock()
		s.store.RemoveContent(checksum)
		s.mu.Unlock()
		return fmt.Errorf("file changed during staging: %w", err)
	}

	// 5. Check size limit and enqueue
	s.mu.Lock()
	defer s.mu.Unlock()

	contentSize, err := s.store.ContentSize()
	if err != nil {
		s.store.RemoveContent(checksum)
		return fmt.Errorf("getting current size: %w", err)
	}
	if contentSize > s.maxSize {
		s.store.RemoveContent(checksum)
		return fmt.Errorf("staging area full: would exceed max size of %d bytes", s.maxSize)
	}

	// 6. Add operation to queue
	op := &stagedOperation{
		DirectoryID:  directory.ID,
		RelativePath: relativePath,
		Snapshot: sqlc.FileSnapshot{
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

	if err := s.store.Append(op); err != nil {
		s.store.RemoveContent(checksum)
		return fmt.Errorf("adding to queue: %w", err)
	}

	return nil
}

// ProcessNext gets the next staged operation and calls fn with its data.
// If fn returns nil, the staged operation is removed (committed).
// If fn returns an error, the operation stays in queue for retry.
// Returns nil with no error if the queue is empty.
func (s *stagingArea) ProcessNext(fn bt.BackupFunc) error {
	s.mu.Lock()
	op, err := s.store.Peek()
	if err != nil {
		s.mu.Unlock()
		return err
	}
	if op == nil {
		s.mu.Unlock()
		return nil
	}

	checksum := op.Snapshot.ContentID
	reader, err := s.store.OpenContent(checksum)
	s.mu.Unlock()
	if err != nil {
		return fmt.Errorf("content not found: %s", checksum)
	}
	defer reader.Close()

	// Call the backup function outside the lock
	if err := fn(reader, op.Snapshot, op.DirectoryID, op.RelativePath); err != nil {
		return err
	}

	// Success - remove the operation
	s.mu.Lock()
	defer s.mu.Unlock()

	remaining, err := s.store.Pop(op.DirectoryID, op.RelativePath, checksum)
	if err != nil {
		return err
	}

	if remaining == 0 {
		s.store.RemoveContent(checksum)
	}

	return nil
}

// Count returns the number of staged operations in the queue.
func (s *stagingArea) Count() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store.Len()
}

// Size returns the total size of staged content in bytes.
func (s *stagingArea) Size() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store.ContentSize()
}

// IsStaged reports whether a file is currently in the staging queue.
func (s *stagingArea) IsStaged(directoryID string, relativePath string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store.Contains(directoryID, relativePath)
}
