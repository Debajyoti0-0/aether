package rl

import (
	"strings"
	"testing"
)

// Charter fix (H5): the trainer must only emit spine-governable steps.
func TestIsSpineGovernable(t *testing.T) {
	governable := []string{"exec azure", "exec aws", "exec github", "exec gcp"}
	for _, g := range governable {
		if !isSpineGovernable(g) {
			t.Errorf("%q should be governable", g)
		}
	}
	for _, b := range []string{"graph build", "prt convert", "validate", "simulate stream", ""} {
		if isSpineGovernable(b) {
			t.Errorf("%q must not be governable (run plan would mint a guaranteed-to-fail node)", b)
		}
	}
}

func TestGenerateRLPlanSpineOnly(t *testing.T) {
	agent := NewQAgent(HyperParams{LearningRate: 0.1, Discount: 0.9})
	agent.Epsilon = 0 // deterministic: always take the learned best action

	start := State{TokenBucket: 3, GraphDensity: "high", CAPStrictness: "medium", Phase: "execute"}
	// The best learned action at the start state is a catalog command the
	// Action spine cannot represent. Before the fix, the trainer minted
	// `aether graph build` as a plan step — `run plan` guaranteed failure.
	agent.Q[start.Key()] = map[string]float64{"graph build": 5, "validate": 4}

	nodes, err := GenerateRLPlan(agent, start, 8)
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "no plan steps") {
		// A policy that produces nothing governable degrades to an empty
		// plan error — acceptable. Anything else is unexpected.
		t.Fatalf("unexpected trainer error: %v", err)
	}
	for _, n := range nodes {
		cmd := strings.TrimPrefix(n.Cmd, "aether ")
		if !isSpineGovernable(cmd) {
			t.Errorf("trainer minted non-governable plan step %q (charter fix H5)", n.Cmd)
		}
	}
}
