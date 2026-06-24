package config

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashConfig returns the SHA-256 hex digest of the given config bytes.
func HashConfig(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
