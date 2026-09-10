package oauth2

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/Debajyoti0-0/aether/internal/types"
)

// DecodeJWTClaims splits a JWT and decodes its payload without
// verifying the signature (tokens are used, not trusted).
func DecodeJWTClaims(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("not a JWT (got %d parts)", len(parts))
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode jwt payload: %w", err)
	}

	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("parse jwt claims: %w", err)
	}
	return claims, nil
}

// ParseIDToken extracts known OIDC claims from an ID token.
func ParseIDToken(token string) (*types.IDTokenClaims, error) {
	claims, err := DecodeJWTClaims(token)
	if err != nil {
		return nil, err
	}

	out := &types.IDTokenClaims{Raw: claims}
	getStr := func(key string) string {
		if v, ok := claims[key].(string); ok {
			return v
		}
		return ""
	}
	out.Issuer = getStr("iss")
	out.Subject = getStr("sub")
	out.Audience = getStr("aud")
	out.TenantID = getStr("tid")
	out.UPN = getStr("upn")
	out.Email = getStr("email")
	out.Name = getStr("name")
	out.ObjectID = getStr("oid")
	out.DeviceID = getStr("deviceid")
	out.Nonce = getStr("nonce")
	if v, ok := claims["exp"].(float64); ok {
		out.Expiry = int64(v)
	}
	if v, ok := claims["iat"].(float64); ok {
		out.IssuedAt = int64(v)
	}
	return out, nil
}

// TokenExpired reports whether a JWT's exp claim has passed.
func TokenExpired(token string) (bool, error) {
	claims, err := DecodeJWTClaims(token)
	if err != nil {
		return false, err
	}
	exp, ok := claims["exp"].(float64)
	if !ok {
		return false, fmt.Errorf("no exp claim")
	}
	return time.Now().After(time.Unix(int64(exp), 0)), nil
}

// ParseUnverified is a thin wrapper over jwt/v5 for full claim access.
func ParseUnverified(token string) (jwt.MapClaims, error) {
	parsed, _, err := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("unexpected claims type")
	}
	return claims, nil
}
