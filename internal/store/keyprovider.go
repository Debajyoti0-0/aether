package store

import (
	"context"
	"crypto/ed25519"
	"errors"
	"time"
)

// ErrUnsupportedOperation indicates the provider does not support this operation.
var ErrUnsupportedOperation = errors.New("operation not supported by this provider")

// KeyProviderCapabilities describes what a provider supports.
type KeyProviderCapabilities struct {
	SupportsEd25519Signing   bool
	SupportsPrivateKeyExport bool
	SupportsKeyRotation      bool
	SupportsKeyVersioning    bool
	KeyType                  string // "ed25519", "ec-p256", "rsa", etc.
}

// KeyProvider abstracts audit signing key storage and operations.
// Implementations can use local file, HSM, KMS, OS keychain, etc.
//
// Not all providers support all operations. Use Capabilities() to
// determine supported operations before calling.
type KeyProvider interface {
	// GetSigningKey returns the current Ed25519 private key for signing.
	// Returns ErrUnsupportedOperation if the provider does not support Ed25519 signing.
	GetSigningKey(ctx context.Context) (ed25519.PrivateKey, error)

	// GetVerificationKey returns the current Ed25519 public key for verification.
	// Returns ErrUnsupportedOperation if the provider does not support Ed25519 keys.
	GetVerificationKey(ctx context.Context) (ed25519.PublicKey, error)

	// GetKeyVersion returns the current key version identifier.
	GetKeyVersion(ctx context.Context) (string, error)

	// RotateKey generates a new key, archives the old one, and returns the new version.
	// Returns ErrUnsupportedOperation if the provider does not support key rotation.
	RotateKey(ctx context.Context) (string, error)

	// ListKeyVersions returns all known key versions with metadata.
	ListKeyVersions(ctx context.Context) ([]KeyVersionInfo, error)

	// Close releases any resources held by the provider.
	Close() error

	// Capabilities returns the provider's supported operations.
	Capabilities() KeyProviderCapabilities
}

// KeyVersionInfo describes a key version in the audit key hierarchy.
type KeyVersionInfo struct {
	Version     string    `json:"version"`
	PublicKey   string    `json:"public_key_b64"` // base64 encoded public key (Ed25519 or other)
	CreatedAt   time.Time `json:"created_at"`
	Active      bool      `json:"active"`
	Source      string    `json:"source"` // "local", "hsm", "kms", "os_keychain", "azure_kv"
}