package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// GraphNode is a cross-provider identity object.
type GraphNode struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`     // user, group, sp, role, aws_user, aws_role, gcp_sa
	Provider string            `json:"provider"` // entra, aws, gcp, onprem
	Label    string            `json:"label"`
	Props    map[string]string `json:"props,omitempty"`
}

// GraphEdge is a directed relationship between nodes.
type GraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"` // member_of, owns, can_assume, has_role
	Weight int    `json:"weight,omitempty"`
}

// IdentityGraph is the merged cross-provider graph.
type IdentityGraph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// AddNode upserts a node by ID.
func (g *IdentityGraph) AddNode(n GraphNode) {
	for i := range g.Nodes {
		if g.Nodes[i].ID == n.ID {
			if g.Nodes[i].Label == "" {
				g.Nodes[i].Label = n.Label
			}
			for k, v := range n.Props {
				if g.Nodes[i].Props == nil {
					g.Nodes[i].Props = map[string]string{}
				}
				g.Nodes[i].Props[k] = v
			}
			return
		}
	}
	g.Nodes = append(g.Nodes, n)
}

// AddEdge upserts an edge by (source, target, type).
func (g *IdentityGraph) AddEdge(e GraphEdge) {
	for i := range g.Edges {
		if g.Edges[i].Source == e.Source && g.Edges[i].Target == e.Target && g.Edges[i].Type == e.Type {
			return
		}
	}
	g.Edges = append(g.Edges, e)
}

// Merge merges another graph into this one (deduped).
func (g *IdentityGraph) Merge(other *IdentityGraph) {
	for _, n := range other.Nodes {
		g.AddNode(n)
	}
	for _, e := range other.Edges {
		g.AddEdge(e)
	}
}

// IngestEntraJSON ingests a Graph API JSON export and enriches the graph.
func (g *IdentityGraph) IngestEntraJSON(kind string, data []byte) error {
	switch kind {
	case "users":
		return g.ingestGraphList(data, func(v map[string]any) {
			id := str(v, "id")
			upn := str(v, "userPrincipalName")
			g.AddNode(GraphNode{ID: id, Type: "user", Provider: "entra", Label: upn})
			if dept := str(v, "department"); dept != "" {
				g.AddNode(GraphNode{ID: "dept:" + dept, Type: "group", Provider: "entra", Label: dept})
				g.AddEdge(GraphEdge{Source: id, Target: "dept:" + dept, Type: "member_of", Weight: 1})
			}
		})
	case "servicePrincipals":
		return g.ingestGraphList(data, func(v map[string]any) {
			id := str(v, "id")
			display := str(v, "displayName")
			g.AddNode(GraphNode{ID: id, Type: "sp", Provider: "entra", Label: display})
		})
	case "roleAssignments":
		return g.ingestGraphList(data, func(v map[string]any) {
			principal := str(v, "principalId")
			role := str(v, "roleDefinitionId")
			if principal == "" || role == "" {
				return
			}
			roleNode := "role:" + role
			g.AddNode(GraphNode{ID: roleNode, Type: "role", Provider: "entra", Label: role})
			g.AddEdge(GraphEdge{Source: principal, Target: roleNode, Type: "has_role", Weight: 20})
		})
	default:
		return fmt.Errorf("unknown entra ingest kind %q (users, servicePrincipals, roleAssignments)", kind)
	}
}

func (g *IdentityGraph) ingestGraphList(data []byte, fn func(map[string]any)) error {
	var wrapper struct {
		Value []map[string]any `json:"value"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		var single map[string]any
		if err2 := json.Unmarshal(data, &single); err2 != nil {
			return fmt.Errorf("parse graph json: %w", err)
		}
		wrapper.Value = []map[string]any{single}
	}
	for _, v := range wrapper.Value {
		fn(v)
	}
	return nil
}

// IngestAWSJSON ingests an AWS IAM export (users/roles).
func (g *IdentityGraph) IngestAWSJSON(data []byte) error {
	var doc struct {
		Users []struct {
			UserName string   `json:"userName"`
			Arn      string   `json:"arn"`
			Groups   []string `json:"groups"`
		} `json:"users"`
		Roles []struct {
			RoleName  string `json:"roleName"`
			Arn       string `json:"arn"`
			AssumedBy []struct {
				Principal string `json:"principal"`
			} `json:"assumedBy"`
		} `json:"roles"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse aws json: %w", err)
	}

	for _, u := range doc.Users {
		name := u.UserName
		if name == "" {
			name = u.Arn
		}
		g.AddNode(GraphNode{ID: u.Arn, Type: "aws_user", Provider: "aws", Label: name})
		for _, grp := range u.Groups {
			grpID := "aws-group:" + grp
			g.AddNode(GraphNode{ID: grpID, Type: "group", Provider: "aws", Label: grp})
			g.AddEdge(GraphEdge{Source: u.Arn, Target: grpID, Type: "member_of", Weight: 1})
		}
	}
	for _, r := range doc.Roles {
		g.AddNode(GraphNode{ID: r.Arn, Type: "aws_role", Provider: "aws", Label: r.RoleName})
		for _, a := range r.AssumedBy {
			g.AddEdge(GraphEdge{Source: a.Principal, Target: r.Arn, Type: "can_assume", Weight: 15})
		}
	}
	return nil
}

// IngestGCPJSON ingests a GCP IAM export (service accounts + bindings).
func (g *IdentityGraph) IngestGCPJSON(data []byte) error {
	var doc struct {
		ServiceAccounts []struct {
			Email string `json:"email"`
		} `json:"serviceAccounts"`
		Bindings []struct {
			Role    string   `json:"role"`
			Members []string `json:"members"`
		} `json:"bindings"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse gcp json: %w", err)
	}

	for _, sa := range doc.ServiceAccounts {
		g.AddNode(GraphNode{ID: sa.Email, Type: "gcp_sa", Provider: "gcp", Label: sa.Email})
	}
	for _, b := range doc.Bindings {
		roleID := "gcp-role:" + b.Role
		g.AddNode(GraphNode{ID: roleID, Type: "role", Provider: "gcp", Label: b.Role})
		for _, m := range b.Members {
			g.AddEdge(GraphEdge{Source: m, Target: roleID, Type: "has_role", Weight: 20})
		}
	}
	return nil
}

// Save writes the graph as JSON.
func (g *IdentityGraph) Save(path string) error {
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// LoadGraph reads a saved identity graph.
func LoadGraph(path string) (*IdentityGraph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read graph: %w", err)
	}
	g := &IdentityGraph{}
	if err := json.Unmarshal(data, g); err != nil {
		return nil, fmt.Errorf("parse graph: %w", err)
	}
	return g, nil
}

// FetchEntra pulls users and service principals live from Graph.
func FetchEntra(ctx context.Context, hc *http.Client, token, apiBase string) (*IdentityGraph, error) {
	if apiBase == "" {
		apiBase = "https://graph.microsoft.com/v1.0"
	}
	g := &IdentityGraph{}

	for _, kind := range []string{"users", "servicePrincipals"} {
		body, err := graphGet(ctx, hc, token, strings.TrimRight(apiBase, "/")+"/"+kind)
		if err != nil {
			return nil, fmt.Errorf("fetch %s: %w", kind, err)
		}
		if err := g.IngestEntraJSON(kind, body); err != nil {
			return nil, err
		}
	}
	return g, nil
}

func graphGet(ctx context.Context, hc *http.Client, token, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("ConsistencyLevel", "eventual")

	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("graph http %d: %s", resp.StatusCode, truncateStr(string(body), 256))
	}
	return body, nil
}

func str(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
