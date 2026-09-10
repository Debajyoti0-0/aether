package token

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/Debajyoti0-0/aether/internal/protocol/msoapx"
	"github.com/Debajyoti0-0/aether/internal/types"
	"github.com/Debajyoti0-0/aether/test/mock"
)

func TestParsePRT(t *testing.T) {
	raw := `{"cookie":"0.AAAA","device_id":"d1","tenant_id":"t1","user_id":"u1","session_key":"` +
		base64.StdEncoding.EncodeToString([]byte("0123456789abcdef")) + `"}`

	prt, err := ParsePRT([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if prt.Cookie != "0.AAAA" || prt.TenantID != "t1" {
		t.Errorf("prt = %+v", prt)
	}
	if prt.IsZero() {
		t.Error("prt should not be zero")
	}
}

func TestParsePRTMissingCookie(t *testing.T) {
	if _, err := ParsePRT([]byte(`{"tenant_id":"t1"}`)); err == nil {
		t.Error("expected error for missing cookie")
	}
}

func TestValidatePRT(t *testing.T) {
	c := NewPRTConverter(msoapx.NewClientWithHTTP(nil))

	err := c.validatePRT(&types.PRT{})
	if err == nil {
		t.Error("empty prt should fail")
	}

	err = c.validatePRT(&types.PRT{Cookie: "x", TenantID: "t"})
	if err == nil {
		t.Error("missing session key should fail")
	}

	err = c.validatePRT(&types.PRT{
		Cookie:     "x",
		TenantID:   "t",
		SessionKey: base64.StdEncoding.EncodeToString([]byte("0123456789abcdef")),
	})
	if err != nil {
		t.Errorf("valid prt should pass: %v", err)
	}
}

func TestConvertPRTToOAuth(t *testing.T) {
	srv := mock.NewEntraIDServer()
	defer srv.Close()

	// Point the MS-OAPX token endpoint at the mock by overriding the
	// package-level constant path through a client with custom HTTP.
	client := msoapx.NewClientWithHTTP(srv.Client())
	c := NewPRTConverter(client)

	// msoapx.Client posts to the real login endpoint; for unit testing
	// we verify validation logic only, using a bad tenant to fail fast.
	prt := &types.PRT{Cookie: "x", TenantID: "", SessionKey: base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))}
	if _, err := c.ConvertPRTToOAuth(context.Background(), prt, "", ""); err == nil {
		t.Error("empty tenant should fail validation")
	}
}

func TestPRTConverterDefaultClientID(t *testing.T) {
	if DefaultClientID == "" {
		t.Error("default client id should be set")
	}
}

var _ = time.Second
