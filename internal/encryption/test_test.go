package encryption

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"
)

func TestTestEncryptor_Setup(t *testing.T) {
	t.Parallel()
	e := NewTestEncryptor()
	if err := e.Setup("any-passphrase"); err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	if !e.setupCalled {
		t.Error("Setup() did not record that it was called")
	}
}

func TestTestEncryptor_IsConfigured(t *testing.T) {
	t.Parallel()
	e := NewTestEncryptor()
	if !e.IsConfigured() {
		t.Error("IsConfigured() = false, want true")
	}
}

func TestTestEncryptor_EncryptDecrypt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []byte
	}{
		{name: "simple text", input: []byte("hello world")},
		{name: "empty", input: []byte{}},
		{name: "binary data", input: []byte{0x00, 0xff, 0x01, 0xfe}},
		{name: "large data", input: bytes.Repeat([]byte("abcdef"), 10000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e := NewTestEncryptor()

			// Encrypt
			var encrypted bytes.Buffer
			if err := e.Encrypt(bytes.NewReader(tt.input), &encrypted); err != nil {
				t.Fatalf("Encrypt() error = %v", err)
			}

			// Encrypted output should differ from plaintext (unless input is empty,
			// in which case the header itself makes it different).
			if bytes.Equal(encrypted.Bytes(), tt.input) {
				t.Error("encrypted output is identical to plaintext")
			}

			// Encrypted output should start with the header
			if !bytes.HasPrefix(encrypted.Bytes(), testHeader) {
				t.Error("encrypted output does not start with test header")
			}

			// Decrypt
			ctx, err := e.Unlock("any-passphrase")
			if err != nil {
				t.Fatalf("Unlock() error = %v", err)
			}

			var decrypted bytes.Buffer
			if err := ctx.Decrypt(bytes.NewReader(encrypted.Bytes()), &decrypted); err != nil {
				t.Fatalf("Decrypt() error = %v", err)
			}

			if !bytes.Equal(decrypted.Bytes(), tt.input) {
				t.Errorf("round-trip failed: got %q, want %q", decrypted.Bytes(), tt.input)
			}
		})
	}
}

func TestTestEncryptor_ChecksumsDiffer(t *testing.T) {
	t.Parallel()

	input := []byte("some file content")

	e := NewTestEncryptor()
	var encrypted bytes.Buffer
	if err := e.Encrypt(bytes.NewReader(input), &encrypted); err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	plainHash := sha256.Sum256(input)
	encHash := sha256.Sum256(encrypted.Bytes())

	if hex.EncodeToString(plainHash[:]) == hex.EncodeToString(encHash[:]) {
		t.Error("plaintext and encrypted checksums should differ")
	}
}

func TestTestEncryptor_Deterministic(t *testing.T) {
	t.Parallel()

	input := []byte("deterministic test")
	e := NewTestEncryptor()

	var enc1, enc2 bytes.Buffer
	if err := e.Encrypt(bytes.NewReader(input), &enc1); err != nil {
		t.Fatalf("first Encrypt() error = %v", err)
	}
	if err := e.Encrypt(bytes.NewReader(input), &enc2); err != nil {
		t.Fatalf("second Encrypt() error = %v", err)
	}

	if !bytes.Equal(enc1.Bytes(), enc2.Bytes()) {
		t.Error("same input produced different encrypted output")
	}
}

func TestTestDecryptionContext_InvalidHeader(t *testing.T) {
	t.Parallel()

	ctx := &TestDecryptionContext{}
	badData := bytes.NewReader([]byte("NOT_VALID_HEADER_data"))
	var out bytes.Buffer
	err := ctx.Decrypt(badData, &out)
	if err == nil {
		t.Error("Decrypt() with invalid header should return error")
	}
}

func TestTestDecryptionContext_TruncatedHeader(t *testing.T) {
	t.Parallel()

	ctx := &TestDecryptionContext{}
	short := bytes.NewReader([]byte("BT"))
	var out bytes.Buffer
	err := ctx.Decrypt(short, &out)
	if err == nil {
		t.Error("Decrypt() with truncated data should return error")
	}
}

func TestTestDecryptionContext_EmptyInput(t *testing.T) {
	t.Parallel()

	ctx := &TestDecryptionContext{}
	var out bytes.Buffer
	err := ctx.Decrypt(bytes.NewReader(nil), &out)
	if err == nil {
		t.Error("Decrypt() with empty input should return error")
	}
	if err != io.ErrUnexpectedEOF && err.Error() != "reading test header: unexpected EOF" {
		// io.ReadFull returns ErrUnexpectedEOF when it can't fill the buffer
		t.Logf("got error: %v (acceptable)", err)
	}
}
