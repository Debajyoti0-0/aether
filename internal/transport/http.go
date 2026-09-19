package transport

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// NewUTLSClient creates an HTTP client whose TLS handshake mimics the
// given browser preset. HTTP/2 is disabled: the uTLS dialer negotiates
// http/1.1 only (see TLSDialer.DialTLSContext).
func NewUTLSClient(preset BrowserPreset, timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	dialer := &TLSDialer{Preset: preset, Timeout: timeout}
	tr := &http.Transport{
		DialTLSContext:    dialer.DialTLSContext,
		DialContext:       (&net.Dialer{Timeout: timeout}).DialContext,
		ForceAttemptHTTP2: false,
		MaxIdleConns:      100,
		IdleConnTimeout:   90 * time.Second,
		TLSHandshakeTimeout: 15 * time.Second,
	}

	return &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}
}

// NewUTLSClientWithPool creates an HTTP client whose TLS fingerprint
// rotates per connection from the supplied JA4 pool. HTTP/2 is disabled:
// the uTLS dialer negotiates http/1.1 only (see TLSDialer.DialTLSContext).
func NewUTLSClientWithPool(pool *JA4Pool, timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	dialer := &TLSDialer{Pool: pool, Timeout: timeout}
	tr := &http.Transport{
		DialTLSContext:    dialer.DialTLSContext,
		DialContext:       (&net.Dialer{Timeout: timeout}).DialContext,
		ForceAttemptHTTP2: false,
		MaxIdleConns:      100,
		IdleConnTimeout:   90 * time.Second,
		TLSHandshakeTimeout: 15 * time.Second,
	}

	return &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}
}

// Options configures client construction beyond the preset.
type Options struct {
	Preset    string
	Timeout   time.Duration
	// ProxyURL routes all traffic through an HTTP/SOCKS5 forward proxy
	// (e.g. socks5://127.0.0.1:1080 or http://proxy:8080).
	ProxyURL  string
	// RotateJA4, when true, rotates TLS fingerprints per connection.
	RotateJA4 bool
}

// NewClientWithOptions builds a client honoring proxy + rotation options.
func NewClientWithOptions(opts Options) (*http.Client, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}

	dialer := &TLSDialer{Timeout: opts.Timeout}
	if opts.RotateJA4 {
		dialer.Pool = NewJA4Profiles()
	} else {
		preset, err := ParsePreset(opts.Preset)
		if err != nil {
			return nil, err
		}
		dialer.Preset = preset
	}

	tr := &http.Transport{
		DialTLSContext:      dialer.DialTLSContext,
		DialContext:         (&net.Dialer{Timeout: opts.Timeout}).DialContext,
		ForceAttemptHTTP2:   false,
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 15 * time.Second,
	}

	if opts.ProxyURL != "" {
		u, err := url.Parse(opts.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("parse proxy url: %w", err)
		}
		switch strings.ToLower(u.Scheme) {
		case "http", "https", "socks5", "socks5h":
			tr.Proxy = http.ProxyURL(u)
		default:
			return nil, fmt.Errorf("unsupported proxy scheme %q (want http, https, socks5)", u.Scheme)
		}
	}

	return &http.Client{Transport: tr, Timeout: opts.Timeout}, nil
}

// NewClient builds the default aether HTTP client from a preset string.
func NewClient(preset string, timeout time.Duration) (*http.Client, error) {
	return NewClientWithOptions(Options{Preset: preset, Timeout: timeout})
}

// NewRequest is a small helper that adds JSON content type headers.
func NewRequest(method, url string, body interface {
	Read([]byte) (int, error)
}) *http.Request {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil
	}
	return req
}
