package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/engine/graph"
	"github.com/Debajyoti0-0/aether/internal/engine/mutation"
	"github.com/Debajyoti0-0/aether/internal/engine/rollback"
	"github.com/Debajyoti0-0/aether/internal/engine/validate"
	"github.com/Debajyoti0-0/aether/internal/store"
	"github.com/Debajyoti0-0/aether/internal/transport"
	"github.com/Debajyoti0-0/aether/internal/workspace"
)

// ---------------------------------------------------------------- graph visualize

var graphVisualizeCmd = &cobra.Command{
	Use:   "visualize",
	Short: "Export the identity graph as an interactive HTML visualization",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := graph.LoadGraph(visGraphFile)
		if err != nil {
			return err
		}
		if visOut == "" {
			visOut = strings.TrimSuffix(visGraphFile, ".json") + ".html"
		}
		if err := graph.SaveVisualize(g, visTitle, visOut); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Visualization written to %s (%d nodes, %d edges)\n", visOut, len(g.Nodes), len(g.Edges))
		return nil
	},
}

var (
	visGraphFile string
	visOut       string
	visTitle     string
)

// ---------------------------------------------------------------- plan generate

var planGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Auto-generate a DAG plan from a graph path",
	Long: `Qualify a node path against the identity graph and emit a ready-to-run
DAG plan (consumable by 'aether run plan'). Auth-shaped steps get a
CAE claims-handler recovery node when --cae-fallback is set.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := graph.LoadGraph(genGraphFile)
		if err != nil {
			return err
		}
		engine := graph.NewGraphEngine(g, genRisk)

		var pathIDs []string
		for _, id := range splitComma(genPath) {
			pathIDs = append(pathIDs, id)
		}

		plan, err := engine.GeneratePlanFromPath(pathIDs, genCAEFallback)
		if err != nil {
			return err
		}

		data, err := plan.PlanJSON()
		if err != nil {
			return err
		}
		if genOut != "" {
			if err := os.WriteFile(genOut, []byte(data), 0o600); err != nil {
				return err
			}
		}
		fmt.Print(plan.RenderPlanSummary())
		return nil
	},
}

var (
	genGraphFile  string
	genPath       string
	genOut        string
	genRisk       int
	genCAEFallback bool
)

// ---------------------------------------------------------------- rollback

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Revert engagement changes (deterministic cleanup)",
	Long: `Replay the reverse-action stack recorded during an engagement:
added SP secrets are removed, group memberships reverted, dispatched
pipelines noted. Undo runs newest-first and survives partial failure.`,
}

var rollbackPushCmd = &cobra.Command{
	Use:   "push",
	Short: "Record a reverse action manually",
	RunE: func(cmd *cobra.Command, args []string) error {
		w, err := workspace.Open(rbWorkspace, os.Getenv("AETHER_PASSPHRASE"))
		if err != nil {
			return err
		}
		s := w.RollbackStack()

		if err := s.Push(rollback.Action{
			Kind:   rbKind,
			Target: rbTarget,
			Detail: rbDetail,
			Undo: rollback.UndoAction{
				Provider: rbProvider,
				Op:       rbOp,
				Command:  rbUndoCmd,
			},
		}); err != nil {
			return err
		}
		depth, _ := s.Len()
		fmt.Printf("Recorded: %s → %s (stack depth %d)\n", rbKind, rbTarget, depth)
		return nil
	},
}

var rollbackUndoCmd2 = &cobra.Command{
	Use:   "undo",
	Short: "Undo all recorded actions (newest first)",
	RunE: func(cmd *cobra.Command, args []string) error {
		w, err := workspace.Open(rbWorkspace, os.Getenv("AETHER_PASSPHRASE"))
		if err != nil {
			return err
		}
		s := w.RollbackStack()

		outcomes, err := s.UndoAll(context.Background(), func(ctx context.Context, a *rollback.Action) error {
			// Stage 2 (F7): undo commands execute through the Action
			// spine via the intent whitelist — the CLI root is never
			// re-entered. Provider primitives without an executable
			// command are honestly reported as requiring manual reversal.
			if a.Undo.Command != "" {
				res, err := runIntent(ctx, w, a.Undo.Command, "rollback-undo")
				if err != nil {
					return err
				}
				fmt.Printf("undo action %s status %s\n", res.ActionID, res.Status)
				return nil
			}
			return fmt.Errorf("provider %q op %q requires manual reversal (recorded in report)", a.Undo.Provider, a.Undo.Op)
		})
		if err != nil && len(outcomes) == 0 {
			return err
		}
		fmt.Print(rollback.RenderOutcomes(outcomes))
		return nil
	},
}

var rollbackListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recorded reversible actions",
	RunE: func(cmd *cobra.Command, args []string) error {
		w, err := workspace.Open(rbWorkspace, os.Getenv("AETHER_PASSPHRASE"))
		if err != nil {
			return err
		}
		s := w.RollbackStack()

		actions, err := s.List()
		if err != nil {
			return err
		}
		if len(actions) == 0 {
			fmt.Println("Rollback stack is empty.")
			return nil
		}
		for i, a := range actions {
			fmt.Printf("%d. %-18s %-20s undo=%s/%s\n", i+1, a.Kind, a.Target, a.Undo.Provider, a.Undo.Op)
		}
		return nil
	},
}

var (
	rbWorkspace string
	rbKind      string
	rbTarget    string
	rbDetail    string
	rbProvider  string
	rbOp        string
	rbUndoCmd   string
)

func rollbackPath(w *workspace.Workspace) string {
	return w.Root + string(os.PathSeparator) + "db" + string(os.PathSeparator) + "rollback.jsonl"
}

// ---------------------------------------------------------------- export attck

var exportAttckCmd = &cobra.Command{
	Use:   "attck",
	Short: "Export exercised techniques as a MITRE ATT&CK Navigator layer",
	RunE: func(cmd *cobra.Command, args []string) error {
		actions := splitComma(attckActions)
		if len(actions) == 0 {
			return fmt.Errorf("--actions is required (comma-separated: prt_exchange, imds_hijack, sp_hijack, ...)")
		}
		layer := validate.BuildNavigatorLayer(attckWorkspace, actions)

		if attckOut == "" {
			attckOut = attckWorkspace + "-navigator.json"
		}
		data, err := layer.NavigatorJSON()
		if err != nil {
			return err
		}
		if err := os.WriteFile(attckOut, []byte(data), 0o600); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Navigator layer written to %s (%d techniques)\n", attckOut, len(layer.Techniques))
		fmt.Println(data)
		return nil
	},
}

var (
	attckWorkspace string
	attckActions   string
	attckOut       string
)

// ---------------------------------------------------------------- simulate stream

var simulateStreamCmd = &cobra.Command{
	Use:   "stream",
	Short: "Stream fuzzed telemetry directly into Splunk HEC (governed: audited)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := openGovernedWorkspace(execWorkspace)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(streamSeed)
		if err != nil {
			return fmt.Errorf("read seed: %w", err)
		}
		var base map[string]any
		if err := json.Unmarshal(data, &base); err != nil {
			return fmt.Errorf("parse seed: %w", err)
		}

		variants := validate.FuzzTelemetry(base, streamCount)

		hec := validate.NewHECClient(streamHEC, streamToken, "aether", "aether:sim", streamIndex)
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(streamTimeout)*time.Second)
		defer cancel()

		var n int
		_, err = mutation.Run(ctx, ws, &cliMutation{
			kindV:   "simulate.stream",
			targetV: streamHEC,
			undoV: &mutation.UndoSpec{
				Irreversible: true,
				Detail:       "injected telemetry cannot be recalled from the target SIEM",
			},
			execFn: func(ctx context.Context) (string, error) {
				count, err := hec.StreamFuzzBatch(ctx, variants)
				n = count
				return "", err
			},
		})
		if err != nil {
			return err
		}
		fmt.Printf("Streamed %d telemetry variants to %s\n", n, streamHEC)
		return nil
	},
}

var (
	streamHEC     string
	streamToken   string
	streamIndex   string
	streamSeed    string
	streamCount   int
	streamTimeout int
)

// ---------------------------------------------------------------- env profile

var envProfileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Engagement profile presets (banking, red-team, incident-response, ...)",
}

var envProfileShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show all built-in profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print(validate.RenderProfiles())
		return nil
	},
}

var envProfileApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply a profile to a config file",
	RunE: func(cmd *cobra.Command, args []string) error {
		profile, err := validate.GetProfile(profileName)
		if err != nil {
			return err
		}

		// Load existing config or start from defaults.
		cfg, err := storeLoadConfig(profileOut)
		if err != nil {
			return err
		}
		cfg.MaxRiskThreshold = profile.MaxRiskThreshold
		cfg.BrowserPreset = profile.BrowserPreset
		if profile.LowSlow {
			cfg.LowSlow = true
		}
		if profile.AuditLogging {
			cfg.LogLevel = "debug"
		}
		if profileOut == "aether.json" {
			_ = os.Setenv("AETHER_PROFILE", profile.Name)
		}

		if err := storeSaveConfig(profileOut, cfg); err != nil {
			return err
		}
		fmt.Printf("Profile %q applied to %s\n  risk ceiling: %d\n  browser preset: %s\n  low-slow: %t\n  audit logging: %t\n",
			profile.Name, profileOut, cfg.MaxRiskThreshold, cfg.BrowserPreset, cfg.LowSlow, profile.AuditLogging)
		return nil
	},
}

var (
	profileName string
	profileOut  string
)

func storeLoadConfig(path string) (*store.Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return store.DefaultConfig(), nil
	}
	return store.LoadConfig(path)
}

func storeSaveConfig(path string, cfg *store.Config) error {
	return store.SaveConfig(path, cfg)
}

// ---------------------------------------------------------------- tunnel (proxy)

var tunnelCmd = &cobra.Command{
	Use:   "tunnel",
	Short: "Single proxy reachability probe via an external proxy (HTTP/SOCKS5) — not a tunnel",
	Long:  `Sends ONE GET request through the configured proxy and reports the status code and TLS preset used. There is no persistent tunnel; use this to verify proxy routing before other commands.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := transport.NewClientWithOptions(transport.Options{
			Preset:  tunnelPreset,
			Timeout: 30,
			ProxyURL: tunnelProxy,
			RotateJA4: tunnelRotate,
		})
		if err != nil {
			return err
		}

		// Probe the proxy by fetching a plain URL.
		resp, err := client.Get(tunnelTestURL)
		if err != nil {
			return fmt.Errorf("proxy probe %s via %s: %w", tunnelTestURL, tunnelProxy, err)
		}
		defer resp.Body.Close()
		fmt.Printf("Proxy %s → %s returned HTTP %d (TLS fingerprint: %s)\n",
			tunnelProxy, tunnelTestURL, resp.StatusCode, tunnelPreset)
		return nil
	},
}

var (
	tunnelProxy    string
	tunnelTestURL  string
	tunnelPreset   string
	tunnelRotate   bool
)

func init() {
	graphCmd.AddCommand(graphVisualizeCmd, planGenerateCmd)
	rootCmd.AddCommand(rollbackCmd, tunnelCmd)
	rollbackCmd.AddCommand(rollbackPushCmd, rollbackUndoCmd2, rollbackListCmd)
	exportCmd.AddCommand(exportAttckCmd)
	simulateCmd.AddCommand(simulateStreamCmd)
	validateCmd.AddCommand(envProfileCmd)

	graphVisualizeCmd.Flags().StringVar(&visGraphFile, "graph", "", "Graph JSON file (required)")
	graphVisualizeCmd.Flags().StringVar(&visOut, "output", "", "Output HTML file")
	graphVisualizeCmd.Flags().StringVar(&visTitle, "title", "", "Visualization title")
	_ = graphVisualizeCmd.MarkFlagRequired("graph")

	planGenerateCmd.Flags().StringVar(&genGraphFile, "graph", "", "Graph JSON file (required)")
	planGenerateCmd.Flags().StringVar(&genPath, "path", "", "Comma-separated node IDs (required)")
	planGenerateCmd.Flags().StringVar(&genOut, "output", "", "Write plan JSON to file")
	planGenerateCmd.Flags().IntVar(&genRisk, "risk-threshold", 60, "Risk ceiling")
	planGenerateCmd.Flags().BoolVar(&genCAEFallback, "cae-fallback", true, "Wire CAE claims-handler fallback into auth steps")
	_ = planGenerateCmd.MarkFlagRequired("graph")
	_ = planGenerateCmd.MarkFlagRequired("path")

	for _, c := range []*cobra.Command{rollbackPushCmd, rollbackUndoCmd2, rollbackListCmd} {
		c.Flags().StringVar(&rbWorkspace, "workspace", "", "Workspace name (required)")
		_ = c.MarkFlagRequired("workspace")
	}
	rollbackPushCmd.Flags().StringVar(&rbKind, "kind", "", "Action kind e.g. sp_secret (required)")
	rollbackPushCmd.Flags().StringVar(&rbTarget, "target", "", "Action target (required)")
	rollbackPushCmd.Flags().StringVar(&rbDetail, "detail", "", "Human detail")
	rollbackPushCmd.Flags().StringVar(&rbProvider, "provider", "entra", "Reversal provider")
	rollbackPushCmd.Flags().StringVar(&rbOp, "op", "", "Reversal primitive e.g. remove_password")
	rollbackPushCmd.Flags().StringVar(&rbUndoCmd, "undo-cmd", "", "Full aether command to execute on undo")
	_ = rollbackPushCmd.MarkFlagRequired("kind")
	_ = rollbackPushCmd.MarkFlagRequired("target")

	exportAttckCmd.Flags().StringVar(&attckWorkspace, "workspace", "", "Engagement label for the Navigator layer title (inputs come from --actions)")
	exportAttckCmd.Flags().StringVar(&attckActions, "actions", "", "Comma-separated actions (required)")
	exportAttckCmd.Flags().StringVar(&attckOut, "output", "", "Output Navigator JSON file")
	_ = exportAttckCmd.MarkFlagRequired("workspace")
	_ = exportAttckCmd.MarkFlagRequired("actions")

	simulateStreamCmd.Flags().StringVar(&streamHEC, "hec-url", "", "Splunk HEC endpoint (required)")
	simulateStreamCmd.Flags().StringVar(&streamToken, "hec-token", "", "HEC token (required)")
	simulateStreamCmd.Flags().StringVar(&streamIndex, "index", "redteam", "Splunk index")
	simulateStreamCmd.Flags().StringVar(&streamSeed, "seed", "", "Seed telemetry JSON (required)")
	simulateStreamCmd.Flags().IntVar(&streamCount, "count", 20, "Variant count")
	simulateStreamCmd.Flags().IntVar(&streamTimeout, "timeout", 30, "Timeout seconds")
	simulateStreamCmd.Flags().StringVar(&execWorkspace, "workspace", "", "Workspace for audit+rollback records (required)")
	_ = simulateStreamCmd.MarkFlagRequired("hec-url")
	_ = simulateStreamCmd.MarkFlagRequired("hec-token")
	_ = simulateStreamCmd.MarkFlagRequired("seed")

	envProfileCmd.AddCommand(envProfileShowCmd, envProfileApplyCmd)
	envProfileApplyCmd.Flags().StringVar(&profileName, "profile", "", "Profile name (required)")
	envProfileApplyCmd.Flags().StringVar(&profileOut, "output", "aether.json", "Config file to write")
	_ = envProfileApplyCmd.MarkFlagRequired("profile")

	tunnelCmd.Flags().StringVar(&tunnelProxy, "proxy", "", "Proxy URL: http(s):// or socks5:// (required)")
	tunnelCmd.Flags().StringVar(&tunnelTestURL, "test-url", "https://login.microsoftonline.com/common/discovery/instance", "URL to probe through the proxy")
	tunnelCmd.Flags().StringVar(&tunnelPreset, "browser-preset", "chrome", "TLS preset")
	tunnelCmd.Flags().BoolVar(&tunnelRotate, "rotate-ja4", false, "Rotate TLS fingerprints per connection")
	_ = tunnelCmd.MarkFlagRequired("proxy")
}
