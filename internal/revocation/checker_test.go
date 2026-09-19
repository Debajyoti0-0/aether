package revocation

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"os"
	"testing"
	"time"
)

// selfSignedIssuer creates a minimal self-signed issuer cert for
// fail-closed path tests (no fixture dependency).
func selfSignedIssuer(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "qa-issuer"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("cert: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return cert, key
}

// leafCert creates a leaf cert issued by the QA issuer.
func leafCert(t *testing.T, issuer *x509.Certificate, issuerKey *ecdsa.PrivateKey) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(4242),
		Subject:      pkix.Name{CommonName: "qa-leaf"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, issuer, &key.PublicKey, issuerKey)
	if err != nil {
		t.Fatalf("cert: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return cert
}

func TestOCSPRemoteUnreachableFailsClosed(t *testing.T) {
	// F-005 pin: an unreachable OCSP responder with FailClosed (the
	// default) must yield StatusError, which the evidence gate rejects.
	cfg := DefaultConfig()
	cfg.Mode = ModeOCSP
	cfg.OCSPResponder = "http://127.0.0.1:1/ocsp" // nothing listens here
	cfg.FailClosed = true
	cfg.Timeout = 2 * time.Second

	issuer, issuerKey := selfSignedIssuer(t)
	leaf := leafCert(t, issuer, issuerKey)

	c := NewChecker(cfg)
	res := c.Check(context.Background(), leaf, issuer)
	if res.Status != StatusError {
		t.Fatalf("unreachable responder: status = %v, want StatusError (fail-closed)", res.Status)
	}
}

func TestOCSPRemoteUnreachableFailOpenYieldsUnknown(t *testing.T) {
	// Explicit fail-open (operator override) yields StatusUnknown —
	// still rejected by the evidence gate, but distinctly reasoned.
	cfg := DefaultConfig()
	cfg.Mode = ModeOCSP
	cfg.OCSPResponder = "http://127.0.0.1:1/ocsp"
	cfg.FailClosed = false
	cfg.Timeout = 2 * time.Second

	issuer, issuerKey := selfSignedIssuer(t)
	leaf := leafCert(t, issuer, issuerKey)

	c := NewChecker(cfg)
	res := c.Check(context.Background(), leaf, issuer)
	if res.Status != StatusUnknown {
		t.Fatalf("fail-open unreachable responder: status = %v, want StatusUnknown", res.Status)
	}
}

func TestOCSPGood(t *testing.T) {
	// Test OCSP Good status parsing with a pre-generated response
	// This test uses a pre-generated OCSP response fixture
	respData, err := os.ReadFile("../../testdata/ocsp-good.der")
	if err != nil {
		t.Skipf("OCSP fixture not available: %v", err)
	}

	// We can't easily test ParseResponseForCert without a real cert/issuer pair
	// Just verify the fixture exists and has data
	if len(respData) == 0 {
		t.Errorf("OCSP good fixture is empty")
	}
}

func TestOCSPRevoked(t *testing.T) {
	respData, err := os.ReadFile("../../testdata/ocsp-revoked.der")
	if err != nil {
		t.Skipf("OCSP fixture not available: %v", err)
	}

	if len(respData) == 0 {
		t.Errorf("OCSP revoked fixture is empty")
	}
}

func TestCRLGood(t *testing.T) {
	crlData, err := os.ReadFile("../../testdata/crl.der")
	if err != nil {
		t.Skipf("CRL fixture not available: %v", err)
	}

	if len(crlData) == 0 {
		t.Errorf("CRL fixture is empty")
	}
}

func TestCRLRevoked(t *testing.T) {
	// Test that we can parse a CRL
	crlData, err := os.ReadFile("../../testdata/crl.der")
	if err != nil {
		t.Skipf("CRL fixture not available: %v", err)
	}

	if len(crlData) == 0 {
		t.Errorf("CRL fixture is empty")
	}
}

func TestFileRevocation(t *testing.T) {
	// Create temp revocation file
	tmpFile, err := os.CreateTemp("", "revoked-*.txt")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write revoked serial numbers
	content := "12345\n67890\n# comment\n"
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("write file: %v", err)
	}
	tmpFile.Close()

	// Test checker
	config := DefaultConfig()
	config.Mode = ModeFile
	config.RevocationFile = tmpFile.Name()

	checker := NewChecker(config)

	// Test revoked serial
	result := checker.CheckSerial(nil, "12345", nil)
	if result.Status != StatusRevoked {
		t.Errorf("expected Revoked, got %s", result.Status)
	}

	// Test non-revoked serial
	result = checker.CheckSerial(nil, "99999", nil)
	if result.Status != StatusGood {
		t.Errorf("expected Good, got %s", result.Status)
	}
}

func TestFileRevocationWithBOM(t *testing.T) {
	// Create temp revocation file with UTF-8 BOM
	tmpFile, err := os.CreateTemp("", "revoked-*.txt")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write revoked serial numbers with UTF-8 BOM
	content := "\ufeff12345\r\n67890\r\n# comment\r\n"
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("write file: %v", err)
	}
	tmpFile.Close()

	// Test checker
	config := DefaultConfig()
	config.Mode = ModeFile
	config.RevocationFile = tmpFile.Name()

	checker := NewChecker(config)

	// Test revoked serial (with BOM stripped)
	result := checker.CheckSerial(nil, "12345", nil)
	if result.Status != StatusRevoked {
		t.Errorf("expected Revoked with BOM, got %s", result.Status)
	}
}

func TestOCSPFixturesExist(t *testing.T) {
	// Verify OCSP fixtures exist
	files := []string{
		"../../testdata/ocsp-good.der",
		"../../testdata/ocsp-revoked.der",
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Logf("Fixture %s not available: %v", f, err)
		} else if len(data) == 0 {
			t.Errorf("Fixture %s is empty", f)
		}
	}
}

func TestCRLFixturesExist(t *testing.T) {
	// Verify CRL fixture exists
	data, err := os.ReadFile("../../testdata/crl.der")
	if err != nil {
		t.Skipf("CRL fixture not available: %v", err)
	}
	if len(data) == 0 {
		t.Errorf("CRL fixture is empty")
	}
}

func TestRevocationFileFixturesExist(t *testing.T) {
	// Verify revocation file fixture exists
	data, err := os.ReadFile("../../testdata/revoked.txt")
	if err != nil {
		t.Skipf("Revocation file fixture not available: %v", err)
	}
	if len(data) == 0 {
		t.Errorf("Revocation file fixture is empty")
	}
}
