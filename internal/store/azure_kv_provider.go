package store

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/keyvault/azkeys"
)

// AzureKVProvider implements KeyProvider using Azure Key Vault.
type AzureKVProvider struct {
	client       *azkeys.Client
	keyName      string
	vaultURL     string
	cred         azcore.TokenCredential
	keyVersions  []KeyVersionInfo
	currentVersion string
}

// AzureKVConfig holds configuration for the Azure Key Vault provider.
type AzureKVConfig struct {
	VaultURL   string // e.g., "https://myvault.vault.azure.net"
	KeyName    string // e.g., "aether-audit-key"
	Credential azcore.TokenCredential // Optional: uses DefaultAzureCredential if nil
}

// NewAzureKVProvider creates a new Azure Key Vault key provider.
// If credential is nil, uses DefaultAzureCredential (requires AZURE_CLIENT_ID, etc. or managed identity).
func NewAzureKVProvider(ctx context.Context, cfg AzureKVConfig) (*AzureKVProvider, error) {
	if cfg.VaultURL == "" {
		return nil, errors.New("vault URL is required")
	}
	if cfg.KeyName == "" {
		return nil, errors.New("key name is required")
	}

	vaultURL := strings.TrimSuffix(cfg.VaultURL, "/")
	cred := cfg.Credential
	if cred == nil {
		var err error
		cred, err = azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create default Azure credential: %w", err)
		}
	}

	client, err := azkeys.NewClient(vaultURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Key Vault client: %w", err)
	}

	p := &AzureKVProvider{
		client:   client,
		keyName:  cfg.KeyName,
		vaultURL: vaultURL,
		cred:     cred,
	}

	// Ensure key exists and load versions
	if err := p.ensureKey(ctx); err != nil {
		return nil, err
	}
	if err := p.loadVersions(ctx); err != nil {
		return nil, err
	}

	return p, nil
}

// NewAzureKVProviderFromEnv creates a provider from environment variables.
// Required env vars: AZURE_KEY_VAULT_URL, AZURE_KEY_VAULT_KEY_NAME
func NewAzureKVProviderFromEnv(ctx context.Context) (*AzureKVProvider, error) {
	vaultURL := os.Getenv("AZURE_KEY_VAULT_URL")
	keyName := os.Getenv("AZURE_KEY_VAULT_KEY_NAME")
	if vaultURL == "" || keyName == "" {
		return nil, errors.New("AZURE_KEY_VAULT_URL and AZURE_KEY_VAULT_KEY_NAME env vars required")
	}
	return NewAzureKVProvider(ctx, AzureKVConfig{
		VaultURL: vaultURL,
		KeyName:  keyName,
	})
}

// ensureKey creates the key in Key Vault if it doesn't exist.
func (p *AzureKVProvider) ensureKey(ctx context.Context) error {
	_, err := p.client.GetKey(ctx, p.keyName, "", nil)
	if err != nil {
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) && respErr.StatusCode == 404 {
			// Key doesn't exist, create it
			kty := azkeys.JSONWebKeyTypeEC
			crv := azkeys.JSONWebKeyCurveNameP256
			_, err = p.client.CreateKey(ctx, p.keyName, azkeys.CreateKeyParameters{
				Kty: &kty,
				Curve: &crv,
				KeyOps: []*azkeys.JSONWebKeyOperation{
					ptr(azkeys.JSONWebKeyOperationSign),
					ptr(azkeys.JSONWebKeyOperationVerify),
				},
			}, nil)
			if err != nil {
				return fmt.Errorf("failed to create EC-P256 key: %w", err)
			}
		} else {
			return fmt.Errorf("failed to get key: %w", err)
		}
	}
	return nil
}

// ptr returns a pointer to the given value (helper for pointer constants)
func ptr[T any](v T) *T { return &v }

// loadVersions loads all key versions from Key Vault.
func (p *AzureKVProvider) loadVersions(ctx context.Context) error {
	pager := p.client.NewListKeyVersionsPager(p.keyName, nil)
	p.keyVersions = nil
	p.currentVersion = ""

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list key versions: %w", err)
		}
		for _, k := range page.Value {
			if k.Attributes == nil || k.KID == nil {
				continue
			}
			version := extractVersionFromKID(string(*k.KID))
			info := KeyVersionInfo{
				Version:   version,
				PublicKey: "",
				CreatedAt: time.Time{},
				Active:    false,
				Source:    "azure_kv",
			}
			if k.Attributes.Created != nil {
				info.CreatedAt = *k.Attributes.Created
			}
			if k.Attributes.Enabled != nil && *k.Attributes.Enabled {
				info.Active = true
				p.currentVersion = version
			}
			p.keyVersions = append(p.keyVersions, info)
		}
	}

	// If no active version found, use the latest
	if p.currentVersion == "" && len(p.keyVersions) > 0 {
		p.currentVersion = p.keyVersions[len(p.keyVersions)-1].Version
		p.keyVersions[len(p.keyVersions)-1].Active = true
	}

	return nil
}

// extractVersionFromKID extracts the version from a Key Vault key ID.
// Format: https://vault.vault.azure.net/keys/keyname/version
func extractVersionFromKID(kid string) string {
	parts := strings.Split(kid, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// GetSigningKey returns the current Ed25519 private key for signing.
// Note: Azure Key Vault doesn't export private keys. This returns an error.
// Use the Sign method on the client for signing operations.
func (p *AzureKVProvider) GetSigningKey(ctx context.Context) (ed25519.PrivateKey, error) {
	return nil, errors.New("azure_kv provider does not export private keys; use client.Sign() for signing")
}

// GetVerificationKey returns the current Ed25519 public key for verification.
//
// Fail-closed: Azure Key Vault supports EC/RSA key types only (no Ed25519) and
// never exports key material, so this provider cannot satisfy the Ed25519
// KeyProvider contract for local audit signing/verification. It supports key
// lifecycle operations (create, rotate, list) only. Callers must NOT fall back
// to another provider when this error is returned.
func (p *AzureKVProvider) GetVerificationKey(ctx context.Context) (ed25519.PublicKey, error) {
	return nil, fmt.Errorf("azure_kv: Ed25519 verification key unavailable (provider %s): Azure Key Vault supports EC/RSA keys only and does not export key material; audit signing via this provider is unsupported", p.keyName)
}

// GetKeyVersion returns the current key version identifier.
func (p *AzureKVProvider) GetKeyVersion(ctx context.Context) (string, error) {
	if p.currentVersion == "" {
		if err := p.loadVersions(ctx); err != nil {
			return "", err
		}
	}
	if p.currentVersion == "" {
		return "", errors.New("no current key version")
	}
	return p.currentVersion, nil
}

// RotateKey creates a new key version in Key Vault.
func (p *AzureKVProvider) RotateKey(ctx context.Context) (string, error) {
	newKey, err := p.client.RotateKey(ctx, p.keyName, nil)
	if err != nil {
		return "", fmt.Errorf("failed to rotate key: %w", err)
	}
	if newKey.Key == nil || newKey.Key.KID == nil {
		return "", errors.New("rotated key missing version")
	}
	newVersion := extractVersionFromKID(string(*newKey.Key.KID))
	p.currentVersion = newVersion
	
	if err := p.loadVersions(ctx); err != nil {
		return "", err
	}
	return newVersion, nil
}

// ListKeyVersions returns all known key versions with metadata.
func (p *AzureKVProvider) ListKeyVersions(ctx context.Context) ([]KeyVersionInfo, error) {
	if err := p.loadVersions(ctx); err != nil {
		return nil, err
	}
	result := make([]KeyVersionInfo, len(p.keyVersions))
	copy(result, p.keyVersions)
	return result, nil
}

// Close releases any resources held by the provider.
func (p *AzureKVProvider) Close() error {
	p.client = nil
	p.cred = nil
	return nil
}