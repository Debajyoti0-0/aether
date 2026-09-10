package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/engine/validate"
	"github.com/Debajyoti0-0/aether/internal/types"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Attack path validation and OPSEC scoring",
	Long:  "Validate BloodHound attack paths non-destructively and score OPSEC risk.",
}

var validatePathCmd = &cobra.Command{
	Use:   "path",
	Short: "Validate a BloodHound JSON attack path",
	RunE:  runValidatePath,
}

var validateRiskCmd = &cobra.Command{
	Use:   "risk",
	Short: "Score OPSEC risk for an action",
	RunE:  runValidateRisk,
}

var (
	valPathFile  string
	valGraphToken string
	valGraphBase string
	valRiskOut   string
	valRiskThres int
	valAction    string
	valTarget    string
	valEDR       bool
	valSIEM      bool
	valBlueTeam  bool
	valBizHours  bool
)

func init() {
	rootCmd.AddCommand(validateCmd)
	validateCmd.AddCommand(validatePathCmd, validateRiskCmd)

	validatePathCmd.Flags().StringVar(&valPathFile, "bh-json", "", "BloodHound path JSON file (required)")
	validatePathCmd.Flags().StringVar(&valGraphToken, "graph-token", "", "Optional Graph token for live user checks")
	validatePathCmd.Flags().StringVar(&valGraphBase, "graph-base", "https://graph.microsoft.com/v1.0", "Graph base URL")
	validatePathCmd.Flags().StringVar(&valRiskOut, "output", "", "Report output file (markdown)")
	validatePathCmd.Flags().IntVar(&valRiskThres, "risk-threshold", 50, "Warn when risk exceeds this")
	_ = validatePathCmd.MarkFlagRequired("bh-json")

	validateRiskCmd.Flags().StringVar(&valAction, "action", "", "Action name e.g. prt_exchange (required)")
	validateRiskCmd.Flags().StringVar(&valTarget, "target", "", "Target of the action")
	validateRiskCmd.Flags().BoolVar(&valEDR, "edr", false, "EDR detected on target")
	validateRiskCmd.Flags().BoolVar(&valSIEM, "siem", false, "SIEM logging assumed")
	validateRiskCmd.Flags().BoolVar(&valBlueTeam, "blueteam", false, "Blue team active")
	validateRiskCmd.Flags().BoolVar(&valBizHours, "business-hours", false, "It is business hours")
	validateRiskCmd.Flags().StringVar(&valRiskOut, "output", "", "Report output file (markdown)")
	validateRiskCmd.Flags().IntVar(&valRiskThres, "risk-threshold", 50, "Warn when risk exceeds this")
	_ = validateRiskCmd.MarkFlagRequired("action")
}

func runValidatePath(cmd *cobra.Command, args []string) error {
	path, err := validate.LoadPath(valPathFile)
	if err != nil {
		return err
	}

	v := validate.NewPathValidator(valGraphToken, valGraphBase, nil)
	result, err := v.ValidatePath(context.Background(), path)
	if err != nil {
		return err
	}

	report := &validate.Report{}
	report.AddPath(valPathFile, result)

	fmt.Printf("Path: %s\nValid: %t\nOverall risk: %d/100\n\nSteps:\n",
		valPathFile, result.IsValid, result.OverallRisk)
	for _, s := range result.Steps {
		marker := "OK "
		if !s.Valid {
			marker = "BAD"
		}
		fmt.Printf("  [%s] step %d (%d/100): %s — %s\n", marker, s.StepIndex, s.RiskScore, s.Edge, s.Reason)
		if s.RiskScore > valRiskThres {
			fmt.Fprintf(os.Stderr, "  [!] step %d exceeds risk threshold %d\n", s.StepIndex, valRiskThres)
		}
	}

	if valRiskOut != "" {
		if err := report.WriteMarkdown(valRiskOut); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Report written to %s\n", valRiskOut)
	}
	return nil
}

func runValidateRisk(cmd *cobra.Command, args []string) error {
	input := types.RiskInput{
		EDRDetected:    valEDR,
		SIEMLogging:    valSIEM,
		BlueTeamActive: valBlueTeam,
	}
	if valBizHours {
		input.TimeOfDay = "business_hours"
	}

	scorer := validate.NewRiskScorer(input)
	score, reasoning, err := scorer.ScoreAction(valAction, valTarget)
	if err != nil {
		return err
	}

	report := &validate.Report{}
	report.AddAction(valAction, valTarget, score, reasoning)

	fmt.Printf("Action: %s -> %s\nRisk: %d/100\nReasoning: %s\n", valAction, valTarget, score, reasoning)
	if validate.AboveThreshold(score, valRiskThres) {
		fmt.Fprintf(os.Stderr, "[!] risk %d exceeds threshold %d\n", score, valRiskThres)
	}

	if valRiskOut != "" {
		if err := report.WriteJSON(valRiskOut + ".json"); err != nil {
			return err
		}
		if err := report.WriteMarkdown(valRiskOut); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Report written to %s\n", valRiskOut)
	}
	return nil
}
