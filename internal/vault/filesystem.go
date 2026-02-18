package vault

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"bt-go/internal/bt"
)

// FileSystemVault is a filesystem-based implementation of the Vault interface.
// It stores content and metadata as files in a directory structure:
//
//	<root>/
//	  content/
//	    <checksum>     (content files, named by SHA-256)
//	  metadata/
//	    <hostID>.db    (per-host metadata files)
type FileSystemVault struct {
	name        string
	root        string
	contentDir  string
	metadataDir string
}

// NewFileSystemVault creates a new filesystem vault rooted at the given path.
func NewFileSystemVault(name, root string) (*FileSystemVault, error) {
	contentDir := filepath.Join(root, "content")
	metadataDir := filepath.Join(root, "metadata")

	// Create directory structure
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create content directory: %w", err)
	}
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create metadata directory: %w", err)
	}

	return &FileSystemVault{
		name:        name,
		root:        root,
		contentDir:  contentDir,
		metadataDir: metadataDir,
	}, nil
}

// PutContent stores content identified by its checksum.
// The operation is idempotent: storing the same checksum multiple times is safe.
func (v *FileSystemVault) PutContent(checksum string, r io.Reader, size int64) error {
	destPath := filepath.Join(v.contentDir, checksum)

	// If content already exists, skip (idempotent)
	if _, err := os.Stat(destPath); err == nil {
		// Consume the reader to maintain expected behavior
		written, err := io.Copy(io.Discard, r)
		if err != nil {
			return fmt.Errorf("failed to read content: %w", err)
		}
		if written != size {
			return fmt.Errorf("size mismatch: expected %d bytes, got %d", size, written)
		}
		return nil
	}

	return v.writeFile(destPath, r, size)
}

// GetContent retrieves content by checksum and writes it to w.
func (v *FileSystemVault) GetContent(checksum string, w io.Writer) error {
	srcPath := filepath.Join(v.contentDir, checksum)
	return v.readFile(srcPath, w, fmt.Sprintf("content not found: %s", checksum))
}

// PutMetadata stores metadata for a specific host along with a version marker.
func (v *FileSystemVault) PutMetadata(hostID string, r io.Reader, size int64, version int64) error {
	destPath := filepath.Join(v.metadataDir, hostID+".db")
	if err := v.writeFile(destPath, r, size); err != nil {
		return err
	}

	// Write version file
	versionPath := filepath.Join(v.metadataDir, hostID+".version")
	versionData := strconv.FormatInt(version, 10)
	return os.WriteFile(versionPath, []byte(versionData), 0644)
}

// GetMetadataVersion returns the metadata version for a host.
// Returns 0 if no version file exists.
func (v *FileSystemVault) GetMetadataVersion(hostID string) (int64, error) {
	versionPath := filepath.Join(v.metadataDir, hostID+".version")
	data, err := os.ReadFile(versionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("reading version file: %w", err)
	}

	version, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing version: %w", err)
	}
	return version, nil
}

// GetMetadata retrieves metadata for a specific host and writes it to w.
func (v *FileSystemVault) GetMetadata(hostID string, w io.Writer) error {
	srcPath := filepath.Join(v.metadataDir, hostID+".db")
	return v.readFile(srcPath, w, fmt.Sprintf("metadata not found for host: %s", hostID))
}

// ValidateSetup verifies that the vault directories are accessible.
func (v *FileSystemVault) ValidateSetup() error {
	// Check that root directory exists and is a directory
	info, err := os.Stat(v.root)
	if err != nil {
		return fmt.Errorf("vault root not accessible: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("vault root is not a directory: %s", v.root)
	}

	// Check that subdirectories exist and are writable
	for _, dir := range []string{v.contentDir, v.metadataDir} {
		info, err := os.Stat(dir)
		if err != nil {
			return fmt.Errorf("vault directory not accessible: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("vault path is not a directory: %s", dir)
		}
	}

	return nil
}

// writeFile writes data from r to the specified path using atomic write (temp file + rename).
func (v *FileSystemVault) writeFile(destPath string, r io.Reader, expectedSize int64) error {
	// Create temp file in the same directory to ensure atomic rename works
	dir := filepath.Dir(destPath)
	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Clean up temp file on failure
	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

	// Copy data to temp file
	written, err := io.Copy(tmpFile, r)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write data: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Verify size
	if written != expectedSize {
		return fmt.Errorf("size mismatch: expected %d bytes, got %d", expectedSize, written)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	success = true
	return nil
}

// readFile reads from the specified path and writes to w.
func (v *FileSystemVault) readFile(srcPath string, w io.Writer, notFoundMsg string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s", notFoundMsg)
		}
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(w, f); err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	return nil
}

// Compile-time check that FileSystemVault implements bt.Vault interface
var _ bt.Vault = (*FileSystemVault)(nil)
