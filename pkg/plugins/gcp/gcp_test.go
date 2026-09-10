package gcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGCPValidateToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			w.WriteHeader(401)
			return
		}
		if r.URL.Path == "/compute/v1/projects/proj-1" {
			json.NewEncoder(w).Encode(map[string]any{"id": "proj-1", "name": "proj-1"})
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	p := newForTest(srv.URL, srv.URL, "proj-1", "gcp-token")
	prov := p
	if err := prov.ValidateToken(context.Background()); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestGCPListInstances(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/aggregated/instances") {
			json.NewEncoder(w).Encode(map[string]any{
				"items": map[string]any{
					"zones/us-east1-b": map[string]any{
						"instances": []map[string]any{
							{"name": "vm-1", "zone": "zones/us-east1-b", "status": "RUNNING",
								"tags": map[string]any{"items": []string{"prod"}}},
						},
					},
				},
			})
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	g := newForTest(srv.URL, srv.URL, "proj-1", "tok")
	insts, err := g.ListInstances(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(insts) != 1 || insts[0].Name != "vm-1" || insts[0].Status != "RUNNING" {
		t.Errorf("instances = %+v", insts)
	}
}

func TestGCPDiscoverIAM(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/serviceAccounts") {
			json.NewEncoder(w).Encode(map[string]any{
				"accounts": []map[string]string{
					{"email": "sa@proj.iam.gserviceaccount.com", "uniqueId": "123", "displayName": "CI SA"},
				},
			})
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	g := newForTest(srv.URL, srv.URL, "proj-1", "tok")
	accts, err := g.DiscoverIAM(context.Background())
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(accts) != 1 || accts[0].Email != "sa@proj.iam.gserviceaccount.com" {
		t.Errorf("accounts = %+v", accts)
	}
}

func TestGCPExecuteStages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/instances/vm-1") {
			json.NewEncoder(w).Encode(map[string]any{"status": "RUNNING"})
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	p := newForTest(srv.URL, srv.URL, "proj-1", "tok")
	prov := p
	res, err := prov.Execute(context.Background(), "zones/us-east1-b/vm-1", "id")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.Status != "staged" || !strings.Contains(res.Output, "gcloud compute ssh") {
		t.Errorf("result = %+v", res)
	}
}

func TestGCPExecuteBadTarget(t *testing.T) {
	g := newForTest("http://x", "http://x", "p", "t")
	if _, err := g.Execute(context.Background(), "just-a-name", "id"); err == nil {
		t.Error("bad target should fail")
	}
}

// newForTest builds a provider with overridden API bases.
func newForTest(computeBase, iamBase, projectID, token string) *gcpProvider {
	return &gcpProvider{
		token:     token,
		projectID: projectID,
		base:      computeBase,
		iamBase:   iamBase,
		http:      &http.Client{Timeout: 5 * time.Second},
	}
}
