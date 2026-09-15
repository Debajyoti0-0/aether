//go:build integration

// Stage 4 backfill PKI lifecycle evidence (B4-G11): issuance, chain
// validation, renewal, rotation, revocation, expiry, cross-sign
// isolation, malformed-cert rejection, ASCII-only operator name
// enforcement, and the IA5String issuance guard — all against the real
// teamserver PKI (internal/api/ca.go, identity.go).
package integration

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Debajyoti0-0/aether/internal/api"
)

// initCA creates a fresh CA hierarchy in a per-test temp dir.
func initCA(t *testing.T) *api.CertPaths {
	t.Helper()
	paths, err := api.InitCA(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

// parseCertPEM decodes the first CERTIFICATE block in a PEM body.
func parseCertPEM(t *testing.T, pemBytes []byte) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		t.Fatal("no PEM block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return cert
}

func TestPKILifecycle(t *testing.T) {
	t.Run("issuance", func(t *testing.T) {
		paths := initCA(t)
		certPEM, keyPEM, err := api.IssueOperatorCert(paths.CACert, paths.CAKey, "alice", 365)
		if err != nil {
			t.Fatal(err)
		}
		cert := parseCertPEM(t, certPEM)
		if cert.Subject.CommonName != "alice" {
			t.Fatalf("CN = %q", cert.Subject.CommonName)
		}
		wantSAN := api.OperatorURIPrefix + "alice"
		if len(cert.URIs) != 1 || cert.URIs[0].String() != wantSAN {
			t.Fatalf("URIs = %v, want [%s]", cert.URIs, wantSAN)
		}
		if len(keyPEM) == 0 || !strings.Contains(string(keyPEM), "EC PRIVATE KEY") {
			t.Fatal("key PEM missing")
		}
		// IA5String sanity: every SAN URI byte is ASCII.
		for _, b := range []byte(cert.URIs[0].String()) {
			if b > 0x7f {
				t.Fatalf("SAN byte 0x%02x non-ASCII", b)
			}
		}
	})

	t.Run("chain_validation", func(t *testing.T) {
		paths := initCA(t)
		certPEM, _, err := api.IssueOperatorCert(paths.CACert, paths.CAKey, "bob", 365)
		if err != nil {
			t.Fatal(err)
		}
		cert := parseCertPEM(t, certPEM)
		caPEM, err := os.ReadFile(paths.CACert)
		if err != nil {
			t.Fatal(err)
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(caPEM) {
			t.Fatal("CA pool empty")
		}
		if _, err := cert.Verify(x509.VerifyOptions{
			Roots:     roots,
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		}); err != nil {
			t.Fatalf("chain validation failed: %v", err)
		}
	})

	t.Run("renewal_extends_validity", func(t *testing.T) {
		paths := initCA(t)
		shortPEM, _, err := api.IssueOperatorCert(paths.CACert, paths.CAKey, "carol", 7)
		if err != nil {
			t.Fatal(err)
		}
		longPEM, _, err := api.IssueOperatorCert(paths.CACert, paths.CAKey, "carol", 365)
		if err != nil {
			t.Fatal(err)
		}
		if !parseCertPEM(t, longPEM).NotAfter.After(parseCertPEM(t, shortPEM).NotAfter) {
			t.Fatal("renewal did not extend NotAfter")
		}
	})

	t.Run("rotation_new_keypair", func(t *testing.T) {
		paths := initCA(t)
		p1, _, err := api.IssueOperatorCert(paths.CACert, paths.CAKey, "dave", 365)
		if err != nil {
			t.Fatal(err)
		}
		p2, _, err := api.IssueOperatorCert(paths.CACert, paths.CAKey, "dave", 365)
		if err != nil {
			t.Fatal(err)
		}
		c1, c2 := parseCertPEM(t, p1), parseCertPEM(t, p2)
		if c1.SerialNumber.Cmp(c2.SerialNumber) == 0 {
			t.Fatal("rotation reused serial")
		}
		if string(c1.Raw) == string(c2.Raw) {
			t.Fatal("rotation produced identical DER")
		}
		pub1, ok1 := c1.PublicKey.(*ecdsa.PublicKey)
		pub2, ok2 := c2.PublicKey.(*ecdsa.PublicKey)
		if !ok1 || !ok2 || pub1.Equal(pub2) {
			t.Fatal("rotation did not mint a new keypair")
		}
	})

	t.Run("revocation", func(t *testing.T) {
		rl := api.LoadRevocationList([]byte("# revoked operators\nmallory\n\neve\n"))
		if !rl.IsRevoked("mallory") || !rl.IsRevoked("eve") {
			t.Fatal("revoked names not detected")
		}
		if rl.IsRevoked("alice") {
			t.Fatal("unrevoked name flagged")
		}
		if api.LoadRevocationList(nil).IsRevoked("mallory") {
			t.Fatal("empty list flagged a name")
		}
	})

	t.Run("expiry_fail_closed", func(t *testing.T) {
		paths := initCA(t)
		certPEM, _, err := api.IssueOperatorCert(paths.CACert, paths.CAKey, "frank", 1)
		if err != nil {
			t.Fatal(err)
		}
		cert := parseCertPEM(t, certPEM)
		if _, err := api.FromClientCert(cert, cert.NotAfter.Add(time.Hour)); err == nil {
			t.Fatal("expired certificate accepted")
		}
		if _, err := api.FromClientCert(cert, cert.NotBefore.Add(-2*time.Hour)); err == nil {
			t.Fatal("not-yet-valid certificate accepted")
		}
		if _, err := api.FromClientCert(cert, time.Now()); err != nil {
			t.Fatalf("valid certificate rejected: %v", err)
		}
	})

	t.Run("cross_sign_isolation", func(t *testing.T) {
		ca1 := initCA(t)
		ca2 := initCA(t)
		p1, _, err := api.IssueOperatorCert(ca1.CACert, ca1.CAKey, "grace", 365)
		if err != nil {
			t.Fatal(err)
		}
		cert := parseCertPEM(t, p1)
		poolFor := func(p *api.CertPaths) *x509.CertPool {
			caPEM, err := os.ReadFile(p.CACert)
			if err != nil {
				t.Fatal(err)
			}
			pool := x509.NewCertPool()
			pool.AppendCertsFromPEM(caPEM)
			return pool
		}
		if _, err := cert.Verify(x509.VerifyOptions{Roots: poolFor(ca1), KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
			t.Fatalf("own-CA validation failed: %v", err)
		}
		if _, err := cert.Verify(x509.VerifyOptions{Roots: poolFor(ca2)}); err == nil {
			t.Fatal("cross-CA validation succeeded (chain isolation broken)")
		}
	})

	t.Run("malformed_cert_rejection", func(t *testing.T) {
		paths := initCA(t)
		dir := t.TempDir()
		bad := filepath.Join(dir, "bad.crt")
		if err := os.WriteFile(bad, []byte("not a certificate at all"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := api.LoadOperatorTLS(bad, bad, bad); err == nil {
			t.Fatal("garbage PEM accepted by LoadOperatorTLS")
		}
		// Certificate without the operator URI SAN.
		if _, err := api.FromClientCert(&x509.Certificate{
			NotBefore: time.Now().Add(-time.Hour),
			NotAfter:  time.Now().Add(time.Hour),
		}, time.Now()); err == nil {
			t.Fatal("cert without operator SAN accepted")
		}
		// Truncated DER.
		good, _, err := api.IssueOperatorCert(paths.CACert, paths.CAKey, "gina", 1)
		if err != nil {
			t.Fatal(err)
		}
		der := parseCertPEM(t, good).Raw[:32]
		trunc := filepath.Join(dir, "trunc.crt")
		if err := os.WriteFile(trunc, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := api.LoadOperatorTLS(trunc, bad, paths.CACert); err == nil {
			t.Fatal("truncated DER accepted")
		}
	})

	t.Run("ascii_only_operator_name", func(t *testing.T) {
		paths := initCA(t)
		hostile := []string{
			"operatör",        // Latin small o with diaeresis
			"оperator",        // Cyrillic o homoglyph prefix
			"操作员",             // CJK
			"operator\u00a0x", // NBSP
			"operator\tx",     // tab (control)
			"../escape",       // traversal
			" with space ",    // whitespace-padded
			"",                // empty
		}
		for _, name := range hostile {
			if _, _, err := api.IssueOperatorCert(paths.CACert, paths.CAKey, name, 1); err == nil {
				t.Fatalf("hostile operator name %q accepted at issuance", name)
			}
			// Identity-derivation leg: only for names that parse as a
			// URI at all (net/url rejects control characters before
			// validation could run; issuance already rejects them).
			u, uerr := url.Parse(api.OperatorURIPrefix + name)
			if uerr != nil {
				continue
			}
			cert := &x509.Certificate{URIs: []*url.URL{u}}
			if _, err := api.FromClientCert(cert, time.Now()); err == nil {
				t.Fatalf("hostile operator name %q accepted at identity derivation", name)
			}
		}
		// ASCII names remain valid.
		if _, _, err := api.IssueOperatorCert(paths.CACert, paths.CAKey, "operator-1", 1); err != nil {
			t.Fatalf("ASCII name rejected: %v", err)
		}
	})

	t.Run("ca_overwrite_fail_closed", func(t *testing.T) {
		paths := initCA(t)
		dir := filepath.Dir(paths.CACert)
		if _, err := api.InitCA(dir, nil); err == nil {
			t.Fatal("second InitCA overwrote existing CA")
		}
	})
}


