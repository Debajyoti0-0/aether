package plugins

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testIndex() Index {
	artifact, _ := json.Marshal(map[string]string{"type": "provider", "provider": "gcp"})
	sum := sha256.Sum256(artifact)
	return Index{
		Updated: "2026-09-09",
		Plugins: []PluginManifest{
			{Name: "aether-gcp", Version: "1.0.0", Provider: "gcp",
				Description: "Google Cloud execution + IAM discovery",
				DownloadURL: "/artifacts/aether-gcp.json", SHA256: hex.EncodeToString(sum[:])},
			{Name: "aether-vmware", Version: "0.9.0", Provider: "vmware",
				Description: "VMware Cloud SDDC operations",
				DownloadURL: "/artifacts/aether-vmware.json"},
		},
	}
}

func TestRemoteRegistrySearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(testIndex())
	}))
	defer srv.Close()

	reg := NewRemoteRegistry(srv.URL, filepath.Join(t.TempDir(), "plugins"))

	all, err := reg.Search(context.Background(), "")
	if err != nil || len(all) != 2 {
		t.Fatalf("search all = %d err=%v", len(all), err)
	}

	gcp, err := reg.Search(context.Background(), "gcp")
	if err != nil || len(gcp) != 1 || gcp[0].Name != "aether-gcp" {
		t.Fatalf("search gcp = %+v err=%v", gcp, err)
	}
}

func TestRemoteRegistryInstallVerified(t *testing.T) {
	artifact, _ := json.Marshal(map[string]string{"type": "provider", "provider": "gcp"})
	sum := sha256.Sum256(artifact)
	want := hex.EncodeToString(sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/index.json":
			json.NewEncoder(w).Encode(testIndex())
		case r.URL.Path == "/artifacts/aether-gcp.json":
			w.Write(artifact)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	dir := filepath.Join(t.TempDir(), "plugins")
	reg := NewRemoteRegistry(srv.URL+"/index.json", dir)

	idx, _ := reg.Fetch(context.Background())
	path, err := reg.Install(context.Background(), idx.Plugins[0])
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !strings.Contains(path, "aether-gcp-1.0.0.json") {
		t.Errorf("installed path = %q", path)
	}

	// The stored entry records the verified checksum.
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), want) {
		t.Errorf("verified sha not recorded: %s", data)
	}

	installed, err := reg.Installed()
	if err != nil || len(installed) != 1 || installed[0].Name != "aether-gcp" {
		t.Errorf("installed = %+v err=%v", installed, err)
	}
}

func TestRemoteRegistryInstallChecksumMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/index.json":
			json.NewEncoder(w).Encode(testIndex())
		case r.URL.Path == "/artifacts/aether-gcp.json":
			w.Write([]byte(`{"tampered":true}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	dir := filepath.Join(t.TempDir(), "plugins")
	reg := NewRemoteRegistry(srv.URL+"/index.json", dir)

	idx, _ := reg.Fetch(context.Background())
	_, err := reg.Install(context.Background(), idx.Plugins[0])
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
}

func TestRemoteRegistryInstallUndeclaredChecksumFailsClosed(t *testing.T) {
	// F-003 regression: a manifest that declares no sha256 must NOT
	// install (the old code activated the artifact unverified).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/index.json":
			json.NewEncoder(w).Encode(testIndex())
		case r.URL.Path == "/artifacts/aether-vmware.json":
			w.Write([]byte(`{"type":"provider","provider":"vmware"}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	dir := filepath.Join(t.TempDir(), "plugins")
	reg := NewRemoteRegistry(srv.URL+"/index.json", dir)

	idx, err := reg.Fetch(context.Background())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	// idx.Plugins[1] is aether-vmware — declares no SHA256.
	path, err := reg.Install(context.Background(), idx.Plugins[1])
	if err == nil || !strings.Contains(err.Error(), "declares no sha256") {
		t.Fatalf("expected fail-closed on undeclared checksum, got path=%q err=%v", path, err)
	}
	// Nothing was installed.
	installed, _ := reg.Installed()
	if len(installed) != 0 {
		t.Errorf("unverified install leaked: %+v", installed)
	}
}

func TestRemoteRegistryInstallAllowUnsignedOverride(t *testing.T) {
	// F-003: the explicit insecure override installs, but the stored
	// entry records that no checksum was verified.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/index.json":
			json.NewEncoder(w).Encode(testIndex())
		case r.URL.Path == "/artifacts/aether-vmware.json":
			w.Write([]byte(`{"type":"provider","provider":"vmware"}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	dir := filepath.Join(t.TempDir(), "plugins")
	reg := NewRemoteRegistry(srv.URL+"/index.json", dir)
	reg.AllowUnsigned = true // deliberate insecure override

	idx, _ := reg.Fetch(context.Background())
	path, err := reg.Install(context.Background(), idx.Plugins[1])
	if err != nil {
		t.Fatalf("allow-unsigned install: %v", err)
	}
	// The stored entry records the computed hash of what was installed
	// (a fingerprint, not a verified claim — no checksum was declared).
	artifact := []byte(`{"type":"provider","provider":"vmware"}`)
	sum := sha256.Sum256(artifact)
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), hex.EncodeToString(sum[:])) {
		t.Errorf("allow-unsigned entry should record computed artifact sha: %s", data)
	}
}

func TestRemoteRegistryNoURL(t *testing.T) {
	reg := NewRemoteRegistry("", filepath.Join(t.TempDir(), "plugins"))
	if _, err := reg.Search(context.Background(), ""); err == nil {
		t.Error("missing URL should fail")
	}
}

func TestRenderSearch(t *testing.T) {
	out := RenderSearch(testIndex().Plugins)
	if !strings.Contains(out, "aether-gcp") || !strings.Contains(out, "DESCRIPTION") {
		t.Errorf("render = %q", out)
	}
	empty := RenderSearch(nil)
	if !strings.Contains(empty, "No plugins matched") {
		t.Errorf("empty render = %q", empty)
	}
}
