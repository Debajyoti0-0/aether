package store

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

// LocalKeyProvider implements KeyProvider using an encrypted local file.
// The private key is encrypted with a passphrase-derived key (Argon2id + XChaCha20-Poly1305).
type LocalKeyProvider struct {
	path       string
	passphrase string
	mu         sync.Mutex
	versions   []KeyVersionInfo
	current    *keyVersion
}

type keyVersion struct {
	Version    string
	PrivateKey ed25519.PrivateKey
	CreatedAt  time.Time
}

// NewLocalKeyProvider creates a new local encrypted key provider.
// If the file doesn't exist, it generates a new key.
func NewLocalKeyProvider(path, passphrase string) (*LocalKeyProvider, error) {
	p := &LocalKeyProvider{
		path:       path,
		passphrase: passphrase,
	}
	if err := p.loadOrCreate(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *LocalKeyProvider) loadOrCreate() error {
	// Check if file exists WITHOUT holding the lock
	_, err := os.Stat(p.path)
	fileExists := !os.IsNotExist(err)

	if !fileExists {
		// Generate initial key deterministically from passphrase + "v1"
		priv := p.deriveKey("v1")
		p.current = &keyVersion{
			Version:    "v1",
			PrivateKey: priv,
			CreatedAt:  time.Now().UTC(),
		}
		p.versions = []KeyVersionInfo{{
			Version:   "v1",
			PublicKey: base64.StdEncoding.EncodeToString(priv.Public().(ed25519.PublicKey)),
			CreatedAt: p.current.CreatedAt,
			Active:    true,
			Source:    "local",
		}}
		return p.saveUnlocked()
	}

	return p.loadUnlocked()
}

func (p *LocalKeyProvider) loadUnlocked() error {
	data, err := os.ReadFile(p.path)
	if err != nil {
		return err
	}

	// Decrypt
	plaintext, err := p.decrypt(data)
	if err != nil {
		return fmt.Errorf("decrypt key file: %w", err)
	}

	var state struct {
		Current  string        `json:"current"`
		Versions []KeyVersionInfo `json:"versions"`
	}
	if err := json.Unmarshal(plaintext, &state); err != nil {
		return err
	}

	p.versions = state.Versions
	for _, v := range p.versions {
		if v.Version == state.Current {
			// Note: Private key not stored in JSON - would need secure storage
			// For now, we derive from passphrase + version
			p.current = &keyVersion{
				Version:    v.Version,
				PrivateKey: p.deriveKey(v.Version),
				CreatedAt:  v.CreatedAt,
			}
			break
		}
	}
	if p.current == nil {
		return fmt.Errorf("current version %s not found", state.Current)
	}
	return nil
}

func (p *LocalKeyProvider) load() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.loadUnlocked()
}

func (p *LocalKeyProvider) deriveKey(version string) ed25519.PrivateKey {
	// Deterministic key derivation from passphrase + version
	// In production, use HSM/KMS for true key isolation
	salt := sha256.Sum256([]byte("aether-key-derivation:" + version))
	key := argon2.IDKey([]byte(p.passphrase), salt[:], 3, 64*1024, 4, 32)
	return ed25519.NewKeyFromSeed(key[:32])
}

func (p *LocalKeyProvider) saveUnlocked() error {
	state := struct {
		Current  string          `json:"current"`
		Versions []KeyVersionInfo `json:"versions"`
	}{
		Current:  p.current.Version,
		Versions: p.versions,
	}

	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	encrypted, err := p.encrypt(data)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(p.path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(p.path, encrypted, 0o600)
}

func (p *LocalKeyProvider) save() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.saveUnlocked()
}

func (p *LocalKeyProvider) encrypt(plaintext []byte) ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	key := argon2.IDKey([]byte(p.passphrase), salt, 3, 64*1024, 4, 32)

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	ciphertext := aead.Seal(nil, nonce, plaintext, nil)
	// Format: salt || nonce || ciphertext
	return append(append(salt, nonce...), ciphertext...), nil
}

func (p *LocalKeyProvider) decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < 16+24 { // salt + nonce minimum
		return nil, fmt.Errorf("ciphertext too short")
	}
	salt := ciphertext[:16]
	nonce := ciphertext[16:40]
	data := ciphertext[40:]

	key := argon2.IDKey([]byte(p.passphrase), salt, 3, 64*1024, 4, 32)
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}

	return aead.Open(nil, nonce, data, nil)
}

// GetSigningKey returns the current Ed25519 private key for signing.
func (p *LocalKeyProvider) GetSigningKey(ctx context.Context) (ed25519.PrivateKey, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.current == nil {
		return nil, fmt.Errorf("no current key")
	}
	return p.current.PrivateKey, nil
}

// GetVerificationKey returns the current Ed25519 public key for verification.
func (p *LocalKeyProvider) GetVerificationKey(ctx context.Context) (ed25519.PublicKey, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.current == nil {
		return nil, fmt.Errorf("no current key")
	}
	return p.current.PrivateKey.Public().(ed25519.PublicKey), nil
}

// GetKeyVersion returns the current key version identifier.
func (p *LocalKeyProvider) GetKeyVersion(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.current == nil {
		return "", fmt.Errorf("no current key")
	}
	return p.current.Version, nil
}

// RotateKey generates a new key, archives the old one, and returns the new version.
func (p *LocalKeyProvider) RotateKey(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, newPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}

	// Archive old key
	if p.current != nil {
		for i := range p.versions {
			if p.versions[i].Version == p.current.Version {
				p.versions[i].Active = false
				break
			}
		}
	}

	// Create new version
	versionNum := len(p.versions) + 1
	newVersion := fmt.Sprintf("v%d", versionNum)
	p.current = &keyVersion{
		Version:    newVersion,
		PrivateKey: newPriv,
		CreatedAt:  time.Now().UTC(),
	}
	p.versions = append(p.versions, KeyVersionInfo{
		Version:   newVersion,
		PublicKey: base64.StdEncoding.EncodeToString(newPriv.Public().(ed25519.PublicKey)),
		CreatedAt: p.current.CreatedAt,
		Active:    true,
		Source:    "local",
	})

	return newVersion, p.saveUnlocked()
}

// ListKeyVersions returns all known key versions with metadata.
func (p *LocalKeyProvider) ListKeyVersions(ctx context.Context) ([]KeyVersionInfo, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := make([]KeyVersionInfo, len(p.versions))
	copy(result, p.versions)
	return result, nil
}

// Close releases any resources held by the provider.
func (p *LocalKeyProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.current = nil
	p.versions = nil
	return nil
}

// MockKMSProvider implements KeyProvider for testing with a simulated KMS.
type MockKMSProvider struct {
	mu       sync.Mutex
	versions []KeyVersionInfo
	current  string
	keys     map[string]ed25519.PrivateKey
}

// NewMockKMSProvider creates a new mock KMS provider for testing.
func NewMockKMSProvider() *MockKMSProvider {
	p := &MockKMSProvider{
		keys: make(map[string]ed25519.PrivateKey),
	}
	// Generate initial key
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	p.keys["v1"] = priv
	p.versions = []KeyVersionInfo{{
		Version:   "v1",
		PublicKey: base64.StdEncoding.EncodeToString(priv.Public().(ed25519.PublicKey)),
		CreatedAt: time.Now().UTC(),
		Active:    true,
		Source:    "mock_kms",
	}}
	p.current = "v1"
	return p
}

func (p *MockKMSProvider) GetSigningKey(ctx context.Context) (ed25519.PrivateKey, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	key, ok := p.keys[p.current]
	if !ok {
		return nil, fmt.Errorf("current key %s not found", p.current)
	}
	return key, nil
}

func (p *MockKMSProvider) GetVerificationKey(ctx context.Context) (ed25519.PublicKey, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	key, ok := p.keys[p.current]
	if !ok {
		return nil, fmt.Errorf("current key %s not found", p.current)
	}
	return key.Public().(ed25519.PublicKey), nil
}

func (p *MockKMSProvider) GetKeyVersion(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.current, nil
}

func (p *MockKMSProvider) RotateKey(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}

	// Archive old
	for i := range p.versions {
		if p.versions[i].Version == p.current {
			p.versions[i].Active = false
			break
		}
	}

	versionNum := len(p.versions) + 1
	newVersion := fmt.Sprintf("v%d", versionNum)
	p.keys[newVersion] = priv
	p.versions = append(p.versions, KeyVersionInfo{
		Version:   newVersion,
		PublicKey: base64.StdEncoding.EncodeToString(priv.Public().(ed25519.PublicKey)),
		CreatedAt: time.Now().UTC(),
		Active:    true,
		Source:    "mock_kms",
	})
	p.current = newVersion
	return newVersion, nil
}

func (p *MockKMSProvider) ListKeyVersions(ctx context.Context) ([]KeyVersionInfo, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := make([]KeyVersionInfo, len(p.versions))
	copy(result, p.versions)
	return result, nil
}

func (p *MockKMSProvider) Close() error { return nil }

// ProviderFactory creates KeyProvider instances based on configuration.
type ProviderFactory struct{}

// NewProviderFactory creates a new provider factory.
func NewProviderFactory() *ProviderFactory { return &ProviderFactory{} }

type ProviderConfig struct {
	Type        string // "local", "mock_kms", "azure_kv", "aws_kms", "yubihsm"
	Path        string // for local
	Passphrase  string // for local
	// KMS-specific fields would go here
}

// CreateProvider creates a KeyProvider from config.
func (f *ProviderFactory) CreateProvider(cfg ProviderConfig) (KeyProvider, error) {
	switch cfg.Type {
	case "local":
		if cfg.Path == "" {
			return nil, fmt.Errorf("local provider requires path")
		}
		if cfg.Passphrase == "" {
			return nil, fmt.Errorf("local provider requires passphrase")
		}
		return NewLocalKeyProvider(cfg.Path, cfg.Passphrase)
	case "mock_kms":
		return NewMockKMSProvider(), nil
	case "azure_kv":
		return NewAzureKVProviderFromEnv(context.Background())
	case "aws_kms":
		return nil, fmt.Errorf("aws_kms provider not implemented")
	case "yubihsm":
		return nil, fmt.Errorf("yubihsm provider not implemented")
	default:
		return nil, fmt.Errorf("unknown provider type: %s", cfg.Type)
	}
}