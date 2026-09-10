package planner

import (
	"fmt"
	"testing"
)

func TestDebugLinearLearning(t *testing.T) {
	a := NewLinearQAgent(HyperParams{LearningRate: 0.05, Discount: 0.4, Epsilon: 0, EpsilonMin: 0.01, EpsilonDecay: 0.95, Seed: 2})

	chain := []struct {
		state  State
		action string
		next   State
		reward float64
	}{
		{State{TokenBucket: 0, GraphDensity: "low", CAPStrictness: "open", Phase: "recon"}, "graph build",
			State{TokenBucket: 0, GraphDensity: "medium", CAPStrictness: "open", Phase: "access"}, RewardPathDiscovered},
		{State{TokenBucket: 0, GraphDensity: "medium", CAPStrictness: "open", Phase: "access"}, "prt convert",
			State{TokenBucket: 1, GraphDensity: "medium", CAPStrictness: "open", Phase: "access"}, RewardTokenGained},
		{State{TokenBucket: 1, GraphDensity: "medium", CAPStrictness: "open", Phase: "access"}, "exec azure",
			State{TokenBucket: 2, GraphDensity: "medium", CAPStrictness: "open", Phase: "done"}, RewardExecution},
	}

	for epoch := 0; epoch < 10000; epoch++ {
		for _, t3 := range chain {
			a.Update(t3.state, t3.action, t3.reward, t3.next)
		}
	}

	for _, t3 := range chain {
		best, q := a.TopAction(t3.state)
		fmt.Printf("state %s\n  best=%s q=%.3f (want %s)\n", t3.state.Key(), best.Command, q, t3.action)
		for _, act := range ActionCatalog {
			fmt.Printf("    %-24s q=%.3f\n", act.Command, a.QValue(t3.state, act.Command))
		}
	}
}
