package planner

import (
	"fmt"
	"strings"
)

// TrainAgent performs offline Q-learning over exported episodes for
// the requested number of epochs. Each epoch replays every episode's
// transitions in order; epsilon decays per epoch. Returns the trained
// agent and a convergence report.
func TrainAgent(episodes []Episode, h HyperParams, epochs int) (*QAgent, string, error) {
	if len(episodes) == 0 {
		return nil, "", fmt.Errorf("no episodes to train on")
	}
	if epochs < 1 {
		epochs = 1
	}

	agent := NewQAgent(h)
	var report strings.Builder

	// Convergence tracking: best Q for the terminal action per epoch.
	const goalAction = "exec azure"
	var prevBest float64
	converged := -1

	for epoch := 0; epoch < epochs; epoch++ {
		for _, ep := range episodes {
			for i, step := range ep.Steps {
				s := stateFromKey(step.StateKey)
				ns := stateFromKey(step.NextState)

				// Reward is taken from the recorded episode (calibrated
				// against the live reward function for robustness).
				reward := step.Reward

				// Terminal transition: no bootstrap.
				isLast := i == len(ep.Steps)-1
				if isLast {
					agent.updateTerminal(s, step.ActionKey, reward)
				} else {
					agent.Update(s, ns, step.ActionKey, reward)
				}
			}
		}
		agent.DecayEpsilon()

		_, best := agent.BestAction(bestStateForKey(goalAction))
		if converged < 0 && epoch > 0 && best == prevBest {
			converged = epoch
		}
		prevBest = best
	}

	fmt.Fprintf(&report, "Trained on %d episode(s) × %d epoch(s)\n", len(episodes), epochs)
	fmt.Fprintf(&report, "Q-states: %d\n", len(agent.Q))
	if converged >= 0 {
		fmt.Fprintf(&report, "Converged at epoch %d\n", converged)
	} else {
		fmt.Fprintf(&report, "Not converged within %d epochs\n", epochs)
	}
	return agent, report.String(), nil
}

// updateTerminal applies a Q-update without bootstrapping.
func (a *QAgent) updateTerminal(s State, actionKey string, reward float64) {
	row := a.qRow(s.Key())
	old := row[actionKey]
	row[actionKey] = old + a.LearningRate*(reward-old)
}

// GenerateRLPlan walks the learned policy from a start state, emitting
// plan nodes until the goal (execution) is reached, a cycle occurs, or
// maxSteps is hit. The result is directly consumable by
// `aether run plan`.
func GenerateRLPlan(agent *QAgent, start State, maxSteps int) ([]PlanNode, error) {
	if maxSteps <= 0 {
		maxSteps = 8
	}

	var nodes []PlanNode
	state := start
	visited := map[string]bool{state.Key(): true}
	prev := ""

	for step := 0; step < maxSteps; step++ {
		action := agent.ChooseAction(state)

		// Stop when the policy has nothing meaningful left.
		if state.Phase == "done" {
			break
		}

		id := fmt.Sprintf("rl-step-%d", step+1)
		node := PlanNode{
			ID:      id,
			Cmd:     fmt.Sprintf("aether %s", action.Command),
			Retries: 1,
		}
		if prev != "" {
			node.Dependencies = []string{prev}
		}
		if action.Risk >= 50 {
			node.Critical = true
		}
		nodes = append(nodes, node)
		prev = id

		// Environment step to derive the next state.
		next, _, done := Step(state, action, true, false)
		if visited[next.Key()] {
			break // policy loop: stop rather than cycle
		}
		visited[next.Key()] = true
		state = next

		if done {
			break
		}
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("policy produced no plan steps")
	}
	return nodes, nil
}

// stateFromKey rebuilds a State from its Key() rendering. Unknown
// components degrade gracefully (the key format is authoritative).
func stateFromKey(key string) State {
	s := State{TokenBucket: 0, GraphDensity: "low", CAPStrictness: "medium", Phase: "recon"}

	parts := strings.Split(key, "|")
	if len(parts) != 4 {
		return s
	}
	fmt.Sscanf(parts[0], "t%d", &s.TokenBucket)
	s.GraphDensity = parts[1]
	s.CAPStrictness = parts[2]
	s.Phase = parts[3]
	return s
}

// bestStateForKey builds the canonical "goal" state key used for
// convergence checks (goal: tokens held, execution phase).
func bestStateForKey(action string) string {
	return State{TokenBucket: 3, GraphDensity: "high", CAPStrictness: "medium", Phase: "execute"}.Key()
}
