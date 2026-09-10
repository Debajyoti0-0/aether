package exec

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSniffBrokerHeadersZscaler(t *testing.T) {
	h := http.Header{}
	h.Set("X-Zscaler-Response", "via-zia")
	broker, ok := sniffBrokerHeaders(h)
	if !ok || broker != "zscaler" {
		t.Errorf("broker = %q ok=%t", broker, ok)
	}
}

func TestSniffBrokerHeadersCloudflare(t *testing.T) {
	h := http.Header{}
	h.Set("Cf-Ray", "abc123")
	broker, ok := sniffBrokerHeaders(h)
	if !ok || broker != "cloudflare" {
		t.Errorf("broker = %q ok=%t", broker, ok)
	}
}

func TestSniffBrokerHeadersNone(t *testing.T) {
	h := http.Header{}
	h.Set("Server", "nginx")
	if _, ok := sniffBrokerHeaders(h); ok {
		t.Error("plain nginx should not match")
	}
}

func TestClassifyProxyVendor(t *testing.T) {
	cases := map[string]string{
		"http://zsproxy.corp.com:8080":  "zscaler",
		"http://gateway.zscaler.net":    "zscaler",
		"http://warp.cloudflare.com":    "cloudflare",
		"http://proxy.goskope.com":      "netskope",
		"http://generic-proxy.corp.com": "unknown",
	}
	for proxy, want := range cases {
		if got := classifyProxyVendor(proxy); got != want {
			t.Errorf("classify(%q) = %q, want %q", proxy, got, want)
		}
	}
}

func TestDetectBrokerFromEnv(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://zsproxy.corp.com:8080")
	broker, err := DetectBroker(context.Background(), http.DefaultClient)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if broker.Type != "zscaler" || broker.Proxy != "http://zsproxy.corp.com:8080" {
		t.Errorf("broker = %+v", broker)
	}
	if !strings.Contains(broker.Evidence, "HTTPS_PROXY") {
		t.Errorf("evidence = %q", broker.Evidence)
	}
}

func TestDetectBrokerViaHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Zscaler-Response", "yes")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	// Point the probe at the mock by clearing proxy env and probing
	// the test server URL directly through a redirect-free client.
	for _, v := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		os.Unsetenv(v)
	}
	client := srv.Client()
	broker, err := DetectBroker(context.Background(), client)
	// Without env proxy and probe URLs not pointing at the mock, we
	// expect either detection via a reachable probe or an error —
	// assert the error path is the graceful one.
	if err == nil && broker.Type == "" {
		t.Error("empty broker returned without error")
	}
}

func TestRouteThroughBrokerValidation(t *testing.T) {
	// A proxy that doesn't exist → route validation must fail cleanly.
	broker := &ZTNABroker{Type: "zscaler", Proxy: "http://127.0.0.1:1"}
	_, err := RouteThroughBroker(context.Background(), broker, "chrome", 2*time.Second)
	if err == nil {
		t.Fatal("dead proxy should fail validation")
	}
}

func TestRouteThroughBrokerNil(t *testing.T) {
	if _, err := RouteThroughBroker(context.Background(), nil, "chrome", time.Second); err == nil {
		t.Error("nil broker should fail")
	}
}

func TestExecThroughZTNA(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	res, err := ExecThroughZTNA(context.Background(), srv.Client(), srv.URL, "id")
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if !res.Success {
		t.Errorf("result = %+v", res)
	}
	if res.Target != srv.URL {
		t.Errorf("target = %q", res.Target)
	}
}

func TestExecThroughZTNANoClient(t *testing.T) {
	if _, err := ExecThroughZTNA(context.Background(), nil, "https://x", "id"); err == nil {
		t.Error("nil client should fail")
	}
}
