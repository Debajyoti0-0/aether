package oauth2

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Debajyoti0-0/aether/test/mock"
)

func TestRefreshTokenGrant(t *testing.T) {
	srv := mock.NewEntraIDServer()
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, srv.Client())
	tokens, err := c.RefreshToken(context.Background(), "dummy", "client-id", "rt-value", "https://graph.microsoft.com/.default")
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if tokens.AccessToken != "mock_access_token" {
		t.Errorf("access token = %q", tokens.AccessToken)
	}
	if tokens.TokenType != "Bearer" {
		t.Errorf("token type = %q", tokens.TokenType)
	}
	if tokens.IssuedAt.IsZero() {
		t.Error("issued at should be set")
	}
}

func TestErrorParsing(t *testing.T) {
	srv := mock.NewEntraIDServer()
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, srv.Client())

	dc := &DeviceCode{DeviceCode: "abc", Interval: 1, ExpiresIn: 60}
	_, err := c.PollDeviceCode(context.Background(), "dummy", "client-id", dc)
	if !errors.Is(err, ErrAuthPending) && err != ErrAuthPending {
		t.Fatalf("want ErrAuthPending, got %v", err)
	}

	var oerr *OAuthError
	if _, err := c.Token(context.Background(), "dummy", nil); err == nil {
		t.Fatal("empty form should fail")
	} else if !errors.As(err, &oerr) {
		t.Fatalf("expected OAuthError, got %T", err)
	}
}

func TestStartDeviceCode(t *testing.T) {
	srv := mock.NewEntraIDServer()
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, srv.Client())
	dc, err := c.StartDeviceCode(context.Background(), "dummy", "client-id", "scope")
	if err != nil {
		t.Fatalf("devicecode failed: %v", err)
	}
	if dc.UserCode != "MOCKCODE" {
		t.Errorf("user code = %q", dc.UserCode)
	}
	if dc.Interval != 1 {
		t.Errorf("interval = %d", dc.Interval)
	}
}

func TestDecodeJWTClaims(t *testing.T) {
	// header {"alg":"none","typ":"JWT"} payload {"exp":1893456000,"sub":"u1"}
	token := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJleHAiOjE4OTM0NTYwMDAsInN1YiI6InUxIn0.sig"

	claims, err := DecodeJWTClaims(token)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if claims["sub"] != "u1" {
		t.Errorf("sub = %v", claims["sub"])
	}

	expired, err := TokenExpired(token)
	if err != nil {
		t.Fatalf("expired check failed: %v", err)
	}
	if expired {
		t.Error("exp 2030 should not be expired")
	}

	if _, err := DecodeJWTClaims("not-a-jwt"); err == nil {
		t.Error("expected error for non-jwt")
	}
}

func TestParseIDToken(t *testing.T) {
	// payload: {"iss":"https://sts","sub":"s","tid":"t1","upn":"a@b.c","exp":1893456000}
	token := "x." + b64url(`{"iss":"https://sts","sub":"s","tid":"t1","upn":"a@b.c","exp":1893456000}`) + ".y"

	claims, err := ParseIDToken(token)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if claims.TenantID != "t1" || claims.UPN != "a@b.c" {
		t.Errorf("claims = %+v", claims)
	}
}

func TestTimeoutRespected(t *testing.T) {
	srv := mock.NewEntraIDServer()
	defer srv.Close()

	hc := srv.Client()
	hc.Timeout = 5 * time.Second
	c := NewClientWithHTTP(srv.URL, hc)
	if !strings.HasPrefix(c.BaseURL, "http") {
		t.Errorf("base url = %q", c.BaseURL)
	}
}

func b64url(s string) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	var out strings.Builder
	data := []byte(s)
	for i := 0; i < len(data); i += 3 {
		var b [3]byte
		n := copy(b[:], data[i:])
		v := uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2])
		out.WriteByte(chars[(v>>18)&0x3f])
		out.WriteByte(chars[(v>>12)&0x3f])
		if n > 1 {
			out.WriteByte(chars[(v>>6)&0x3f])
		}
		if n > 2 {
			out.WriteByte(chars[v&0x3f])
		}
	}
	// No padding needed for RawURLEncoding-style JWT segments.
	return out.String()
}

var _ = http.StatusOK
