package intel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func kevJSON() string {
	return `{
  "title": "CISA Catalog of Known Exploited Vulnerabilities",
  "catalogVersion": "2026-09-01",
  "count": 2,
  "vulnerabilities": [
    {"cveID": "CVE-2026-1234", "vendorProject": "Microsoft", "product": "Entra ID", "dateAdded": "2026-08-01", "knownRansomwareUse": true},
    {"cveID": "CVE-2026-5678", "vendorProject": "AWS", "product": "SSM", "dateAdded": "2026-08-15"}
  ]
}`
}

func TestParseKEVAndLookup(t *testing.T) {
	c, err := parseKEV([]byte(kevJSON()), "test")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Len() != 2 {
		t.Fatalf("len = %d", c.Len())
	}
	if _, ok := c.IsKnownExploited("cve-2026-1234"); !ok {
		t.Error("case-insensitive lookup failed")
	}
	if _, ok := c.IsKnownExploited("CVE-9999-0000"); ok {
		t.Error("unknown CVE should not match")
	}
}

func TestLoadKEVFromURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(kevJSON()))
	}))
	defer srv.Close()

	c, err := LoadKEVFromURL(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.Len() != 2 {
		t.Errorf("len = %d", c.Len())
	}
}

func TestLoadKEVFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kev.json")
	os.WriteFile(path, []byte(kevJSON()), 0o600)

	c, err := LoadKEVFromFile(path)
	if err != nil {
		t.Fatalf("load file: %v", err)
	}
	if c.Len() != 2 {
		t.Errorf("len = %d", c.Len())
	}
}

func TestPrioritizeWithKEVBoost(t *testing.T) {
	c, _ := parseKEV([]byte(kevJSON()), "test")

	paths := [][]string{
		{"u1", "role-a", "vm-1"}, // exploits CVE-2026-1234 (KEV)
		{"u2", "role-b", "vm-2"}, // no KEV CVEs
	}
	cves := map[string][]string{
		"u1>role-a>vm-1": {"CVE-2026-1234"},
		"u2>role-b>vm-2": {"CVE-9999-0000"},
	}

	prioritized := Prioritize(paths, cves, c)
	if len(prioritized) != 2 {
		t.Fatalf("paths = %d", len(prioritized))
	}

	// KEV path gets +30% and sorts first.
	if prioritized[0].Path[0] != "u1" {
		t.Errorf("KEV path should sort first: %+v", prioritized)
	}
	if !prioritized[0].Critical {
		t.Error("KEV path should be critical")
	}
	if len(prioritized[0].KEVCVEs) != 1 || prioritized[0].KEVCVEs[0] != "CVE-2026-1234" {
		t.Errorf("kev cves = %v", prioritized[0].KEVCVEs)
	}
	boosted := 50.0 * 1.3 // base 50 for a 3-hop path × KEV boost
	if prioritized[0].Priority != boosted {
		t.Errorf("priority = %.1f, want %.1f", prioritized[0].Priority, boosted)
	}
	if prioritized[1].Critical {
		t.Error("non-KEV path should not be critical")
	}
}

func TestPrioritizeWithoutKEVCatalog(t *testing.T) {
	paths := [][]string{{"a", "b"}}
	out := Prioritize(paths, nil, nil)
	if len(out) != 1 || out[0].Critical {
		t.Errorf("out = %+v", out)
	}
}

func TestRenderPrioritized(t *testing.T) {
	out := RenderPrioritized([]PrioritizedPath{
		{Path: []string{"a", "b"}, Priority: 100, Critical: true},
	})
	if !strings.Contains(out, "KEV-CRITICAL") {
		t.Errorf("render = %q", out)
	}
}

func TestCacheKEVRoundTrip(t *testing.T) {
	c, _ := parseKEV([]byte(kevJSON()), "test")
	path := filepath.Join(t.TempDir(), "kev-cache.json")
	if err := CacheKEV(c, path); err != nil {
		t.Fatalf("cache: %v", err)
	}
	reloaded, err := LoadKEVFromFile(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Len() != 2 {
		t.Errorf("reloaded len = %d", reloaded.Len())
	}
	_ = json.RawMessage{}
}
