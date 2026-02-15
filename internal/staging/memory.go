package staging

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"sync"

	"bt-go/internal/bt"
	"bt-go/internal/database/sqlc"
)

// MemoryStagingArea is an in-memory implementation of the StagingArea interface.
// It stores staged files in memory, making it useful for testing.
// This implementation is safe for concurrent use.
type MemoryStagingArea struct {
	fsmgr       bt.FilesystemManager
	maxSize     int64
	currentSize int64
	content     map[string][]byte       // checksum -> content
	queue       []*bt.StagedOperation   // ordered queue of operations
	refCount    map[string]int          // checksum -> number of operations referencing it
	mu          sync.Mutex
}

// NewMemoryStagingArea creates a new in-memory staging area.
// maxSize is the maximum total size in bytes; must be positive.
func NewMemoryStagingArea(fsmgr bt.FilesystemManager, maxSize int64) *MemoryStagingArea {
	return &MemoryStagingArea{
		fsmgr:    fsmgr,
		maxSize:  maxSize,
		content:  make(map[string][]byte),
		queue:    make([]*bt.StagedOperation, 0),
		refCount: make(map[string]int),
	}
}

// Stage stages a file for backup.
func (m *MemoryStagingArea) Stage(directory *sqlc.Directory, file *sqlc.File, path *bt.Path) error {
	// 1. Get initial stat from the path
	info1 := path.Info()
	stat1, err := extractStatData(info1)
	if err != nil {
		return fmt.Errorf("extracting stat data: %w", err)
	}

	// 2. Open and read the file, computing checksum
	reader, err := m.fsmgr.Open(path)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}

	content, checksum, err := m.readAndHash(reader)
	reader.Close()
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	// 3. Re-stat to validate file hasn't changed
	info2, err := m.fsmgr.Stat(path)
	if err != nil {
		return fmt.Errorf("re-stat file: %w", err)
	}
	stat2, err := extractStatData(info2)
	if err != nil {
		return fmt.Errorf("extracting re-stat data: %w", err)
	}

	if err := validateStatUnchanged(info1, info2, stat1, stat2); err != nil {
		return fmt.Errorf("file changed during staging: %w", err)
	}

	// 4. Check size limit and store
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if content already exists (dedup)
	if _, exists := m.content[checksum]; !exists {
		// Check size limit
		if m.currentSize+int64(len(content)) > m.maxSize {
			return fmt.Errorf("staging area full: would exceed max size of %d bytes", m.maxSize)
		}
		m.content[checksum] = content
		m.currentSize += int64(len(content))
	}

	// 5. Add operation to queue
	op := &bt.StagedOperation{
		DirectoryID: directory.ID,
		Snapshot: sqlc.FileSnapshot{
			FileID:      file.ID,
			ContentID:   checksum,
			Size:        info1.Size(),
			Permissions: int64(info1.Mode().Perm()),
			Uid:         stat1.UID,
			Gid:         stat1.GID,
			AccessedAt:  stat1.Atime,
			ModifiedAt:  info1.ModTime(),
			ChangedAt:   stat1.Ctime,
			BornAt:      stat1.BirthTime,
		},
	}
	m.queue = append(m.queue, op)
	m.refCount[checksum]++

	return nil
}

// Next returns the next staged operation from the queue.
func (m *MemoryStagingArea) Next() (*bt.StagedOperation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.queue) == 0 {
		return nil, nil
	}
	return m.queue[0], nil
}

// GetContent returns a reader for the staged content by checksum.
func (m *MemoryStagingArea) GetContent(checksum string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	content, ok := m.content[checksum]
	if !ok {
		return nil, fmt.Errorf("content not found: %s", checksum)
	}
	return io.NopCloser(bytes.NewReader(content)), nil
}

// Remove removes a staged operation after successful backup.
func (m *MemoryStagingArea) Remove(op *bt.StagedOperation) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find and remove from queue
	found := false
	for i, queued := range m.queue {
		if queued == op {
			m.queue = append(m.queue[:i], m.queue[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("operation not found in queue")
	}

	// Decrement ref count and remove content if no more references
	checksum := op.Snapshot.ContentID
	m.refCount[checksum]--
	if m.refCount[checksum] <= 0 {
		if content, ok := m.content[checksum]; ok {
			m.currentSize -= int64(len(content))
			delete(m.content, checksum)
		}
		delete(m.refCount, checksum)
	}

	return nil
}

// Count returns the number of staged operations in the queue.
func (m *MemoryStagingArea) Count() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.queue), nil
}

// Size returns the total size of staged content in bytes.
func (m *MemoryStagingArea) Size() (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.currentSize, nil
}

// readAndHash reads all content and computes SHA-256 checksum.
func (m *MemoryStagingArea) readAndHash(r io.Reader) ([]byte, string, error) {
	hash := sha256.New()
	var buf bytes.Buffer
	writer := io.MultiWriter(hash, &buf)

	if _, err := io.Copy(writer, r); err != nil {
		return nil, "", err
	}

	checksum := hex.EncodeToString(hash.Sum(nil))
	return buf.Bytes(), checksum, nil
}

// validateStatUnchanged checks that file metadata hasn't changed.
// We ignore access time as it may change from our read.
func validateStatUnchanged(info1, info2 fs.FileInfo, stat1, stat2 *statData) error {
	if info1.Size() != info2.Size() {
		return fmt.Errorf("size changed: %d -> %d", info1.Size(), info2.Size())
	}
	if info1.Mode() != info2.Mode() {
		return fmt.Errorf("mode changed: %v -> %v", info1.Mode(), info2.Mode())
	}
	if !info1.ModTime().Equal(info2.ModTime()) {
		return fmt.Errorf("mtime changed: %v -> %v", info1.ModTime(), info2.ModTime())
	}
	if !stat1.Ctime.Equal(stat2.Ctime) {
		return fmt.Errorf("ctime changed: %v -> %v", stat1.Ctime, stat2.Ctime)
	}
	if stat1.UID != stat2.UID {
		return fmt.Errorf("uid changed: %d -> %d", stat1.UID, stat2.UID)
	}
	if stat1.GID != stat2.GID {
		return fmt.Errorf("gid changed: %d -> %d", stat1.GID, stat2.GID)
	}
	return nil
}

// Compile-time check that MemoryStagingArea implements bt.StagingArea interface
var _ bt.StagingArea = (*MemoryStagingArea)(nil)
