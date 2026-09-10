package validate

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

// PathValidator validates BloodHound attack paths non-destructively.
type PathValidator struct {
	Token      string
	GraphBase  string
	HTTPClient *http.Client
}

// NewPathValidator builds a validator; token may be empty for pure
// structural validation without live checks.
func NewPathValidator(token, graphBase string, hc *http.Client) *PathValidator {
	if hc == nil {
		hc = http.DefaultClient
	}
	if graphBase == "" {
		graphBase = "https://graph.microsoft.com/v1.0"
	}
	return &PathValidator{Token: token, GraphBase: graphBase, HTTPClient: hc}
}

// LoadPath reads a BloodHound-style JSON export from disk.
func LoadPath(path string) (*types.AttackPath, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read path file: %w", err)
	}
	p := &types.AttackPath{}
	if err := json.Unmarshal(data, p); err != nil {
		return nil, fmt.Errorf("parse path json: %w", err)
	}
	if len(p.Edges) == 0 {
		return nil, fmt.Errorf("path json contains no edges")
	}
	return p, nil
}

// ValidatePath checks each step in the path and accumulates risk.
func (v *PathValidator) ValidatePath(ctx context.Context, path *types.AttackPath) (*types.ValidationResult, error) {
	result := &types.ValidationResult{IsValid: true}

	for i, step := range path.Edges {
		stepResult := v.validateStep(ctx, i, step, path)
		result.Steps = append(result.Steps, stepResult)

		if !stepResult.Valid {
			result.IsValid = false
		}
		result.OverallRisk += stepResult.RiskScore
	}

	if len(result.Steps) > 0 {
		result.OverallRisk /= len(result.Steps)
	}
	return result, nil
}

func (v *PathValidator) validateStep(ctx context.Context, idx int, edge types.PathEdge, path *types.AttackPath) types.StepResult {
	res := types.StepResult{
		StepIndex: idx,
		Edge:      fmt.Sprintf("%s -> %s -> %s", edge.Source, edge.Type, edge.Target),
		Valid:     true,
		RiskScore: baseRiskForEdge(edge.Type),
	}

	srcNode := findNode(path, edge.Source)
	dstNode := findNode(path, edge.Target)
	if srcNode == nil {
		res.Valid = false
		res.Reason = fmt.Sprintf("source node %q missing from path", edge.Source)
	}
	if dstNode == nil {
		res.Valid = false
		res.Reason = fmt.Sprintf("target node %q missing from path", edge.Target)
	}
	if srcNode != nil && dstNode != nil && edge.Type == "" {
		res.Valid = false
		res.Reason = "edge type is empty"
	}

	// Live existence check via Graph when a token is available.
	if v.Token != "" && isUserNode(dstNode) {
		if err := v.checkUserExists(ctx, dstNode.ID); err != nil {
			res.Valid = false
			res.Reason = fmt.Sprintf("target user not resolvable: %v", err)
		}
	}

	if res.Reason == "" {
		res.Reason = "structure valid; edge risk " + fmt.Sprint(res.RiskScore)
	}
	return res
}

func (v *PathValidator) checkUserExists(ctx context.Context, user string) error {
	url := strings.TrimRight(v.GraphBase, "/") + "/users/" + user
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+v.Token)

	resp, err := v.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("http 404")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http %d", resp.StatusCode)
	}
	return nil
}

func isUserNode(n *types.PathNode) bool {
	if n == nil {
		return false
	}
	return strings.EqualFold(n.Label, "User")
}

func findNode(path *types.AttackPath, id string) *types.PathNode {
	for i := range path.Nodes {
		if path.Nodes[i].ID == id {
			return &path.Nodes[i]
		}
	}
	return nil
}

func baseRiskForEdge(edgeType string) int {
	switch strings.ToLower(edgeType) {
	case "memberof":
		return 5
	case "hasession":
		return 25
	case "adminto", "genericall", "all extended rights":
		return 40
	case "resetpassword":
		return 30
	case "getchanges", "getchangesall", "dcsync":
		return 50
	default:
		return 15
	}
}
