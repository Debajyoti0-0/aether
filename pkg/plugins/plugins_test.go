package plugins_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Debajyoti0-0/aether/pkg/plugins"
	"github.com/Debajyoti0-0/aether/pkg/plugins/gitlab"
	"github.com/Debajyoti0-0/aether/pkg/plugins/kubernetes"
	"github.com/Debajyoti0-0/aether/pkg/plugins/okta"
	"github.com/Debajyoti0-0/aether/pkg/plugins/sdk"
)

// silentHost satisfies sdk.Host.
type silentHost struct{}

func (silentHost) Logf(string, ...any) {}

func TestOfficialPluginsRegister(t *testing.T) {
	registry := plugins.NewRegistry()

	if err := registry.Register(okta.New("https://x.okta.com", "tok")); err != nil {
		t.Fatalf("register okta: %v", err)
	}
	if err := registry.Register(gitlab.New("https://gitlab.example.com", "tok")); err != nil {
		t.Fatalf("register gitlab: %v", err)
	}
	if err := registry.Register(kubernetes.New("https://127.0.0.1:6443", "tok", "default")); err != nil {
		t.Fatalf("register kubernetes: %v", err)
	}

	if len(registry.Names()) != 3 {
		t.Fatalf("names = %v", registry.Names())
	}
	if err := registry.Register(okta.New("https://x.okta.com", "tok")); err == nil {
		t.Error("duplicate registration should fail")
	}
	if err := registry.InitAll(silentHost{}); err != nil {
		t.Errorf("init all: %v", err)
	}
}

func TestOktaProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/users/me":
			if !strings.HasPrefix(r.Header.Get("Authorization"), "SSWS ") {
				w.WriteHeader(401)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"id": "u1", "login": "admin@corp.com"})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	prov, err := plugins.NewRegistry().Provider("okta")
	if err == nil {
		t.Fatal("unregistered provider should fail")
	}

	registry := plugins.NewRegistry()
	registry.Register(okta.New(srv.URL, "okta-token"))
	prov, err = registry.Provider("okta")
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	if err := prov.ValidateToken(context.Background()); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestGitLabProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4/user":
			json.NewEncoder(w).Encode(map[string]string{"username": "op1"})
		case r.URL.Path == "/api/v4/projects/1/pipeline" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": 42, "status": "pending"})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	registry := plugins.NewRegistry()
	registry.Register(gitlab.New(srv.URL, "glpat-token"))
	prov, err := registry.Provider("gitlab")
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	if err := prov.ValidateToken(context.Background()); err != nil {
		t.Fatalf("validate: %v", err)
	}
	res, err := prov.Execute(context.Background(), "1", "main")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.Status != "dispatched" {
		t.Errorf("status = %q", res.Status)
	}
}

func TestKubernetesProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			w.WriteHeader(401)
			return
		}
		switch {
		case r.URL.Path == "/apis/authentication.k8s.io/v1/selfsubjectreviews":
			json.NewEncoder(w).Encode(map[string]any{"status": map[string]any{"authenticated": true}})
		case strings.HasSuffix(r.URL.Path, "/namespaces/default/pods/web-1"):
			json.NewEncoder(w).Encode(map[string]any{
				"spec": map[string]string{"serviceAccountName": "web-sa", "nodeName": "node-1"},
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	registry := plugins.NewRegistry()
	registry.Register(kubernetes.New(srv.URL, "sa-token", "default"))
	prov, err := registry.Provider("kubernetes")
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	if err := prov.ValidateToken(context.Background()); err != nil {
		t.Fatalf("validate: %v", err)
	}
	res, err := prov.Execute(context.Background(), "default/web-1", "id")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(res.Output, "web-sa") {
		t.Errorf("output = %q", res.Output)
	}
}

func TestSDKAdapterValidation(t *testing.T) {
	if err := sdk.NewAdapter("x", "1.0.0", nil).Init(silentHost{}); err == nil {
		t.Error("adapter without provider should fail init")
	}
	if _, err := plugins.NewRegistry().Provider("ghost"); err == nil {
		t.Error("missing plugin should fail")
	}
}
