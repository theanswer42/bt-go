package encryption

import (
	"bytes"
	"fmt"
	"io"

	"bt-go/internal/bt"
)

// testHeader is prepended to data by TestEncryptor to make encrypted output
// clearly different from plaintext while remaining deterministic and reversible.
var testHeader = []byte("BTENC\x00\x00\x00")

// TestEncryptor is a simple, deterministic encryptor for testing.
// It prepends a fixed 8-byte header during encryption and strips it during
// decryption. This ensures encrypted output differs from plaintext (so content
// checksums differ) while being trivially reversible and requiring no crypto.
type TestEncryptor struct {
	setupCalled bool
}

var _ bt.Encryptor = (*TestEncryptor)(nil)

// NewTestEncryptor creates a new TestEncryptor.
func NewTestEncryptor() *TestEncryptor {
	return &TestEncryptor{}
}

func (e *TestEncryptor) Setup(passphrase string) error {
	e.setupCalled = true
	return nil
}

func (e *TestEncryptor) Encrypt(r io.Reader, w io.Writer) error {
	if _, err := w.Write(testHeader); err != nil {
		return fmt.Errorf("writing test header: %w", err)
	}
	if _, err := io.Copy(w, r); err != nil {
		return fmt.Errorf("copying data: %w", err)
	}
	return nil
}

func (e *TestEncryptor) Unlock(passphrase string) (bt.DecryptionContext, error) {
	return &TestDecryptionContext{}, nil
}

func (e *TestEncryptor) IsConfigured() bool {
	return true
}

// TestDecryptionContext strips the test header added by TestEncryptor.
type TestDecryptionContext struct{}

var _ bt.DecryptionContext = (*TestDecryptionContext)(nil)

func (c *TestDecryptionContext) Decrypt(r io.Reader, w io.Writer) error {
	header := make([]byte, len(testHeader))
	if _, err := io.ReadFull(r, header); err != nil {
		return fmt.Errorf("reading test header: %w", err)
	}
	if !bytes.Equal(header, testHeader) {
		return fmt.Errorf("invalid test encryption header")
	}
	if _, err := io.Copy(w, r); err != nil {
		return fmt.Errorf("copying data: %w", err)
	}
	return nil
}
