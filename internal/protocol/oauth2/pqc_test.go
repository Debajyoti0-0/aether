package oauth2

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func pqcJWKS() JWKS {
	// Minimal RSA JWK (n/e are tiny but valid base64url).
	return JWKS{Keys: []JWK{
		{Kty: "RSA", Alg: "RS256", Kid: "classic-1", N: "AQAB", E: "AQAB"},
		{Kty: "RSA", Alg: "Kyber512-Dilithium3", Kid: "pq-1", PQ: true},
	}}
}

func TestIsPQCAlg(t *testing.T) {
	cases := map[string]bool{
		"RS256":                false,
		"Kyber512":             true,
		"Dilithium3":           true,
		"ML-DSA-44":            true,
		"Falcon512":            true,
		"RS256-pq-hybrid":      true,
	}
	for alg, want := range cases {
		if got := IsPQCAlg(alg); got != want {
			t.Errorf("IsPQCAlg(%q) = %t, want %t", alg, got, want)
		}
	}
}

func TestDetectAndDowngradePQCWithRSA(t *testing.T) {
	jwks := pqcJWKS()
	d, err := DetectAndDowngrade(&jwks)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if !d.PQCDetected {
		t.Error("PQC not detected")
	}
	if len(d.PQCAlgs) != 1 || !strings.Contains(d.PQCAlgs[0], "Kyber") {
		t.Errorf("pqc algs = %v", d.PQCAlgs)
	}
	if d.DowngradeAlg != "HS256" {
		t.Errorf("downgrade = %q, want HS256 (RSA key present → alg confusion)", d.DowngradeAlg)
	}
	if !d.FallbackViable {
		t.Error("fallback should be viable")
	}
	if d.RSAPubKey == nil {
		t.Error("RSA pub key not extracted")
	}
}

func TestDetectAndDowngradeNoPQC(t *testing.T) {
	jwks := JWKS{Keys: []JWK{{Kty: "RSA", Alg: "RS256", Kid: "k1", N: "AQAB", E: "AQAB"}}}
	d, err := DetectAndDowngrade(&jwks)
	if err != nil {
		t.Fatal(err)
	}
	if d.PQCDetected {
		t.Error("false PQC detection")
	}
	if d.DowngradeAlg != "RS256" {
		t.Errorf("downgrade = %q", d.DowngradeAlg)
	}
}

func TestDetectAndDowngradePQCWithoutRSA(t *testing.T) {
	jwks := JWKS{Keys: []JWK{{Kty: "EC", Alg: "Kyber512", Kid: "pq"}}}
	d, err := DetectAndDowngrade(&jwks)
	if err != nil {
		t.Fatal(err)
	}
	if !d.PQCDetected || d.RSAPubKey != nil {
		t.Errorf("decision = %+v", d)
	}
	if d.DowngradeAlg != "RS256" || !d.FallbackViable {
		t.Errorf("fallback = %q viable=%t", d.DowngradeAlg, d.FallbackViable)
	}
}

func TestDetectAndDowngradeEmpty(t *testing.T) {
	if _, err := DetectAndDowngrade(nil); err == nil {
		t.Error("nil jwks should fail")
	}
	if _, err := DetectAndDowngrade(&JWKS{}); err == nil {
		t.Error("empty jwks should fail")
	}
}

func TestFetchJWKS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(pqcJWKS())
	}))
	defer srv.Close()

	jwks, err := FetchJWKS(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(jwks.Keys) != 2 {
		t.Errorf("keys = %d", len(jwks.Keys))
	}

	// PQC detection end-to-end through the fetch path.
	d, err := DetectAndDowngrade(jwks)
	if err != nil || !d.PQCDetected {
		t.Errorf("end-to-end detection failed: %+v err=%v", d, err)
	}
}

func TestFetchJWKSHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	if _, err := FetchJWKS(context.Background(), srv.Client(), srv.URL); err == nil {
		t.Error("500 should fail")
	}
	_ = base64.StdEncoding
}
