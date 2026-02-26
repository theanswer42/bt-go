package bt

import "io"

// Encryptor handles encryption of content files and unlocking for decryption.
// Encryption uses the public key only — no user intervention required.
// Decryption requires a passphrase to unlock the private key, producing a
// DecryptionContext for the session.
type Encryptor interface {
	// Setup performs one-time key generation. Called during `bt config init`.
	// Generates a key pair, stores the public key in plaintext, and encrypts
	// the private key with the provided passphrase.
	Setup(passphrase string) error

	// Encrypt encrypts data read from r and writes ciphertext to w.
	// Uses the public key only — no passphrase required.
	Encrypt(r io.Reader, w io.Writer) error

	// Unlock decrypts the private key using the passphrase and returns a
	// DecryptionContext that can decrypt data for the duration of the session.
	// Returns an error if the passphrase is incorrect.
	Unlock(passphrase string) (DecryptionContext, error)

	// IsConfigured returns true if both key files exist at configured paths.
	IsConfigured() bool
}

// DecryptionContext holds an unlocked private key in memory for the duration
// of a restore session. Created by Encryptor.Unlock. The unlocked key is held
// in memory only and never written to disk.
type DecryptionContext interface {
	// Decrypt decrypts data read from r and writes plaintext to w.
	Decrypt(r io.Reader, w io.Writer) error
}
