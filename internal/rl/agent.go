package rl

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"
)

// QAgent is a tabular Q-learning agent with epsilon-greedy exploration.
type QAgent struct {
	Q            map[string]map[string]float64 `json:"q"`
	LearningRate float64                       `json:"learning_rate"`
	Discount     float64                       `json:"discount"`
	Epsilon      float64                       `json:"epsilon"`
	EpsilonMin   float64                       `json:"epsilon_min"`
	EpsilonDecay float64                       `json:"epsilon_decay"`

	rng *rand.Rand
}

// HyperParams configures the agent.
type HyperParams struct {
	LearningRate float64 // alpha (default 0.1)
	Discount     float64 // gamma (default 0.9)
	Epsilon      float64 // initial exploration rate (default 0.3)
	EpsilonMin   float64 // floor (default 0.05)
	EpsilonDecay float64 // multiplicative decay per episode (default 0.97)
	Seed         int64   // -1 = random
}

// DefaultHyperParams returns blueprint-baseline hyperparameters.
func DefaultHyperParams() HyperParams {
	return HyperParams{LearningRate: 0.1, Discount: 0.9, Epsilon: 0.3, EpsilonMin: 0.05, EpsilonDecay: 0.97, Seed: -1}
}

// NewQAgent builds an agent from hyperparameters.
func NewQAgent(h HyperParams) *QAgent {
	if h.LearningRate <= 0 {
		h.LearningRate = 0.1
	}
	if h.Discount <= 0 {
		h.Discount = 0.9
	}
	if h.Epsilon <= 0 {
		h.Epsilon = 0.3
	}
	if h.EpsilonMin <= 0 {
		h.EpsilonMin = 0.05
	}
	if h.EpsilonDecay <= 0 || h.EpsilonDecay >= 1 {
		h.EpsilonDecay = 0.97
	}

	var rng *rand.Rand
	if h.Seed >= 0 {
		rng = rand.New(rand.NewSource(h.Seed))
	} else {
		rng = rand.New(rand.NewSource(timeNowUnixNano()))
	}

	return &QAgent{
		Q:            map[string]map[string]float64{},
		LearningRate: h.LearningRate,
		Discount:     h.Discount,
		Epsilon:      h.Epsilon,
		EpsilonMin:   h.EpsilonMin,
		EpsilonDecay: h.EpsilonDecay,
		rng:          rng,
	}
}

// qRow returns (creating if needed) the Q-row for a state.
func (a *QAgent) qRow(stateKey string) map[string]float64 {
	row, ok := a.Q[stateKey]
	if !ok {
		row = map[string]float64{}
		a.Q[stateKey] = row
	}
	return row
}

// BestAction returns the greedy action key for a state ("" if unknown).
func (a *QAgent) BestAction(stateKey string) (string, float64) {
	row := a.Q[stateKey]
	bestKey, bestVal := "", 0.0
	found := false
	for k, v := range row {
		if !found || v > bestVal || (v == bestVal && k < bestKey) {
			bestKey, bestVal, found = k, v, true
		}
	}
	return bestKey, bestVal
}

// ChooseAction selects an action key using epsilon-greedy over the
// catalog. Unvisited (state, action) pairs optimistic-init at 0.
func (a *QAgent) ChooseAction(s State) Action {
	if a.rng.Float64() < a.Epsilon {
		return a.randomAction()
	}

	row := a.qRow(s.Key())
	best, bestVal := "", 0.0
	found := false
	for _, act := range ActionCatalog {
		// Terminal phase has nothing left to do.
		if s.Phase == "done" {
			break
		}
		// Prefer forward-phase actions; heavily explore backward ones.
		v, seen := row[act.Command]
		if !seen {
			v = optimisticValue(s, act)
		}
		if !found || v > bestVal {
			best, bestVal, found = act.Command, v, true
		}
	}
	if !found {
		return a.randomAction()
	}
	a2, _ := ActionByKey(best)
	return a2
}

// optimisticValue gives unseen forward actions a small bonus so the
// greedy policy explores the workflow instead of looping in place.
func optimisticValue(s State, a Action) float64 {
	order := []string{"recon", "access", "persist", "execute", "done"}
	ci, ai := phaseIndex(order, s.Phase), phaseIndex(order, a.Phase)
	if ai > ci {
		return 0.5 // small forward bias
	}
	return -0.5 // in-phase or backward: slightly pessimistic
}

func (a *QAgent) randomAction() Action {
	// Uniform over the catalog.
	idx := a.rng.Intn(len(ActionCatalog))
	return ActionCatalog[idx]
}

// Update applies the Q-learning update:
// Q(s,a) += alpha * (r + gamma * max Q(s',·) - Q(s,a)).
func (a *QAgent) Update(s, nextState State, actionKey string, reward float64) {
	row := a.qRow(s.Key())
	nextRow := a.qRow(nextState.Key())

	maxNext := 0.0
	first := true
	for _, v := range nextRow {
		if first || v > maxNext {
			maxNext, first = v, false
		}
	}

	old := row[actionKey]
	row[actionKey] = old + a.LearningRate*(reward+a.Discount*maxNext-old)
}

// DecayEpsilon shrinks exploration after an episode.
func (a *QAgent) DecayEpsilon() {
	a.Epsilon *= a.EpsilonDecay
	if a.Epsilon < a.EpsilonMin {
		a.Epsilon = a.EpsilonMin
	}
}

// SavePolicy writes the Q-table + hyperparameters as JSON.
func (a *QAgent) SavePolicy(path string) error {
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// LoadPolicy reads a saved policy file.
func LoadPolicy(path string) (*QAgent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read policy: %w", err)
	}
	a := &QAgent{}
	if err := json.Unmarshal(data, a); err != nil {
		return nil, fmt.Errorf("parse policy: %w", err)
	}
	if a.Q == nil {
		return nil, fmt.Errorf("policy has no Q-table")
	}
	a.rng = rand.New(rand.NewSource(timeNowUnixNano()))
	return a, nil
}

// TopActions lists the best-known actions for a state, sorted by Q.
func (a *QAgent) TopActions(stateKey string, limit int) []string {
	row := a.Q[stateKey]
	type kv struct{ k string; v float64 }
	var pairs []kv
	for k, v := range row {
		pairs = append(pairs, kv{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].v == pairs[j].v {
			return pairs[i].k < pairs[j].k
		}
		return pairs[i].v > pairs[j].v
	})
	if limit > 0 && len(pairs) > limit {
		pairs = pairs[:limit]
	}
	out := make([]string, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, fmt.Sprintf("%s (%.2f)", p.k, p.v))
	}
	return out
}

// RenderPolicy prints a human-readable policy digest.
func (a *QAgent) RenderPolicy() string {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Learned Policy (states=%d, epsilon=%.2f) ===\n", len(a.Q), a.Epsilon)

	stateKeys := make([]string, 0, len(a.Q))
	for k := range a.Q {
		stateKeys = append(stateKeys, k)
	}
	sort.Strings(stateKeys)

	for _, sk := range stateKeys {
		best, val := a.BestAction(sk)
		if best == "" {
			continue
		}
		fmt.Fprintf(&b, "  %-32s → %-24s q=%.2f\n", sk, best, val)
	}
	return b.String()
}
