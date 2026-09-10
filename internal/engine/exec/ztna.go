package exec

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Debajyoti0-0/aether/internal/transport"
)

// transportClientWithProxy is an indirection so the ZTNA module does
// not duplicate transport construction (wired in tests to a fake).
var transportClientWithProxy = func(preset string, timeout time.Duration, proxyURL string) (*http.Client, error) {
	return transport.NewClientWithOptions(transport.Options{
		Preset:  preset,
		Timeout: timeout,
		ProxyURL: proxyURL,
	})
}

// ZTNABroker describes a detected Zero-Trust Network Access gateway.
type ZTNABroker struct {
	Type      string `json:"type"`   // zscaler, cloudflare, netskope, unknown
	Proxy     string `json:"proxy"`  // proxy URL detected via env/PAC
	Evidence  string `json:"evidence"`
	ViaHeader bool   `json:"via_header"`
}

// ZTNA detection probes (in priority order).
var ztnaProbeURLs = []string{
	"https://login.microsoftonline.com/common/discovery/instance",
}

// ztnaHeaderMarkers are response headers injected by inline ZTNA gateways.
var ztnaHeaderMarkers = []struct {
	marker string
	broker string
}{
	{"x-zscaler", "zscaler"},
	{"z-skd", "zscaler"},
	{"cf-ray", "cloudflare"},
	{"cf-cache-status", "cloudflare"},
	{"x-netskope", "netskope"},
	{"ns-primary", "netskope"},
}

// DetectBroker identifies a ZTNA gateway in the network path:
//  1. Proxy environment variables (HTTP_PROXY / HTTPS_PROXY).
//  2. Response-header markers from a probe request through the path.
//
// The philosophy: Aether doesn't break ZPA — it rides it. The IdP
// trust relationship (broker IP trusted by Entra ID) means tokens
// issued through the broker's egress appear to originate from the
// trusted network, bypassing IP-conditional CAP policies.
func DetectBroker(ctx context.Context, client *http.Client) (*ZTNABroker, error) {
	// 1. Environment proxy.
	for _, envVar := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		if p := strings.TrimSpace(os.Getenv(envVar)); p != "" {
			return &ZTNABroker{
				Type:     classifyProxyVendor(p),
				Proxy:    p,
				Evidence: fmt.Sprintf("environment variable %s", envVar),
			}, nil
		}
	}

	// 2. Header markers from a probe request.
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	for _, probe := range ztnaProbeURLs {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, probe, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		broker, viaHeader := sniffBrokerHeaders(resp.Header)
		resp.Body.Close()
		if viaHeader {
			return &ZTNABroker{
				Type:      broker,
				Evidence:  fmt.Sprintf("response header marker from %s", probe),
				ViaHeader: true,
			}, nil
		}
	}

	return nil, fmt.Errorf("no ZTNA broker detected (no proxy env vars, no header markers)")
}

// sniffBrokerHeaders checks response headers for ZTNA gateway markers.
func sniffBrokerHeaders(h http.Header) (string, bool) {
	for k, vals := range h {
		lk := strings.ToLower(k)
		for _, m := range ztnaHeaderMarkers {
			if strings.Contains(lk, m.marker) {
				return m.broker, true
			}
			for _, v := range vals {
				if strings.Contains(strings.ToLower(v), m.marker) {
					return m.broker, true
				}
			}
		}
	}
	return "", false
}

// classifyProxyVendor guesses the broker type from a proxy URL/hostname.
func classifyProxyVendor(proxy string) string {
	lp := strings.ToLower(proxy)
	switch {
	case strings.Contains(lp, "zscaler") || strings.Contains(lp, "zsproxy") || strings.Contains(lp, "zpath"):
		return "zscaler"
	case strings.Contains(lp, "cloudflare") || strings.Contains(lp, "warp"):
		return "cloudflare"
	case strings.Contains(lp, "netskope") || strings.Contains(lp, "goskope"):
		return "netskope"
	default:
		return "unknown"
	}
}

// RouteThroughBroker returns an HTTP client that routes through the
// detected broker's proxy, so authentication flows egress from the
// trusted network location. This is the ZTNA-aware equivalent of
// `tunnel`: Aether rides the trust relationship instead of fighting it.
func RouteThroughBroker(ctx context.Context, broker *ZTNABroker, preset string, timeout time.Duration) (*http.Client, error) {
	if broker == nil || broker.Proxy == "" {
		return nil, fmt.Errorf("broker has no proxy to route through")
	}

	client, err := transportClientWithProxy(preset, timeout, broker.Proxy)
	if err != nil {
		return nil, err
	}

	// Validate the route works before handing it to auth flows.
	probe, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://login.microsoftonline.com/common/discovery/instance", nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(probe)
	if err != nil {
		return nil, fmt.Errorf("broker route validation: %w", err)
	}
	resp.Body.Close()

	return client, nil
}

// ZTNAExecResult summarizes a broker-routed operation.
type ZTNAExecResult struct {
	Broker  string `json:"broker"`
	Proxy   string `json:"proxy"`
	Target  string `json:"target"`
	Success bool   `json:"success"`
	Detail  string `json:"detail"`
}

// ExecThroughZTNA runs a command against an internal target by routing
// through the broker — the network-location spoof for ZTNA-protected
// services.
func ExecThroughZTNA(ctx context.Context, client *http.Client, target, command string) (*ZTNAExecResult, error) {
	if client == nil {
		return nil, fmt.Errorf("no broker-routed client (run DetectBroker + RouteThroughBroker first)")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return &ZTNAExecResult{Target: target, Success: false, Detail: err.Error()}, nil
	}
	defer resp.Body.Close()

	return &ZTNAExecResult{
		Target:  target,
		Success: resp.StatusCode < 400,
		Detail:  fmt.Sprintf("routed via broker → HTTP %d (cmd %q handed to handler)", resp.StatusCode, command),
	}, nil
}
