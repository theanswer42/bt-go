package encryption

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"filippo.io/age"

	"bt-go/internal/bt"
	"bt-go/internal/config"
)

// AgeEncryptor implements bt.Encryptor using filippo.io/age with X25519 keys.
// The public key is stored in plaintext; the private key is encrypted with the
// user's passphrase using age's scrypt-based passphrase encryption.
type AgeEncryptor struct {
	publicKeyPath  string
	privateKeyPath string
}

var _ bt.Encryptor = (*AgeEncryptor)(nil)

// NewAgeEncryptor creates a new AgeEncryptor from configuration.
func NewAgeEncryptor(cfg config.EncryptionConfig) *AgeEncryptor {
	return &AgeEncryptor{
		publicKeyPath:  cfg.PublicKeyPath,
		privateKeyPath: cfg.PrivateKeyPath,
	}
}

// Setup generates a new X25519 key pair, stores the public key in plaintext,
// and encrypts the private key with the passphrase using age's scrypt-based
// passphrase encryption.
func (e *AgeEncryptor) Setup(passphrase string) error {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return fmt.Errorf("generating key pair: %w", err)
	}

	// Ensure key directories exist.
	if err := os.MkdirAll(filepath.Dir(e.publicKeyPath), 0700); err != nil {
		return fmt.Errorf("creating public key directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(e.privateKeyPath), 0700); err != nil {
		return fmt.Errorf("creating private key directory: %w", err)
	}

	// Write public key in plaintext.
	if err := os.WriteFile(e.publicKeyPath, []byte(identity.Recipient().String()+"\n"), 0644); err != nil {
		return fmt.Errorf("writing public key: %w", err)
	}

	// Encrypt private key with passphrase and write it.
	privFile, err := os.OpenFile(e.privateKeyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("creating private key file: %w", err)
	}
	defer privFile.Close()

	recipient, err := age.NewScryptRecipient(passphrase)
	if err != nil {
		return fmt.Errorf("creating scrypt recipient: %w", err)
	}

	w, err := age.Encrypt(privFile, recipient)
	if err != nil {
		return fmt.Errorf("creating encrypted writer: %w", err)
	}

	if _, err := io.WriteString(w, identity.String()+"\n"); err != nil {
		return fmt.Errorf("writing encrypted private key: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("finalizing encrypted private key: %w", err)
	}

	return nil
}

// Encrypt reads plaintext from r and writes age-encrypted ciphertext to w
// using the stored public key.
func (e *AgeEncryptor) Encrypt(r io.Reader, w io.Writer) error {
	recipient, err := e.loadRecipient()
	if err != nil {
		return fmt.Errorf("loading public key: %w", err)
	}

	encWriter, err := age.Encrypt(w, recipient)
	if err != nil {
		return fmt.Errorf("creating encrypted writer: %w", err)
	}

	if _, err := io.Copy(encWriter, r); err != nil {
		return fmt.Errorf("encrypting data: %w", err)
	}

	if err := encWriter.Close(); err != nil {
		return fmt.Errorf("finalizing encryption: %w", err)
	}

	return nil
}

// Unlock decrypts the private key using the passphrase and returns an
// AgeDecryptionContext holding the unlocked identity.
func (e *AgeEncryptor) Unlock(passphrase string) (bt.DecryptionContext, error) {
	privData, err := os.ReadFile(e.privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("reading private key file: %w", err)
	}

	identity, err := age.NewScryptIdentity(passphrase)
	if err != nil {
		return nil, fmt.Errorf("creating scrypt identity: %w", err)
	}

	decReader, err := age.Decrypt(bytes.NewReader(privData), identity)
	if err != nil {
		return nil, fmt.Errorf("decrypting private key: %w", err)
	}

	keyData, err := io.ReadAll(decReader)
	if err != nil {
		return nil, fmt.Errorf("reading decrypted private key: %w", err)
	}

	identities, err := age.ParseIdentities(bytes.NewReader(keyData))
	if err != nil {
		return nil, fmt.Errorf("parsing private key: %w", err)
	}

	if len(identities) == 0 {
		return nil, fmt.Errorf("no identities found in private key")
	}

	return &AgeDecryptionContext{identity: identities[0]}, nil
}

// IsConfigured returns true if both key files exist.
func (e *AgeEncryptor) IsConfigured() bool {
	if _, err := os.Stat(e.publicKeyPath); err != nil {
		return false
	}
	if _, err := os.Stat(e.privateKeyPath); err != nil {
		return false
	}
	return true
}

// loadRecipient reads the public key from disk and parses it.
func (e *AgeEncryptor) loadRecipient() (age.Recipient, error) {
	pubData, err := os.ReadFile(e.publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("reading public key: %w", err)
	}

	recipients, err := age.ParseRecipients(bytes.NewReader(pubData))
	if err != nil {
		return nil, fmt.Errorf("parsing public key: %w", err)
	}

	if len(recipients) == 0 {
		return nil, fmt.Errorf("no recipients found in public key file")
	}

	return recipients[0], nil
}

// AgeDecryptionContext holds an unlocked age identity for decrypting data.
type AgeDecryptionContext struct {
	identity age.Identity
}

var _ bt.DecryptionContext = (*AgeDecryptionContext)(nil)

// Decrypt reads age-encrypted ciphertext from r and writes plaintext to w.
func (c *AgeDecryptionContext) Decrypt(r io.Reader, w io.Writer) error {
	decReader, err := age.Decrypt(r, c.identity)
	if err != nil {
		return fmt.Errorf("creating decrypted reader: %w", err)
	}

	if _, err := io.Copy(w, decReader); err != nil {
		return fmt.Errorf("decrypting data: %w", err)
	}

	return nil
}
