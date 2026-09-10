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

func clientHelloID(p BrowserPreset) utls.ClientHelloID {
	switch p {
	case Edge:
		return utls.HelloEdge_Auto
	case Firefox:
		return utls.HelloFirefox_Auto
	default:
		return utls.HelloChrome_Auto
	}
}

// TLSDialer dials TLS connections with a spoofed browser fingerprint
// (JA3/JA4) using uTLS. It satisfies http.Transport's DialTLSContext.
type TLSDialer struct {
	Preset   BrowserPreset
	Insecure bool
	Timeout  time.Duration
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
		NextProtos:         []string{"h2", "http/1.1"},
	}

	conn := utls.UClient(raw, cfg, clientHelloID(d.Preset))
	if err := conn.HandshakeContext(ctx); err != nil {
		raw.Close()
		return nil, fmt.Errorf("tls handshake %s: %w", addr, err)
	}
	return conn, nil
}
