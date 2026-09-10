package planner

import (
	"math/rand"
	"time"
)

// randSource is a small indirection over rand.Rand for the linear agent.
type randSource struct{ r *rand.Rand }

func newRNG(seed int64) *randSource {
	if seed >= 0 {
		return &randSource{r: rand.New(rand.NewSource(seed))}
	}
	return &randSource{r: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

func (r *randSource) Float64() float64 { return r.r.Float64() }
func (r *randSource) Intn(n int) int   { return r.r.Intn(n) }

// normalized fills zero hyperparameters with defaults.
func normalized(h HyperParams) HyperParams {
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
	return h
}
