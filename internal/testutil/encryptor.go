package testutil

import (
	"bt-go/internal/bt"
	"bt-go/internal/encryption"
)

// NewTestEncryptor creates a new test encryptor for testing.
func NewTestEncryptor() bt.Encryptor {
	return encryption.NewTestEncryptor()
}
