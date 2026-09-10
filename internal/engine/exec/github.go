package exec

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Debajyoti0-0/aether/internal/types"
)

// GitHubAPIBase is the REST API endpoint.
const GitHubAPIBase = "https://api.github.com"

// GitHubExecutor dispatches workflow runs on GitHub Actions runners
// (the CI/CD equivalent of cloud VM command execution).
type GitHubExecutor struct {
	Token      string
	HTTPClient *http.Client
	APIBase    string
}

// NewGitHubExecutor builds a GitHub executor.
func NewGitHubExecutor(token string, hc *http.Client) *GitHubExecutor {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &GitHubExecutor{Token: token, HTTPClient: hc, APIBase: GitHubAPIBase}
}

// DispatchInputs are the workflow_dispatch inputs.
type DispatchInputs struct {
	Ref    string            `json:"-"`
	Inputs map[string]string `json:"inputs,omitempty"`
}

// ExecuteOnRunner dispatches a workflow on a repository's Actions runner.
func (e *GitHubExecutor) ExecuteOnRunner(ctx context.Context, repo, workflow, ref string, inputs map[string]string) (*types.CommandResult, error) {
	if e.Token == "" {
		return nil, fmt.Errorf("github token is required")
	}
	if ref == "" {
		ref = "main"
	}
	if e.APIBase == "" {
		e.APIBase = GitHubAPIBase
	}

	payload := struct {
		Ref    string            `json:"ref"`
		Inputs map[string]string `json:"inputs,omitempty"`
	}{Ref: ref, Inputs: inputs}

	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/repos/%s/actions/workflows/%s/dispatches", strings.TrimRight(e.APIBase, "/"), repo, workflow)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+e.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("workflow dispatch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusCreated {
		return &types.CommandResult{
			Status:   "dispatched",
			Output:   fmt.Sprintf("workflow %s dispatched on %s @ %s", workflow, repo, ref),
			ExitCode: -1,
		}, nil
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("workflow dispatch http %d: %s", resp.StatusCode, truncateBytes(respBody, 512))
}

// ListRuns lists recent workflow runs for verification.
func (e *GitHubExecutor) ListRuns(ctx context.Context, repo string, limit int) (string, error) {
	if e.APIBase == "" {
		e.APIBase = GitHubAPIBase
	}

	url := fmt.Sprintf("%s/repos/%s/actions/runs?per_page=%d", strings.TrimRight(e.APIBase, "/"), repo, limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+e.Token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("list runs http %d: %s", resp.StatusCode, truncateBytes(body, 512))
	}

	var runs struct {
		WorkflowRuns []struct {
			Name        string `json:"name"`
			Status      string `json:"status"`
			Conclusion  string `json:"conclusion"`
			CreatedAt   string `json:"created_at"`
			HTMLURL     string `json:"html_url"`
		} `json:"workflow_runs"`
	}
	if err := json.Unmarshal(body, &runs); err != nil {
		return "", err
	}

	var b strings.Builder
	for _, r := range runs.WorkflowRuns {
		fmt.Fprintf(&b, "%-20s %-12s %-12s %s\n", r.Name, r.Status, r.Conclusion, r.HTMLURL)
	}
	return b.String(), nil
}
