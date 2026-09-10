package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Debajyoti0-0/aether/internal/types"
)

// Report is an engagement record bundling validation + risk output.
type Report struct {
	GeneratedAt time.Time              `json:"generated_at"`
	Paths       []PathReport           `json:"paths,omitempty"`
	Actions     []ActionReport         `json:"actions,omitempty"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
}

// PathReport is one validated path.
type PathReport struct {
	Source    string                `json:"source_file"`
	IsValid   bool                  `json:"is_valid"`
	OverallRisk int                 `json:"overall_risk"`
	Steps     []types.StepResult    `json:"steps"`
}

// ActionReport is one risk-scored action.
type ActionReport struct {
	Action    string `json:"action"`
	Target    string `json:"target"`
	Risk      int    `json:"risk"`
	Reasoning string `json:"reasoning"`
}

// AddPath appends a validated path to the report.
func (r *Report) AddPath(source string, result *types.ValidationResult) {
	r.Paths = append(r.Paths, PathReport{
		Source:      source,
		IsValid:     result.IsValid,
		OverallRisk: result.OverallRisk,
		Steps:       result.Steps,
	})
}

// AddAction appends a scored action to the report.
func (r *Report) AddAction(action, target string, risk int, reasoning string) {
	r.Actions = append(r.Actions, ActionReport{Action: action, Target: target, Risk: risk, Reasoning: reasoning})
}

// WriteJSON writes the report as JSON.
func (r *Report) WriteJSON(path string) error {
	r.GeneratedAt = time.Now().UTC()
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// Markdown renders the report as a human-readable markdown document.
func (r *Report) Markdown() string {
	var b strings.Builder

	b.WriteString("# Aether Engagement Report\n\n")
	fmt.Fprintf(&b, "Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339))

	if len(r.Paths) > 0 {
		b.WriteString("## Attack Path Validation\n\n")
		for _, p := range r.Paths {
			status := "VALID"
			if !p.IsValid {
				status = "INVALID"
			}
			fmt.Fprintf(&b, "### %s — %s (risk %d/100)\n\n", p.Source, status, p.OverallRisk)
			b.WriteString("| Step | Edge | Valid | Risk | Reason |\n|---|---|---|---|---|\n")
			for _, s := range p.Steps {
				fmt.Fprintf(&b, "| %d | `%s` | %t | %d | %s |\n",
					s.StepIndex, s.Edge, s.Valid, s.RiskScore, strings.ReplaceAll(s.Reason, "|", "\\|"))
			}
			b.WriteString("\n")
		}
	}

	if len(r.Actions) > 0 {
		b.WriteString("## OPSEC Risk Assessment\n\n")
		b.WriteString("| Action | Target | Risk | Reasoning |\n|---|---|---|---|\n")
		for _, a := range r.Actions {
			fmt.Fprintf(&b, "| %s | %s | %d/100 | %s |\n", a.Action, a.Target, a.Risk, a.Reasoning)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// WriteMarkdown writes the report as markdown.
func (r *Report) WriteMarkdown(path string) error {
	return os.WriteFile(path, []byte(r.Markdown()), 0o600)
}
