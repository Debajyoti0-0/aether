package validate

import (
	"testing"

	"github.com/Debajyoti0-0/aether/internal/types"
)

func TestScoreActionBase(t *testing.T) {
	s := NewRiskScorer(types.RiskInput{TimeOfDay: "off_hours"})
	score, reasoning, err := s.ScoreAction("graph_query", "tenant")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 10 {
		t.Errorf("base risk for graph_query = %d, want 10", score)
	}
	if reasoning == "" {
		t.Error("reasoning should not be empty")
	}
}

func TestScoreActionAccumulates(t *testing.T) {
	s := NewRiskScorer(types.RiskInput{
		EDRDetected:    true,
		SIEMLogging:    true,
		BlueTeamActive: true,
		TimeOfDay:      "business_hours",
	})
	// 60 + 20 + 15 + 25 + 10 = 130 → capped at 100
	score, _, err := s.ScoreAction("vm_runcommand", "vm-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 100 {
		t.Errorf("capped risk = %d, want 100", score)
	}
}

func TestScoreActionUnknown(t *testing.T) {
	s := NewRiskScorer(types.RiskInput{})
	if _, _, err := s.ScoreAction("not_a_thing", "x"); err == nil {
		t.Error("expected error for unknown action")
	}
}

func TestAboveThreshold(t *testing.T) {
	if !AboveThreshold(51, 50) {
		t.Error("51 should exceed 50")
	}
	if AboveThreshold(50, 50) {
		t.Error("50 should not exceed 50")
	}
}
