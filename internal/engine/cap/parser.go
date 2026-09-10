package cap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/Debajyoti0-0/aether/internal/types"
)

// GraphPolicies is the Graph API response shape for CAP policies.
type GraphPolicies struct {
	Value []GraphPolicy `json:"value"`
}

// GraphPolicy is a single policy as returned by Graph.
type GraphPolicy struct {
	ID            string          `json:"id"`
	DisplayName   string          `json:"displayName"`
	State         string          `json:"state"`
	Conditions    json.RawMessage `json:"conditions"`
	GrantControls json.RawMessage `json:"grantControls"`
}

// ParseFromGraph converts a Graph API JSON payload into typed policies.
func ParseFromGraph(data []byte) ([]types.ConditionalAccessPolicy, error) {
	var gp GraphPolicies
	if err := json.Unmarshal(data, &gp); err != nil {
		// Maybe a single policy was supplied.
		var single GraphPolicy
		if err2 := json.Unmarshal(data, &single); err2 != nil {
			return nil, fmt.Errorf("parse graph policies: %w", err)
		}
		gp.Value = []GraphPolicy{single}
	}

	policies := make([]types.ConditionalAccessPolicy, 0, len(gp.Value))
	for _, p := range gp.Value {
		typed := types.ConditionalAccessPolicy{
			ID:          p.ID,
			DisplayName: p.DisplayName,
			State:       p.State,
		}
		if len(p.Conditions) > 0 {
			if err := json.Unmarshal(p.Conditions, &typed.Conditions); err != nil {
				return nil, fmt.Errorf("parse conditions of %q: %w", p.DisplayName, err)
			}
		}
		if len(p.GrantControls) > 0 {
			if err := json.Unmarshal(p.GrantControls, &typed.GrantControls); err != nil {
				return nil, fmt.Errorf("parse grant controls of %q: %w", p.DisplayName, err)
			}
		}
		policies = append(policies, typed)
	}
	return policies, nil
}

// ParseFromFile loads policies from a JSON file (Graph export format).
func ParseFromFile(path string) ([]types.ConditionalAccessPolicy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read policies file: %w", err)
	}
	return ParseFromGraph(data)
}

// FetchFromGraph retrieves CAP policies live from Microsoft Graph.
func FetchFromGraph(ctx context.Context, httpClient *http.Client, token, apiBase string) ([]types.ConditionalAccessPolicy, error) {
	if apiBase == "" {
		apiBase = "https://graph.microsoft.com/v1.0"
	}

	url := strings.TrimRight(apiBase, "/") + "/identity/conditionalAccess/policies"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("ConsistencyLevel", "eventual")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("graph request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("graph returned http %d: %s", resp.StatusCode, string(body[:min(len(body), 512)]))
	}
	return ParseFromGraph(body)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
