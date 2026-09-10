package gitlab

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

// gitlabProvider implements sdk.Provider for GitLab (OIDC + PAT auth).
type gitlabProvider struct {
	token string
	base  string // https://gitlab.example.com
	http  *http.Client
}

// New builds a GitLab provider plugin.
func New(baseURL, token string) sdk.Plugin {
	return sdk.NewAdapter("gitlab", "1.0.0", &gitlabProvider{
		token: token,
		base:  strings.TrimRight(baseURL, "/"),
		http:  &http.Client{Timeout: 15 * time.Second},
	})
}

func (g *gitlabProvider) Name() string { return "gitlab" }

// ValidateToken checks the PAT against /api/v4/user.
func (g *gitlabProvider) ValidateToken(ctx context.Context) error {
	body, err := g.get(ctx, "/api/v4/user")
	if err != nil {
		return err
	}
	var user map[string]any
	if err := json.Unmarshal(body, &user); err != nil {
		return err
	}
	if _, ok := user["username"]; !ok {
		return fmt.Errorf("gitlab token validation failed")
	}
	return nil
}

// GitLabProject is a minimal project record.
type GitLabProject struct {
	ID                int    `json:"id"`
	PathWithNamespace string `json:"path_with_namespace"`
	DefaultBranch     string `json:"default_branch"`
}

// ListProjects enumerates projects the token can access.
func (g *gitlabProvider) ListProjects(ctx context.Context, limit int) ([]GitLabProject, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	body, err := g.get(ctx, fmt.Sprintf("/api/v4/projects?per_page=%d&membership=true", limit))
	if err != nil {
		return nil, err
	}
	var projects []GitLabProject
	if err := json.Unmarshal(body, &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

// Execute triggers a pipeline on a project (CI/CD runner exec).
// target = "id/path", command = ref to run.
func (g *gitlabProvider) Execute(ctx context.Context, target, command string) (*sdk.Result, error) {
	if command == "" {
		command = "main"
	}
	url := fmt.Sprintf("%s/api/v4/projects/%s/pipeline", g.base, target)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url,
		strings.NewReader(fmt.Sprintf(`{"ref":%q}`, command)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", g.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab pipeline: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return &sdk.Result{
			Output:   fmt.Sprintf("pipeline dispatched on %s @ %s", target, command),
			ExitCode: -1,
			Status:   "dispatched",
		}, nil
	}
	return nil, fmt.Errorf("gitlab http %d: %s", resp.StatusCode, truncateStr(string(body), 256))
}

func (g *gitlabProvider) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.base+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", g.token)
	return g.do(req)
}

func (g *gitlabProvider) do(req *http.Request) ([]byte, error) {
	resp, err := g.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gitlab http %d: %s", resp.StatusCode, truncateStr(string(body), 256))
	}
	return body, nil
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
