package testutil

import (
	"bt-go/internal/bt"
	"bt-go/internal/vault"
)

// NewTestVault creates a new in-memory vault for testing.
func NewTestVault() bt.Vault {
	return vault.NewMemoryVault("test-vault")
}
