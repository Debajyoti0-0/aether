package cli

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const verifyTestdata = "../../testdata"

// setVerifyFlags resets the verify-evidence flag globals to a baseline
// before each test.
func setVerifyFlags(t *testing.T) {
	t.Helper()
	evRevocationMode = "none"
	evCertFile = ""
	evIssuerCert = ""
	evResponderURL = ""
	evCRLURL = ""
	evCRLFile = ""
	evRevocationFile = ""
	evSkipSigVerify = false
	evTimeout = "10s"
	evCacheTTL = "5m"
	evFailClosed = true
	evJSON = false
}

// startOCSPTestResponder serves the given fixture bytes as an
// application/ocsp-response for both GET and POST.
func startOCSPTestResponder(t *testing.T, fixture string) *httptest.Server {
	t.Helper()
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("read fixture %s: %v", fixture, err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/ocsp-response")
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestVerifyEvidenceOCSPGood(t *testing.T) {
	setVerifyFlags(t)
	srv := startOCSPTestResponder(t, filepath.Join(verifyTestdata, "ocsp-good.der"))
	evRevocationMode = "ocsp"
	evCertFile = filepath.Join(verifyTestdata, "leaf.pem")
	evIssuerCert = filepath.Join(verifyTestdata, "ca.pem")
	evResponderURL = srv.URL
	if err := runExportVerify(exportVerifyCmd, nil); err != nil {
		t.Fatalf("expected GOOD evidence accepted (nil), got: %v", err)
	}
}

func TestVerifyEvidenceOCSPRevoked(t *testing.T) {
	setVerifyFlags(t)
	srv := startOCSPTestResponder(t, filepath.Join(verifyTestdata, "ocsp-revoked.der"))
	evRevocationMode = "ocsp"
	evCertFile = filepath.Join(verifyTestdata, "leaf.pem")
	evIssuerCert = filepath.Join(verifyTestdata, "ca.pem")
	evResponderURL = srv.URL
	err := runExportVerify(exportVerifyCmd, nil)
	if !errors.Is(err, ErrEvidenceRevoked) {
		t.Fatalf("expected ErrEvidenceRevoked, got: %v", err)
	}
}

func TestVerifyEvidenceOCSPUnknown(t *testing.T) {
	setVerifyFlags(t)
	srv := startOCSPTestResponder(t, filepath.Join(verifyTestdata, "ocsp-unknown.der"))
	evRevocationMode = "ocsp"
	evCertFile = filepath.Join(verifyTestdata, "leaf.pem")
	evIssuerCert = filepath.Join(verifyTestdata, "ca.pem")
	evResponderURL = srv.URL
	err := runExportVerify(exportVerifyCmd, nil)
	if !errors.Is(err, ErrEvidenceUnknown) {
		t.Fatalf("expected ErrEvidenceUnknown, got: %v", err)
	}
}

func TestVerifyEvidenceOCSPUnavailable(t *testing.T) {
	setVerifyFlags(t)
	evRevocationMode = "ocsp"
	evCertFile = filepath.Join(verifyTestdata, "leaf.pem")
	evIssuerCert = filepath.Join(verifyTestdata, "ca.pem")
	// Port 1 on loopback: nothing listens there; fail-closed must
	// convert this into a check failure, not UNKNOWN-then-accept.
	evResponderURL = "http://127.0.0.1:1/ocsp"
	err := runExportVerify(exportVerifyCmd, nil)
	if err == nil {
		t.Fatal("expected non-nil error for unavailable responder")
	}
	if errors.Is(err, nil) || err.Error() == "" {
		t.Fatalf("error must carry the failure: %v", err)
	}
}

func TestVerifyEvidenceOCSPWrongCertificate(t *testing.T) {
	setVerifyFlags(t)
	// The responder serves a response for the LEAF serial; presenting
	// the dummy-12345 certificate must not validate against it.
	srv := startOCSPTestResponder(t, filepath.Join(verifyTestdata, "ocsp-good.der"))
	evRevocationMode = "ocsp"
	evCertFile = filepath.Join(verifyTestdata, "dummy-12345.pem")
	evIssuerCert = filepath.Join(verifyTestdata, "ca.pem")
	evResponderURL = srv.URL
	if err := runExportVerify(exportVerifyCmd, nil); err == nil {
		t.Fatal("expected non-nil error: response does not match supplied certificate")
	}
}

func TestVerifyEvidenceMissingCert(t *testing.T) {
	setVerifyFlags(t)
	evRevocationMode = "ocsp"
	evIssuerCert = filepath.Join(verifyTestdata, "ca.pem")
	err := runExportVerify(exportVerifyCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "--cert") {
		t.Fatalf("expected --cert required error, got: %v", err)
	}
}

func TestVerifyEvidenceUnreadableCert(t *testing.T) {
	setVerifyFlags(t)
	evRevocationMode = "ocsp"
	evIssuerCert = filepath.Join(verifyTestdata, "ca.pem")
	evCertFile = filepath.Join(verifyTestdata, "does-not-exist.pem")
	err := runExportVerify(exportVerifyCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "read cert") {
		t.Fatalf("expected read error, got: %v", err)
	}
}

func TestVerifyEvidenceMalformedCert(t *testing.T) {
	setVerifyFlags(t)
	bad := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(bad, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	evRevocationMode = "ocsp"
	evIssuerCert = filepath.Join(verifyTestdata, "ca.pem")
	evCertFile = bad
	err := runExportVerify(exportVerifyCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "PEM CERTIFICATE") {
		t.Fatalf("expected malformed PEM error, got: %v", err)
	}
}

func TestVerifyEvidenceCRLGood(t *testing.T) {
	setVerifyFlags(t)
	evRevocationMode = "crl"
	evCertFile = filepath.Join(verifyTestdata, "leaf.pem")
	evIssuerCert = filepath.Join(verifyTestdata, "ca.pem")
	evCRLFile = filepath.Join(verifyTestdata, "crl.pem")
	if err := runExportVerify(exportVerifyCmd, nil); err != nil {
		t.Fatalf("expected GOOD (leaf not on CRL), got: %v", err)
	}
}

func TestVerifyEvidenceCRLRevoked(t *testing.T) {
	setVerifyFlags(t)
	evRevocationMode = "crl"
	evCertFile = filepath.Join(verifyTestdata, "leaf.pem")
	evIssuerCert = filepath.Join(verifyTestdata, "ca.pem")
	evCRLFile = filepath.Join(verifyTestdata, "crl-revoked.pem")
	err := runExportVerify(exportVerifyCmd, nil)
	if !errors.Is(err, ErrEvidenceRevoked) {
		t.Fatalf("expected ErrEvidenceRevoked, got: %v", err)
	}
}

func TestVerifyEvidenceCRLWrongIssuer(t *testing.T) {
	setVerifyFlags(t)
	// The CRL is signed by the test CA; presenting the leaf as the
	// issuer must fail signature verification.
	evRevocationMode = "crl"
	evCertFile = filepath.Join(verifyTestdata, "leaf.pem")
	evIssuerCert = filepath.Join(verifyTestdata, "leaf.pem")
	evCRLFile = filepath.Join(verifyTestdata, "crl.pem")
	if err := runExportVerify(exportVerifyCmd, nil); err == nil {
		t.Fatal("expected non-nil error for CRL signed by wrong issuer")
	}
}

func TestVerifyEvidenceFileMode(t *testing.T) {
	t.Run("revoked", func(t *testing.T) {
		setVerifyFlags(t)
		evRevocationMode = "file"
		evCertFile = filepath.Join(verifyTestdata, "dummy-12345.pem") // serial 12345
		evRevocationFile = filepath.Join(verifyTestdata, "revoked.txt")
		err := runExportVerify(exportVerifyCmd, nil)
		if !errors.Is(err, ErrEvidenceRevoked) {
			t.Fatalf("expected ErrEvidenceRevoked, got: %v", err)
		}
	})
	t.Run("good", func(t *testing.T) {
		setVerifyFlags(t)
		evRevocationMode = "file"
		evCertFile = filepath.Join(verifyTestdata, "leaf.pem")
		goodList := filepath.Join(t.TempDir(), "good.txt")
		if err := os.WriteFile(goodList, []byte("12345\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		evRevocationFile = goodList
		if err := runExportVerify(exportVerifyCmd, nil); err != nil {
			t.Fatalf("expected GOOD (serial not listed), got: %v", err)
		}
	})
	t.Run("missing revocation file", func(t *testing.T) {
		setVerifyFlags(t)
		evRevocationMode = "file"
		evCertFile = filepath.Join(verifyTestdata, "leaf.pem")
		evRevocationFile = filepath.Join(verifyTestdata, "no-such-list.txt")
		if err := runExportVerify(exportVerifyCmd, nil); err == nil {
			t.Fatal("expected non-nil error for missing revocation file (fail closed)")
		}
	})
}

func TestVerifyEvidenceJSONOutputValid(t *testing.T) {
	setVerifyFlags(t)
	// Capture stdout while emitting JSON.
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	evJSON = true
	evRevocationMode = "crl"
	evCertFile = filepath.Join(verifyTestdata, "leaf.pem")
	evIssuerCert = filepath.Join(verifyTestdata, "ca.pem")
	evCRLFile = filepath.Join(verifyTestdata, "crl.pem")
	runErr := runExportVerify(exportVerifyCmd, nil)
	_ = w.Close()
	os.Stdout = old
	outBytes, _ := io.ReadAll(r)
	out := string(outBytes)
	if runErr != nil {
		t.Fatalf("expected success, got: %v", runErr)
	}
	for _, want := range []string{`"status":"GOOD"`, `"accepted":true`, `"certificate_subject":"CN=Aether Test Leaf,O=Aether"`, `"serial":"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("JSON output missing %s; got: %s", want, out)
		}
	}
	if !json.Valid([]byte(strings.TrimSpace(out))) {
		t.Fatalf("output is not valid JSON: %s", out)
	}
}
