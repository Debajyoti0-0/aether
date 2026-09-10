package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Debajyoti0-0/aether/internal/types"
)

func osWrite(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}

// EngagementKPIs are the headline metrics of an engagement.
type EngagementKPIs struct {
	PathsAnalyzed   int     `json:"paths_analyzed"`
	PathsValid      int     `json:"paths_valid"`
	SuccessRate     float64 `json:"success_rate_pct"`
	AvgRisk         int     `json:"avg_risk"`
	MaxRisk         int     `json:"max_risk"`
	CriticalFindings int    `json:"critical_findings"`
	DetectionExposure string `json:"detection_exposure"` // low/medium/high
}

// Remediation is one recommended hardening action.
type Remediation struct {
	Priority  string `json:"priority"` // P0, P1, P2
	Area      string `json:"area"`     // CAP, Token Protection, Kerberos, ...
	Title     string `json:"title"`
	Detail    string `json:"detail"`
}

// ExecutiveReport aggregates validated paths + risk assessments into a
// board-ready summary with KPIs and remediation guidance.
type ExecutiveReport struct {
	Workspace    string             `json:"workspace"`
	GeneratedAt  time.Time          `json:"generated_at"`
	KPIs         EngagementKPIs     `json:"kpis"`
	TopRisks     []string           `json:"top_risks"`
	Remediations []Remediation      `json:"remediations"`
	Summary      string             `json:"summary"`
	// MITRETechniques lists exercised technique IDs (for the PDF heatmap).
	MITRETechniques []string        `json:"mitre_techniques,omitempty"`
	// Actions backing MITRETechniques.
	Actions      []string           `json:"actions,omitempty"`
}

// Executive builds the report from validation results. The optional
// actions list drives the MITRE ATT&CK technique mapping.
func Executive(workspace string, results []*types.ValidationResult, riskScores []int, actions ...string) *ExecutiveReport {
	r := &ExecutiveReport{
		Workspace:   workspace,
		GeneratedAt: time.Now().UTC(),
	}

	r.KPIs.PathsAnalyzed = len(results)
	for _, res := range results {
		if res.IsValid {
			r.KPIs.PathsValid++
		}
		for _, s := range res.Steps {
			if s.RiskScore > r.KPIs.MaxRisk {
				r.KPIs.MaxRisk = s.RiskScore
			}
			if s.RiskScore >= 40 {
				r.KPIs.CriticalFindings++
			}
		}
		r.KPIs.AvgRisk += res.OverallRisk
	}
	r.KPIs.AvgRisk += sumInt(riskScores)
	denom := len(results) + len(riskScores)
	if denom > 0 {
		r.KPIs.AvgRisk /= denom
	}
	if r.KPIs.PathsAnalyzed > 0 {
		r.KPIs.SuccessRate = float64(r.KPIs.PathsValid) / float64(r.KPIs.PathsAnalyzed) * 100
	}

	switch {
	case r.KPIs.MaxRisk >= 50 || r.KPIs.CriticalFindings >= 2:
		r.KPIs.DetectionExposure = "high"
	case r.KPIs.MaxRisk >= 40 || r.KPIs.CriticalFindings >= 1:
		r.KPIs.DetectionExposure = "medium"
	default:
		r.KPIs.DetectionExposure = "low"
	}

	r.TopRisks = topRisks(results)
	r.Remediations = remediationsFor(results)
	if len(actions) > 0 {
		r.Actions = actions
		layer := BuildNavigatorLayer(workspace, actions)
		for _, t := range layer.Techniques {
			r.MITRETechniques = append(r.MITRETechniques, t.TechniqueID)
		}
	}
	r.Summary = r.buildSummary()
	return r
}

func sumInt(list []int) int {
	total := 0
	for _, v := range list {
		total += v
	}
	return total
}

func topRisks(results []*types.ValidationResult) []string {
	var findings []string
	for _, res := range results {
		for _, s := range res.Steps {
			if s.RiskScore >= 40 {
				findings = append(findings, fmt.Sprintf("%s (risk %d)", s.Edge, s.RiskScore))
			}
		}
	}
	sort.Strings(findings)
	if len(findings) > 5 {
		findings = findings[:5]
	}
	return findings
}

func remediationsFor(results []*types.ValidationResult) []Remediation {
	seen := map[string]bool{}
	var out []Remediation

	add := func(r Remediation) {
		if !seen[r.Title] {
			seen[r.Title] = true
			out = append(out, r)
		}
	}

	hasEdge := func(substr string) bool {
		for _, res := range results {
			for _, s := range res.Steps {
				if strings.Contains(strings.ToLower(s.Edge), substr) {
					return true
				}
			}
		}
		return false
	}

	if hasEdge("dcsync") || hasEdge("getchanges") {
		add(Remediation{Priority: "P0", Area: "Active Directory", Title: "Tighten Replication Rights",
			Detail: "DCSync-class edges detected. Audit and restrict 'Replicating Directory Changes' permissions; enable DS audit events 4662/5136."})
	}
	if hasEdge("adminto") || hasEdge("genericall") {
		add(Remediation{Priority: "P0", Area: "Privileged Access", Title: "Implement Tiered Admin Model",
			Detail: "Direct admin edges detected. Enforce tier 0/1/2 separation and remove standing admin rights via PAM/JIT."})
	}
	if hasEdge("hasession") {
		add(Remediation{Priority: "P1", Area: "Credential Exposure", Title: "Reduce Credential Exposure on Hosts",
			Detail: "Session edges detected. Deploy LSA Protection and Credential Guard; restrict local admin membership."})
	}
	if hasEdge("resetpassword") {
		add(Remediation{Priority: "P1", Area: "Delegation", Title: "Audit Reset Password Delegation",
			Detail: "Password-reset edges detected. Review GenericWrite/ResetPassword ACEs on sensitive accounts."})
	}
	if len(results) > 0 {
		add(Remediation{Priority: "P1", Area: "Identity", Title: "Enforce Phishing-Resistant MFA",
			Detail: "Validate that FIDO2/passkey enforcement is paired with Token Protection and legacy auth (WS-Trust usernamemixed) is disabled."})
		add(Remediation{Priority: "P2", Area: "Monitoring", Title: "Alert on Hybrid Identity Tokens",
			Detail: "Configure Sentinel analytics for anomalous cloud Kerberos ticket grants (Entra 50142) and PRT replay patterns."})
	}
	return out
}

func (r *ExecutiveReport) buildSummary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Engagement %s analyzed %d attack path(s): %d valid. ", r.Workspace, r.KPIs.PathsAnalyzed, r.KPIs.PathsValid)
	fmt.Fprintf(&b, "Average risk %d/100 (peak %d) with %d critical finding(s). ", r.KPIs.AvgRisk, r.KPIs.MaxRisk, r.KPIs.CriticalFindings)
	fmt.Fprintf(&b, "Detection exposure: %s. ", r.KPIs.DetectionExposure)
	if len(r.Remediations) > 0 {
		fmt.Fprintf(&b, "%d prioritized remediation(s) provided.", len(r.Remediations))
	}
	return b.String()
}

// Markdown renders the executive report.
func (r *ExecutiveReport) Markdown() string {
	var b strings.Builder
	b.WriteString("# Aether Executive Report\n\n")
	fmt.Fprintf(&b, "**Workspace:** %s  \n", r.Workspace)
	fmt.Fprintf(&b, "**Generated:** %s\n\n", r.GeneratedAt.Format(time.RFC3339))

	b.WriteString("## Executive Summary\n\n")
	b.WriteString(r.Summary + "\n\n")

	b.WriteString("## KPIs\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n|---|---|\n")
	fmt.Fprintf(&b, "| Paths analyzed | %d |\n", r.KPIs.PathsAnalyzed)
	fmt.Fprintf(&b, "| Paths valid | %d |\n", r.KPIs.PathsValid)
	fmt.Fprintf(&b, "| Success rate | %.1f%% |\n", r.KPIs.SuccessRate)
	fmt.Fprintf(&b, "| Average risk | %d/100 |\n", r.KPIs.AvgRisk)
	fmt.Fprintf(&b, "| Peak risk | %d/100 |\n", r.KPIs.MaxRisk)
	fmt.Fprintf(&b, "| Critical findings | %d |\n", r.KPIs.CriticalFindings)
	fmt.Fprintf(&b, "| Detection exposure | %s |\n\n", r.KPIs.DetectionExposure)

	if len(r.TopRisks) > 0 {
		b.WriteString("## Top Risks\n\n")
		for _, t := range r.TopRisks {
			fmt.Fprintf(&b, "- %s\n", t)
		}
		b.WriteString("\n")
	}

	if len(r.Remediations) > 0 {
		b.WriteString("## Recommended Remediation\n\n")
		for _, rem := range r.Remediations {
			fmt.Fprintf(&b, "### [%s] %s — %s\n\n%s\n\n", rem.Priority, rem.Title, rem.Area, rem.Detail)
		}
	}
	return b.String()
}

// JSON renders the report as indented JSON.
func (r *ExecutiveReport) JSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteExecutive writes both markdown and JSON files.
func (r *ExecutiveReport) WriteExecutive(basePath string) error {
	if err := osWrite(basePath+".md", []byte(r.Markdown())); err != nil {
		return err
	}
	data, err := r.JSON()
	if err != nil {
		return err
	}
	return osWrite(basePath+".json", []byte(data))
}
