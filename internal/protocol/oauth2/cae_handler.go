package oauth2

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

	"github.com/Debajyoti0-0/aether/internal/types"
)

var (
	stdEncoding = base64.StdEncoding
	urlEncoding = base64.RawURLEncoding
)

// CAEChallengeHeader is the response header carrying a claims challenge.
const CAEChallengeHeader = "WWW-Authenticate"

// CAEErrorHeader is Microsoft's structured claims challenge header.
const CAEErrorHeader = "x-ms-cae-error"

// ClaimsChallenge detects a CAE claims challenge in a 401 response and
// extracts the claims payload the STS demands.
func ClaimsChallenge(resp *http.Response) (map[string]any, bool, error) {
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		return nil, false, nil
	}

	auth := resp.Header.Get(CAEChallengeHeader)
	if auth == "" {
		return nil, false, nil
	}

	// Format: Bearer authorization_uri="https://login...", error="insufficient_claims",
	//         claims="eyJhY2Nlc3NfdG9rZW4iOns...}"
	claimsB64 := extractParam(auth, "claims")
	if claimsB64 == "" {
		return nil, false, nil
	}

	claimsJSON, err := base64Decode(claimsB64)
	if err != nil {
		return nil, true, fmt.Errorf("decode claims challenge: %w", err)
	}

	var claims map[string]any
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, true, fmt.Errorf("parse claims challenge: %w", err)
	}
	return claims, true, nil
}

func extractParam(header, name string) string {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		prefix := name + `="`
		if strings.HasPrefix(part, prefix) {
			val := strings.TrimPrefix(part, prefix)
			// The value ends at the closing quote; some servers append
			// stray chars (e.g. '}') after it per RFC 6750 quirks.
			if end := strings.LastIndex(val, `"`); end >= 0 {
				return val[:end]
			}
			return strings.TrimSuffix(val, `"`)
		}
	}
	return ""
}

func base64Decode(s string) ([]byte, error) {
	// JWT-style base64url without padding is common in challenges.
	if dec, err := stdEncoding.DecodeString(s); err == nil {
		return dec, nil
	}
	return urlEncoding.DecodeString(s)
}

// HandleCAEChallenge intercepts a 401 with a claims challenge and uses
// the refresh token to re-authenticate with the required claims merged
// in. This keeps sessions alive during risk spikes / CAE revocations.
func (c *Client) HandleCAEChallenge(ctx context.Context, tenant, clientID, refreshToken string, challenge map[string]any) (*types.OAuthTokens, error) {
	if refreshToken == "" {
		return nil, fmt.Errorf("refresh token is required for CAE handling")
	}
	if len(challenge) == 0 {
		return nil, fmt.Errorf("empty CAE challenge")
	}

	claimsJSON, err := json.Marshal(challenge)
	if err != nil {
		return nil, err
	}

	form := strings.NewReader(url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"refresh_token": {refreshToken},
		"scope":         {"https://graph.microsoft.com/.default offline_access"},
		"claims":        {string(claimsJSON)},
	}.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL(tenant), form)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cae refresh: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp.StatusCode, body)
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

// RequestWithCAERetry executes fn, and when it returns a 401 carrying
// a claims challenge, refreshes with the demanded claims and retries once.
func (c *Client) RequestWithCAERetry(ctx context.Context, tenant, clientID string, tokens *types.OAuthTokens, fn func(bearer string) (*http.Response, error)) (*http.Response, *types.OAuthTokens, error) {
	if tokens == nil || tokens.AccessToken == "" {
		return nil, tokens, fmt.Errorf("no access token")
	}

	resp, err := fn(tokens.AccessToken)
	if err != nil {
		return nil, tokens, err
	}

	challenge, isChallenge, err := ClaimsChallenge(resp)
	if err != nil {
		return resp, tokens, err
	}
	if !isChallenge {
		return resp, tokens, nil
	}
	// Drain the challenge response before retrying.
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()

	if tokens.RefreshToken == "" {
		return nil, tokens, fmt.Errorf("cae challenge received but no refresh token available")
	}

	refreshed, err := c.HandleCAEChallenge(ctx, tenant, clientID, tokens.RefreshToken, challenge)
	if err != nil {
		return nil, tokens, fmt.Errorf("cae challenge handling failed: %w", err)
	}

	resp2, err := fn(refreshed.AccessToken)
	return resp2, refreshed, err
}
