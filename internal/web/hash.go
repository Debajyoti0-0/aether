package web

import (
	"crypto/sha256"
	"encoding/hex"
)

// sha256Sum returns the raw SHA-256 digest of b.
func sha256Sum(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}

// sha256Hex returns the lowercase hex SHA-256 digest of b. This matches the
// encoding store.computeHash produces, so a value computed here is directly
// comparable with an e.Hash read from the chain.
func sha256Hex(b []byte) string {
	return hex.EncodeToString(sha256Sum(b))
}
