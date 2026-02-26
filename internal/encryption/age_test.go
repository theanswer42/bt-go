package encryption

import (
	"bytes"
	"path/filepath"
	"testing"

	"bt-go/internal/config"
)

func newTestAgeEncryptor(t *testing.T) *AgeEncryptor {
	t.Helper()
	dir := t.TempDir()
	cfg := config.EncryptionConfig{
		PublicKeyPath:  filepath.Join(dir, "keys", "bt.pub"),
		PrivateKeyPath: filepath.Join(dir, "keys", "bt.key"),
	}
	return NewAgeEncryptor(cfg)
}

func TestAgeEncryptor_IsConfigured_BeforeSetup(t *testing.T) {
	t.Parallel()
	e := newTestAgeEncryptor(t)
	if e.IsConfigured() {
		t.Error("IsConfigured() = true before Setup, want false")
	}
}

func TestAgeEncryptor_Setup_IsConfigured(t *testing.T) {
	t.Parallel()
	e := newTestAgeEncryptor(t)

	if err := e.Setup("test-passphrase"); err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	if !e.IsConfigured() {
		t.Error("IsConfigured() = false after Setup, want true")
	}
}

func TestAgeEncryptor_EncryptDecryptRoundTrip(t *testing.T) {
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

			passphrase := "test-passphrase"
			e := newTestAgeEncryptor(t)
			if err := e.Setup(passphrase); err != nil {
				t.Fatalf("Setup() error = %v", err)
			}

			// Encrypt
			var encrypted bytes.Buffer
			if err := e.Encrypt(bytes.NewReader(tt.input), &encrypted); err != nil {
				t.Fatalf("Encrypt() error = %v", err)
			}

			// Encrypted output should differ from plaintext
			if len(tt.input) > 0 && bytes.Equal(encrypted.Bytes(), tt.input) {
				t.Error("encrypted output is identical to plaintext")
			}

			// Decrypt
			ctx, err := e.Unlock(passphrase)
			if err != nil {
				t.Fatalf("Unlock() error = %v", err)
			}

			var decrypted bytes.Buffer
			if err := ctx.Decrypt(bytes.NewReader(encrypted.Bytes()), &decrypted); err != nil {
				t.Fatalf("Decrypt() error = %v", err)
			}

			if !bytes.Equal(decrypted.Bytes(), tt.input) {
				t.Errorf("round-trip failed: got %d bytes, want %d bytes", decrypted.Len(), len(tt.input))
			}
		})
	}
}

func TestAgeEncryptor_UnlockWrongPassphrase(t *testing.T) {
	t.Parallel()

	e := newTestAgeEncryptor(t)
	if err := e.Setup("correct-passphrase"); err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	_, err := e.Unlock("wrong-passphrase")
	if err == nil {
		t.Error("Unlock() with wrong passphrase should return error")
	}
}

func TestAgeEncryptor_EncryptBeforeSetup(t *testing.T) {
	t.Parallel()

	e := newTestAgeEncryptor(t)
	var buf bytes.Buffer
	err := e.Encrypt(bytes.NewReader([]byte("data")), &buf)
	if err == nil {
		t.Error("Encrypt() before Setup should return error")
	}
}

func TestAgeEncryptor_UnlockBeforeSetup(t *testing.T) {
	t.Parallel()

	e := newTestAgeEncryptor(t)
	_, err := e.Unlock("passphrase")
	if err == nil {
		t.Error("Unlock() before Setup should return error")
	}
}
