package exec

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIMDSGetIdentityToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metadata/identity/oauth2/token" {
			w.WriteHeader(404)
			return
		}
		if r.Header.Get("Metadata") != "true" {
			w.WriteHeader(400)
			return
		}
		if r.URL.Query().Get("client_id") != "cid-1" {
			t.Errorf("client_id = %q", r.URL.Query().Get("client_id"))
		}
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": "managed-identity-token",
			"client_id":    "cid-1",
			"expires_in":   "8599",
			"resource":     "https://management.azure.com/",
			"token_type":   "Bearer",
		})
	}))
	defer srv.Close()

	c := NewIMDSClientAt(srv.URL, srv.Client())
	tokens, err := c.GetIdentityToken(context.Background(), "https://management.azure.com/", "cid-1", "")
	if err != nil {
		t.Fatalf("identity token: %v", err)
	}
	if tokens.AccessToken != "managed-identity-token" {
		t.Errorf("token = %q", tokens.AccessToken)
	}
	if tokens.ExpiresIn != 8599 {
		t.Errorf("expires = %d", tokens.ExpiresIn)
	}
	if tokens.TokenType != "Bearer" {
		t.Errorf("type = %q", tokens.TokenType)
	}
}

func TestIMDSGetIdentityTokenErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":"invalid_request"}`))
	}))
	defer srv.Close()

	c := NewIMDSClientAt(srv.URL, srv.Client())
	if _, err := c.GetIdentityToken(context.Background(), "", "", ""); err == nil {
		t.Error("expected http error")
	}
}

func TestIMDSInstanceMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/metadata/instance/compute") {
			w.WriteHeader(404)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"vmId":           "vm-123",
			"name":           "CoreDC",
			"resourceGroupName": "rg-prod",
			"subscriptionId": "sub-1",
		})
	}))
	defer srv.Close()

	c := NewIMDSClientAt(srv.URL, srv.Client())
	doc, err := c.InstanceMetadata(context.Background())
	if err != nil {
		t.Fatalf("instance metadata: %v", err)
	}
	if doc["name"] != "CoreDC" {
		t.Errorf("name = %v", doc["name"])
	}
}

func TestIMDSFetchToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			w.WriteHeader(405)
			return
		}
		w.Write([]byte("imds-session-token"))
	}))
	defer srv.Close()

	c := NewIMDSClientAt(srv.URL, srv.Client())
	if err := c.FetchToken(context.Background(), 300); err != nil {
		t.Fatalf("fetch token: %v", err)
	}
	if c.Token != "imds-session-token" {
		t.Errorf("token = %q", c.Token)
	}
}

func TestSPList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/servicePrincipals") {
			w.WriteHeader(404)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"value": []map[string]string{
				{"id": "sp-1", "appId": "app-1", "displayName": "CI-Runner"},
			},
		})
	}))
	defer srv.Close()

	c := NewSPClientAt("graph-token", srv.URL+"/v1.0", srv.Client())
	sps, err := c.List(context.Background(), "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(sps) != 1 || sps[0].DisplayName != "CI-Runner" {
		t.Errorf("sps = %+v", sps)
	}
}

func TestSPAddPassword(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/addPassword") {
			w.WriteHeader(404)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{
			"secretText": "very-new-secret",
		})
	}))
	defer srv.Close()

	c := NewSPClientAt("graph-token", srv.URL+"/v1.0", srv.Client())
	secret, err := c.AddPassword(context.Background(), "sp-1", "aether-persist", 6)
	if err != nil {
		t.Fatalf("addPassword: %v", err)
	}
	if secret != "very-new-secret" {
		t.Errorf("secret = %q", secret)
	}
}

func TestSPGraphError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(`{"error":{"message":"insufficient privileges"}}`))
	}))
	defer srv.Close()

	c := NewSPClientAt("graph-token", srv.URL+"/v1.0", srv.Client())
	if _, err := c.List(context.Background(), ""); err == nil {
		t.Error("expected graph error")
	}
	if _, err := c.GetOwnedObjects(context.Background(), "sp-1"); err == nil {
		t.Error("expected graph error")
	}
}
