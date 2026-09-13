package store

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestLocalKeyProvider_New(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.enc")
	
	p, err := NewLocalKeyProvider(path, "test-passphrase")
	if err != nil {
		t.Fatalf("NewLocalKeyProvider failed: %v", err)
	}
	defer p.Close()
	
	// Verify key was created
	ver, err := p.GetKeyVersion(context.Background())
	if err != nil {
		t.Fatalf("GetKeyVersion failed: %v", err)
	}
	if ver != "v1" {
		t.Fatalf("expected version v1, got %s", ver)
	}
	
	priv, err := p.GetSigningKey(context.Background())
	if err != nil {
		t.Fatalf("GetSigningKey failed: %v", err)
	}
	if len(priv) != ed25519.PrivateKeySize {
		t.Fatalf("invalid private key size")
	}
	
	pub, err := p.GetVerificationKey(context.Background())
	if err != nil {
		t.Fatalf("GetVerificationKey failed: %v", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		t.Fatalf("invalid public key size")
	}
}

func TestLocalKeyProvider_PersistAndReload(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.enc")
	passphrase := "test-passphrase"
	
	// Create provider and get initial key
	p1, err := NewLocalKeyProvider(path, passphrase)
	if err != nil {
		t.Fatalf("NewLocalKeyProvider failed: %v", err)
	}
	ver1, _ := p1.GetKeyVersion(context.Background())
	priv1, _ := p1.GetSigningKey(context.Background())
	p1.Close()
	
	// Reload with same passphrase
	p2, err := NewLocalKeyProvider(path, passphrase)
	if err != nil {
		t.Fatalf("Reload failed: %v", err)
	}
	defer p2.Close()
	
	ver2, _ := p2.GetKeyVersion(context.Background())
	priv2, _ := p2.GetSigningKey(context.Background())
	
	if ver1 != ver2 {
		t.Fatalf("version mismatch after reload: %s vs %s", ver1, ver2)
	}
	// Keys should be equal (deterministic derivation)
	if string(priv1) != string(priv2) {
		t.Fatalf("private key mismatch after reload")
	}
}

func TestLocalKeyProvider_WrongPassphrase(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.enc")
	
	_, err := NewLocalKeyProvider(path, "correct-passphrase")
	if err != nil {
		t.Fatalf("initial create failed: %v", err)
	}
	
	// Try to load with wrong passphrase
	_, err = NewLocalKeyProvider(path, "wrong-passphrase")
	if err == nil {
		t.Fatal("expected error with wrong passphrase")
	}
}

func TestLocalKeyProvider_RotateKey(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.enc")
	
	p, err := NewLocalKeyProvider(path, "test-passphrase")
	if err != nil {
		t.Fatalf("NewLocalKeyProvider failed: %v", err)
	}
	defer p.Close()
	
	// Rotate key
	newVer, err := p.RotateKey(context.Background())
	if err != nil {
		t.Fatalf("RotateKey failed: %v", err)
	}
	if newVer != "v2" {
		t.Fatalf("expected v2, got %s", newVer)
	}
	
	// Verify new key is active
	ver, _ := p.GetKeyVersion(context.Background())
	if ver != "v2" {
		t.Fatalf("current version should be v2, got %s", ver)
	}
	
	// Verify old key is archived
	versions, _ := p.ListKeyVersions(context.Background())
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
	if versions[0].Active {
		t.Fatal("v1 should not be active")
	}
	if !versions[1].Active {
		t.Fatal("v2 should be active")
	}
}

func TestLocalKeyProvider_SignVerify(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.enc")
	
	p, err := NewLocalKeyProvider(path, "test-passphrase")
	if err != nil {
		t.Fatalf("NewLocalKeyProvider failed: %v", err)
	}
	defer p.Close()
	
	priv, _ := p.GetSigningKey(context.Background())
	pub, _ := p.GetVerificationKey(context.Background())
	
	message := []byte("test message")
	sig := ed25519.Sign(priv, message)
	
	if !ed25519.Verify(pub, message, sig) {
		t.Fatal("signature verification failed")
	}
	
	// Tampered message should fail
	if ed25519.Verify(pub, []byte("tampered"), sig) {
		t.Fatal("tampered message should not verify")
	}
}

func TestMockKMSProvider_Basic(t *testing.T) {
	p := NewMockKMSProvider()
	defer p.Close()
	
	ver, _ := p.GetKeyVersion(context.Background())
	if ver != "v1" {
		t.Fatalf("expected v1, got %s", ver)
	}
	
	priv, _ := p.GetSigningKey(context.Background())
	if len(priv) != ed25519.PrivateKeySize {
		t.Fatal("invalid key size")
	}
	
	// Rotate
	newVer, _ := p.RotateKey(context.Background())
	if newVer != "v2" {
		t.Fatalf("expected v2, got %s", newVer)
	}
	
	versions, _ := p.ListKeyVersions(context.Background())
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
}

func TestMockKMSProvider_SignVerify(t *testing.T) {
	p := NewMockKMSProvider()
	defer p.Close()
	
	priv, _ := p.GetSigningKey(context.Background())
	pub, _ := p.GetVerificationKey(context.Background())
	
	message := []byte("test message")
	sig := ed25519.Sign(priv, message)
	
	if !ed25519.Verify(pub, message, sig) {
		t.Fatal("signature verification failed")
	}
}

func TestProviderFactory(t *testing.T) {
	f := NewProviderFactory()
	
	// Test local provider
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.enc")
	p, err := f.CreateProvider(ProviderConfig{
		Type:       "local",
		Path:       path,
		Passphrase: "test",
	})
	if err != nil {
		t.Fatalf("CreateProvider local failed: %v", err)
	}
	p.Close()
	
	// Test mock KMS provider
	p, err = f.CreateProvider(ProviderConfig{Type: "mock_kms"})
	if err != nil {
		t.Fatalf("CreateProvider mock_kms failed: %v", err)
	}
	p.Close()
	
	// Test unknown type
	_, err = f.CreateProvider(ProviderConfig{Type: "unknown"})
	if err == nil {
		t.Fatal("expected error for unknown provider type")
	}
}

func TestKeyVersionInfo_Serialization(t *testing.T) {
	_, pub, _ := ed25519.GenerateKey(rand.Reader)
	info := KeyVersionInfo{
		Version:   "v1",
		PublicKey: base64.StdEncoding.EncodeToString(pub),
		CreatedAt: time.Now().UTC(),
		Active:    true,
		Source:    "local",
	}
	
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	
	var info2 KeyVersionInfo
	if err := json.Unmarshal(data, &info2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	
	if info.Version != info2.Version || info.PublicKey != info2.PublicKey || info.Source != info2.Source {
		t.Fatal("serialization round-trip failed")
	}
}

// Test concurrent access to LocalKeyProvider
func TestLocalKeyProvider_Concurrent(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keys.enc")
	
	p, err := NewLocalKeyProvider(path, "test-passphrase")
	if err != nil {
		t.Fatalf("NewLocalKeyProvider failed: %v", err)
	}
	defer p.Close()
	
	done := make(chan struct{})
	errChan := make(chan error, 100)
	
	// Concurrent readers
	for i := 0; i < 10; i++ {
		go func() {
			for {
				select {
				case <-done:
					return
				default:
					_, err := p.GetSigningKey(context.Background())
					if err != nil {
						errChan <- err
						return
					}
					_, err = p.GetVerificationKey(context.Background())
					if err != nil {
						errChan <- err
						return
					}
					_, err = p.GetKeyVersion(context.Background())
					if err != nil {
						errChan <- err
						return
					}
				}
			}
		}()
	}
	
	// Concurrent rotations (sequential due to mutex, but test contention)
	for i := 0; i < 5; i++ {
		_, err := p.RotateKey(context.Background())
		if err != nil {
			t.Fatalf("RotateKey failed: %v", err)
		}
	}
	
	close(done)
	
	select {
	case err := <-errChan:
		t.Fatalf("concurrent access error: %v", err)
	default:
		// OK
	}
}