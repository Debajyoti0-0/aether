package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/engine/cap"
	"github.com/Debajyoti0-0/aether/internal/transport"
)

var capCmd = &cobra.Command{
	Use:   "cap",
	Short: "Conditional Access policy operations",
	Long:  "Parse and offline-evaluate Entra ID Conditional Access policies.",
}

var capParseCmd = &cobra.Command{
	Use:   "parse",
	Short: "Fetch or load CAP policies",
	RunE:  runCAPParse,
}

var capEvaluateCmd = &cobra.Command{
	Use:   "evaluate",
	Short: "Evaluate policies offline for a user/app pair",
	RunE:  runCAPEvaluate,
}

var (
	capToken   string
	capTenant  string
	capFile    string
	capOutput  string
	capUser    string
	capApp     string
	capPreset  string
	capTimeout int
	capGraph   string
)

func init() {
	rootCmd.AddCommand(capCmd)
	capCmd.AddCommand(capParseCmd, capEvaluateCmd)

	capParseCmd.Flags().StringVar(&capToken, "token", "", "Access token for Microsoft Graph (required)")
	capParseCmd.Flags().StringVar(&capTenant, "tenant", "", "Tenant id (required)")
	capParseCmd.Flags().StringVar(&capOutput, "output", "", "Output file for policies (JSON)")
	capParseCmd.Flags().StringVar(&capPreset, "browser-preset", "chrome", "TLS fingerprint preset")
	capParseCmd.Flags().IntVar(&capTimeout, "timeout", 30, "Request timeout seconds")
	capParseCmd.Flags().StringVar(&capGraph, "graph-base", "https://graph.microsoft.com/v1.0", "Graph API base URL")
	_ = capParseCmd.MarkFlagRequired("token")
	_ = capParseCmd.MarkFlagRequired("tenant")

	capEvaluateCmd.Flags().StringVar(&capFile, "policies", "", "Policies JSON file (from export)")
	capEvaluateCmd.Flags().StringVar(&capUser, "user", "", "Target user (UPN)")
	capEvaluateCmd.Flags().StringVar(&capApp, "app", "", "Target app (id or name)")
	capEvaluateCmd.Flags().StringVar(&capOutput, "output", "", "Output file for strategy (JSON)")
}

func runCAPParse(cmd *cobra.Command, args []string) error {
	hc, err := transport.NewClient(capPreset, time.Duration(capTimeout)*time.Second)
	if err != nil {
		return err
	}

	policies, err := cap.FetchFromGraph(context.Background(), hc, capToken, capGraph)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(policies, "", "  ")
	if err != nil {
		return err
	}
	if capOutput != "" {
		if err := os.WriteFile(capOutput, data, 0o600); err != nil {
			return err
		}
	}

	fmt.Println(cap.RenderPolicies(policies))
	fmt.Fprintf(os.Stderr, "Fetched %d policies\n", len(policies))
	return nil
}

func runCAPEvaluate(cmd *cobra.Command, args []string) error {
	if capFile == "" {
		return fmt.Errorf("--policies file is required")
	}

	policies, err := cap.ParseFromFile(capFile)
	if err != nil {
		return err
	}

	evaluator := cap.NewEvaluator(policies)
	strategy, err := evaluator.Evaluate(capUser, capApp)
	if err != nil {
		return err
	}

	if capOutput != "" {
		s, err := cap.StrategyJSON(strategy)
		if err != nil {
			return err
		}
		if err := os.WriteFile(capOutput, []byte(s), 0o600); err != nil {
			return err
		}
	}

	fmt.Println(cap.RenderStrategy(strategy))
	return nil
}
