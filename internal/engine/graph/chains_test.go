package graph

import (
	"fmt"
	"strings"
	"testing"
)

// chainTestGraph wires entra → aws → gcp:
//
//	u1 (entra) --can_assume--> arn:...:EC2-Admin (aws) --can_assume--> sa@proj (gcp)
func chainTestGraph(t *testing.T) *IdentityGraph {
	t.Helper()
	g := &IdentityGraph{}
	g.AddNode(GraphNode{ID: "u1", Type: "user", Provider: "entra", Label: "admin@corp.com"})
	g.AddNode(GraphNode{ID: "aws-role", Type: "aws_role", Provider: "aws", Label: "EC2-Admin"})
	g.AddNode(GraphNode{ID: "gcp-sa", Type: "gcp_sa", Provider: "gcp", Label: "sa@proj"})
	g.AddNode(GraphNode{ID: "entra-group", Type: "group", Provider: "entra", Label: "IT"})
	g.AddEdge(GraphEdge{Source: "u1", Target: "entra-group", Type: "member_of"})
	g.AddEdge(GraphEdge{Source: "u1", Target: "aws-role", Type: "can_assume"})
	g.AddEdge(GraphEdge{Source: "aws-role", Target: "gcp-sa", Type: "can_assume"})
	return g
}

func TestSynthesizeChainsThreeProviders(t *testing.T) {
	g := chainTestGraph(t)
	e := NewGraphEngine(g, 60)
	e.MarkTokensHeld("u1")

	chains, err := e.SynthesizeChains(2, 10) // 2 hops = 3 providers
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	if len(chains) == 0 {
		t.Fatal("no chains found")
	}

	c := chains[0]
	if len(c.Providers) != 3 {
		t.Errorf("providers = %v, want 3 unique", c.Providers)
	}
	if !strings.Contains(strings.Join(c.Providers, ","), "entra") ||
		!strings.Contains(strings.Join(c.Providers, ","), "aws") ||
		!strings.Contains(strings.Join(c.Providers, ","), "gcp") {
		t.Errorf("providers = %v", c.Providers)
	}
	if len(c.Runbook) == 0 {
		t.Error("runbook empty")
	}
}

func TestSynthesizeChainsInsufficientProviders(t *testing.T) {
	g := &IdentityGraph{}
	g.AddNode(GraphNode{ID: "a", Provider: "entra"})
	g.AddNode(GraphNode{ID: "b", Provider: "aws"})
	g.AddEdge(GraphEdge{Source: "a", Target: "b", Type: "can_assume"})

	e := NewGraphEngine(g, 60)
	if _, err := e.SynthesizeChains(2, 10); err == nil {
		t.Fatal("2-provider graph should not support a 2-hop chain")
	}
}

func TestSynthesizeChainsCycleSafe(t *testing.T) {
	g := chainTestGraph(t)
	// Introduce a cycle: gcp-sa -> u1.
	g.AddEdge(GraphEdge{Source: "gcp-sa", Target: "u1", Type: "can_assume"})

	e := NewGraphEngine(g, 60)
	chains, err := e.SynthesizeChains(2, 10)
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	// Must terminate; no chain repeats a node.
	for _, c := range chains {
		seen := map[string]bool{}
		for _, id := range c.Path {
			if seen[id] {
				t.Errorf("chain repeats node %s: %v", id, c.Path)
			}
			seen[id] = true
		}
	}
}

func TestSynthesizeChainsMaxBound(t *testing.T) {
	g := chainTestGraph(t)
	// Add extra entra seeds to create multiple chains.
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("u%d", i+2)
		g.AddNode(GraphNode{ID: id, Type: "user", Provider: "entra", Label: id})
		g.AddEdge(GraphEdge{Source: id, Target: "aws-role", Type: "can_assume"})
	}

	e := NewGraphEngine(g, 60)
	chains, err := e.SynthesizeChains(2, 3)
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	if len(chains) > 3 {
		t.Errorf("chains = %d, want <= 3", len(chains))
	}
}

func TestRenderChains(t *testing.T) {
	g := chainTestGraph(t)
	e := NewGraphEngine(g, 60)
	chains, _ := e.SynthesizeChains(2, 10)

	out := RenderChains(chains)
	if !strings.Contains(out, "Cross-Cloud Chains") {
		t.Errorf("render = %q", out)
	}
	if !strings.Contains(out, "→") {
		t.Errorf("render missing chain arrow: %q", out)
	}

	empty := RenderChains(nil)
	if !strings.Contains(empty, "No multi-provider chains") {
		t.Errorf("empty render = %q", empty)
	}
}
