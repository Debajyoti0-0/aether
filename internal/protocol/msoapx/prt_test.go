package msoapx

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/Debajyoti0-0/aether/internal/types"
)

func TestComputeSessionKeyProof(t *testing.T) {
	key := []byte("0123456789abcdef")
	proof1 := ComputeSessionKeyProof(key, []byte("nonce"), []byte("ctx"))
	proof2 := ComputeSessionKeyProof(key, []byte("nonce"), []byte("ctx"))
	if proof1 != proof2 {
		t.Error("proof should be deterministic")
	}
	if proof3 := ComputeSessionKeyProof(key, []byte("other"), []byte("ctx")); proof3 == proof1 {
		t.Error("different nonce should produce different proof")
	}
}

func TestURLValuesEncoding(t *testing.T) {
	buf := urlValues(map[string]string{
		"a": "x y", "b": "1+2", "skip": "",
	})
	s := buf.String()
	if !strings.Contains(s, "a=x+y") {
		t.Errorf("space not encoded: %s", s)
	}
	if !strings.Contains(s, "b=1%2B2") {
		t.Errorf("plus not encoded: %s", s)
	}
	if strings.Contains(s, "skip") {
		t.Errorf("empty value should be skipped: %s", s)
	}
}

func TestDecodeKey(t *testing.T) {
	if _, err := decodeKey(""); err == nil {
		t.Error("empty key should fail")
	}
	if _, err := decodeKey("not-base64!!"); err == nil {
		t.Error("invalid base64 should fail")
	}
	if _, err := decodeKey(base64.StdEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Error("short key should fail")
	}
	if _, err := decodeKey(base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))); err != nil {
		t.Errorf("valid key should pass: %v", err)
	}
}

func TestExchangeValidation(t *testing.T) {
	c := NewClientWithHTTP(nil)

	if _, err := c.Exchange(context.Background(), ExchangeRequest{}); err == nil || !strings.Contains(err.Error(), "prt is required") {
		t.Errorf("expected prt required error, got %v", err)
	}

	if _, err := c.Exchange(context.Background(), ExchangeRequest{PRT: &types.PRT{Cookie: "x"}}); err == nil || !strings.Contains(err.Error(), "tenant is required") {
		t.Errorf("expected tenant required error, got %v", err)
	}

	if _, err := c.Exchange(context.Background(), ExchangeRequest{
		PRT:    &types.PRT{Cookie: "x", SessionKey: base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))},
		Tenant: "t",
	}); err == nil || !strings.Contains(err.Error(), "client_id is required") {
		t.Errorf("expected client_id required error, got %v", err)
	}
}

func TestGenerateDeviceKeyPairAndCSR(t *testing.T) {
	key, pemData, err := GenerateDeviceKeyPair(2048)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	if !strings.Contains(pemData, "BEGIN RSA PRIVATE KEY") {
		t.Error("pem should contain RSA key")
	}

	csr, err := BuildCSR(key, RegistrationRequest{Tenant: "t", DeviceName: "dev"})
	if err != nil {
		t.Fatalf("csr: %v", err)
	}
	if !strings.Contains(csr, "BEGIN CERTIFICATE REQUEST") {
		t.Error("csr pem malformed")
	}
}

