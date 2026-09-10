package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/behavior"
	"github.com/Debajyoti0-0/aether/internal/engine/cap"
)

// ---------------------------------------------------------- run --behavior

// behaviorPacer builds a pacer honoring the run flags (declared in
// run.go).
func behaviorPacer() *behavior.Pacer {
	return behavior.NewPacer(behavior.Persona(runPersonaName), runSeed)
}

// behaviorWaitActive aligns execution with the persona's active window.
func behaviorWaitActive(ctx context.Context, p *behavior.Pacer, now time.Time) error {
	if runBehavior != "realistic" || p == nil {
		return nil
	}
	next := p.NextActiveTime(now)
	if !next.Equal(now) {
		fmt.Fprintf(os.Stderr, "[behavior] outside active window; waiting until %s (persona %s)\n",
			next.Format("15:04 MST"), p.Profile().Name)
	}
	return p.WaitActive(ctx, now)
}

// ---------------------------------------------------------- cap predict

var capPredictCmd = &cobra.Command{
	Use:   "predict",
	Short: "Forecast CAP activity windows from historical state changes",
	Long: `Analyze historical policy activation/deactivation observations and
forecast which policies will be active at a given time. Recommends the
least-restrictive execution window in the next 24 hours.

History JSON shape:
[{"policy_id":"mfa-window","active":true,"timestamp":"2026-09-07T09:00:00Z"}, ...]`,
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(predHistoryFile)
		if err != nil {
			return fmt.Errorf("read history: %w", err)
		}
		var observations []cap.Observation
		if err := json.Unmarshal(data, &observations); err != nil {
			return fmt.Errorf("parse history: %w", err)
		}

		p := cap.NewPredictor(observations)

		// Windows table.
		windows := p.Windows()
		fmt.Println("Inferred policy windows:")
		for _, w := range windows {
			state := fmt.Sprintf("%02d:00-%02d:00", w.ActiveFrom, w.ActiveUntil%24)
			if w.AlwaysOn {
				state = "always-on"
			}
			fmt.Printf("  %-24s %-14s confidence %.0f%%\n", w.PolicyID, state, w.Confidence*100)
		}

		at := time.Now()
		if predAt != "" {
			parsed, err := time.Parse(time.RFC3339, predAt)
			if err != nil {
				return fmt.Errorf("parse --at (RFC3339): %w", err)
			}
			at = parsed
		}

		forecast := p.Forecast(at)
		fmt.Println()
		fmt.Print(cap.RenderForecast(forecast))

		if predOut != "" {
			data, err := json.MarshalIndent(forecast, "", "  ")
			if err != nil {
				return err
			}
			if err := os.WriteFile(predOut, data, 0o600); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Forecast written to %s\n", predOut)
		}
		return nil
	},
}

var (
	predHistoryFile string
	predAt          string
	predOut         string
)

// ---------------------------------------------------------- plan generate --behavior note

func init() {
	// run flags
	runCmd.Flags().StringVar(&runBehavior, "behavior", "", "Behavior mode: realistic (human pacing) or empty (fast)")
	runCmd.Flags().StringVar(&runPersonaName, "persona", "engineer", "Persona for realistic behavior: engineer, hr, executive, analyst")
	runCmd.Flags().Int64Var(&runSeed, "seed", -1, "Deterministic seed for behavior engine (-1 = random)")
	runCmd.Flags().BoolVar(&runRespectHours, "respect-hours", false, "Delay execution until the persona's active window")

	// cap predict
	capCmd.AddCommand(capPredictCmd)
	capPredictCmd.Flags().StringVar(&predHistoryFile, "history", "", "Historical observations JSON (required)")
	capPredictCmd.Flags().StringVar(&predAt, "at", "", "Forecast time (RFC3339; default now)")
	capPredictCmd.Flags().StringVar(&predOut, "output", "", "Write forecast JSON to file")
	_ = capPredictCmd.MarkFlagRequired("history")
}

// validateBehaviorFlags rejects incompatible combinations early.
func validateBehaviorFlags() error {
	if runBehavior != "" && runBehavior != "realistic" {
		return fmt.Errorf("unknown --behavior %q (want empty or realistic)", runBehavior)
	}
	return nil
}
