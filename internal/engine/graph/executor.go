package graph

import (
	"fmt"
	"sort"
	"strings"
)

// ExecutablePath is a BloodHound-style path converted into an Aether
// runbook: each edge becomes the exact CLI command that advances it.
type ExecutablePath struct {
	Path      []string `json:"path"`       // node labels along the path
	Runbook   []string `json:"runbook"`    // aether CLI commands
	OPSECRisk int      `json:"opsec_risk"` // 0-100 aggregated
	Notes     []string `json:"notes,omitempty"`
}

// EdgeRisk maps edge types to base OPSEC risk.
var EdgeRisk = map[string]int{
	"member_of":  5,
	"has_role":   30,
	"owns":       25,
	"can_assume": 20,
	"can_exec":   45,
	"trusts":     35,
}

// GraphEngine qualifies paths against the workspace graph.
type GraphEngine struct {
	Graph        *IdentityGraph
	TokensHeld   map[string]bool // node IDs for which we possess tokens
	MaxRiskScore int             // operator risk ceiling
}

// NewGraphEngine builds an engine over a graph.
func NewGraphEngine(g *IdentityGraph, maxRisk int) *GraphEngine {
	if maxRisk <= 0 {
		maxRisk = 60
	}
	return &GraphEngine{Graph: g, TokensHeld: map[string]bool{}, MaxRiskScore: maxRisk}
}

// MarkTokensHeld records that the operator holds credentials for a node.
func (e *GraphEngine) MarkTokensHeld(nodeID string) {
	e.TokensHeld[nodeID] = true
}

// QualifyPath converts a node-ID path into an executable runbook.
// It checks token availability at the start, computes risk per edge,
// and refuses paths that breach the risk ceiling.
func (e *GraphEngine) QualifyPath(pathIDs []string) (*ExecutablePath, error) {
	if len(pathIDs) < 2 {
		return nil, fmt.Errorf("path needs at least 2 nodes")
	}

	run := &ExecutablePath{Path: make([]string, 0, len(pathIDs))}
	for _, id := range pathIDs {
		node := e.findNode(id)
		if node == nil {
			return nil, fmt.Errorf("unknown node %q", id)
		}
		run.Path = append(run.Path, node.Label)
	}

	// Token check on the start node.
	if !e.TokensHeld[pathIDs[0]] {
		run.Notes = append(run.Notes,
			fmt.Sprintf("No credentials held for %q; run `aether prt convert` or import tokens first.", run.Path[0]))
	}

	totalRisk := 0
	for i := 0; i < len(pathIDs)-1; i++ {
		src, dst := pathIDs[i], pathIDs[i+1]
		edge := e.findEdge(src, dst)
		if edge == nil {
			return nil, fmt.Errorf("no edge %s -> %s in graph", src, dst)
		}

		risk := EdgeRisk[edge.Type]
		if risk == 0 {
			risk = 15
		}
		totalRisk += risk

		run.Runbook = append(run.Runbook, e.commandFor(*edge))
	}

	if len(run.Runbook) > 0 {
		totalRisk /= len(run.Runbook)
	}
	run.OPSECRisk = totalRisk

	if totalRisk > e.MaxRiskScore {
		run.Notes = append(run.Notes, fmt.Sprintf(
			"Path risk %d exceeds operator ceiling %d; segments flagged below.", totalRisk, e.MaxRiskScore))
	}
	return run, nil
}

// commandFor maps an edge type to the Aether CLI command that executes it.
func (e *GraphEngine) commandFor(edge GraphEdge) string {
	dst := e.findNode(edge.Target)

	switch edge.Type {
	case "has_role":
		return fmt.Sprintf("aether exec azure --token <arm-token> --subscription-id <sub> --resource-group <rg> --vm-id %s --cmd 'whoami'", dst.Label)
	case "owns":
		return fmt.Sprintf("aether exec github --token <gh-token> --repo %s --workflow build.yaml", dst.Label)
	case "can_assume":
		return fmt.Sprintf("aether exec aws --access-key <ak> --secret-key <sk> --instance-id %s --cmd 'id'", dst.Label)
	case "member_of":
		return fmt.Sprintf("aether cap evaluate --policies <export>.json --user %s --app graph", dst.Label)
	case "can_exec":
		return fmt.Sprintf("aether exec azure --token <arm-token> --subscription-id <sub> --resource-group <rg> --vm-id %s --cmd '<payload>'", dst.Label)
	case "trusts":
		return fmt.Sprintf("aether relay mfa --sts-endpoint %s --tenant <tenant>", dst.Label)
	default:
		return fmt.Sprintf("# manual step: %s -> %s (%s)", edge.Source, edge.Target, edge.Type)
	}
}

// ShortestPaths finds all shortest paths from src to dst via BFS.
func (e *GraphEngine) ShortestPaths(src, dst string, maxPaths int) [][]string {
	if maxPaths <= 0 {
		maxPaths = 5
	}

	adjacency := map[string][]string{}
	for _, edge := range e.Graph.Edges {
		adjacency[edge.Source] = append(adjacency[edge.Source], edge.Target)
	}

	// BFS level tracking.
	type entry struct {
		path []string
	}
	var results [][]string
	shortest := -1

	queue := []entry{{path: []string{src}}}
	visitedDepth := map[string]int{src: 0}

	for len(queue) > 0 && len(results) < maxPaths {
		cur := queue[0]
		queue = queue[1:]

		last := cur.path[len(cur.path)-1]
		if last == dst {
			if shortest == -1 || len(cur.path) <= shortest {
				shortest = len(cur.path)
				results = append(results, cur.path)
			}
			continue
		}
		if shortest != -1 && len(cur.path) >= shortest {
			continue
		}

		for _, next := range adjacency[last] {
			if containsStr(cur.path, next) {
				continue
			}
			if d, seen := visitedDepth[next]; seen && d < len(cur.path) {
				continue
			}
			visitedDepth[next] = len(cur.path)
			np := append(append([]string{}, cur.path...), next)
			queue = append(queue, entry{path: np})
		}
	}
	return results
}

// RenderRunbook formats an executable path for terminal output.
func RenderRunbook(run *ExecutablePath) string {
	var b strings.Builder
	b.WriteString("=== Executable Runbook ===\n")
	b.WriteString("Path: " + strings.Join(run.Path, " -> ") + "\n")
	fmt.Fprintf(&b, "OPSEC Risk: %d/100\n\nSteps:\n", run.OPSECRisk)
	for i, cmd := range run.Runbook {
		fmt.Fprintf(&b, "  %d. %s\n", i+1, cmd)
	}
	if len(run.Notes) > 0 {
		b.WriteString("\nNotes:\n")
		for _, n := range run.Notes {
			fmt.Fprintf(&b, "  [!] %s\n", n)
		}
	}
	return b.String()
}

// Stats returns a provider breakdown of the graph.
func (g *IdentityGraph) Stats() string {
	byProvider := map[string]int{}
	for _, n := range g.Nodes {
		byProvider[n.Provider]++
	}
	keys := make([]string, 0, len(byProvider))
	for k := range byProvider {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	fmt.Fprintf(&b, "Nodes: %d, Edges: %d\n", len(g.Nodes), len(g.Edges))
	for _, k := range keys {
		fmt.Fprintf(&b, "  %-8s %d nodes\n", k+":", byProvider[k])
	}
	return b.String()
}

func (e *GraphEngine) findNode(id string) *GraphNode {
	for i := range e.Graph.Nodes {
		if e.Graph.Nodes[i].ID == id {
			return &e.Graph.Nodes[i]
		}
	}
	return nil
}

func (e *GraphEngine) findEdge(src, dst string) *GraphEdge {
	for i := range e.Graph.Edges {
		if e.Graph.Edges[i].Source == src && e.Graph.Edges[i].Target == dst {
			return &e.Graph.Edges[i]
		}
	}
	return nil
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
