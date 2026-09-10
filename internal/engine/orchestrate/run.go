package orchestrate

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Debajyoti0-0/aether/internal/engine/cap"
	"github.com/Debajyoti0-0/aether/internal/engine/validate"
	"github.com/Debajyoti0-0/aether/internal/transport"
	"github.com/Debajyoti0-0/aether/internal/workspace"
)

// Phase names of the Aether kill chain.
const (
	PhaseBypass    = "bypass"    // CAP evaluation + strategy
	PhaseConvert   = "convert"   // PRT → OAuth
	PhaseValidate  = "validate"  // path validation
	PhasePredict   = "predict"   // SOC signal prediction
	PhaseReport    = "report"    // engagement report
)

// Orchestrator executes the kill chain inside a workspace.
type Orchestrator struct {
	WS           *workspace.Workspace
	MaxRisk      int
	LowSlow      transport.Jitter
	PRTFile      string
	PoliciesFile string
	PathFile     string
}

// PhaseResult is the outcome of one kill-chain phase.
type PhaseResult struct {
	Phase   string   `json:"phase"`
	OK      bool     `json:"ok"`
	Output  string   `json:"output,omitempty"`
	Skipped bool     `json:"skipped,omitempty"`
	Signals []string `json:"signals,omitempty"`
}

// Run executes the chain in order, honoring risk gates and jitter.
func (o *Orchestrator) Run(ctx context.Context) ([]PhaseResult, error) {
	if o.MaxRisk <= 0 {
		o.MaxRisk = 50
	}
	if o.LowSlow == nil {
		o.LowSlow = NoJitter{}
	}

	var results []PhaseResult

	// Phase: CAP bypass strategy.
	results = append(results, o.runPhase(ctx, PhaseBypass, o.phaseBypass))

	// Phase: path validation.
	results = append(results, o.runPhase(ctx, PhaseValidate, o.phaseValidate))

	// Phase: SOC prediction.
	results = append(results, o.runPhase(ctx, PhasePredict, o.phasePredict))

	// Phase: report.
	results = append(results, o.runPhase(ctx, PhaseReport, o.phaseReport))

	return results, nil
}

func (o *Orchestrator) runPhase(ctx context.Context, name string, fn func(context.Context) (string, []string, error)) PhaseResult {
	res := PhaseResult{Phase: name}

	if err := o.LowSlow.Wait(ctx); err != nil {
		res.OK = false
		res.Output = "cancelled: " + err.Error()
		return res
	}

	output, signals, err := fn(ctx)
	res.Output = output
	res.Signals = signals
	res.OK = err == nil
	if err != nil && !res.Skipped {
		res.Output = strings.TrimSpace(output + "\nerror: " + err.Error())
	}
	return res
}

func (o *Orchestrator) phaseBypass(ctx context.Context) (string, []string, error) {
	if o.PoliciesFile == "" {
		return "", nil, nil // skipped silently
	}
	policies, err := cap.ParseFromFile(o.PoliciesFile)
	if err != nil {
		return "", nil, err
	}
	strategy, err := cap.NewEvaluator(policies).Evaluate("", "")
	if err != nil {
		return "", nil, err
	}

	if strategy.RiskLevel > o.MaxRisk {
		return cap.RenderStrategy(strategy),
			[]string{fmt.Sprintf("CAP strategy risk %d exceeds ceiling %d — abort recommended", strategy.RiskLevel, o.MaxRisk)},
			fmt.Errorf("risk gate: %d > %d", strategy.RiskLevel, o.MaxRisk)
	}
	return cap.RenderStrategy(strategy), nil, nil
}

func (o *Orchestrator) phaseValidate(ctx context.Context) (string, []string, error) {
	if o.PathFile == "" {
		return "", nil, nil
	}
	path, err := validate.LoadPath(o.PathFile)
	if err != nil {
		return "", nil, err
	}
	result, err := validate.NewPathValidator("", "", nil).ValidatePath(ctx, path)
	if err != nil {
		return "", nil, err
	}

	signals := []string{}
	if result.OverallRisk > o.MaxRisk {
		signals = append(signals, fmt.Sprintf("path risk %d exceeds ceiling %d", result.OverallRisk, o.MaxRisk))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Path valid: %t (risk %d/100)\n", result.IsValid, result.OverallRisk)
	for _, s := range result.Steps {
		fmt.Fprintf(&b, "  step %d: %s [%t] risk %d\n", s.StepIndex, s.Edge, s.Valid, s.RiskScore)
	}
	return b.String(), signals, nil
}

func (o *Orchestrator) phasePredict(ctx context.Context) (string, []string, error) {
	p := validate.NewPredictor()
	env := &validate.Environment{TimeOfDay: "business_hours"}

	var lines []string
	for _, action := range []string{"prt_exchange", "policy_export", "path_validation"} {
		for _, s := range p.Simulate(action, env) {
			lines = append(lines, fmt.Sprintf("%s: Entra %s (%s) — %s", action, s.EntraIDLogID, s.EntraIDLogName, s.Probability))
		}
	}
	return strings.Join(lines, "\n"), lines, nil
}

func (o *Orchestrator) phaseReport(ctx context.Context) (string, []string, error) {
	events, err := o.WS.Events()
	if err != nil {
		return "", nil, err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Workspace %s journal: %d events\n", o.WS.Name, len(events))
	for _, ev := range events {
		fmt.Fprintf(&b, "  %s — %s: %s\n", ev.Time.Format(time.RFC3339), ev.Kind, ev.Detail)
	}

	if err := o.WS.LogEvent("run_completed", fmt.Sprintf("max_risk=%d", o.MaxRisk)); err != nil {
		return b.String(), nil, err
	}
	return b.String(), nil, nil
}

// NoJitter is the default (no delay between phases).
type NoJitter struct{}

// Wait returns immediately.
func (NoJitter) Wait(context.Context) error { return nil }
