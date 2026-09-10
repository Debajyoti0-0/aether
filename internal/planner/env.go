package planner

import (
	"fmt"
	"strings"
)

// State is a discretized environment snapshot. Discretization keeps the
// tabular Q-table small: counts are bucketed, densities named.
type State struct {
	// TokenBucket: 0, 1, 2, 3 (3 = 3+ tokens held).
	TokenBucket int
	// GraphDensity: low, medium, high.
	GraphDensity string
	// CAPStrictness: open, medium, strict.
	CAPStrictness string
	// Phase: recon, access, persist, execute, done.
	Phase string
	// WorkspaceID scopes the state (not part of the Q-key).
	WorkspaceID string
}

// BucketTokens clamps a raw token count into the 0-3 bucket.
func BucketTokens(n int) int {
	if n < 0 {
		return 0
	}
	if n > 3 {
		return 3
	}
	return n
}

// DensityFromCounts maps node/edge counts to a density name.
func DensityFromCounts(nodes, edges int) string {
	if nodes == 0 {
		return "low"
	}
	switch ratio := float64(edges) / float64(nodes); {
	case ratio < 0.5:
		return "low"
	case ratio < 2:
		return "medium"
	default:
		return "high"
	}
}

// StrictnessFromControls maps active CAP control count to strictness.
func StrictnessFromControls(activeControls int) string {
	switch {
	case activeControls <= 0:
		return "open"
	case activeControls <= 2:
		return "medium"
	default:
		return "strict"
	}
}

// Key renders the state as a compact Q-table key.
func (s State) Key() string {
	return fmt.Sprintf("t%d|%s|%s|%s", s.TokenBucket, s.GraphDensity, s.CAPStrictness, s.Phase)
}

// Action is a parameterized aether command the agent can choose.
type Action struct {
	Command string `json:"command"`
	Risk    int    `json:"risk"`
	// Phase is the workflow stage this action belongs to.
	Phase string `json:"phase"`
	// Gain models the utility of a successful execution (token gained,
	// path discovered, execution achieved).
	Gain string `json:"gain"` // none, token, path, execution
}

// Key renders the action as a Q-table key.
func (a Action) Key() string {
	return a.Command
}

// ActionCatalog is the fixed action space (parameterized commands with
// their OPSEC risk and expected gain).
var ActionCatalog = []Action{
	{Command: "graph build", Risk: 10, Phase: "recon", Gain: "path"},
	{Command: "cap evaluate", Risk: 8, Phase: "recon", Gain: "path"},
	{Command: "prt convert", Risk: 30, Phase: "access", Gain: "token"},
	{Command: "relay cae-handler", Risk: 25, Phase: "access", Gain: "token"},
	{Command: "token protect", Risk: 32, Phase: "access", Gain: "token"},
	{Command: "exec azure", Risk: 60, Phase: "execute", Gain: "execution"},
	{Command: "exec aws", Risk: 60, Phase: "execute", Gain: "execution"},
	{Command: "relay fido2-downgrade", Risk: 45, Phase: "access", Gain: "token"},
	{Command: "pivot cloud-to-onprem", Risk: 55, Phase: "persist", Gain: "execution"},
}

// ActionByKey finds a catalog action by its command.
func ActionByKey(key string) (Action, bool) {
	for _, a := range ActionCatalog {
		if a.Command == key {
			return a, true
		}
	}
	return Action{}, false
}

// Rewards calibrate the learning signal.
const (
	RewardTokenGained    = 10.0
	RewardPathDiscovered = 4.0
	RewardExecution      = 20.0
	RewardFailure        = -10.0
	RewardHighRisk       = -5.0
	RewardStepCost       = -1.0
	RewardDetection      = -15.0
)

// Transition is one (state, action, reward, nextState) record.
type Transition struct {
	StateKey   string
	ActionKey  string
	Reward     float64
	NextState  string
	Done       bool
}

// Step applies an action to a state and returns the reward + outcome.
// success models the environment response; detected models a SOC hit.
func Step(s State, a Action, success, detected bool) (State, float64, bool) {
	reward := RewardStepCost

	if detected {
		reward += RewardDetection
	}
	if a.Risk >= 50 {
		reward += RewardHighRisk
	}

	next := s
	done := false

	if !success {
		reward += RewardFailure
		return next, reward, false
	}

	// Phase progression: actions of the next stage move the phase.
	next.Phase = advancePhase(s.Phase, a.Phase)

	// Progress: state advances along the workflow when the action's
	// phase matches or moves forward. Gains only pay when the state
	// actually changed — otherwise an infinite reward-farming loop
	// (e.g. repeated prt convert at max tokens) makes Q diverge.
	switch a.Gain {
	case "token":
		if next.TokenBucket < 3 {
			reward += RewardTokenGained
			next.TokenBucket++
		}
	case "path":
		if next.GraphDensity != "high" {
			reward += RewardPathDiscovered
			next.GraphDensity = densify(next.GraphDensity)
		}
	case "execution":
		reward += RewardExecution
		next.Phase = "done"
		done = true
	}

	return next, reward, done
}

func densify(d string) string {
	switch d {
	case "low":
		return "medium"
	case "medium":
		return "high"
	}
	return d
}

// advancePhase moves the workflow stage to the executed action's phase
// (only forward — never backward).
func advancePhase(current, actionPhase string) string {
	order := []string{"recon", "access", "persist", "execute", "done"}
	ci, ai := indexOfPhase(order, current), indexOfPhase(order, actionPhase)
	if ai > ci {
		return order[ai]
	}
	return current
}

func indexOfPhase(order []string, phase string) int {
	for i, p := range order {
		if p == phase {
			return i
		}
	}
	return 0
}

// phaseIndex is the canonical phase lookup used by the agent.
func phaseIndex(order []string, phase string) int {
	return indexOfPhase(order, phase)
}

// PlanNode is a DAG node emitted by RL plan generation (mirrors the
// graph plan shape consumed by `aether run plan`).
type PlanNode struct {
	ID           string   `json:"id"`
	Cmd          string   `json:"cmd"`
	Dependencies []string `json:"dependencies,omitempty"`
	Fallback     string   `json:"fallback,omitempty"`
	Retries      int      `json:"retries,omitempty"`
	Critical     bool     `json:"critical,omitempty"`
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// RenderStates prints a list of state keys (debug/report helper).
func RenderStates(keys []string) string {
	return strings.Join(keys, "\n")
}
