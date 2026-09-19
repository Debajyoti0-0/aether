package transport

import (
	"context"
	"fmt"
	"net"
	"time"

	utls "github.com/refraction-networking/utls"
)

// BrowserPreset defines which browser to impersonate at the TLS layer.
type BrowserPreset string

const (
	Chrome  BrowserPreset = "chrome"
	Edge    BrowserPreset = "edge"
	Firefox BrowserPreset = "firefox"
)

// ParsePreset maps a user-supplied string to a BrowserPreset.
func ParsePreset(s string) (BrowserPreset, error) {
	switch BrowserPreset(s) {
	case Chrome, Edge, Firefox:
		return BrowserPreset(s), nil
	default:
		return "", fmt.Errorf("unknown browser preset %q (want chrome, edge, firefox)", s)
	}
}

func clientHelloSpecFromID(id utls.ClientHelloID) utls.ClientHelloSpec {
	spec, err := utls.UTLSIdToSpec(id)
	if err != nil {
		// Fallback to Chrome spec if conversion fails
		spec, _ = utls.UTLSIdToSpec(utls.HelloChrome_Auto)
	}
	// Override ALPN to http/1.1 only (F-40-1: prevent h2 negotiation
	// mismatch with HTTP/1.1 transport).
	for i := range spec.Extensions {
		if alpn, ok := spec.Extensions[i].(*utls.ALPNExtension); ok {
			alpn.AlpnProtocols = []string{"http/1.1"}
			break
		}
	}
	return spec
}

// TLSDialer dials TLS connections with a spoofed browser fingerprint
// (JA3/JA4) using uTLS. It satisfies http.Transport's DialTLSContext.
// When Pool is set, the fingerprint rotates per connection instead of
// using the fixed Preset.
type TLSDialer struct {
	Preset   BrowserPreset
	Insecure bool
	Timeout  time.Duration
	Pool     *JA4Pool
}

// DialTLSContext dials addr and performs a uTLS handshake.
func (d *TLSDialer) DialTLSContext(ctx context.Context, network, addr string) (net.Conn, error) {
	timeout := d.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}

	raw, err := (&net.Dialer{Timeout: timeout}).DialContext(ctx, network, addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		raw.Close()
		return nil, fmt.Errorf("split host:port %s: %w", addr, err)
	}

	cfg := &utls.Config{
		ServerName:         host,
		InsecureSkipVerify: d.Insecure,
		// ALPN http/1.1 only: the consumer of this dialer is
		// http.Transport (HTTP/1.1). Negotiating h2 here mismatches
		// the protocol the Transport speaks (h2-only endpoints reply
		// with SETTINGS frames the h1 parser rejects).
		NextProtos: []string{"http/1.1"},
	}

	var baseID utls.ClientHelloID
	if d.Pool != nil {
		baseID = d.Pool.Next()
	} else {
		switch d.Preset {
		case Edge:
			baseID = utls.HelloEdge_Auto
		case Firefox:
			baseID = utls.HelloFirefox_Auto
		default:
			baseID = utls.HelloChrome_Auto
		}
	}

	// Get the browser fingerprint spec, override ALPN to http/1.1 only
	// (F-40-1: prevent h2 negotiation mismatch with HTTP/1.1 transport).
	spec := clientHelloSpecFromID(baseID)

	// Use HelloCustom + ApplyPreset to apply the modified spec while
	// preserving the browser fingerprint (JA3/JA4).
	conn := utls.UClient(raw, cfg, utls.HelloCustom)
	if err := conn.ApplyPreset(&spec); err != nil {
		raw.Close()
		return nil, fmt.Errorf("apply preset for %s: %w", addr, err)
	}
	if err := conn.HandshakeContext(ctx); err != nil {
		raw.Close()
		return nil, fmt.Errorf("tls handshake %s: %w", addr, err)
	}
	return conn, nil
}

// presetFromHello maps a rotation profile back to the dialer's preset
// switch. Rotation profiles are a superset; unknown profiles map to
// Chrome (the safest default that still works against Entra).
func presetFromHello(hello utls.ClientHelloID) BrowserPreset {
	switch hello {
	case utls.HelloEdge_Auto:
		return Edge
	case utls.HelloFirefox_Auto:
		return Firefox
	default:
		return Chrome
	}
}
