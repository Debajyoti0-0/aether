package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/behavior"
	"github.com/Debajyoti0-0/aether/internal/engine/orchestrate"
	"github.com/Debajyoti0-0/aether/internal/transport"
	"github.com/Debajyoti0-0/aether/internal/workspace"
)

// runCmd implements `aether run` — full kill-chain orchestration.
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute the kill chain inside a workspace",
	Long: `Run the Aether kill chain in sequence: CAP bypass evaluation →
path validation → SOC signal prediction → engagement report.
Each phase is gated by the operator risk ceiling and paced by
--low-slow jitter when enabled.`,
	RunE: runKillChain,
}

var (
	runWorkspace    string
	runPoliciesFile string
	runPathFile     string
	runMaxRisk      int
	runLowSlow      bool
	runPassphrase   string
	runAuto         bool
	runPRTFile      string
	runBehavior     string
	runPersonaName  string
	runSeed         int64
	runRespectHours bool
)

// behaviorJitter adapts the human pacer to the orchestrator's Jitter
// interface (delays drawn from the persona's Gaussian model).
type behaviorJitter struct {
	pacer *behavior.Pacer
}

func (j *behaviorJitter) Wait(ctx context.Context) error {
	return j.pacer.Wait(ctx)
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVar(&runWorkspace, "workspace", "", "Workspace name (required)")
	runCmd.Flags().StringVar(&runPoliciesFile, "policies", "", "CAP export JSON (optional)")
	runCmd.Flags().StringVar(&runPathFile, "bh-json", "", "BloodHound path JSON (optional)")
	runCmd.Flags().StringVar(&runPRTFile, "prt-file", "", "PRT JSON to convert in the chain (optional; auto mode uses the stored PRT)")
	runCmd.Flags().IntVar(&runMaxRisk, "risk-threshold", 50, "Risk ceiling 0-100")
	runCmd.Flags().BoolVar(&runLowSlow, "low-slow", false, "Randomize 1-5s jitter between phases")
	runCmd.Flags().BoolVar(&runAuto, "auto", false, "Autonomous mode: continue past risk gates, use stored PRT")
	// Charter fix (L2): runPassphrase was declared but never registered
	// nor read — dead state. Register it as the AETHER_PASSPHRASE fallback.
	runCmd.Flags().StringVar(&runPassphrase, "passphrase", "", "Workspace passphrase (falls back to AETHER_PASSPHRASE)")
	_ = runCmd.MarkFlagRequired("workspace")
}

func runKillChain(cmd *cobra.Command, args []string) error {
	if err := validateBehaviorFlags(); err != nil {
		return err
	}

	pass := os.Getenv("AETHER_PASSPHRASE")
	if pass == "" {
		pass = runPassphrase
	}
	ws, err := workspace.Open(runWorkspace, pass)
	if err != nil {
		return err
	}

	o := &orchestrate.Orchestrator{
		WS:           ws,
		MaxRisk:      runMaxRisk,
		PoliciesFile: runPoliciesFile,
		PathFile:     runPathFile,
		PRTFile:      runPRTFile,
		Auto:         runAuto,
	}

	// Behavioral mimicry: human pacing + active-window alignment.
	var pacer *behavior.Pacer
	if runBehavior == "realistic" {
		pacer = behaviorPacer()
		o.LowSlow = &behaviorJitter{pacer: pacer}
		if runRespectHours {
			if err := behaviorWaitActive(context.Background(), pacer, time.Now()); err != nil {
				return err
			}
		}
		fmt.Fprintf(os.Stderr, "[behavior] realistic mode: persona=%s session=%s\n",
			pacer.Profile().Name, pacer.SessionLength().Round(time.Minute))
	} else if runLowSlow {
		o.LowSlow = transport.NewLowSlowJitter()
	}

	results, err := o.Run(context.Background())
	if err != nil {
		return err
	}

	for _, r := range results {
		status := "OK  "
		if !r.OK {
			status = "FAIL"
		}
		fmt.Printf("[%s] %s\n", status, r.Phase)
		if r.Output != "" {
			for _, line := range splitLines(r.Output) {
				fmt.Printf("      %s\n", line)
			}
		}
		for _, sig := range r.Signals {
			fmt.Printf("      [soc] %s\n", sig)
		}
	}
	return nil
}

func splitLines(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == '\n' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
