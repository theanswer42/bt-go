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
	content     map[string][]byte      // checksum -> content
	queue       []*stagedOperation     // ordered queue of operations
	refCount    map[string]int         // checksum -> number of operations referencing it
	mu          sync.Mutex
}

// NewMemoryStagingArea creates a new in-memory staging area.
// maxSize is the maximum total size in bytes; must be positive.
func NewMemoryStagingArea(fsmgr bt.FilesystemManager, maxSize int64) *MemoryStagingArea {
	return &MemoryStagingArea{
		fsmgr:    fsmgr,
		maxSize:  maxSize,
		content:  make(map[string][]byte),
		queue:    make([]*stagedOperation, 0),
		refCount: make(map[string]int),
	}
}

// Stage stages a file for backup.
func (m *MemoryStagingArea) Stage(directory *sqlc.Directory, relativePath string, path *bt.Path) error {
	// 1. Get initial stat from the path
	info1 := path.Info()
	stat1, err := m.fsmgr.ExtractStatData(info1)
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
	stat2, err := m.fsmgr.ExtractStatData(info2)
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
	op := &stagedOperation{
		DirectoryID:  directory.ID,
		RelativePath: relativePath,
		Snapshot: sqlc.FileSnapshot{
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

// ProcessNext gets the next staged operation and calls fn with its data.
// If fn returns nil, the staged operation is removed (committed).
// If fn returns an error, the operation stays in queue for retry.
// Returns nil with no error if the queue is empty.
func (m *MemoryStagingArea) ProcessNext(fn bt.BackupFunc) error {
	m.mu.Lock()
	if len(m.queue) == 0 {
		m.mu.Unlock()
		return nil
	}
	op := m.queue[0]
	checksum := op.Snapshot.ContentID
	content, ok := m.content[checksum]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("content not found: %s", checksum)
	}
	// Make a copy of content so we can release the lock during callback
	contentCopy := make([]byte, len(content))
	copy(contentCopy, content)
	m.mu.Unlock()

	// Call the backup function
	reader := bytes.NewReader(contentCopy)
	if err := fn(reader, op.Snapshot, op.DirectoryID, op.RelativePath); err != nil {
		return err
	}

	// Success - remove the operation
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove the processed operation from the front of the queue
	if len(m.queue) > 0 && m.queue[0].DirectoryID == op.DirectoryID &&
		m.queue[0].RelativePath == op.RelativePath &&
		m.queue[0].Snapshot.ContentID == op.Snapshot.ContentID {
		m.queue = m.queue[1:]
	}

	// Decrement ref count and remove content if no more references
	m.refCount[checksum]--
	if m.refCount[checksum] <= 0 {
		if c, ok := m.content[checksum]; ok {
			m.currentSize -= int64(len(c))
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
func validateStatUnchanged(info1, info2 fs.FileInfo, stat1, stat2 *bt.StatData) error {
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
