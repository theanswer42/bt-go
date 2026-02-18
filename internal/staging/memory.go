package staging

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"bt-go/internal/bt"
)

// memoryStore is an in-memory implementation of stagingStore.
// It stores staged content in memory, making it useful for testing.
// Concurrency is managed by the caller (stagingArea.mu).
type memoryStore struct {
	content     map[string][]byte  // checksum -> content
	queue       []*stagedOperation // ordered queue of operations
	refCount    map[string]int     // checksum -> number of operations referencing it
	currentSize int64
}

var _ stagingStore = (*memoryStore)(nil)

func newMemoryStore() *memoryStore {
	return &memoryStore{
		content:  make(map[string][]byte),
		queue:    make([]*stagedOperation, 0),
		refCount: make(map[string]int),
	}
}

// NewMemoryStagingArea creates a new in-memory staging area.
// maxSize is the maximum total size in bytes; must be positive.
func NewMemoryStagingArea(fsmgr bt.FilesystemManager, maxSize int64) bt.StagingArea {
	return &stagingArea{
		fsmgr:   fsmgr,
		store:   newMemoryStore(),
		maxSize: maxSize,
	}
}

func (m *memoryStore) StoreContent(r io.Reader) (string, int64, error) {
	hash := sha256.New()
	var buf bytes.Buffer
	writer := io.MultiWriter(hash, &buf)

	if _, err := io.Copy(writer, r); err != nil {
		return "", 0, err
	}

	checksum := hex.EncodeToString(hash.Sum(nil))
	data := buf.Bytes()
	size := int64(len(data))

	// Dedup: only store if not already present
	if _, exists := m.content[checksum]; !exists {
		m.content[checksum] = data
		m.currentSize += size
	}

	return checksum, size, nil
}

func (m *memoryStore) RemoveContent(checksum string) {
	if c, ok := m.content[checksum]; ok {
		m.currentSize -= int64(len(c))
		delete(m.content, checksum)
	}
	delete(m.refCount, checksum)
}

func (m *memoryStore) OpenContent(checksum string) (io.ReadCloser, error) {
	content, ok := m.content[checksum]
	if !ok {
		return nil, fmt.Errorf("content not found: %s", checksum)
	}
	// Copy so the reader is independent of the stored slice
	cp := make([]byte, len(content))
	copy(cp, content)
	return io.NopCloser(bytes.NewReader(cp)), nil
}

func (m *memoryStore) ContentSize() (int64, error) {
	return m.currentSize, nil
}

func (m *memoryStore) Append(op *stagedOperation) error {
	m.queue = append(m.queue, op)
	m.refCount[op.Snapshot.ContentID]++
	return nil
}

func (m *memoryStore) Peek() (*stagedOperation, error) {
	if len(m.queue) == 0 {
		return nil, nil
	}
	return m.queue[0], nil
}

func (m *memoryStore) Pop(directoryID, relativePath, checksum string) (int, error) {
	for i, op := range m.queue {
		if op.DirectoryID == directoryID && op.RelativePath == relativePath && op.Snapshot.ContentID == checksum {
			m.queue = append(m.queue[:i], m.queue[i+1:]...)
			m.refCount[checksum]--
			remaining := m.refCount[checksum]
			if remaining <= 0 {
				delete(m.refCount, checksum)
				remaining = 0
			}
			return remaining, nil
		}
	}
	return 0, nil
}

func (m *memoryStore) Len() (int, error) {
	return len(m.queue), nil
}

func (m *memoryStore) Contains(directoryID, relativePath string) (bool, error) {
	for _, op := range m.queue {
		if op.DirectoryID == directoryID && op.RelativePath == relativePath {
			return true, nil
		}
	}
	return false, nil
}
