package transport

import (
	utls "github.com/refraction-networking/utls"

	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// baseIDForPreset mirrors the fingerprint selection in DialTLSContext for
// fixed-preset dialers (Pool == nil).
func baseIDForPreset(p BrowserPreset) utls.ClientHelloID {
	switch p {
	case Edge:
		return utls.HelloEdge_Auto
	case Firefox:
		return utls.HelloFirefox_Auto
	default:
		return utls.HelloChrome_Auto
	}
}

// Stage 40-R (F-40-1 reconciliation) — the dual-offer class must be pinned:
// against a dual-ALPN server (h2 + http/1.1 — exactly what the _Auto
// profiles would negotiate if ALPN were left unmodified), the TLSDialer
// must deliver an HTTP/1.1 conn to its h1 caller. This is the wire-level
// failure the external report found, pinned network-free.
func TestALPNDualOfferDeliversHTTP1(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"proto": r.Proto})
	})
	srv := httptest.NewUnstartedServer(mux)
	srv.EnableHTTP2 = true // TLS NextProtos [h2, http/1.1], speaks both
	srv.StartTLS()
	defer srv.Close()

	d := &TLSDialer{Preset: Chrome, Timeout: 10 * time.Second, Insecure: true}
	tr := &http.Transport{DialTLSContext: d.DialTLSContext}
	client := &http.Client{Transport: tr, Timeout: 15 * time.Second}

	resp, err := client.Get(srv.URL + "/echo")
	if err != nil {
		t.Fatalf("live-class dispatch through TLSDialer FAILED: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
	if resp.ProtoMajor != 1 {
		t.Fatalf("conn must be delivered as HTTP/1.1 to the h1 caller; got %s (body %q)", resp.Proto, string(body))
	}
	var out map[string]string
	_ = json.Unmarshal(body, &out)
	if out["proto"] != "HTTP/1.1" {
		t.Fatalf("server saw %q, want HTTP/1.1", out["proto"])
	}
}

// Stage 40-R (F-40-1 invariants) — the spec the dialer applies must offer
// http/1.1 only, for every selected fingerprint: the contradictory class
// (client speaks h1 but offers h2) must be impossible by config.
func TestALPNSpecOffersHTTP1Only(t *testing.T) {
	for _, preset := range []BrowserPreset{Chrome, Edge, Firefox} {
		spec := clientHelloSpecFromID(baseIDForPreset(preset))
		var found bool
		for _, ext := range spec.Extensions {
			if alpn, ok := ext.(*utls.ALPNExtension); ok {
				found = true
				if len(alpn.AlpnProtocols) != 1 || alpn.AlpnProtocols[0] != "http/1.1" {
					t.Fatalf("%s: ALPN offer = %v, want [http/1.1]", preset, alpn.AlpnProtocols)
				}
			}
		}
		if !found {
			t.Fatalf("%s: selected fingerprint has no ALPN extension", preset)
		}
	}
}
