package vault

import (
	"bytes"
	"strings"
	"testing"
)

func TestMemoryVault_PutAndGetContent(t *testing.T) {
	vault := NewMemoryVault("test-vault")

	tests := []struct {
		name     string
		checksum string
		content  string
		wantErr  bool
	}{
		{
			name:     "store and retrieve content",
			checksum: "abc123",
			content:  "hello world",
			wantErr:  false,
		},
		{
			name:     "store empty content",
			checksum: "empty",
			content:  "",
			wantErr:  false,
		},
		{
			name:     "store large content",
			checksum: "large",
			content:  strings.Repeat("x", 10000),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Put content
			r := strings.NewReader(tt.content)
			err := vault.PutContent(tt.checksum, r, int64(len(tt.content)))
			if (err != nil) != tt.wantErr {
				t.Errorf("PutContent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Get content
			var buf bytes.Buffer
			err = vault.GetContent(tt.checksum, &buf)
			if err != nil {
				t.Errorf("GetContent() unexpected error: %v", err)
				return
			}

			if got := buf.String(); got != tt.content {
				t.Errorf("GetContent() = %q, want %q", got, tt.content)
			}
		})
	}
}

func TestMemoryVault_PutContentIdempotent(t *testing.T) {
	vault := NewMemoryVault("test-vault")

	content := "test content"
	checksum := "test-checksum"

	// Store same content twice
	for i := 0; i < 2; i++ {
		r := strings.NewReader(content)
		err := vault.PutContent(checksum, r, int64(len(content)))
		if err != nil {
			t.Fatalf("PutContent() iteration %d error: %v", i+1, err)
		}
	}

	// Should still retrieve the content
	var buf bytes.Buffer
	err := vault.GetContent(checksum, &buf)
	if err != nil {
		t.Fatalf("GetContent() error: %v", err)
	}

	if got := buf.String(); got != content {
		t.Errorf("GetContent() = %q, want %q", got, content)
	}
}

func TestMemoryVault_GetContentNotFound(t *testing.T) {
	vault := NewMemoryVault("test-vault")

	var buf bytes.Buffer
	err := vault.GetContent("nonexistent", &buf)
	if err == nil {
		t.Error("GetContent() expected error for nonexistent checksum, got nil")
	}
}

func TestMemoryVault_PutContentSizeMismatch(t *testing.T) {
	vault := NewMemoryVault("test-vault")

	content := "test"
	r := strings.NewReader(content)
	// Pass wrong size
	err := vault.PutContent("checksum", r, int64(len(content)+10))
	if err == nil {
		t.Error("PutContent() expected error for size mismatch, got nil")
	}
}

func TestMemoryVault_PutAndGetMetadata(t *testing.T) {
	vault := NewMemoryVault("test-vault")

	metadata := "database content"
	hostID := "host-123"

	// Put metadata
	r := strings.NewReader(metadata)
	err := vault.PutMetadata(hostID, r, int64(len(metadata)))
	if err != nil {
		t.Fatalf("PutMetadata() error: %v", err)
	}

	// Get metadata
	var buf bytes.Buffer
	err = vault.GetMetadata(hostID, &buf)
	if err != nil {
		t.Fatalf("GetMetadata() error: %v", err)
	}

	if got := buf.String(); got != metadata {
		t.Errorf("GetMetadata() = %q, want %q", got, metadata)
	}
}

func TestMemoryVault_GetMetadataNotFound(t *testing.T) {
	vault := NewMemoryVault("test-vault")

	var buf bytes.Buffer
	err := vault.GetMetadata("nonexistent-host", &buf)
	if err == nil {
		t.Error("GetMetadata() expected error for nonexistent host, got nil")
	}
}

func TestMemoryVault_ValidateSetup(t *testing.T) {
	vault := NewMemoryVault("test-vault")

	err := vault.ValidateSetup()
	if err != nil {
		t.Errorf("ValidateSetup() unexpected error: %v", err)
	}
}
