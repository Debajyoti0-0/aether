package rl

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestStateKeyFormat(t *testing.T) {
	s := State{TokenBucket: 2, GraphDensity: "medium", CAPStrictness: "strict", Phase: "access"}
	if got := s.Key(); got != "t2|medium|strict|access" {
		t.Errorf("key = %q", got)
	}
}

func TestBucketTokens(t *testing.T) {
	cases := map[int]int{-1: 0, 0: 0, 1: 1, 3: 3, 7: 3}
	for in, want := range cases {
		if got := BucketTokens(in); got != want {
			t.Errorf("BucketTokens(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestDensityAndStrictness(t *testing.T) {
	if d := DensityFromCounts(10, 3); d != "low" {
		t.Errorf("density = %q", d)
	}
	if d := DensityFromCounts(10, 15); d != "medium" {
		t.Errorf("density = %q", d)
	}
	if d := DensityFromCounts(5, 20); d != "high" {
		t.Errorf("density = %q", d)
	}
	if s := StrictnessFromControls(0); s != "open" {
		t.Errorf("strictness = %q", s)
	}
	if s := StrictnessFromControls(3); s != "strict" {
		t.Errorf("strictness = %q", s)
	}
}

func TestStepRewards(t *testing.T) {
	s := State{Phase: "recon", GraphDensity: "low", TokenBucket: 0}

	// Token gain on success (action carries its workflow phase).
	next, reward, done := Step(s, Action{Command: "prt convert", Risk: 30, Gain: "token", Phase: "access"}, true, false)
	if reward != RewardStepCost+RewardTokenGained {
		t.Errorf("token reward = %.1f", reward)
	}
	if next.TokenBucket != 1 {
		t.Errorf("token bucket = %d", next.TokenBucket)
	}
	if done {
		t.Error("token gain should not terminate")
	}

	// Execution terminates with the big reward.
	next, reward, done = Step(s, Action{Command: "exec azure", Risk: 60, Gain: "execution", Phase: "execute"}, true, false)
	if !done || reward != RewardStepCost+RewardExecution+RewardHighRisk {
		t.Errorf("execution reward = %.1f done=%t", reward, done)
	}
	if next.Phase != "done" {
		t.Errorf("phase = %q", next.Phase)
	}

	// Failure + detection is heavily penalized.
	_, reward, _ = Step(s, Action{Command: "exec aws", Risk: 60, Gain: "execution", Phase: "execute"}, false, true)
	want := RewardStepCost + RewardFailure + RewardHighRisk + RewardDetection
	if reward != want {
		t.Errorf("failure reward = %.1f, want %.1f", reward, want)
	}
}

func TestPhaseProgression(t *testing.T) {
	s := State{Phase: "recon", GraphDensity: "low", TokenBucket: 0}
	next, _, _ := Step(s, Action{Command: "prt convert", Risk: 30, Gain: "token", Phase: "access"}, true, false)
	if next.Phase != "access" {
		t.Errorf("phase = %q, want access", next.Phase)
	}
}

func TestAgentConvergesWithin100Episodes(t *testing.T) {
	// Deterministic mock environment: recon → access → execute chain.
	agent := NewQAgent(HyperParams{LearningRate: 0.3, Discount: 0.9, Epsilon: 0.5, EpsilonMin: 0.01, EpsilonDecay: 0.95, Seed: 1})

	chain := []Action{
		{Command: "graph build", Risk: 10, Phase: "recon", Gain: "path"},
		{Command: "prt convert", Risk: 30, Phase: "access", Gain: "token"},
		{Command: "exec azure", Risk: 60, Phase: "execute", Gain: "execution"},
	}

	converged := false
	for ep := 0; ep < 100 && !converged; ep++ {
		state := State{TokenBucket: 0, GraphDensity: "low", CAPStrictness: "medium", Phase: "recon"}

		for step := 0; step < 6; step++ {
			// Stop when the episode reached the goal phase.
			if state.Phase == "done" {
				break
			}
			var action Action
			if step < len(chain) {
				action = chain[step]
			} else {
				action = agent.ChooseAction(state)
			}
			success := action.Command == chain[step].Command
			next, reward, done := Step(state, action, success, false)
			agent.Update(state, next, action.Command, reward)
			state = next
			if done {
				break
			}
		}
		agent.DecayEpsilon()

		// Convergence: greedy path from start reaches exec azure.
		cur := State{TokenBucket: 0, GraphDensity: "low", CAPStrictness: "medium", Phase: "recon"}
		ok := true
		for step := 0; step < 3; step++ {
			a := agent.ChooseAction(cur)
			if a.Command != chain[step].Command {
				ok = false
				break
			}
			next, _, _ := Step(cur, a, true, false)
			cur = next
		}
		if ok {
			converged = true
		}
	}

	if !converged {
		t.Fatal("agent did not converge within 100 episodes")
	}
}

func TestPolicyRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")

	agent := NewQAgent(DefaultHyperParams())
	// Seed some Q-values.
	state := State{TokenBucket: 0, GraphDensity: "low", CAPStrictness: "medium", Phase: "recon"}
	agent.Update(state, State{Phase: "access"}, "prt convert", 5)

	if err := agent.SavePolicy(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadPolicy(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got, val := loaded.BestAction(state.Key())
	if got != "prt convert" || val <= 0 {
		t.Errorf("best action = %q (%.2f)", got, val)
	}
}

func TestLoadPolicyMissing(t *testing.T) {
	if _, err := LoadPolicy(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Error("missing policy should fail")
	}
}

func TestEpisodeStoreRoundTrip(t *testing.T) {
	s := NewEpisodeStore(filepath.Join(t.TempDir(), "episodes.jsonl"))

	ep := Episode{Workspace: "ClientX"}
	state := State{Phase: "recon"}
	next := State{Phase: "access"}
	ep.Steps = append(ep.Steps, EpisodeStep{StateKey: state.Key(), ActionKey: "prt convert", Reward: 9, NextState: next.Key()})

	if err := s.Append(ep); err != nil {
		t.Fatalf("append: %v", err)
	}

	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 1 || len(loaded[0].Steps) != 1 || loaded[0].Steps[0].ActionKey != "prt convert" {
		t.Errorf("loaded = %+v", loaded)
	}
}

func TestTrainAgentFromEpisodes(t *testing.T) {
	s := NewEpisodeStore(filepath.Join(t.TempDir(), "episodes.jsonl"))
	for i := 0; i < 5; i++ {
		s.Append(BuildEpisode("ClientX", []JournalEvent{
			{Kind: "graph_built", Detail: "ok"},
			{Kind: "prt_converted", Detail: "ok"},
			{Kind: "command_executed", Detail: "succeeded"},
		}))
	}

	episodes, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}

	agent, report, err := TrainAgent(episodes, DefaultHyperParams(), 20)
	if err != nil {
		t.Fatalf("train: %v", err)
	}
	if !strings.Contains(report, "Trained on 5 episode") {
		t.Errorf("report = %q", report)
	}
	if len(agent.Q) == 0 {
		t.Error("empty Q-table after training")
	}
}

func TestBuildEpisodeMapsEvents(t *testing.T) {
	ep := BuildEpisode("W", []JournalEvent{
		{Kind: "graph_built", Detail: "ok"},
		{Kind: "command_executed", Detail: "failed"},
	})
	if len(ep.Steps) != 2 {
		t.Fatalf("steps = %d", len(ep.Steps))
	}
	if ep.Steps[0].ActionKey != "graph build" {
		t.Errorf("step0 = %q", ep.Steps[0].ActionKey)
	}
	// The failed execution earns a failure penalty.
	if ep.Steps[1].Reward >= 0 {
		t.Errorf("failed step reward = %.1f", ep.Steps[1].Reward)
	}
}

func TestGenerateRLPlan(t *testing.T) {
	// Train on the deterministic chain, then generate a plan.
	agent := NewQAgent(HyperParams{LearningRate: 0.3, Discount: 0.9, Epsilon: 0.5, EpsilonMin: 0.01, EpsilonDecay: 0.95, Seed: 3})
	chain := []Action{
		{Command: "graph build", Risk: 10, Phase: "recon", Gain: "path"},
		{Command: "prt convert", Risk: 30, Phase: "access", Gain: "token"},
		{Command: "exec azure", Risk: 60, Phase: "execute", Gain: "execution"},
	}

	for ep := 0; ep < 100; ep++ {
		state := State{TokenBucket: 0, GraphDensity: "low", CAPStrictness: "medium", Phase: "recon"}
		for step := 0; step < 6; step++ {
			if state.Phase == "done" {
				break
			}
			var action Action
			if step < len(chain) {
				action = chain[step]
			} else {
				action = agent.ChooseAction(state)
			}
			success := action.Command == chain[step].Command
			next, reward, done := Step(state, action, success, false)
			agent.Update(state, next, action.Command, reward)
			state = next
			if done {
				break
			}
		}
		agent.DecayEpsilon()
	}

	start := State{TokenBucket: 0, GraphDensity: "low", CAPStrictness: "medium", Phase: "recon"}
	nodes, err := GenerateRLPlan(agent, start, 8)
	if err != nil {
		t.Fatalf("generate rl plan: %v", err)
	}
	if len(nodes) == 0 {
		t.Fatal("plan empty")
	}
	// Nodes chained by dependencies.
	if len(nodes) > 1 && nodes[1].Dependencies[0] != nodes[0].ID {
		t.Errorf("chain broken: %+v", nodes)
	}
	// High-risk exec node is critical.
	last := nodes[len(nodes)-1]
	if strings.Contains(last.Cmd, "exec") && !last.Critical {
		t.Errorf("exec node should be critical: %+v", last)
	}
}

func TestGenerateRLPlanCycleSafe(t *testing.T) {
	// Untrained agent: policy loops → plan must terminate via visited set.
	// Stage 43 adjustment (charter fix 4 follow-up): the spine-governable
	// whitelist may filter every sampled action of an untrained walk, in
	// which case the trainer returns the bounded empty-plan outcome
	// (empty-plan error). Termination + the max-step bound are the intent
	// of this test; both outcomes honor them.
	agent := NewQAgent(DefaultHyperParams())
	start := State{TokenBucket: 0, GraphDensity: "low", CAPStrictness: "medium", Phase: "recon"}
	nodes, err := GenerateRLPlan(agent, start, 8)
	if err != nil && !strings.Contains(err.Error(), "no plan steps") {
		t.Fatalf("generate: %v", err)
	}
	if len(nodes) > 8 {
		t.Errorf("plan exceeded max steps: %d", len(nodes))
	}
}
