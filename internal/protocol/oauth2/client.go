package oauth2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Debajyoti0-0/aether/internal/transport"
	"github.com/Debajyoti0-0/aether/internal/types"
)

// DefaultTokenEndpoint is the Entra ID token endpoint base.
const DefaultTokenEndpoint = "https://login.microsoftonline.com"

// Client is a low-level OAuth2 token endpoint client. BaseURL can be
// overridden to point at a mock server for testing.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient builds an OAuth2 client using the aether uTLS transport.
func NewClient(preset string, timeout time.Duration) (*Client, error) {
	hc, err := transport.NewClient(preset, timeout)
	if err != nil {
		return nil, err
	}
	return &Client{BaseURL: DefaultTokenEndpoint, HTTPClient: hc}, nil
}

// NewClientWithHTTP builds a client around a supplied HTTP client.
func NewClientWithHTTP(baseURL string, hc *http.Client) *Client {
	if baseURL == "" {
		baseURL = DefaultTokenEndpoint
	}
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), HTTPClient: hc}
}

// tokenURL returns the v2 token endpoint for a tenant.
func (c *Client) tokenURL(tenant string) string {
	return fmt.Sprintf("%s/%s/oauth2/v2.0/token", c.BaseURL, tenant)
}

// deviceURL returns the v2 device code initiation endpoint.
func (c *Client) deviceURL(tenant string) string {
	return fmt.Sprintf("%s/%s/oauth2/v2.0/devicecode", c.BaseURL, tenant)
}

// Token posts an arbitrary grant to the token endpoint.
func (c *Client) Token(ctx context.Context, tenant string, form url.Values) (*types.OAuthTokens, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL(tenant), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
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
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	tokens.IssuedAt = time.Now().UTC()
	if tokens.TokenType == "" {
		tokens.TokenType = "Bearer"
	}
	return tokens, nil
}

// ClientCredentials performs the client credentials grant.
func (c *Client) ClientCredentials(ctx context.Context, tenant, clientID, clientSecret, scope string) (*types.OAuthTokens, error) {
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"scope":         {scope},
	}
	return c.Token(ctx, tenant, form)
}

// RefreshToken performs the refresh token grant.
func (c *Client) RefreshToken(ctx context.Context, tenant, clientID, refreshToken, scope string) (*types.OAuthTokens, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"refresh_token": {refreshToken},
		"scope":         {scope},
	}
	return c.Token(ctx, tenant, form)
}

// ResourceOwnerPassword performs the ROPC grant (for WS-Trust fallback).
func (c *Client) ResourceOwnerPassword(ctx context.Context, tenant, clientID, username, password, scope string) (*types.OAuthTokens, error) {
	form := url.Values{
		"grant_type": {"password"},
		"client_id":  {clientID},
		"username":   {username},
		"password":   {password},
		"scope":      {scope},
	}
	return c.Token(ctx, tenant, form)
}

// DeviceCode is the response from the device code initiation endpoint.
type DeviceCode struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	Message         string `json:"message"`
}

// StartDeviceCode initiates the device code flow.
func (c *Client) StartDeviceCode(ctx context.Context, tenant, clientID, scope string) (*DeviceCode, error) {
	form := url.Values{"client_id": {clientID}, "scope": {scope}}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.deviceURL(tenant), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("devicecode request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp.StatusCode, body)
	}

	dc := &DeviceCode{}
	if err := json.Unmarshal(body, dc); err != nil {
		return nil, err
	}
	if dc.Interval == 0 {
		dc.Interval = 5
	}
	return dc, nil
}

// ErrAuthPending is returned while the device flow is still awaiting approval.
var ErrAuthPending = fmt.Errorf("authorization pending")

// PollDeviceCode polls the token endpoint once for the device flow.
// It returns ErrAuthPending while awaiting user authorization.
func (c *Client) PollDeviceCode(ctx context.Context, tenant, clientID string, dc *DeviceCode) (*types.OAuthTokens, error) {
	form := url.Values{
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"client_id":   {clientID},
		"device_code": {dc.DeviceCode},
	}

	tokens, err := c.Token(ctx, tenant, form)
	if err != nil {
		var oerr *OAuthError
		if ok := asOAuthError(err, &oerr); ok && oerr.Code == "authorization_pending" {
			return nil, ErrAuthPending
		}
		return nil, err
	}
	return tokens, nil
}

// OAuthError represents a token endpoint error response.
type OAuthError struct {
	Status int
	Code   string `json:"error"`
	Desc   string `json:"error_description"`
}

func (e *OAuthError) Error() string {
	if e.Desc != "" {
		return fmt.Sprintf("oauth2 error %q: %s", e.Code, e.Desc)
	}
	return fmt.Sprintf("oauth2 error %q (http %d)", e.Code, e.Status)
}

func parseError(status int, body []byte) error {
	oerr := &OAuthError{Status: status}
	if json.Unmarshal(body, oerr) != nil {
		oerr.Code = fmt.Sprintf("http_%d", status)
		oerr.Desc = string(body)
	}
	return oerr
}

func asOAuthError(err error, target **OAuthError) bool {
	if e, ok := err.(*OAuthError); ok {
		*target = e
		return true
	}
	return false
}
