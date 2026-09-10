package graph

import (
	"strings"
	"testing"
)

// correlatedTestGraph wires Entra → AWS via a has_role edge from a user
// to an AWS role node (the hybrid trust shape).
func correlatedTestGraph(t *testing.T) *IdentityGraph {
	t.Helper()
	g := buildTestGraph(t)
	// u1 (entra) can_assume the AWS role: a realistic hybrid pivot edge.
	g.AddEdge(GraphEdge{Source: "u1", Target: "arn:aws:iam::1:role/EC2-Admin", Type: "can_assume", Weight: 20})
	return g
}

func TestCorrelateFindsCrossProviderPath(t *testing.T) {
	g := correlatedTestGraph(t)
	e := NewGraphEngine(g, 60)
	e.MarkTokensHeld("u1")

	paths, err := e.Correlate(5)
	if err != nil {
		t.Fatalf("correlate: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no cross-provider paths found")
	}

	p := paths[0]
	if p.FromProvider == p.ToProvider {
		t.Errorf("path is not cross-provider: %+v", p)
	}
	if len(p.Runbook) == 0 {
		t.Error("runbook empty")
	}
	if len(p.Labels) < 2 {
		t.Errorf("labels = %v", p.Labels)
	}
}

func TestCorrelateNoCrossProvider(t *testing.T) {
	// Single-provider graph: entra only.
	g := &IdentityGraph{}
	g.AddNode(GraphNode{ID: "a", Type: "user", Provider: "entra", Label: "a"})
	g.AddNode(GraphNode{ID: "b", Type: "group", Provider: "entra", Label: "b"})
	g.AddEdge(GraphEdge{Source: "a", Target: "b", Type: "member_of"})

	e := NewGraphEngine(g, 60)
	paths, err := e.Correlate(5)
	if err != nil {
		t.Fatalf("correlate: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected 0 paths, got %d", len(paths))
	}
	if !strings.Contains(RenderCorrelated(paths), "No cross-provider paths") {
		t.Error("empty render failed")
	}
}

func TestCorrelateMaxPathsBound(t *testing.T) {
	g := correlatedTestGraph(t)
	e := NewGraphEngine(g, 60)
	e.MarkTokensHeld("u1")

	paths, err := e.Correlate(1)
	if err != nil {
		t.Fatalf("correlate: %v", err)
	}
	if len(paths) > 1 {
		t.Errorf("maxPaths=1 but got %d", len(paths))
	}
}

func TestRenderCorrelated(t *testing.T) {
	g := correlatedTestGraph(t)
	e := NewGraphEngine(g, 60)
	e.MarkTokensHeld("u1")

	paths, _ := e.Correlate(5)
	out := RenderCorrelated(paths)
	if !strings.Contains(out, "Cross-Provider Attack Paths") {
		t.Errorf("render = %q", out)
	}
	if !strings.Contains(out, "entra") || !strings.Contains(out, "aws") {
		t.Errorf("render missing providers: %q", out)
	}
	if !strings.Contains(out, "$ aether") {
		t.Errorf("render missing runbook: %q", out)
	}
}
