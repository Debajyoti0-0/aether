package api

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDashboardAuth(t *testing.T) {
	d, err := NewDashboard()
	if err != nil {
		t.Fatal(err)
	}
	d.Publish(DashboardEvent{Kind: "path_detected", Detail: "entra->aws", Risk: 20})
	d.Publish(DashboardEvent{Kind: "command_executed", Detail: "prt convert"})

	html, _ := buildTestGraphHTML()
	handler := d.Handler(html, map[string]int{"nodes": 2, "edges": 1})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	routes := []string{"/", "/api/graph", "/api/events", "/api/health"}

	// No token → 401 on every route.
	for _, route := range routes {
		resp, _ := http.Get(srv.URL + route)
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s without token = %d, want 401", route, resp.StatusCode)
		}
	}

	// Wrong token → 401.
	for _, route := range routes {
		req, _ := http.NewRequest("GET", srv.URL+route, nil)
		req.Header.Set("X-Aether-Token", "wrong-token")
		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s with wrong token = %d, want 401", route, resp.StatusCode)
		}
	}

	// Correct token via header → 200.
	for _, route := range routes {
		req, _ := http.NewRequest("GET", srv.URL+route, nil)
		req.Header.Set("X-Aether-Token", d.Token())
		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s with header token = %d, want 200", route, resp.StatusCode)
		}
	}

	// Query parameter must NOT authenticate anymore — the ?token= channel
	// was removed (query strings land in browser history and proxy logs).
	for _, route := range routes {
		sep := "?"
		if strings.Contains(route, "?") {
			sep = "&"
		}
		resp, _ := http.Get(srv.URL + route + sep + "token=" + d.Token())
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s with query token = %d, want 401 (query channel removed)", route, resp.StatusCode)
		}
	}

	// Correct token via Authorization: Bearer → 200.
	req, _ := http.NewRequest("GET", srv.URL+"/api/health", nil)
	req.Header.Set("Authorization", "Bearer "+d.Token())
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/api/health with bearer token = %d, want 200", resp.StatusCode)
	}

	// Graph JSON content with token.
	req, _ = http.NewRequest("GET", srv.URL+"/api/graph", nil)
	req.Header.Set("X-Aether-Token", d.Token())
	resp, _ = http.DefaultClient.Do(req)
	var graphOut map[string]int
	json.NewDecoder(resp.Body).Decode(&graphOut)
	resp.Body.Close()
	if graphOut["nodes"] != 2 {
		t.Errorf("graph = %v", graphOut)
	}
}

func TestDashboardEventCap(t *testing.T) {
	d, err := NewDashboard()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 250; i++ {
		d.Publish(DashboardEvent{Kind: "tick"})
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	if len(d.events) > 200 {
		t.Errorf("events = %d, want <= 200", len(d.events))
	}
}

func TestDashboardTokensUniquePerStart(t *testing.T) {
	d1, err := NewDashboard()
	if err != nil {
		t.Fatal(err)
	}
	d2, err := NewDashboard()
	if err != nil {
		t.Fatal(err)
	}
	if d1.Token() == "" || len(d1.Token()) != 64 {
		t.Fatalf("token length = %d, want 64 hex chars", len(d1.Token()))
	}
	if d1.Token() == d2.Token() {
		t.Fatal("two dashboards generated the same token")
	}
}

func TestIsLoopback(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1:8080":    true,
		"127.0.0.1":         true,
		"localhost:8080":    true,
		"localhost":         true,
		"[::1]:8080":        true,
		":8080":             false, // wildcard bind — all interfaces
		"0.0.0.0:8080":      false,
		"192.168.1.10:8080": false,
		"10.0.0.1:8080":     false,
		"[fe80::1]:8080":    false,
	} {
		if got := IsLoopback(addr); got != want {
			t.Errorf("IsLoopback(%q) = %t, want %t", addr, got, want)
		}
	}
}

// Non-loopback bind without TLS must be refused at the CLI layer; this
// test pins the api-level helper used for that decision.
func TestDashboardTLSListenerSmoke(t *testing.T) {
	// Sanity: ServeTLS with an invalid cert path must fail (fail closed),
	// proving TLS is actually engaged rather than silently skipped.
	d, err := NewDashboard()
	if err != nil {
		t.Fatal(err)
	}
	handler := d.Handler("<html></html>", map[string]int{})
	if err := d.ServeTLS("127.0.0.1:0", "does-not-exist.crt", "does-not-exist.key", handler); err == nil {
		t.Fatal("ServeTLS with missing cert = nil, want error")
	}
	_ = tls.VersionTLS12
}

func buildTestGraphHTML() (string, error) {
	return "<!DOCTYPE html><html><body>graph</body></html>", nil
}
