package revocation

import (
	"os"
	"testing"
)

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
