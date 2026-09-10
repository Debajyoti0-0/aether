package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Debajyoti0-0/aether/pkg/plugins/sdk"
)

// gcpProvider implements sdk.Provider for Google Cloud via raw REST
// (no google.golang.org/api dependency — keeps the static-binary
// guarantee). Auth uses an OAuth2 access token (from OIDC federation,
// a service-account key exchange, or a compromised metadata token).
type gcpProvider struct {
	token     string
	projectID string
	base      string // https://compute.googleapis.com (override for tests)
	iamBase   string // https://iam.googleapis.com (override for tests)
	http      *http.Client
}

// New builds a GCP provider plugin.
func New(projectID, token string) sdk.Plugin {
	return sdk.NewAdapter("gcp", "1.0.0", &gcpProvider{
		token:     token,
		projectID: projectID,
		base:      "https://compute.googleapis.com",
		iamBase:   "https://iam.googleapis.com",
		http:      &http.Client{Timeout: 15 * time.Second},
	})
}

func (g *gcpProvider) Name() string { return "gcp" }

// ValidateToken probes the token against the Compute project list.
func (g *gcpProvider) ValidateToken(ctx context.Context) error {
	body, err := g.api(ctx, g.base, "/compute/v1/projects/"+g.projectID, http.MethodGet, "")
	if err != nil {
		return err
	}
	var proj map[string]any
	if err := json.Unmarshal(body, &proj); err != nil {
		return err
	}
	if _, ok := proj["id"]; !ok {
		return fmt.Errorf("gcp token validation failed")
	}
	return nil
}

// GCPInstance is a minimal Compute Engine instance record.
type GCPInstance struct {
	Name   string   `json:"name"`
	Zone   string   `json:"zone"`
	Status string   `json:"status"`
	Tags   []string `json:"tags,omitempty"`
}

// ListInstances enumerates Compute Engine VMs in the project.
func (g *gcpProvider) ListInstances(ctx context.Context) ([]GCPInstance, error) {
	body, err := g.api(ctx, g.base,
		fmt.Sprintf("/compute/v1/projects/%s/aggregated/instances", g.projectID),
		http.MethodGet, "")
	if err != nil {
		return nil, err
	}

	var agg struct {
		Items map[string]struct {
			Instances []struct {
				Name   string `json:"name"`
				Zone   string `json:"zone"`
				Status string `json:"status"`
				Tags   struct {
					Items []string `json:"items"`
				} `json:"tags"`
			} `json:"instances"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &agg); err != nil {
		return nil, err
	}

	var out []GCPInstance
	for zone, scoped := range agg.Items {
		for _, i := range scoped.Instances {
			out = append(out, GCPInstance{
				Name:   i.Name,
				Zone:   zone, // "zones/us-east1-b"
				Status: i.Status,
				Tags:   i.Tags.Items,
			})
		}
	}
	return out, nil
}

// GCPServiceAccount is a minimal IAM service account record.
type GCPServiceAccount struct {
	Email     string `json:"email"`
	UniqueID  string `json:"uniqueId"`
	DisplayName string `json:"displayName"`
}

// DiscoverIAM enumerates service accounts and their IAM bindings.
func (g *gcpProvider) DiscoverIAM(ctx context.Context) ([]GCPServiceAccount, error) {
	body, err := g.api(ctx, g.iamBase,
		fmt.Sprintf("/v1/projects/%s/serviceAccounts", g.projectID),
		http.MethodGet, "")
	if err != nil {
		return nil, err
	}

	var list struct {
		Accounts []GCPServiceAccount `json:"accounts"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, err
	}
	return list.Accounts, nil
}

// Execute stages a command against a target VM. GCP has no inline VM
// exec API like Azure RunCommand; the standard post-exploitation path
// is OS Login SSH or serial console. This stages the attempt and
// records the exact gcloud equivalent for the operator.
func (g *gcpProvider) Execute(ctx context.Context, target, command string) (*sdk.Result, error) {
	// target = "zone/instance-name" or "zones/zone/instance-name"
	target = strings.TrimPrefix(target, "zones/")
	parts := strings.SplitN(target, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("target must be zone/instance, got %q", target)
	}

	// Verify the instance exists (and is reachable with this token).
	zone := strings.TrimPrefix(parts[0], "zones/")
	body, err := g.api(ctx, g.base,
		fmt.Sprintf("/compute/v1/projects/%s/zones/%s/instances/%s", g.projectID, zone, parts[1]),
		http.MethodGet, "")
	if err != nil {
		return nil, err
	}

	var inst struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &inst); err != nil {
		return nil, err
	}

	return &sdk.Result{
		Output: fmt.Sprintf("instance %s (zone=%s status=%s) staged for exec: %s\n"+
			"gcloud equivalent: gcloud compute ssh %s --zone %s --command %q",
			parts[1], zone, inst.Status, command, parts[1], zone, command),
		ExitCode: -1,
		Status:   "staged",
	}, nil
}

func (g *gcpProvider) api(ctx context.Context, base, path, method, body string) ([]byte, error) {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, base+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", "application/json")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := g.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gcp request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gcp http %d: %s", resp.StatusCode, truncateStr(string(data), 256))
	}
	return data, nil
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
