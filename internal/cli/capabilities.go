package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/engine/cap"
	"github.com/Debajyoti0-0/aether/internal/engine/graph"
	"github.com/Debajyoti0-0/aether/internal/engine/validate"
	"github.com/Debajyoti0-0/aether/internal/types"
	"github.com/Debajyoti0-0/aether/internal/version"
	"github.com/Debajyoti0-0/aether/internal/workspace"
)

func contextBackground() context.Context { return context.Background() }

func secDuration(n int) time.Duration { return time.Duration(n) * time.Second }

func timeNowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

// capExploitCmd — live CAP bypass simulation.
var capExploitCmd = &cobra.Command{
	Use:   "exploit",
	Short: "Simulate a live CAP bypass (OS/IP/Browser spoofing)",
	RunE:  runCAPExploit,
}

// capMatrixCmd — policy decision matrix.
var capMatrixCmd = &cobra.Command{
	Use:   "matrix",
	Short: "Build the user×app policy decision matrix",
	RunE:  runCAPMatrix,
}

var (
	exploitUser    string
	exploitApp     string
	exploitProbe   string
	exploitPreset  string
	exploitTimeout int
	exploitIP      string
	matrixPolicies string
	matrixUsers    []string
	matrixApps     []string
	matrixOutput   string
)

func init() {
	capCmd.AddCommand(capExploitCmd, capMatrixCmd)

	capExploitCmd.Flags().StringVar(&capFile, "policies", "", "Policies JSON file (required)")
	capExploitCmd.Flags().StringVar(&exploitUser, "user", "", "Target user UPN")
	capExploitCmd.Flags().StringVar(&exploitApp, "app", "", "Target app id/name")
	capExploitCmd.Flags().StringVar(&exploitProbe, "probe", "", "URL to probe with spoofed headers (optional; offline without it)")
	capExploitCmd.Flags().StringVar(&exploitIP, "spoof-ip", "", "Source IP to claim (X-Forwarded-For)")
	capExploitCmd.Flags().StringVar(&exploitPreset, "browser-preset", "chrome", "TLS preset")
	capExploitCmd.Flags().IntVar(&exploitTimeout, "timeout", 30, "Timeout seconds")
	_ = capExploitCmd.MarkFlagRequired("policies")

	capMatrixCmd.Flags().StringVar(&matrixPolicies, "policies", "", "Policies JSON file (required)")
	capMatrixCmd.Flags().StringSliceVar(&matrixUsers, "users", nil, "Users to evaluate (required)")
	capMatrixCmd.Flags().StringSliceVar(&matrixApps, "apps", nil, "Apps to evaluate (required)")
	capMatrixCmd.Flags().StringVar(&matrixOutput, "output", "", "Write matrix JSON to file")
	_ = capMatrixCmd.MarkFlagRequired("policies")
	_ = capMatrixCmd.MarkFlagRequired("users")
	_ = capMatrixCmd.MarkFlagRequired("apps")
}

func runCAPExploit(cmd *cobra.Command, args []string) error {
	policies, err := cap.ParseFromFile(capFile)
	if err != nil {
		return err
	}

	sim := cap.NewSimulator(cap.NewEvaluator(policies), exploitPreset, secDuration(exploitTimeout))
	result, err := sim.Simulate(contextBackground(), exploitUser, exploitApp, exploitProbe)
	if err != nil {
		return err
	}
	if exploitIP != "" {
		result.Env.IP = exploitIP
		result.Headers["X-Forwarded-For"] = exploitIP
	}
	fmt.Print(cap.RenderProbe(result))
	return nil
}

func runCAPMatrix(cmd *cobra.Command, args []string) error {
	policies, err := cap.ParseFromFile(matrixPolicies)
	if err != nil {
		return err
	}

	rows := cap.BuildMatrix(policies, matrixUsers, matrixApps)

	if matrixOutput != "" {
		data, err := cap.MatrixJSON(rows)
		if err != nil {
			return err
		}
		if err := os.WriteFile(matrixOutput, []byte(data), 0o600); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Matrix written to %s\n", matrixOutput)
	}
	fmt.Print(cap.RenderMatrix(rows))
	return nil
}

// graphCorrelateCmd — cross-provider path discovery.
var graphCorrelateCmd = &cobra.Command{
	Use:   "correlate",
	Short: "Discover cross-cloud attack paths (Entra → AWS → GCP)",
	RunE:  runGraphCorrelate,
}

var (
	correlateGraph   string
	correlateMaxPath int
	correlateMaxRisk int
)

func init() {
	graphCmd.AddCommand(graphCorrelateCmd)
	graphCorrelateCmd.Flags().StringVar(&correlateGraph, "graph", "", "Graph JSON file (required)")
	graphCorrelateCmd.Flags().IntVar(&correlateMaxPath, "max-paths", 3, "Max paths per provider pair")
	graphCorrelateCmd.Flags().IntVar(&correlateMaxRisk, "risk-threshold", 60, "Risk ceiling")
	_ = graphCorrelateCmd.MarkFlagRequired("graph")
}

func runGraphCorrelate(cmd *cobra.Command, args []string) error {
	g, err := graph.LoadGraph(correlateGraph)
	if err != nil {
		return err
	}
	engine := graph.NewGraphEngine(g, correlateMaxRisk)
	paths, err := engine.Correlate(correlateMaxPath)
	if err != nil {
		return err
	}
	fmt.Print(graph.RenderCorrelated(paths))
	return nil
}

// simulateCmd — SOC telemetry emulation for detection engineering.
var simulateCmd = &cobra.Command{
	Use:   "simulate",
	Short: "Emulate SOC telemetry and log signals (detection engineering)",
	RunE:  runSimulate,
}

var (
	simAction    string
	simLocation  bool
	simDevice    bool
	simTimeOfDay string
	simJSON      bool
)

func init() {
	rootCmd.AddCommand(simulateCmd)
	simulateCmd.Flags().StringVar(&simAction, "action", "", "Action e.g. prt_exchange, wstrust_relay, vm_runcommand (required)")
	simulateCmd.Flags().BoolVar(&simLocation, "unknown-location", false, "Sign-in from unfamiliar location")
	simulateCmd.Flags().BoolVar(&simDevice, "new-device", false, "Sign-in from new device")
	simulateCmd.Flags().StringVar(&simTimeOfDay, "time-of-day", "business_hours", "business_hours or off_hours")
	simulateCmd.Flags().BoolVar(&simJSON, "json", false, "Output JSON")
	_ = simulateCmd.MarkFlagRequired("action")
}

func runSimulate(cmd *cobra.Command, args []string) error {
	p := validate.NewPredictor()
	env := &validate.Environment{
		UnknownLocation: simLocation,
		NewDevice:       simDevice,
		TimeOfDay:       simTimeOfDay,
	}
	signals := p.Simulate(simAction, env)
	if simJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(signals)
	}
	fmt.Print(validate.RenderSignals(simAction, signals))
	return nil
}

// exportCmd — evidence / report / graph export.
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export evidence, reports, and graph topologies",
	Long:  "Export workspace journals and engagement reports as markdown/SARIF for purple-team workflows.",
}

var exportReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Export the workspace journal as a markdown report",
	RunE: func(cmd *cobra.Command, args []string) error {
		w, err := workspace.Open(exportWorkspace, os.Getenv("AETHER_PASSPHRASE"))
		if err != nil {
			return err
		}
		events, err := w.Events()
		if err != nil {
			return err
		}

		var b strings.Builder
		fmt.Fprintf(&b, "# Aether Engagement Report — %s\n\nExported: %s\n\n", w.Name, timeNowUTC())
		fmt.Fprintf(&b, "| Time | Kind | Detail |\n|---|---|---|\n")
		for _, ev := range events {
			detail := strings.ReplaceAll(ev.Detail, "|", "\\|")
			fmt.Fprintf(&b, "| %s | %s | %s |\n", ev.Time.Format("2006-01-02 15:04:05"), ev.Kind, detail)
		}

		if exportOutput == "" {
			exportOutput = w.Name + "-report.md"
		}
		if err := os.WriteFile(exportOutput, []byte(b.String()), 0o600); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Report exported to %s (%d events)\n", exportOutput, len(events))
		return nil
	},
}

var exportSARIFCmd = &cobra.Command{
	Use:   "sarif",
	Short: "Export path validation results as SARIF",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := validate.LoadPath(exportBHFile)
		if err != nil {
			return err
		}
		result, err := validate.NewPathValidator("", "", nil).ValidatePath(contextBackground(), path)
		if err != nil {
			return err
		}

		sarif := buildSARIF(exportBHFile, result)
		data, err := json.MarshalIndent(sarif, "", "  ")
		if err != nil {
			return err
		}
		if exportOutput == "" {
			exportOutput = "aether.sarif"
		}
		if err := os.WriteFile(exportOutput, data, 0o600); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "SARIF exported to %s\n", exportOutput)
		return nil
	},
}

var exportGraphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Export the identity graph topology",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := graph.LoadGraph(exportGraphFile)
		if err != nil {
			return err
		}
		if exportOutput == "" {
			exportOutput = "aether-topology.json"
		}
		return g.Save(exportOutput)
	},
}

var (
	exportWorkspace string
	exportOutput    string
	exportBHFile    string
	exportGraphFile string
)

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.AddCommand(exportReportCmd, exportSARIFCmd, exportGraphCmd)

	exportReportCmd.Flags().StringVar(&exportWorkspace, "workspace", "", "Workspace name (required)")
	exportReportCmd.Flags().StringVar(&exportOutput, "output", "", "Output file")
	_ = exportReportCmd.MarkFlagRequired("workspace")

	exportSARIFCmd.Flags().StringVar(&exportBHFile, "bh-json", "", "BloodHound path JSON (required)")
	exportSARIFCmd.Flags().StringVar(&exportOutput, "output", "", "Output SARIF file")
	_ = exportSARIFCmd.MarkFlagRequired("bh-json")

	exportGraphCmd.Flags().StringVar(&exportGraphFile, "graph", "", "Graph JSON file (required)")
	exportGraphCmd.Flags().StringVar(&exportOutput, "output", "", "Output file")
	_ = exportGraphCmd.MarkFlagRequired("graph")
}

// sarifDoc is a minimal SARIF 2.1.0 document.
type sarifDoc struct {
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations,omitempty"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysical `json:"physicalLocation"`
}

type sarifPhysical struct {
	Region sarifRegion `json:"region"`
}

type sarifRegion struct {
	StartLine int `json:"startLine"`
}

func buildSARIF(source string, result *types.ValidationResult) sarifDoc {
	run := sarifRun{
		Tool: sarifTool{Driver: sarifDriver{Name: "aether", Version: version.Version}},
	}
	for _, s := range result.Steps {
		level := "note"
		if !s.Valid {
			level = "error"
		} else if s.RiskScore > 50 {
			level = "warning"
		}
		run.Results = append(run.Results, sarifResult{
			RuleID:  fmt.Sprintf("AETHER%03d", 100+s.StepIndex),
			Level:   level,
			Message: sarifMessage{Text: fmt.Sprintf("%s — valid=%t risk=%d/100 %s", s.Edge, s.Valid, s.RiskScore, s.Reason)},
			Locations: []sarifLocation{{
				PhysicalLocation: sarifPhysical{Region: sarifRegion{StartLine: s.StepIndex + 1}},
			}},
		})
	}
	return sarifDoc{Version: "2.1.0", Runs: []sarifRun{run}}
}
