package vault

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewFileSystemVault(t *testing.T) {
	t.Run("creates directory structure", func(t *testing.T) {
		tmpDir := t.TempDir()
		root := filepath.Join(tmpDir, "vault")

		v, err := NewFileSystemVault("test", root)
		if err != nil {
			t.Fatalf("NewFileSystemVault() error = %v", err)
		}

		// Check directories were created
		if _, err := os.Stat(filepath.Join(root, "content")); err != nil {
			t.Errorf("content directory not created: %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, "metadata")); err != nil {
			t.Errorf("metadata directory not created: %v", err)
		}

		if v.name != "test" {
			t.Errorf("name = %q, want %q", v.name, "test")
		}
	})

	t.Run("works with existing directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		_, err := NewFileSystemVault("test", tmpDir)
		if err != nil {
			t.Fatalf("NewFileSystemVault() error = %v", err)
		}
	})
}

func TestFileSystemVault_PutContent(t *testing.T) {
	tests := []struct {
		name     string
		checksum string
		data     string
		size     int64
		wantErr  bool
	}{
		{
			name:     "store content successfully",
			checksum: "abc123",
			data:     "hello world",
			size:     11,
			wantErr:  false,
		},
		{
			name:     "size mismatch",
			checksum: "def456",
			data:     "hello",
			size:     100,
			wantErr:  true,
		},
		{
			name:     "empty content",
			checksum: "empty",
			data:     "",
			size:     0,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := NewFileSystemVault("test", t.TempDir())
			if err != nil {
				t.Fatalf("NewFileSystemVault() error = %v", err)
			}

			err = v.PutContent(tt.checksum, strings.NewReader(tt.data), tt.size)
			if (err != nil) != tt.wantErr {
				t.Errorf("PutContent() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				// Verify file exists with correct content
				contentPath := filepath.Join(v.contentDir, tt.checksum)
				data, err := os.ReadFile(contentPath)
				if err != nil {
					t.Fatalf("failed to read content file: %v", err)
				}
				if string(data) != tt.data {
					t.Errorf("content = %q, want %q", string(data), tt.data)
				}
			}
		})
	}
}

func TestFileSystemVault_PutContent_Idempotent(t *testing.T) {
	v, err := NewFileSystemVault("test", t.TempDir())
	if err != nil {
		t.Fatalf("NewFileSystemVault() error = %v", err)
	}

	checksum := "abc123"
	data := "hello world"

	// Store content first time
	if err := v.PutContent(checksum, strings.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("first PutContent() error = %v", err)
	}

	// Store same content again - should succeed
	if err := v.PutContent(checksum, strings.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("second PutContent() error = %v", err)
	}

	// Verify content is still correct
	var buf bytes.Buffer
	if err := v.GetContent(checksum, &buf); err != nil {
		t.Fatalf("GetContent() error = %v", err)
	}
	if buf.String() != data {
		t.Errorf("content = %q, want %q", buf.String(), data)
	}
}

func TestFileSystemVault_GetContent(t *testing.T) {
	v, err := NewFileSystemVault("test", t.TempDir())
	if err != nil {
		t.Fatalf("NewFileSystemVault() error = %v", err)
	}

	t.Run("retrieve existing content", func(t *testing.T) {
		checksum := "abc123"
		data := "hello world"

		if err := v.PutContent(checksum, strings.NewReader(data), int64(len(data))); err != nil {
			t.Fatalf("PutContent() error = %v", err)
		}

		var buf bytes.Buffer
		if err := v.GetContent(checksum, &buf); err != nil {
			t.Fatalf("GetContent() error = %v", err)
		}

		if buf.String() != data {
			t.Errorf("content = %q, want %q", buf.String(), data)
		}
	})

	t.Run("content not found", func(t *testing.T) {
		var buf bytes.Buffer
		err := v.GetContent("nonexistent", &buf)
		if err == nil {
			t.Error("GetContent() expected error for nonexistent content")
		}
		if !strings.Contains(err.Error(), "content not found") {
			t.Errorf("error = %v, want error containing 'content not found'", err)
		}
	})
}

func TestFileSystemVault_PutMetadata(t *testing.T) {
	v, err := NewFileSystemVault("test", t.TempDir())
	if err != nil {
		t.Fatalf("NewFileSystemVault() error = %v", err)
	}

	hostID := "host-123"
	data := "metadata content"

	if err := v.PutMetadata(hostID, strings.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("PutMetadata() error = %v", err)
	}

	// Verify file exists with correct content
	metadataPath := filepath.Join(v.metadataDir, hostID+".db")
	content, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("failed to read metadata file: %v", err)
	}
	if string(content) != data {
		t.Errorf("metadata = %q, want %q", string(content), data)
	}
}

func TestFileSystemVault_PutMetadata_Overwrites(t *testing.T) {
	v, err := NewFileSystemVault("test", t.TempDir())
	if err != nil {
		t.Fatalf("NewFileSystemVault() error = %v", err)
	}

	hostID := "host-123"

	// Store first version
	data1 := "version 1"
	if err := v.PutMetadata(hostID, strings.NewReader(data1), int64(len(data1))); err != nil {
		t.Fatalf("first PutMetadata() error = %v", err)
	}

	// Store second version - should overwrite
	data2 := "version 2"
	if err := v.PutMetadata(hostID, strings.NewReader(data2), int64(len(data2))); err != nil {
		t.Fatalf("second PutMetadata() error = %v", err)
	}

	// Verify second version is stored
	var buf bytes.Buffer
	if err := v.GetMetadata(hostID, &buf); err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}
	if buf.String() != data2 {
		t.Errorf("metadata = %q, want %q", buf.String(), data2)
	}
}

func TestFileSystemVault_GetMetadata(t *testing.T) {
	v, err := NewFileSystemVault("test", t.TempDir())
	if err != nil {
		t.Fatalf("NewFileSystemVault() error = %v", err)
	}

	t.Run("retrieve existing metadata", func(t *testing.T) {
		hostID := "host-123"
		data := "metadata content"

		if err := v.PutMetadata(hostID, strings.NewReader(data), int64(len(data))); err != nil {
			t.Fatalf("PutMetadata() error = %v", err)
		}

		var buf bytes.Buffer
		if err := v.GetMetadata(hostID, &buf); err != nil {
			t.Fatalf("GetMetadata() error = %v", err)
		}

		if buf.String() != data {
			t.Errorf("metadata = %q, want %q", buf.String(), data)
		}
	})

	t.Run("metadata not found", func(t *testing.T) {
		var buf bytes.Buffer
		err := v.GetMetadata("nonexistent", &buf)
		if err == nil {
			t.Error("GetMetadata() expected error for nonexistent metadata")
		}
		if !strings.Contains(err.Error(), "metadata not found") {
			t.Errorf("error = %v, want error containing 'metadata not found'", err)
		}
	})
}

func TestFileSystemVault_ValidateSetup(t *testing.T) {
	t.Run("valid setup", func(t *testing.T) {
		v, err := NewFileSystemVault("test", t.TempDir())
		if err != nil {
			t.Fatalf("NewFileSystemVault() error = %v", err)
		}

		if err := v.ValidateSetup(); err != nil {
			t.Errorf("ValidateSetup() error = %v", err)
		}
	})

	t.Run("missing root directory", func(t *testing.T) {
		v := &FileSystemVault{
			name:        "test",
			root:        "/nonexistent/path",
			contentDir:  "/nonexistent/path/content",
			metadataDir: "/nonexistent/path/metadata",
		}

		if err := v.ValidateSetup(); err == nil {
			t.Error("ValidateSetup() expected error for missing root")
		}
	})
}

func TestFileSystemVault_AtomicWrite(t *testing.T) {
	v, err := NewFileSystemVault("test", t.TempDir())
	if err != nil {
		t.Fatalf("NewFileSystemVault() error = %v", err)
	}

	// Verify no temp files are left after successful write
	checksum := "abc123"
	data := "hello world"

	if err := v.PutContent(checksum, strings.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("PutContent() error = %v", err)
	}

	// Check for leftover temp files
	entries, err := os.ReadDir(v.contentDir)
	if err != nil {
		t.Fatalf("failed to read content dir: %v", err)
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".tmp-") {
			t.Errorf("temp file left behind: %s", entry.Name())
		}
	}
}
