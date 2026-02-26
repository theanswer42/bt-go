package testutil

import (
	"crypto/sha256"
	"encoding/hex"
)

// SHA256Hex returns the SHA-256 checksum of data as a lowercase hex string.
// Matches the checksum format used by the staging area and vault.
func SHA256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
