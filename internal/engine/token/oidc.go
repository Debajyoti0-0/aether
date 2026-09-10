package token

import (
	"fmt"
	"time"

	"github.com/Debajyoti0-0/aether/internal/protocol/oauth2"
	"github.com/Debajyoti0-0/aether/internal/types"
)

// OIDCManipulator inspects and compares OIDC tokens without
// contacting the issuer.
type OIDCManipulator struct{}

// NewOIDCManipulator builds a manipulator.
func NewOIDCManipulator() *OIDCManipulator {
	return &OIDCManipulator{}
}

// Decode decodes an ID token into typed claims.
func (m *OIDCManipulator) Decode(idToken string) (*types.IDTokenClaims, error) {
	if idToken == "" {
		return nil, fmt.Errorf("id token is empty")
	}
	return oauth2.ParseIDToken(idToken)
}

// Diff compares two ID tokens and returns claims that differ.
func (m *OIDCManipulator) Diff(a, b string) (map[string][2]any, error) {
	ca, err := oauth2.DecodeJWTClaims(a)
	if err != nil {
		return nil, fmt.Errorf("token a: %w", err)
	}
	cb, err := oauth2.DecodeJWTClaims(b)
	if err != nil {
		return nil, fmt.Errorf("token b: %w", err)
	}

	diff := map[string][2]any{}
	for k, va := range ca {
		vb, ok := cb[k]
		if !ok || fmt.Sprint(va) != fmt.Sprint(vb) {
			diff[k] = [2]any{va, vb}
		}
	}
	for k, vb := range cb {
		if _, ok := ca[k]; !ok {
			diff[k] = [2]any{nil, vb}
		}
	}
	return diff, nil
}

// Lifetime reports the remaining validity of a token.
func (m *OIDCManipulator) Lifetime(idToken string) (time.Duration, error) {
	expired, err := oauth2.TokenExpired(idToken)
	if err != nil {
		return 0, err
	}
	claims, err := oauth2.DecodeJWTClaims(idToken)
	if err != nil {
		return 0, err
	}
	exp, ok := claims["exp"].(float64)
	if !ok {
		return 0, fmt.Errorf("no exp claim")
	}
	remaining := time.Until(time.Unix(int64(exp), 0))
	if expired {
		return -remaining, nil
	}
	return remaining, nil
}
