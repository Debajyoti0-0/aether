package token

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// buildRS256JWT creates a signed RS256 JWT for testing.
func buildRS256JWT(t *testing.T, claims map[string]any) (string, *rsa.PublicKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	hdr := map[string]any{"alg": "RS256", "typ": "JWT", "kid": "test-key"}
	hdrB64, _ := b64urlJSON(hdr)
	payloadB64, _ := b64urlJSON(claims)

	token := hdrB64 + "." + payloadB64 + ".fakesig"
	return token, &key.PublicKey
}

func TestConfuseJWT(t *testing.T) {
	orig, pub := buildRS256JWT(t, map[string]any{
		"sub": "user1", "aud": "api://orig", "roles": []string{"reader"},
	})

	res, err := ConfuseJWT(orig, pub, map[string]any{"roles": []string{"admin"}})
	if err != nil {
		t.Fatalf("confuse: %v", err)
	}

	parts := strings.Split(res.ForgedToken, ".")
	if len(parts) != 3 {
		t.Fatalf("forged token parts = %d", len(parts))
	}

	// Header is HS256 and preserves kid.
	var hdr map[string]any
	hdrJSON, _ := base64.RawURLEncoding.DecodeString(parts[0])
	json.Unmarshal(hdrJSON, &hdr)
	if hdr["alg"] != "HS256" {
		t.Errorf("alg = %v", hdr["alg"])
	}
	if hdr["kid"] != "test-key" {
		t.Errorf("kid = %v", hdr["kid"])
	}

	// Claims preserved + override applied.
	var claims map[string]any
	payloadJSON, _ := base64.RawURLEncoding.DecodeString(parts[1])
	json.Unmarshal(payloadJSON, &claims)
	if claims["sub"] != "user1" {
		t.Errorf("sub = %v", claims["sub"])
	}
	if roles, ok := claims["roles"].([]any); !ok || roles[0] != "admin" {
		t.Errorf("roles = %v", claims["roles"])
	}
	if res.OriginalAlg != "RS256" || res.ForgedAlg != "HS256" {
		t.Errorf("algs = %s → %s", res.OriginalAlg, res.ForgedAlg)
	}
}

func TestConfuseJWTRejectsNonRS(t *testing.T) {
	hsToken := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ1In0.sig"
	if _, err := ConfuseJWT(hsToken, nil, nil); err == nil {
		t.Error("HS256 input should be rejected")
	}
	if _, err := ConfuseJWT("not-a-jwt", nil, nil); err == nil {
		t.Error("garbage input should fail")
	}
}

func TestConfuseJWTSignatureIsHMACOfInput(t *testing.T) {
	orig, pub := buildRS256JWT(t, map[string]any{"sub": "u"})
	res, err := ConfuseJWT(orig, pub, nil)
	if err != nil {
		t.Fatal(err)
	}

	parts := strings.Split(res.ForgedToken, ".")
	signingInput := parts[0] + "." + parts[1]

	// Recompute the HMAC with the same secret derivation and compare.
	mac := hmac.New(sha256.New, publicKeySecret(pub))
	mac.Write([]byte(signingInput))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if parts[2] != want {
		t.Error("forged signature does not match recomputed HMAC")
	}
}

func TestRepurposeAud(t *testing.T) {
	orig, _ := buildRS256JWT(t, map[string]any{"sub": "u", "aud": "api://arm"})

	res, err := RepurposeAud(orig, "api://graph")
	if err != nil {
		t.Fatalf("repurpose: %v", err)
	}
	if res.OriginalAud != "api://arm" || res.NewAud != "api://graph" {
		t.Errorf("aud = %s → %s", res.OriginalAud, res.NewAud)
	}

	// Unsigned token: header alg=none, signature empty.
	parts := strings.Split(res.UnsignedToken, ".")
	if len(parts) != 3 || parts[2] != "" {
		t.Errorf("unsigned token = %q", res.UnsignedToken)
	}
	var hdr map[string]any
	hdrJSON, _ := base64.RawURLEncoding.DecodeString(parts[0])
	json.Unmarshal(hdrJSON, &hdr)
	if hdr["alg"] != "none" {
		t.Errorf("alg = %v", hdr["alg"])
	}

	var claims map[string]any
	payloadJSON, _ := base64.RawURLEncoding.DecodeString(parts[1])
	json.Unmarshal(payloadJSON, &claims)
	if claims["aud"] != "api://graph" {
		t.Errorf("aud = %v", claims["aud"])
	}
}

func TestRepurposeAudArray(t *testing.T) {
	orig, _ := buildRS256JWT(t, map[string]any{"aud": []string{"a", "b"}})
	res, err := RepurposeAud(orig, "c")
	if err != nil {
		t.Fatal(err)
	}
	if res.OriginalAud != "a" {
		t.Errorf("original aud = %q (first element)", res.OriginalAud)
	}
}
