package store

import (
	"context"
	"crypto/ed25519"
	"time"
)

// KeyProvider abstracts audit signing key storage and operations.
// Implementations can use local file, HSM, KMS, OS keychain, etc.
type KeyProvider interface {
	// GetSigningKey returns the current Ed25519 private key for signing.
	GetSigningKey(ctx context.Context) (ed25519.PrivateKey, error)

	// GetVerificationKey returns the current Ed25519 public key for verification.
	GetVerificationKey(ctx context.Context) (ed25519.PublicKey, error)

	// GetKeyVersion returns the current key version identifier.
	GetKeyVersion(ctx context.Context) (string, error)

	// RotateKey generates a new key, archives the old one, and returns the new version.
	RotateKey(ctx context.Context) (string, error)

	// ListKeyVersions returns all known key versions with metadata.
	ListKeyVersions(ctx context.Context) ([]KeyVersionInfo, error)

	// Close releases any resources held by the provider.
	Close() error
}

// KeyVersionInfo describes a key version in the audit key hierarchy.
type KeyVersionInfo struct {
	Version     string    `json:"version"`
	PublicKey   string    `json:"public_key_b64"` // base64 Ed25519 public key
	CreatedAt   time.Time `json:"created_at"`
	Active      bool      `json:"active"`
	Source      string    `json:"source"` // "local", "hsm", "kms", "os_keychain"
}