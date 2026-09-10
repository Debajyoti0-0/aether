package api

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// issueTestCert builds a client cert with controllable properties.
func issueTestCert(t *testing.T, name string, notBefore, notAfter time.Time, withSAN bool) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	if withSAN {
		u, _ := url.Parse(OperatorURIPrefix + name)
		tmpl.URIs = []*url.URL{u}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

// T1 acceptance: identity is derived from the certificate only.
func TestIdentityFromCert(t *testing.T) {
	now := time.Now()

	// Valid cert with the operator URI SAN.
	cert := issueTestCert(t, "alice", now.Add(-time.Hour), now.Add(time.Hour), true)
	op, err := FromClientCert(cert, now)
	if err != nil {
		t.Fatalf("valid cert rejected: %v", err)
	}
	if op.Name != "alice" {
		t.Errorf("name = %q", op.Name)
	}
	if op.CertFingerprint == "" || len(op.CertFingerprint) != 64 {
		t.Errorf("fingerprint = %q", op.CertFingerprint)
	}

	// Expired cert → deny.
	expired := issueTestCert(t, "alice", now.Add(-48*time.Hour), now.Add(-24*time.Hour), true)
	if _, err := FromClientCert(expired, now); err == nil || !contains(err.Error(), "expired") {
		t.Fatalf("expired cert accepted: %v", err)
	}

	// Not-yet-valid → deny.
	future := issueTestCert(t, "alice", now.Add(time.Hour), now.Add(2*time.Hour), true)
	if _, err := FromClientCert(future, now); err == nil {
		t.Fatal("not-yet-valid cert accepted")
	}

	// Missing URI SAN → deny.
	noSAN := issueTestCert(t, "bob", now.Add(-time.Hour), now.Add(time.Hour), false)
	if _, err := FromClientCert(noSAN, now); err == nil || !contains(err.Error(), "URI SAN") {
		t.Fatalf("cert without operator SAN accepted: %v", err)
	}

	// Hostile operator name in the SAN → deny.
	hostile := issueTestCert(t, "../evil", now.Add(-time.Hour), now.Add(time.Hour), true)
	if _, err := FromClientCert(hostile, now); err == nil {
		t.Fatal("traversal operator name accepted")
	}

	// Nil certificate → deny.
	if _, err := FromClientCert(nil, now); err == nil {
		t.Fatal("nil cert accepted")
	}
}

// T1 acceptance: revoked operators fail closed.
func TestRevokedOperatorRefused(t *testing.T) {
	rl := LoadRevocationList([]byte("# revocations\nalice\n\nbob\n"))
	if !rl.IsRevoked("alice") || !rl.IsRevoked("bob") {
		t.Fatal("revoked names not detected")
	}
	if rl.IsRevoked("carol") {
		t.Error("non-revoked name flagged")
	}
	// Missing file → empty list (handled by the caller via os.ReadFile).
	if _, err := os.ReadFile(filepath.Join(t.TempDir(), "missing.txt")); err == nil {
		t.Fatal("expected not-exist")
	}
}

// T1: capability files — defaults are read-only; the file replaces
// execute grants; "-cap" entries deny.
func TestOperatorCapabilities(t *testing.T) {
	dir := t.TempDir()

	// No file → default read-only set (execute denied — fail closed).
	caps, err := LoadOperatorCaps(dir, "newop")
	if err != nil {
		t.Fatal(err)
	}
	if caps.Has(CapExecAzure) {
		t.Error("execute capability granted by default — authorization regression")
	}
	if !caps.Has(CapReadAudit) {
		t.Error("read.audit missing from defaults")
	}

	// Granted execute caps.
	if err := WriteOperatorCaps(dir, "executor", []string{CapReadAudit, CapExecAzure}); err != nil {
		t.Fatal(err)
	}
	caps2, err := LoadOperatorCaps(dir, "executor")
	if err != nil {
		t.Fatal(err)
	}
	if !caps2.Has(CapExecAzure) || !caps2.Has(CapReadAudit) {
		t.Errorf("caps = %+v", caps2)
	}

	// Explicit denial with "-cap".
	if err := WriteOperatorCaps(dir, "restricted", []string{CapReadAudit, "-read.events"}); err != nil {
		t.Fatal(err)
	}
	caps3, err := LoadOperatorCaps(dir, "restricted")
	if err != nil {
		t.Fatal(err)
	}
	if caps3.Has(CapReadEvents) {
		t.Error("denied capability still granted")
	}

	// Hostile operator name → error (no traversal into operators dir).
	if _, err := LoadOperatorCaps(dir, "../evil"); err == nil {
		t.Fatal("traversal operator name accepted")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
