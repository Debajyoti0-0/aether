package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Debajyoti0-0/aether/internal/types"
)

func execTestResults() []*types.ValidationResult {
	return []*types.ValidationResult{
		{
			IsValid: true, OverallRisk: 22,
			Steps: []types.StepResult{
				{StepIndex: 0, Edge: "u1 -> MemberOf -> g1", Valid: true, RiskScore: 5, Reason: "ok"},
				{StepIndex: 1, Edge: "g1 -> AdminTo -> c1", Valid: true, RiskScore: 40, Reason: "ok"},
			},
		},
		{
			IsValid: false, OverallRisk: 50,
			Steps: []types.StepResult{
				{StepIndex: 0, Edge: "x -> DCSync -> dc1", Valid: true, RiskScore: 50, Reason: "ok"},
				{StepIndex: 1, Edge: "x -> HasSession -> y", Valid: false, RiskScore: 25, Reason: "missing node"},
			},
		},
	}
}

func TestExecutiveKPIs(t *testing.T) {
	r := Executive("ClientX", execTestResults(), []int{30, 80})

	if r.KPIs.PathsAnalyzed != 2 {
		t.Errorf("analyzed = %d", r.KPIs.PathsAnalyzed)
	}
	if r.KPIs.PathsValid != 1 {
		t.Errorf("valid = %d", r.KPIs.PathsValid)
	}
	if r.KPIs.SuccessRate != 50.0 {
		t.Errorf("success rate = %.1f", r.KPIs.SuccessRate)
	}
	if r.KPIs.MaxRisk != 50 {
		t.Errorf("max risk = %d", r.KPIs.MaxRisk)
	}
	if r.KPIs.CriticalFindings != 2 { // AdminTo=40, DCSync=50
		t.Errorf("critical = %d", r.KPIs.CriticalFindings)
	}
	if r.KPIs.DetectionExposure != "high" {
		t.Errorf("exposure = %q", r.KPIs.DetectionExposure)
	}
	if len(r.Remediations) < 3 {
		t.Errorf("remediations = %d", len(r.Remediations))
	}
	if r.Summary == "" {
		t.Error("summary empty")
	}
}

func TestExecutiveLowExposure(t *testing.T) {
	mild := []*types.ValidationResult{
		{IsValid: true, OverallRisk: 5, Steps: []types.StepResult{
			{StepIndex: 0, Edge: "u1 -> MemberOf -> g1", Valid: true, RiskScore: 5, Reason: "ok"},
		}},
	}
	r := Executive("Lab", mild, nil)
	if r.KPIs.DetectionExposure != "low" {
		t.Errorf("exposure = %q", r.KPIs.DetectionExposure)
	}
	if len(r.Remediations) == 0 {
		t.Error("should still recommend MFA/token-protection hardening")
	}
}

func TestExecutiveMarkdown(t *testing.T) {
	r := Executive("ClientX", execTestResults(), nil)
	md := r.Markdown()
	for _, want := range []string{"Executive Report", "Executive Summary", "KPIs", "Top Risks", "Remediation", "DCSync"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

func TestExecutiveWriteFiles(t *testing.T) {
	r := Executive("ClientX", execTestResults(), nil)
	base := filepath.Join(t.TempDir(), "exec")
	if err := r.WriteExecutive(base); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := os.Stat(base + ".md"); err != nil {
		t.Error("markdown missing")
	}
	if _, err := os.Stat(base + ".json"); err != nil {
		t.Error("json missing")
	}
}

func TestTopRisksCapped(t *testing.T) {
	steps := make([]types.StepResult, 10)
	for i := range steps {
		steps[i] = types.StepResult{StepIndex: i, Edge: fmt.Sprintf("a -> GenericAll -> t%d", i), Valid: true, RiskScore: 40, Reason: "ok"}
	}
	r := Executive("W", []*types.ValidationResult{{IsValid: true, OverallRisk: 40, Steps: steps}}, nil)
	if len(r.TopRisks) > 5 {
		t.Errorf("top risks = %d, want <= 5", len(r.TopRisks))
	}
}
