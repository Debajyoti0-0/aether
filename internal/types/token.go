package types

import "time"

// OAuthTokens represents a standard OAuth2 token response.
type OAuthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope,omitempty"`
	IssuedAt     time.Time `json:"issued_at,omitempty"`
}

// Expired reports whether the access token has passed its lifetime.
func (t *OAuthTokens) Expired() bool {
	if t.IssuedAt.IsZero() {
		return false
	}
	return time.Now().After(t.IssuedAt.Add(time.Duration(t.ExpiresIn) * time.Second))
}

// PRT represents a Primary Refresh Token extracted from a Windows host.
type PRT struct {
	Cookie     string `json:"cookie"`
	DeviceID   string `json:"device_id"`
	TenantID   string `json:"tenant_id"`
	UserID     string `json:"user_id"`
	SessionKey string `json:"session_key"` // base64 encoded
	DerivedKey string `json:"derived_key,omitempty"`
	Context    string `json:"context,omitempty"`
}

// IsZero reports whether the PRT carries the minimum required fields.
func (p *PRT) IsZero() bool {
	return p == nil || p.Cookie == ""
}

// SAMLAssertion wraps a raw SAML assertion with metadata.
type SAMLAssertion struct {
	Raw       string            `json:"raw_xml"`
	ID        string            `json:"id"`
	Issuer    string            `json:"issuer"`
	Audience  string            `json:"audience"`
	NotBefore time.Time         `json:"not_before"`
	NotOnOrAfter time.Time      `json:"not_on_or_after"`
	Claims    map[string]string `json:"claims,omitempty"`
}

// IDTokenClaims holds the decoded OIDC ID token claims of interest.
type IDTokenClaims struct {
	Issuer   string   `json:"iss"`
	Subject  string   `json:"sub"`
	Audience string   `json:"aud"`
	TenantID string   `json:"tid,omitempty"`
	UPN      string   `json:"upn,omitempty"`
	Email    string   `json:"email,omitempty"`
	Name     string   `json:"name,omitempty"`
	ObjectID string   `json:"oid,omitempty"`
	DeviceID string   `json:"deviceid,omitempty"`
	Expiry   int64    `json:"exp"`
	IssuedAt int64    `json:"iat"`
	Nonce    string   `json:"nonce,omitempty"`
	Raw      map[string]any `json:"-"`
}
