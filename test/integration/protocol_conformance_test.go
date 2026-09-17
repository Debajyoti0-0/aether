//go:build integration

// Stage 5 backfill protocol conformance fixtures (B5-G09): 32 fixtures
// across the six implemented protocol families — Kerberos (12), SAML
// (8), OAuth2 (4), WS-Trust (2), MS-OAPX (3), IMDS (3) — each against
// the real package code with positive and negative cases. No real
// cloud services: all network fixtures use local httptest servers.
package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	aexec "github.com/Debajyoti0-0/aether/internal/engine/exec"
	"github.com/Debajyoti0-0/aether/internal/engine/pivot"
	"github.com/Debajyoti0-0/aether/internal/protocol/msoapx"
	"github.com/Debajyoti0-0/aether/internal/protocol/oauth2"
	"github.com/Debajyoti0-0/aether/internal/protocol/saml"
	"github.com/Debajyoti0-0/aether/internal/protocol/wstrust"
)

func TestProtocolConformance(t *testing.T) {
	t.Run("Kerberos", conformanceKerberos)
	t.Run("SAML", conformanceSAML)
	t.Run("OAuth2", conformanceOAuth2)
	t.Run("WS-Trust", conformanceWSTrust)
	t.Run("MS-OAPX", conformanceMSOAPX)
	t.Run("IMDS", conformanceIMDS)
}

// --- Kerberos (12 fixtures): ccache FILE format + MS-KKDCP proxy ---

func kerberosFixture(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "krb5cc_test")
	info := &pivot.ASREPInfo{
		CRealm:    "CONTOSO.LOCAL",
		TicketDER: []byte{0x61, 0x82, 0x01, 0x00, 0xDE, 0xAD, 0xBE, 0xEF},
		EncPart:   []byte{0xAA, 0xBB},
		CName:     []byte{0x30},
	}
	if err := pivot.WriteCcache(path, info, "alice@CONTOSO.LOCAL"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// decodeU32 reads a big-endian u32 at off (format check helper).
func decodeU32(t *testing.T, raw []byte, off int) uint32 {
	t.Helper()
	if off+4 > len(raw) {
		t.Fatalf("u32 at %d out of range (len %d)", off, len(raw))
	}
	return binary.BigEndian.Uint32(raw[off : off+4])
}

func conformanceKerberos(t *testing.T) {
	t.Run("fixture01_ccache_magic_0504", func(t *testing.T) {
		raw := kerberosFixture(t)
		if raw[0] != 0x05 || raw[1] != 0x04 {
			t.Fatalf("magic = %#x %#x", raw[0], raw[1])
		}
	})
	t.Run("fixture02_ccache_version_zero", func(t *testing.T) {
		raw := kerberosFixture(t)
		if got := decodeU32(t, raw, 2); got != 0 {
			t.Fatalf("version = %d", got)
		}
	})
	t.Run("fixture03_ccache_no_header_tags", func(t *testing.T) {
		raw := kerberosFixture(t)
		if got := decodeU32(t, raw, 6); got != 0 {
			t.Fatalf("tag len = %d", got)
		}
	})
	t.Run("fixture04_ccache_primary_realm", func(t *testing.T) {
		raw := kerberosFixture(t)
		n := int(decodeU32(t, raw, 10))
		if got := string(raw[14 : 14+n]); got != "CONTOSO.LOCAL" {
			t.Fatalf("realm = %q", got)
		}
	})
	t.Run("fixture05_ccache_primary_components", func(t *testing.T) {
		raw := kerberosFixture(t)
		n := int(decodeU32(t, raw, 10))
		off := 14 + n
		parts := int(decodeU32(t, raw, off))
		off += 4
		if parts != 1 {
			t.Fatalf("parts = %d", parts)
		}
		ln := int(decodeU32(t, raw, off))
		off += 4
		if got := string(raw[off : off+ln]); got != "alice" {
			t.Fatalf("component = %q", got)
		}
	})
	t.Run("fixture06_ccache_ticket_der_embedded", func(t *testing.T) {
		raw := kerberosFixture(t)
		if !bytes.Contains(raw, []byte{0xDE, 0xAD, 0xBE, 0xEF}) {
			t.Fatal("ticket DER not embedded")
		}
	})
	t.Run("fixture07_ccache_krbtgt_server_principal", func(t *testing.T) {
		raw := kerberosFixture(t)
		if !bytes.Contains(raw, []byte("krbtgt")) {
			t.Fatal("krbtgt server principal missing")
		}
	})
	t.Run("fixture08_ccache_negative_nil_ticket", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "neg8")
		err := pivot.WriteCcache(path, &pivot.ASREPInfo{CRealm: "R"}, "a@R")
		if err == nil {
			t.Fatal("nil TicketDER accepted")
		}
	})
	t.Run("fixture09_ccache_negative_nil_info", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "neg9")
		if err := pivot.WriteCcache(path, nil, "a@R"); err == nil {
			t.Fatal("nil info accepted")
		}
	})
	t.Run("fixture10_kkdcp_roundtrip", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body := make([]byte, r.ContentLength)
			_, _ = readFull(r, body)
			if len(body) < 4 {
				http.Error(w, "short", http.StatusBadRequest)
				return
			}
			declared := binary.BigEndian.Uint32(body[:4])
			if int(declared) != len(body)-4 {
				http.Error(w, "framing mismatch", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte("KDC-REP-OK"))
		}))
		defer srv.Close()
		c := pivot.NewKKDCPClient(srv.URL, "CONTOSO.LOCAL", srv.Client())
		reply, err := c.Send(context.Background(), []byte{0x6A, 0x00, 0x01})
		if err != nil {
			t.Fatalf("send: %v", err)
		}
		if string(reply) != "KDC-REP-OK" {
			t.Fatalf("reply = %q", reply)
		}
	})
	t.Run("fixture11_kkdcp_negative_http_500", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "kdc overloaded", http.StatusInternalServerError)
		}))
		defer srv.Close()
		c := pivot.NewKKDCPClient(srv.URL, "CONTOSO.LOCAL", srv.Client())
		if _, err := c.Send(context.Background(), []byte{0x6A}); err == nil {
			t.Fatal("HTTP 500 accepted by KKDCP client")
		}
	})
	t.Run("fixture12_kkdcp_short_reply_passthrough", func(t *testing.T) {
		// Documented tolerance (unwrapKDCProxyMessage): a reply that is
		// too short to carry KDC-proxy framing (or whose declared length
		// mismatches) is passed through as the raw KDC reply rather than
		// rejected — proxies that skip framing still work.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte{0x6A, 0x6B})
		}))
		defer srv.Close()
		c := pivot.NewKKDCPClient(srv.URL, "CONTOSO.LOCAL", srv.Client())
		reply, err := c.Send(context.Background(), []byte{0x6A})
		if err != nil {
			t.Fatalf("send: %v", err)
		}
		if !bytes.Equal(reply, []byte{0x6A, 0x6B}) {
			t.Fatalf("reply = %v, want raw passthrough", reply)
		}
	})
}

func readFull(r *http.Request, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := r.Body.Read(buf[total:])
		total += n
		if err != nil {
			if total == len(buf) {
				break
			}
			return total, err
		}
	}
	return total, nil
}

// --- SAML (8 fixtures): build, parse, XML-DSig digest, signature strip ---

func conformanceSAML(t *testing.T) {
	t.Run("fixture13_build_valid_assertion", func(t *testing.T) {
		a, err := saml.NewBuilder("https://sts.example.com").Build(saml.BuildOptions{
			Subject: "user@corp", Audience: "api://vault", Recipient: "https://vault/acs",
			Attributes: []saml.Attribute{{Name: "groups", Values: []string{"devs"}}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if a.ID == "" || a.Issuer != "https://sts.example.com" || a.Audience != "api://vault" {
			t.Fatalf("assertion = %+v", a)
		}
		if !strings.Contains(a.Raw, "user@corp") {
			t.Fatal("subject not in raw XML")
		}
	})
	t.Run("fixture14_build_negative_missing_subject", func(t *testing.T) {
		if _, err := saml.NewBuilder("https://sts.example.com").Build(saml.BuildOptions{}); err == nil {
			t.Fatal("empty subject accepted")
		}
	})
	t.Run("fixture15_parse_roundtrip", func(t *testing.T) {
		a, err := saml.NewBuilder("https://sts.example.com").Build(saml.BuildOptions{
			Subject: "user@corp", Audience: "api://vault",
		})
		if err != nil {
			t.Fatal(err)
		}
		p, err := saml.ParseAssertion(a.Raw)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if p.ID != a.ID || p.Issuer != a.Issuer || p.Audience != a.Audience {
			t.Fatalf("roundtrip mismatch: %+v vs %+v", p, a)
		}
	})
	t.Run("fixture16_parse_negative_malformed_xml", func(t *testing.T) {
		if _, err := saml.ParseAssertion("<Assertion><Unclosed>"); err == nil {
			t.Fatal("malformed XML accepted")
		}
	})
	t.Run("fixture17_parse_negative_empty", func(t *testing.T) {
		if _, err := saml.ParseAssertion(""); err == nil {
			t.Fatal("empty document accepted")
		}
	})
	t.Run("fixture18_signature_verify_positive", func(t *testing.T) {
		key, _ := rsa.GenerateKey(rand.Reader, 2048)
		signer, err := saml.NewSigner(x509.MarshalPKCS1PrivateKey(key), "")
		if err != nil {
			t.Fatal(err)
		}
		canonical := []byte("<Assertion><Subject>user@corp</Subject></Assertion>")
		sig, _, err := signer.SignDigest(canonical)
		if err != nil {
			t.Fatal(err)
		}
		if err := saml.VerifyDigest(&key.PublicKey, canonical, sig); err != nil {
			t.Fatalf("verify: %v", err)
		}
	})
	t.Run("fixture19_signature_negative_tampered", func(t *testing.T) {
		key, _ := rsa.GenerateKey(rand.Reader, 2048)
		signer, _ := saml.NewSigner(x509.MarshalPKCS1PrivateKey(key), "")
		canonical := []byte("<Assertion><Subject>user@corp</Subject></Assertion>")
		sig, _, err := signer.SignDigest(canonical)
		if err != nil {
			t.Fatal(err)
		}
		tampered := []byte("<Assertion><Subject>admin@corp</Subject></Assertion>")
		if err := saml.VerifyDigest(&key.PublicKey, tampered, sig); err == nil {
			t.Fatal("tampered canonical XML verified")
		}
	})
	t.Run("fixture20_strip_signature", func(t *testing.T) {
		doc := "<Assertion><Issuer>sts</Issuer>" +
			`<ds:Signature xmlns:ds="http://www.w3.org/2000/09/xmldsig#"><ds:SignedInfo/><ds:SignatureValue>AAA=</ds:SignatureValue></ds:Signature>` +
			"<Subject>u</Subject></Assertion>"
		res, err := saml.StripDocument(doc)
		if err != nil {
			t.Fatalf("strip: %v", err)
		}
		if res == nil || res.Stripped != 1 || strings.Contains(res.Doc, "SignatureValue") || !res.HasAssertion {
			t.Fatalf("strip result = %+v", res)
		}
	})
}

// --- OAuth2 (4 fixtures): CAE claims challenge + client error paths ---

func conformanceOAuth2(t *testing.T) {
	t.Run("fixture21_cae_challenge_positive", func(t *testing.T) {
		claims := map[string]any{"access_token": map[string]any{"acrs": "c1"}}
		raw, _ := json.Marshal(claims)
		chal := base64.RawURLEncoding.EncodeToString(raw)
		resp := &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     http.Header{},
		}
		resp.Header.Set(oauth2.CAEChallengeHeader,
			`Bearer authorization_uri="https://login.example.com", error="insufficient_claims", claims="`+chal+`"`)
		got, found, err := oauth2.ClaimsChallenge(resp)
		if err != nil || !found {
			t.Fatalf("found=%v err=%v", found, err)
		}
		if _, ok := got["access_token"]; !ok {
			t.Fatalf("claims = %v", got)
		}
	})
	t.Run("fixture22_cae_negative_no_challenge", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusUnauthorized, Header: http.Header{}}
		_, found, err := oauth2.ClaimsChallenge(resp)
		if found || err != nil {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})
	t.Run("fixture23_cae_negative_malformed_claims", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusUnauthorized, Header: http.Header{}}
		// claims must lead its comma-separated segment (RFC 6750 shape).
		resp.Header.Set(oauth2.CAEChallengeHeader,
			`Bearer authorization_uri="https://login.example.com", claims="%%%not-base64%%%"`)
		_, found, err := oauth2.ClaimsChallenge(resp)
		if !found || err == nil {
			t.Fatalf("malformed claims accepted: found=%v err=%v", found, err)
		}
	})
	t.Run("fixture24_client_negative_invalid_credentials", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_client","error_description":"bad secret"}`))
		}))
		defer srv.Close()
		c := oauth2.NewClientWithHTTP(srv.URL, srv.Client())
		if _, err := c.ClientCredentials(context.Background(), "t", "id", "wrong-secret", "https://management.azure.com/.default"); err == nil {
			t.Fatal("invalid_client accepted by OAuth2 client")
		}
	})
}

// --- WS-Trust (2 fixtures): RST construction + MEX/downgrade detection ---

func conformanceWSTrust(t *testing.T) {
	t.Run("fixture25_rst_construction", func(t *testing.T) {
		rst := wstrust.BuildRST("alice@corp", "synthetic-pass", "https://vault.example.com/issue", "https://sts.example.com/adfs/services/trust/2005/usernamemixed")
		s := string(rst)
		for _, want := range []string{"Envelope", "alice@corp", "https://vault.example.com/issue", "RequestSecurityToken"} {
			if !strings.Contains(s, want) {
				t.Fatalf("RST missing %q", want)
			}
		}
	})
	t.Run("fixture26_mex_downgrade_detection", func(t *testing.T) {
		mex := `<wsdl:definitions xmlns:wsdl="http://schemas.xmlsoap.org/wsdl/" xmlns:soap12="http://schemas.xmlsoap.org/wsdl/soap12/">` +
			`<wsdl:service name="SecurityTokenService"><wsdl:port name="CustomBinding_IWSTrust13Async1">` +
			`<soap12:address location="https://sts.example.com/adfs/services/trust/2005/usernamemixed"/>` +
			`</wsdl:port></wsdl:service></wsdl:definitions>`
		endpoint, feasible, err := wstrust.DetectDowngradeOpportunity([]byte(mex))
		if err != nil {
			t.Fatalf("detect: %v", err)
		}
		if !feasible || !strings.Contains(endpoint, "usernamemixed") {
			t.Fatalf("endpoint=%q feasible=%v", endpoint, feasible)
		}
		// Negative: garbage MEX must not yield a feasible endpoint.
		if _, feasible2, err2 := wstrust.DetectDowngradeOpportunity([]byte("<<<not-xml>>>")); err2 == nil && feasible2 {
			t.Fatal("garbage MEX flagged as downgrade-feasible")
		}
	})
}

// --- MS-OAPX (3 fixtures): channel binding / token protection ---

func conformanceMSOAPX(t *testing.T) {
	t.Run("fixture27_channel_binding_base64", func(t *testing.T) {
		raw := make([]byte, 32)
		for i := range raw {
			raw[i] = byte(i)
		}
		cb, err := msoapx.LoadChannelBinding(base64.StdEncoding.EncodeToString(raw))
		if err != nil {
			t.Fatal(err)
		}
		if len(cb.FinishedMessage) != 32 || cb.HashAlg != "SHA256" {
			t.Fatalf("binding = %+v", cb)
		}
		// Deterministic header generation.
		h1, h2 := cb.GenerateHeader(), cb.GenerateHeader()
		if h1 == "" || h1 != h2 {
			t.Fatalf("header nondeterministic/empty: %q vs %q", h1, h2)
		}
	})
	t.Run("fixture28_channel_binding_hex", func(t *testing.T) {
		cb, err := msoapx.LoadChannelBinding("0011ff0a")
		if err != nil {
			t.Fatal(err)
		}
		if got := cb.FinishedMessage; len(got) != 4 || got[0] != 0x00 || got[3] != 0x0a {
			t.Fatalf("hex decode = %v", got)
		}
	})
	t.Run("fixture29_channel_binding_negative", func(t *testing.T) {
		if _, err := msoapx.LoadChannelBinding(""); err == nil {
			t.Fatal("empty binding accepted")
		}
		if _, err := msoapx.LoadChannelBinding("%%%not-an-encoding%%%"); err == nil {
			t.Fatal("invalid encoding accepted")
		}
	})
}

// --- IMDS (3 fixtures): token acquisition + identity hijack (local fake) ---

func conformanceIMDS(t *testing.T) {
	t.Run("fixture30_fetch_token_positive", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut || r.Header.Get("Metadata") != "true" {
				http.Error(w, "missing metadata header", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte("synthetic-imds-session-token"))
		}))
		defer srv.Close()
		c := aexec.NewIMDSClientAt(srv.URL, srv.Client())
		if err := c.FetchToken(context.Background(), 300); err != nil {
			t.Fatal(err)
		}
		if c.Token != "synthetic-imds-session-token" {
			t.Fatalf("token = %q", c.Token)
		}
	})
	t.Run("fixture31_fetch_token_negative_403", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "forbidden", http.StatusForbidden)
		}))
		defer srv.Close()
		c := aexec.NewIMDSClientAt(srv.URL, srv.Client())
		if err := c.FetchToken(context.Background(), 300); err == nil || !strings.Contains(err.Error(), "403") {
			t.Fatalf("err = %v, want http 403", err)
		}
	})
	t.Run("fixture32_identity_token_positive_and_malformed", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("resource") == "https://management.azure.com/" {
				_, _ = w.Write([]byte(`{"access_token":"synthetic-mi-token","token_type":"Bearer","expires_in":"3600","resource":"https://management.azure.com/"}`))
				return
			}
			_, _ = w.Write([]byte("{not-json"))
		}))
		defer srv.Close()
		c := aexec.NewIMDSClientAt(srv.URL, srv.Client())
		tok, err := c.GetIdentityToken(context.Background(), "https://management.azure.com/", "", "")
		if err != nil {
			t.Fatal(err)
		}
		if tok.AccessToken != "synthetic-mi-token" || tok.TokenType != "Bearer" || tok.ExpiresIn != 3600 {
			t.Fatalf("tokens = %+v", tok)
		}
		// Negative: malformed identity response is rejected.
		if _, err := c.GetIdentityToken(context.Background(), "https://other.example.com/", "", ""); err == nil {
			t.Fatal("malformed identity response accepted")
		}
	})
}




