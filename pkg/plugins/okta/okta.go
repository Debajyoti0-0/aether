package okta

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Debajyoti0-0/aether/pkg/plugins/sdk"
)

// oktaProvider implements sdk.Provider for Okta (OAuth2/OIDC auth),
// covering user enumeration, app listing, and token validation via the
// Okta REST API.
type oktaProvider struct {
	token string
	base  string // https://<org>.okta.com
	http  *http.Client
}

// New builds an Okta provider plugin.
func New(domain, token string) sdk.Plugin {
	return sdk.NewAdapter("okta", "1.0.0", &oktaProvider{
		token: token,
		base:  strings.TrimRight(domain, "/"),
		http:  &http.Client{Timeout: 15 * time.Second},
	})
}

func (o *oktaProvider) Name() string { return "okta" }

// ValidateToken checks the API token against /api/v1/users/me.
func (o *oktaProvider) ValidateToken(ctx context.Context) error {
	body, err := o.get(ctx, "/api/v1/users/me")
	if err != nil {
		return err
	}
	var me map[string]any
	if err := json.Unmarshal(body, &me); err != nil {
		return err
	}
	if _, ok := me["id"]; !ok {
		return fmt.Errorf("okta token validation failed")
	}
	return nil
}

// OktaUser is a minimal Okta user record.
type OktaUser struct {
	ID    string `json:"id"`
	Login string `json:"login"`
	Status string `json:"status"`
}

// ListUsers enumerates up to limit users via /api/v1/users.
func (o *oktaProvider) ListUsers(ctx context.Context, limit int) ([]OktaUser, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	body, err := o.get(ctx, fmt.Sprintf("/api/v1/users?limit=%d", limit))
	if err != nil {
		return nil, err
	}
	var users []OktaUser
	if err := json.Unmarshal(body, &users); err != nil {
		return nil, err
	}
	return users, nil
}

// Execute satisfies sdk.Provider; Okta has no VM exec.
// It performs a token exchange probe instead (authorization server).
func (o *oktaProvider) Execute(ctx context.Context, target, command string) (*sdk.Result, error) {
	// target = authorization server id, command = scope to request.
	body, err := o.post(ctx, "/oauth2/"+target+"/v1/token", map[string]string{
		"grant_type": "client_credentials",
		"scope":      command,
	})
	if err != nil {
		return nil, err
	}
	var tok map[string]any
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, err
	}
	if at, ok := tok["access_token"].(string); ok {
		return &sdk.Result{Output: fmt.Sprintf("okta token: %d bytes (scope=%s)", len(at), command), ExitCode: 0, Status: "succeeded"}, nil
	}
	return nil, fmt.Errorf("no access_token in okta response")
}

func (o *oktaProvider) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.base+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "SSWS "+o.token)
	req.Header.Set("Accept", "application/json")
	return o.do(req)
}

func (o *oktaProvider) post(ctx context.Context, path string, form map[string]string) ([]byte, error) {
	var b strings.Builder
	first := true
	for k, v := range form {
		if !first {
			b.WriteString("&")
		}
		first = false
		fmt.Fprintf(&b, "%s=%s", k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.base+path, strings.NewReader(b.String()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	return o.do(req)
}

func (o *oktaProvider) do(req *http.Request) ([]byte, error) {
	resp, err := o.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("okta request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("okta http %d: %s", resp.StatusCode, truncateStr(string(body), 256))
	}
	return body, nil
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
