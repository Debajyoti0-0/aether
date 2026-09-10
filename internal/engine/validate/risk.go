package validate

import (
	"fmt"
	"strings"
	"time"

	"github.com/Debajyoti0-0/aether/internal/types"
)

// baseRisk maps an action to its base OPSEC risk.
var baseRisk = map[string]int{
	"token_refresh":     5,
	"graph_query":       10,
	"prt_exchange":      30,
	"saml_forge":        45,
	"wstrust_relay":     40,
	"vm_runcommand":     60,
	"ssm_runcommand":    60,
	"ci_dispatch":       35,
	"policy_export":     10,
	"path_validation":   8,
}

// RiskScorer assigns unified OPSEC risk scores to actions.
type RiskScorer struct {
	Input types.RiskInput
}

// NewRiskScorer builds a scorer from environmental factors.
func NewRiskScorer(input types.RiskInput) *RiskScorer {
	if input.TimeOfDay == "" {
		input.TimeOfDay = inferTimeOfDay()
	}
	return &RiskScorer{Input: input}
}

// ScoreAction returns the risk score (0-100) and its reasoning.
func (r *RiskScorer) ScoreAction(action, target string) (int, string, error) {
	score, ok := baseRisk[strings.ToLower(action)]
	if !ok {
		return 0, "", fmt.Errorf("unknown action %q", action)
	}
	start := score

	if r.Input.EDRDetected {
		score += 20
	}
	if r.Input.SIEMLogging {
		score += 15
	}
	if r.Input.BlueTeamActive {
		score += 25
	}
	if r.Input.TimeOfDay == "business_hours" {
		score += 10
	}

	if score > 100 {
		score = 100
	}

	reasoning := fmt.Sprintf(
		"Base risk %d (%s on %s) + EDR(%t)+20 + SIEM(%t)+15 + BlueTeam(%t)+25 + Time(%s)+%d",
		start, action, target,
		r.Input.EDRDetected, r.Input.SIEMLogging, r.Input.BlueTeamActive,
		r.Input.TimeOfDay, bonusForTime(r.Input.TimeOfDay))

	return score, reasoning, nil
}

func bonusForTime(t string) int {
	if t == "business_hours" {
		return 10
	}
	return 0
}

// inferTimeOfDay guesses the time bucket from the local clock.
func inferTimeOfDay() string {
	h := time.Now().Hour()
	if h >= 9 && h < 18 {
		return "business_hours"
	}
	return "off_hours"
}

// AboveThreshold reports whether a score breaches the operator's max.
func AboveThreshold(score, threshold int) bool {
	return score > threshold
}
