package planner

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
)

// LinearQAgent approximates Q(s,a) with a linear weight vector over a
// fixed feature encoding. Unlike the tabular agent, it generalizes to
// unseen environments: the agent learns *concepts* (e.g., "more edges
// means higher risk") instead of memorizing specific states.
type LinearQAgent struct {
	// Weights[actionKey][featureIndex].
	Weights map[string][]float64 `json:"weights"`
	// Bias[actionKey].
	Bias map[string]float64 `json:"bias"`

	LearningRate float64 `json:"learning_rate"`
	Discount     float64 `json:"discount"`
	Epsilon      float64 `json:"epsilon"`
	EpsilonMin   float64 `json:"epsilon_min"`
	EpsilonDecay float64 `json:"epsilon_decay"`

	rng *randSource
}

// FeatureVec is the fixed-length feature encoding of a state:
// [tokenBucket(0-3)/3, densityLow, densityMed, densityHigh,
//  capOpen, capMed, capStrict, phaseRecon, phaseAccess,
//  phasePersist, phaseExecute, phaseDone, 1.0 (bias term)].
const FeatureLen = 13

// Features encodes a state into the fixed-length vector.
func Features(s State) []float64 {
	v := make([]float64, FeatureLen)
	v[0] = float64(s.TokenBucket) / 3.0

	switch s.GraphDensity {
	case "low":
		v[1] = 1
	case "medium":
		v[2] = 1
	case "high":
		v[3] = 1
	}

	switch s.CAPStrictness {
	case "open":
		v[4] = 1
	case "medium":
		v[5] = 1
	case "strict":
		v[6] = 1
	}

	switch s.Phase {
	case "recon":
		v[7] = 1
	case "access":
		v[8] = 1
	case "persist":
		v[9] = 1
	case "execute":
		v[10] = 1
	case "done":
		v[11] = 1
	}

	v[12] = 1.0 // bias term
	return v
}

// NewLinearQAgent builds a linear-approximation agent (zero weights).
func NewLinearQAgent(h HyperParams) *LinearQAgent {
	normalized(h)
	return &LinearQAgent{
		Weights:      map[string][]float64{},
		Bias:         map[string]float64{},
		LearningRate: h.LearningRate,
		Discount:     h.Discount,
		Epsilon:      h.Epsilon,
		EpsilonMin:   h.EpsilonMin,
		EpsilonDecay: h.EpsilonDecay,
		rng:          newRNG(h.Seed),
	}
}

// weightsFor lazily initializes the weight vector for an action.
func (a *LinearQAgent) weightsFor(actionKey string) []float64 {
	w, ok := a.Weights[actionKey]
	if !ok {
		w = make([]float64, FeatureLen)
		a.Weights[actionKey] = w
	}
	return w
}

// QValue computes Q(s,a) = w_a · features(s).
func (a *LinearQAgent) QValue(s State, actionKey string) float64 {
	w := a.weightsFor(actionKey)
	f := Features(s)
	sum := 0.0
	for i := range f {
		sum += w[i] * f[i]
	}
	return sum
}

// ChooseAction selects an action catalog entry via epsilon-greedy
// over linear Q-values. Terminal states yield the terminal action.
func (a *LinearQAgent) ChooseAction(s State) Action {
	if s.Phase == "done" {
		return ActionCatalog[len(ActionCatalog)-1]
	}
	if a.rng.Float64() < a.Epsilon {
		return a.randomCatalogAction()
	}

	best, bestVal := "", math.Inf(-1)
	for _, act := range ActionCatalog {
		v := a.QValue(s, act.Command)
		// Forward-phase bonus for unseen progress (mirrors tabular).
		if a.weightsUnset(act.Command) && phaseIndex(phaseOrder(), s.Phase) < phaseIndex(phaseOrder(), act.Phase) {
			v += 0.5
		}
		if v > bestVal || best == "" {
			best, bestVal = act.Command, v
		}
	}
	act, _ := ActionByKey(best)
	return act
}

func (a *LinearQAgent) weightsUnset(actionKey string) bool {
	w, ok := a.Weights[actionKey]
	return !ok || allZero(w)
}

func allZero(w []float64) bool {
	for _, x := range w {
		if x != 0 {
			return false
		}
	}
	return true
}

func (a *LinearQAgent) randomCatalogAction() Action {
	return ActionCatalog[a.rng.Intn(len(ActionCatalog))]
}

// weightClamp bounds weights to prevent the classic linear-TD
// divergence (deadly triad: function approximation + bootstrapping).
// The correct fixed point for the reward scale used sits well below
// this bound, so clamping only stops runaway oscillation.
const weightClamp = 1000.0

// Update applies the linear TD update:
// w += alpha * (r + gamma * maxQ(s',·) − Q(s,a)) * features(s),
// with weights clamped to ±weightClamp for stability.
func (a *LinearQAgent) Update(s State, actionKey string, reward float64, nextState State) {
	maxNext := math.Inf(-1)
	for _, act := range ActionCatalog {
		if v := a.QValue(nextState, act.Command); v > maxNext {
			maxNext = v
		}
	}
	if nextState.Phase == "done" {
		maxNext = 0 // terminal: no bootstrap
	}

	tdError := reward + a.Discount*maxNext - a.QValue(s, actionKey)
	w := a.weightsFor(actionKey)
	f := Features(s)
	for i := range f {
		w[i] += a.LearningRate * tdError * f[i]
		if w[i] > weightClamp {
			w[i] = weightClamp
		} else if w[i] < -weightClamp {
			w[i] = -weightClamp
		}
	}
}

// DecayEpsilon shrinks exploration after an episode.
func (a *LinearQAgent) DecayEpsilon() {
	a.Epsilon *= a.EpsilonDecay
	if a.Epsilon < a.EpsilonMin {
		a.Epsilon = a.EpsilonMin
	}
}

// SavePolicy writes the linear policy as JSON.
func (a *LinearQAgent) SavePolicy(path string) error {
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// LoadLinearPolicy reads a saved linear policy.
func LoadLinearPolicy(path string) (*LinearQAgent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read linear policy: %w", err)
	}
	a := &LinearQAgent{}
	if err := json.Unmarshal(data, a); err != nil {
		return nil, fmt.Errorf("parse linear policy: %w", err)
	}
	if a.Weights == nil {
		return nil, fmt.Errorf("linear policy has no weights")
	}
	a.rng = newRNG(-1)
	return a, nil
}

// TopAction returns the greedy action for a state with its Q-value.
func (a *LinearQAgent) TopAction(s State) (Action, float64) {
	best, bestVal := Action{}, math.Inf(-1)
	for _, act := range ActionCatalog {
		if v := a.QValue(s, act.Command); v > bestVal || best.Command == "" {
			best, bestVal = act, v
		}
	}
	return best, bestVal
}

// RenderPolicy prints a digest of learned weights.
func (a *LinearQAgent) RenderPolicy() string {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Linear Policy (actions=%d, epsilon=%.2f) ===\n", len(a.Weights), a.Epsilon)

	actionKeys := make([]string, 0, len(a.Weights))
	for k := range a.Weights {
		actionKeys = append(actionKeys, k)
	}
	sortStrings(actionKeys)

	featureNames := []string{"tokens", "density-low", "density-med", "density-high",
		"cap-open", "cap-med", "cap-strict", "phase-recon", "phase-access",
		"phase-persist", "phase-execute", "phase-done", "bias"}
	for _, ak := range actionKeys {
		w := a.weightsFor(ak)
		// Show the two dominant features.
		i1, i2 := topTwo(w)
		fmt.Fprintf(&b, "  %-24s %s=%.2f  %s=%.2f\n", ak, featureNames[i1], w[i1], featureNames[i2], w[i2])
	}
	return b.String()
}

func topTwo(w []float64) (int, int) {
	i1, i2 := 0, 1
	if math.Abs(w[1]) > math.Abs(w[0]) {
		i1, i2 = 1, 0
	}
	for i := 2; i < len(w)-1; i++ { // exclude bias (last)
		if math.Abs(w[i]) > math.Abs(w[i1]) {
			i2 = i1
			i1 = i
		} else if math.Abs(w[i]) > math.Abs(w[i2]) {
			i2 = i
		}
	}
	return i1, i2
}

func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

func phaseOrder() []string {
	return []string{"recon", "access", "persist", "execute", "done"}
}
