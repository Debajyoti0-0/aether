package transport

import (
	"net"
	"net/http"
	"time"
)

// NewUTLSClient creates an HTTP client whose TLS handshake mimics the
// given browser preset. ALPN negotiates HTTP/2 when the server supports it.
func NewUTLSClient(preset BrowserPreset, timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	dialer := &TLSDialer{Preset: preset, Timeout: timeout}
	tr := &http.Transport{
		DialTLSContext:    dialer.DialTLSContext,
		DialContext:       (&net.Dialer{Timeout: timeout}).DialContext,
		ForceAttemptHTTP2: true,
		MaxIdleConns:      100,
		IdleConnTimeout:   90 * time.Second,
		TLSHandshakeTimeout: 15 * time.Second,
	}

	return &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}
}

// NewClient builds the default aether HTTP client from a preset string.
func NewClient(preset string, timeout time.Duration) (*http.Client, error) {
	p, err := ParsePreset(preset)
	if err != nil {
		return nil, err
	}
	return NewUTLSClient(p, timeout), nil
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
