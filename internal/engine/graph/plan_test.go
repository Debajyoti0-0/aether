package graph

import (
	"strings"
	"testing"
)

func TestGeneratePlanChainsSteps(t *testing.T) {
	g := buildTestGraph(t)
	e := NewGraphEngine(g, 60)
	e.MarkTokensHeld("u1")

	plan, err := e.GeneratePlanFromPath([]string{"u1", "role:role-global-admin"}, false)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(plan.Nodes) != 1 {
		t.Fatalf("nodes = %d", len(plan.Nodes))
	}
	if plan.Nodes[0].ID != "step-1" {
		t.Errorf("id = %q", plan.Nodes[0].ID)
	}
	if !strings.Contains(plan.Nodes[0].Cmd, "aether exec azure") {
		t.Errorf("cmd = %q", plan.Nodes[0].Cmd)
	}
}

func TestGeneratePlanChainedDeps(t *testing.T) {
	// Chain with two edges: u1 -> aws-role -> gcp-sa.
	g := chainTestGraph(t)
	e := NewGraphEngine(g, 60)
	e.MarkTokensHeld("u1")

	run, err := e.QualifyPath([]string{"u1", "aws-role", "gcp-sa"})
	if err != nil {
		t.Fatal(err)
	}

	plan, err := GeneratePlan(run, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2", len(plan.Nodes))
	}
	// step-2 depends on step-1.
	if len(plan.Nodes[1].Dependencies) != 1 || plan.Nodes[1].Dependencies[0] != "step-1" {
		t.Errorf("deps = %v", plan.Nodes[1].Dependencies)
	}
}

func TestGeneratePlanCAEFallback(t *testing.T) {
	// Auth-shaped first step (has_role → exec azure uses tokens) plus a
	// relay step: both should wire the CAE fallback.
	run := &ExecutablePath{
		Path:    []string{"alice", "EC2-Admin"},
		Runbook: []string{"aether relay mfa --sts-endpoint x --tenant t"},
	}
	plan, err := GeneratePlan(run, true)
	if err != nil {
		t.Fatal(err)
	}

	if plan.Nodes[0].ID != "cae-recovery" {
		t.Errorf("first node = %q, want cae-recovery", plan.Nodes[0].ID)
	}
	foundFallback := false
	for _, n := range plan.Nodes {
		if n.Fallback == "cae-recovery" {
			foundFallback = true
		}
	}
	if !foundFallback {
		t.Error("no node wired to cae-recovery fallback")
	}
}

func TestGeneratePlanEmptyRunbook(t *testing.T) {
	if _, err := GeneratePlan(&ExecutablePath{}, false); err == nil {
		t.Error("empty runbook should fail")
	}
}

func TestPlanJSONAndSummary(t *testing.T) {
	g := buildTestGraph(t)
	e := NewGraphEngine(g, 60)
	plan, err := e.GeneratePlanFromPath([]string{"u1", "role:role-global-admin"}, false)
	if err != nil {
		t.Fatal(err)
	}

	data, err := plan.PlanJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(data, `"max_parallel"`) {
		t.Errorf("json = %s", data)
	}

	summary := plan.RenderPlanSummary()
	if !strings.Contains(summary, "Generated Plan") {
		t.Errorf("summary = %q", summary)
	}
}

func TestIsAuthStep(t *testing.T) {
	cases := map[string]bool{
		"aether prt convert --prt-file x": true,
		"aether token protect --x":        true,
		"aether relay mfa --x":            true,
		"aether exec azure --cmd id":      false,
		"aether exec aws --cmd id":        false,
	}
	for cmd, want := range cases {
		if got := isAuthStep(cmd); got != want {
			t.Errorf("isAuthStep(%q) = %t", cmd, got)
		}
	}
}
