package msoapx

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Debajyoti0-0/aether/internal/transport"
	"github.com/Debajyoti0-0/aether/internal/types"
)

// randRead is a thin indirection over crypto/rand for testability.
var randRead = rand.Read

// Entra ID PRT endpoints (MS-OAPX / browser SSO).
const (
	// TokenEndpoint is the Entra ID token endpoint.
	TokenEndpoint = "https://login.microsoftonline.com"

	// RegisterEndpoint is the device registration service.
	RegisterEndpoint = "https://enrollment.manage.microsoft.com"

	// PRTCookieHeaderName is the header carrying the PRT cookie
	// in browser SSO flows (x-ms-RefreshTokenCredential).
	PRTCookieHeaderName = "x-ms-RefreshTokenCredential"
)

// Client implements the MS-OAPX PRT-to-OAuth exchange. This mirrors the
// Windows browser SSO flow: the PRT cookie proves the user+device, and
// a session-key-signed nonce binds the request to the device.
type Client struct {
	HTTPClient *http.Client
	// BaseURL overrides https://login.microsoftonline.com (tests/mocks).
	BaseURL string
	// Binding, when set, is injected as the Token Protection
	// channel-binding header on every exchange request.
	Binding *ChannelBinding
}

// NewClient builds an MS-OAPX client with the aether uTLS transport.
func NewClient(preset string, timeout time.Duration) (*Client, error) {
	hc, err := transport.NewClient(preset, timeout)
	if err != nil {
		return nil, err
	}
	return &Client{HTTPClient: hc}, nil
}

// NewClientWithHTTP builds a client around a supplied HTTP client (tests).
func NewClientWithHTTP(hc *http.Client) *Client {
	return &Client{HTTPClient: hc}
}

// baseURL returns the STS base, defaulting to the real Entra endpoint.
func (c *Client) baseURL() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return TokenEndpoint
}

// ExchangeRequest describes a PRT-to-OAuth token exchange.
type ExchangeRequest struct {
	Tenant       string // tenant id or domain
	ClientID     string // public client id
	Resource     string // scope, e.g. https://graph.microsoft.com/.default
	PRT          *types.PRT
	RedirectURI  string
	CodeChallenge string // PKCE
	// Binding is the Token Protection channel binding to present
	// (tls-unique from the original host). Optional.
	Binding *ChannelBinding
}

// Exchange converts a PRT cookie into OAuth2 tokens.
//
// The flow: build the request with a session-key-derived proof (HMAC of
// the nonce/context per MS-OAPX), present the PRT cookie header, and
// request tokens with grant_type=urn:ietf:params:oauth:grant-type:prt_sso
// (Entra's browser SSO token grant).
func (c *Client) Exchange(ctx context.Context, req ExchangeRequest) (*types.OAuthTokens, error) {
	if req.PRT.IsZero() {
		return nil, fmt.Errorf("prt is required")
	}
	if req.Tenant == "" {
		return nil, fmt.Errorf("tenant is required")
	}
	if req.ClientID == "" {
		return nil, fmt.Errorf("client_id is required")
	}
	if req.Resource == "" {
		req.Resource = "https://graph.microsoft.com/.default"
	}
	if req.RedirectURI == "" {
		req.RedirectURI = "https://login.microsoftonline.com/common/oauth2/nativeclient"
	}

	sessionKey, err := decodeKey(req.PRT.SessionKey)
	if err != nil {
		return nil, fmt.Errorf("decode session key: %w", err)
	}

	nonce := deriveNonce(req.PRT.Context)
	proof := ComputeSessionKeyProof(sessionKey, []byte(nonce), []byte(req.PRT.Context))

	form := strings.NewReader(urlValues(map[string]string{
		"grant_type":    "urn:ietf:params:oauth:grant-type:prt_sso",
		"client_id":     req.ClientID,
		"scope":         req.Resource,
		"redirect_uri":  req.RedirectURI,
		"prt_cookie":    req.PRT.Cookie,
		"nonce":         nonce,
		"code_challenge": req.CodeChallenge,
		"code_challenge_method": "S256",
		"windows_api_version": "2.0",
	}).String())

	endpoint := fmt.Sprintf("%s/%s/oauth2/v2.0/token", c.baseURL(), req.Tenant)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, form)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set(PRTCookieHeaderName, req.PRT.Cookie)
	httpReq.Header.Set("x-ms-Prid", proof)
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Aether/1.0")

	// Token Protection: present the original host's tls-unique binding
	// (x-client-bound) when one was supplied.
	if binding := req.Binding; binding != nil {
		InjectBinding(httpReq, binding)
	} else if c.Binding != nil {
		InjectBinding(httpReq, c.Binding)
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("prt exchange: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prt exchange returned http %d: %s", resp.StatusCode, truncateStr(body, 512))
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

// ComputeSessionKeyProof derives the request proof: HMAC-style SHA-256
// over (nonce || context) keyed with the session key. In production this
// matches the Windows session key signing scheme; here it is the
// protocol-level derivation used to bind the request to the device.
func ComputeSessionKeyProof(sessionKey, nonce, context []byte) string {
	h := sha256.New()
	h.Write(sessionKey)
	h.Write(nonce)
	if len(context) > 0 {
		h.Write(context)
	}
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// deriveNonce produces a deterministic nonce bound to the PRT context,
// or a random one when no context is present.
func deriveNonce(context string) string {
	if context == "" {
		buf := make([]byte, 16)
		_, _ = randRead(buf)
		return hex.EncodeToString(buf)
	}
	sum := sha256.Sum256([]byte(context))
	return hex.EncodeToString(sum[:16])
}

func decodeKey(keyB64 string) ([]byte, error) {
	if keyB64 == "" {
		return nil, fmt.Errorf("session key is empty")
	}
	k, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		k, err = base64.RawURLEncoding.DecodeString(keyB64)
		if err != nil {
			return nil, err
		}
	}
	if len(k) < 16 {
		return nil, fmt.Errorf("session key too short (%d bytes)", len(k))
	}
	return k, nil
}

func urlValues(m map[string]string) *bytes.Buffer {
	var b bytes.Buffer
	first := true
	for k, v := range m {
		if v == "" {
			continue
		}
		if !first {
			b.WriteString("&")
		}
		first = false
		b.WriteString(escapeForm(k))
		b.WriteString("=")
		b.WriteString(escapeForm(v))
	}
	return &b
}

func escapeForm(s string) string {
	// net/url.QueryEscape equivalent without importing net/url here.
	const hexDigits = "0123456789ABCDEF"
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' {
			out.WriteByte(c)
		} else if c == ' ' {
			out.WriteByte('+')
		} else {
			out.WriteByte('%')
			out.WriteByte(hexDigits[c>>4])
			out.WriteByte(hexDigits[c&0x0f])
		}
	}
	return out.String()
}

func truncateStr(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
