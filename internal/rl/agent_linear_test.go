package rl

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFeaturesEncoding(t *testing.T) {
	s := State{TokenBucket: 3, GraphDensity: "high", CAPStrictness: "strict", Phase: "done"}
	f := Features(s)

	if len(f) != FeatureLen {
		t.Fatalf("feature len = %d, want %d", len(f), FeatureLen)
	}
	if f[0] != 1.0 { // 3/3 tokens
		t.Errorf("token feature = %v", f[0])
	}
	if f[3] != 1.0 || f[6] != 1.0 || f[11] != 1.0 || f[12] != 1.0 {
		t.Errorf("one-hot encoding wrong: %v", f)
	}
	// Sibling one-hots are zero.
	if f[1] != 0 || f[2] != 0 || f[4] != 0 || f[7] != 0 {
		t.Errorf("cross-encoded features: %v", f)
	}
}

func TestLinearQValueZeroWeights(t *testing.T) {
	a := NewLinearQAgent(DefaultHyperParams())
	s := State{Phase: "recon"}
	// Zero weights + zero-initialized bias → Q = 0 (bias lives in
	// weights[12], so it starts at 0).
	if q := a.QValue(s, "graph build"); q != 0 {
		t.Errorf("q = %v, want 0", q)
	}
}

func TestLinearUpdateGeneralizes(t *testing.T) {
	// Train on one state; a *similar* state (different token bucket)
	// must inherit the learned preference — the generalization the
	// tabular agent cannot do.
	a := NewLinearQAgent(HyperParams{LearningRate: 0.5, Discount: 0.9, Epsilon: 0, EpsilonMin: 0.01, EpsilonDecay: 0.95, Seed: 1})

	seen := State{TokenBucket: 0, GraphDensity: "low", CAPStrictness: "medium", Phase: "recon"}
	unseen := State{TokenBucket: 2, GraphDensity: "high", CAPStrictness: "strict", Phase: "recon"}

	// Reward "prt convert" heavily in the seen state.
	a.Update(seen, "prt convert", 20.0, State{TokenBucket: 1, Phase: "access"})

	qSeen := a.QValue(seen, "prt convert")
	qUnseen := a.QValue(unseen, "prt convert")
	if qSeen <= 0 {
		t.Errorf("seen q = %v", qSeen)
	}
	if qUnseen <= 0 {
		t.Errorf("unseen q = %v (no generalization)", qUnseen)
	}

	// The unseen state shares phase/token features, so q must be a
	// nontrivial fraction of the seen value.
	if qUnseen < qSeen*0.2 {
		t.Errorf("weak generalization: seen=%.2f unseen=%.2f", qSeen, qUnseen)
	}
}

func TestLinearAgentConvergesOnChain(t *testing.T) {
	// Discount 0.4: token-farming (repeat prt for +10) yields
	// 10 + 0.4×20 = 18 < 20, so immediate execution is optimal —
	// demonstrating that time-discounting shapes the policy.
	// alpha=0.05: linear TD over shared one-hot features needs a small
	// step size for stability (the deadly-triad guard also applies).
	a := NewLinearQAgent(HyperParams{LearningRate: 0.05, Discount: 0.4, Epsilon: 0.4, EpsilonMin: 0.01, EpsilonDecay: 0.95, Seed: 2})

	chain := []struct {
		state  State
		action string
		next   State
		reward float64
	}{
		{State{TokenBucket: 0, GraphDensity: "low", CAPStrictness: "open", Phase: "recon"}, "graph build",
			State{TokenBucket: 0, GraphDensity: "medium", CAPStrictness: "open", Phase: "access"}, RewardPathDiscovered},
		// Token-gain transition: access state stays t0 (prt +1 token → t1).
		{State{TokenBucket: 0, GraphDensity: "medium", CAPStrictness: "open", Phase: "access"}, "prt convert",
			State{TokenBucket: 1, GraphDensity: "medium", CAPStrictness: "open", Phase: "access"}, RewardTokenGained},
		// Exec trains from the post-token state the rollout actually reaches.
		{State{TokenBucket: 1, GraphDensity: "medium", CAPStrictness: "open", Phase: "access"}, "exec azure",
			State{TokenBucket: 2, GraphDensity: "medium", CAPStrictness: "open", Phase: "done"}, RewardExecution},
	}

	for epoch := 0; epoch < 10000; epoch++ {
		for _, t3 := range chain {
			a.Update(t3.state, t3.action, t3.reward, t3.next)
		}
		a.DecayEpsilon()
	}

	// Greedy rollout from recon must terminate in the done phase with
	// exec azure executed. (In the access state, prt convert
	// bootstraps exec's value, so repeating it is mathematically
	// optimal under this reward structure — the rollout is what must
	// terminate correctly, not match a rigid sequence.)
	// Greedy rollout using the real environment Step function (not
	// strict chain lookup — linear approximation legitimately
	// generalizes exec azure to any access state, and the environment
	// confirms it succeeds there).
	cur := chain[0].state
	sawExec, reachedDone := false, false
	for step := 0; step < 6; step++ {
		best, _ := a.TopAction(cur)
		if best.Command == "exec azure" {
			sawExec = true
		}
		next, _, done := Step(cur, best, true, false)
		cur = next
		if done {
			reachedDone = true
			break
		}
	}
	if !sawExec || !reachedDone {
		t.Errorf("rollout incomplete: sawExec=%t reachedDone=%t (final state %s)", sawExec, reachedDone, cur.Key())
	}
}

func TestLinearPolicyRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "linear-policy.json")

	a := NewLinearQAgent(DefaultHyperParams())
	s := State{TokenBucket: 1, GraphDensity: "medium", CAPStrictness: "medium", Phase: "access"}
	a.Update(s, "exec azure", 15.0, State{Phase: "done"})

	if err := a.SavePolicy(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadLinearPolicy(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := loaded.QValue(s, "exec azure"); got != a.QValue(s, "exec azure") {
		t.Errorf("q mismatch after round trip: %v vs %v", got, a.QValue(s, "exec azure"))
	}
}

func TestLinearPolicyGeneralizesToNewVM(t *testing.T) {
	// The gap-analysis scenario: VM-123 vs VM-456 must not be distinct
	// states. Two states differing only in irrelevant detail (here the
	// token bucket) must share learned preferences.
	a := NewLinearQAgent(HyperParams{LearningRate: 0.4, Discount: 0.9, Epsilon: 0, EpsilonMin: 0.01, EpsilonDecay: 0.95, Seed: 5})

	vm123 := State{TokenBucket: 1, GraphDensity: "medium", CAPStrictness: "open", Phase: "execute"}
	vm456 := State{TokenBucket: 2, GraphDensity: "medium", CAPStrictness: "open", Phase: "execute"}

	a.Update(vm123, "exec azure", 20.0, State{Phase: "done"})

	// Same phase → same one-hots dominate → same preference.
	best, _ := a.TopAction(vm456)
	if best.Command != "exec azure" {
		t.Errorf("generalization failed: %q", best.Command)
	}
}

func TestLinearRenderPolicy(t *testing.T) {
	a := NewLinearQAgent(DefaultHyperParams())
	s := State{Phase: "recon"}
	a.Update(s, "graph build", 3.0, State{Phase: "access"})
	out := a.RenderPolicy()
	if !strings.Contains(out, "Linear Policy") {
		t.Errorf("render = %q", out)
	}
}
