package vault

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	"bt-go/internal/bt"
)

// MemoryVault is an in-memory implementation of the Vault interface.
// It stores all content and metadata in memory, making it useful for testing.
// This implementation is safe for concurrent use.
type MemoryVault struct {
	name            string
	content         map[string][]byte // checksum -> content
	metadata        map[string][]byte // "hostID/name" -> metadata
	metadataVersion map[string]int64  // "hostID/name" -> version
	mu              sync.RWMutex
}

// NewMemoryVault creates a new in-memory vault with the given name.
func NewMemoryVault(name string) *MemoryVault {
	return &MemoryVault{
		name:            name,
		content:         make(map[string][]byte),
		metadata:        make(map[string][]byte),
		metadataVersion: make(map[string]int64),
	}
}

// metadataKey returns the map key for a host/name pair.
func metadataKey(hostID, name string) string {
	return hostID + "/" + name
}

// PutContent stores content identified by its checksum.
func (m *MemoryVault) PutContent(checksum string, r io.Reader, size int64) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read content: %w", err)
	}

	if int64(len(data)) != size {
		return fmt.Errorf("size mismatch: expected %d bytes, got %d", size, len(data))
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Idempotent: storing the same checksum multiple times is safe
	m.content[checksum] = data
	return nil
}

// GetContent retrieves content by checksum.
func (m *MemoryVault) GetContent(checksum string, w io.Writer) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, ok := m.content[checksum]
	if !ok {
		return fmt.Errorf("content not found: %s", checksum)
	}

	if _, err := io.Copy(w, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("failed to write content: %w", err)
	}

	return nil
}

// PutMetadata stores a named metadata item for a specific host.
func (m *MemoryVault) PutMetadata(hostID string, name string, r io.Reader, size int64, version int64) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read metadata: %w", err)
	}

	if int64(len(data)) != size {
		return fmt.Errorf("size mismatch: expected %d bytes, got %d", size, len(data))
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	key := metadataKey(hostID, name)
	m.metadata[key] = data
	m.metadataVersion[key] = version
	return nil
}

// GetMetadataVersion returns the metadata version for a named item on a host.
// Returns 0 if no metadata has been stored for this host/name.
func (m *MemoryVault) GetMetadataVersion(hostID string, name string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.metadataVersion[metadataKey(hostID, name)], nil
}

// GetMetadata retrieves a named metadata item for a specific host.
func (m *MemoryVault) GetMetadata(hostID string, name string, w io.Writer) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := metadataKey(hostID, name)
	data, ok := m.metadata[key]
	if !ok {
		return fmt.Errorf("metadata %q not found for host: %s", name, hostID)
	}

	if _, err := io.Copy(w, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

// ValidateSetup always succeeds for in-memory vault.
func (m *MemoryVault) ValidateSetup() error {
	return nil
}

// Compile-time check that MemoryVault implements bt.Vault interface
var _ bt.Vault = (*MemoryVault)(nil)
