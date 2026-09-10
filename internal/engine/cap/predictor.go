package cap

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Observation records one historical state change of a policy.
type Observation struct {
	PolicyID  string    `json:"policy_id"`
	Active    bool      `json:"active"` // became active (true) / inactive (false)
	Timestamp time.Time `json:"timestamp"`
}

// PolicyWindow is the inferred daily activity window of a policy.
type PolicyWindow struct {
	PolicyID    string `json:"policy_id"`
	ActiveFrom  int    `json:"active_from_hour"` // local hour, inclusive
	ActiveUntil int    `json:"active_until_hour"` // local hour, exclusive
	Confidence  float64 `json:"confidence"` // 0-1 based on sample count
	AlwaysOn    bool   `json:"always_on"`
}

// Forecast answers "which policies will be active at time T" and
// recommends the least-restrictive execution window.
type Forecast struct {
	At            time.Time      `json:"at"`
	ActivePolicies []string     `json:"active_policies"`
	BestWindowStart time.Time   `json:"best_window_start"`
	BestWindowEnd time.Time     `json:"best_window_end"`
	BestWindowRisk string       `json:"best_window_risk"` // low/medium/high
	Confidence    float64       `json:"confidence"`
}

// Predictor models historical CAP state changes and forecasts when
// policies will be active (e.g., time-of-day enforcement patterns,
// business-hours MFA windows).
type Predictor struct {
	// observations keyed by policy id, sorted by timestamp.
	observations map[string][]Observation
}

// NewPredictor builds a predictor over historical observations.
func NewPredictor(observations []Observation) *Predictor {
	p := &Predictor{observations: map[string][]Observation{}}
	for _, o := range observations {
		p.observations[o.PolicyID] = append(p.observations[o.PolicyID], o)
	}
	// Sort each policy's observations chronologically.
	for id := range p.observations {
		obs := p.observations[id]
		sort.Slice(obs, func(i, j int) bool { return obs[i].Timestamp.Before(obs[j].Timestamp) })
		p.observations[id] = obs
	}
	return p
}

// Windows infers the daily activity window for each observed policy.
// A policy active in every observation is "always on". Otherwise the
// window spans the median activation hour to the median deactivation
// hour across observed days.
func (p *Predictor) Windows() []PolicyWindow {
	var out []PolicyWindow
	for id, obs := range p.observations {
		if len(obs) == 0 {
			continue
		}

		w := PolicyWindow{PolicyID: id}
		activations := []int{}
		deactivations := []int{}
		days := map[string]bool{}

		for _, o := range obs {
			day := o.Timestamp.Format("2006-01-02")
			days[day] = true
			if o.Active {
				activations = append(activations, o.Timestamp.Hour())
			} else {
				deactivations = append(deactivations, o.Timestamp.Hour())
			}
		}

		w.Confidence = confidence(len(days))

		if len(activations) == 0 {
			// Never seen activating → always on so far.
			w.AlwaysOn = true
			w.ActiveFrom, w.ActiveUntil = 0, 24
		} else if len(deactivations) == 0 {
			w.AlwaysOn = true
			w.ActiveFrom, w.ActiveUntil = 0, 24
		} else {
			w.ActiveFrom = medianHour(activations)
			w.ActiveUntil = medianHour(deactivations)
			if w.ActiveUntil <= w.ActiveFrom {
				// Window wraps midnight.
				w.ActiveUntil += 24
			}
		}
		out = append(out, w)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].PolicyID < out[j].PolicyID })
	return out
}

func medianHour(hours []int) int {
	if len(hours) == 0 {
		return 0
	}
	sort.Ints(hours)
	return hours[len(hours)/2]
}

// confidence grows with distinct observed days, saturating at 3 days
// (a full business cycle for daily patterns).
func confidence(days int) float64 {
	c := float64(days) / 3
	if c > 1 {
		c = 1
	}
	return c
}

// IsActiveAt predicts whether a policy is active at time t.
func (p *Predictor) IsActiveAt(policyID string, t time.Time) (bool, PolicyWindow, bool) {
	for _, w := range p.Windows() {
		if w.PolicyID != policyID {
			continue
		}
		if w.AlwaysOn {
			return true, w, true
		}
		h := t.Hour()
		if w.ActiveUntil > 24 {
			// Wraps midnight: active if h >= from OR h < until-24.
			return h >= w.ActiveFrom || h < w.ActiveUntil-24, w, true
		}
		return h >= w.ActiveFrom && h < w.ActiveUntil, w, true
	}
	// No observations: unknown.
	return false, PolicyWindow{}, false
}

// Forecast evaluates every observed policy at time t and finds the
// least-restrictive execution window in the next 24 hours.
func (p *Predictor) Forecast(t time.Time) *Forecast {
	f := &Forecast{At: t.UTC()}

	windows := p.Windows()
	// Active now.
	for _, w := range windows {
		active, _, known := p.IsActiveAt(w.PolicyID, t)
		if known && active {
			f.ActivePolicies = append(f.ActivePolicies, w.PolicyID)
		}
	}
	// Confidence = min over predicted policies.
	f.Confidence = 1
	for _, w := range windows {
		if w.Confidence < f.Confidence {
			f.Confidence = w.Confidence
		}
	}

	// Best window: scan the next 24h hour-by-hour and pick the hour
	// with the fewest active policies (ties → earliest).
	best := struct {
		t    time.Time
		n    int
	}{n: 1 << 30}

	hour := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
	for i := 0; i < 24; i++ {
		candidate := hour.Add(time.Duration(i) * time.Hour)
		active := 0
		for _, w := range windows {
			if a, _, known := p.IsActiveAt(w.PolicyID, candidate); known && a {
				active++
			}
		}
		if active < best.n {
			best = struct {
				t    time.Time
				n    int
			}{candidate, active}
		}
	}

	f.BestWindowStart = best.t
	f.BestWindowEnd = best.t.Add(time.Hour)
	switch {
	case best.n == 0:
		f.BestWindowRisk = "low"
	case best.n <= 2:
		f.BestWindowRisk = "medium"
	default:
		f.BestWindowRisk = "high"
	}
	return f
}

// RenderForecast formats a forecast for the terminal.
func RenderForecast(f *Forecast) string {
	var b strings.Builder
	fmt.Fprintf(&b, "=== CAP Forecast @ %s (confidence %.0f%%) ===\n",
		f.At.Format("2006-01-02 15:04 MST"), f.Confidence*100)
	if len(f.ActivePolicies) > 0 {
		fmt.Fprintf(&b, "Active now: %s\n", strings.Join(f.ActivePolicies, ", "))
	} else {
		b.WriteString("Active now: none\n")
	}
	fmt.Fprintf(&b, "Least-restrictive window: %s → %s (risk %s)\n",
		f.BestWindowStart.Format("15:04 MST"), f.BestWindowEnd.Format("15:04 MST"), f.BestWindowRisk)
	return b.String()
}
