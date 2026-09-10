package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/engine/exec"
	"github.com/Debajyoti0-0/aether/internal/engine/mutation"
	"github.com/Debajyoti0-0/aether/internal/engine/orchestrator"
	"github.com/Debajyoti0-0/aether/internal/engine/validate"
	"github.com/Debajyoti0-0/aether/internal/transport"
	"github.com/Debajyoti0-0/aether/internal/types"
	"github.com/Debajyoti0-0/aether/pkg/plugins"
	"github.com/Debajyoti0-0/aether/pkg/plugins/gitlab"
	"github.com/Debajyoti0-0/aether/pkg/plugins/kubernetes"
	"github.com/Debajyoti0-0/aether/pkg/plugins/okta"
	"github.com/Debajyoti0-0/aether/pkg/plugins/sdk"
)

// ---------------------------------------------------------------- run --plan

// runPlanCmd — adaptive DAG execution.
var runPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "Execute an adaptive DAG plan (JSON) with fallbacks and retries",
	Long: `Execute a workflow plan as a DAG: nodes declare dependencies,
fallbacks, retries, and criticality. The engine topologically orders
nodes, runs independent nodes concurrently (bounded by --parallel),
falls back to alternative nodes on failure, and skips blocked branches.

Plan shape:
{
  "max_parallel": 3,
  "nodes": [
    {"id": "prt", "cmd": "prt convert --prt-file p.json",
     "fallback": "refresh", "retries": 2, "critical": true},
    {"id": "refresh", "cmd": "relay cae-handler --refresh-token X --tenant T"},
    {"id": "exec", "cmd": "exec azure ...", "dependencies": ["prt"]}
  ]
}`,
	RunE: runPlan,
}

var (
	planFile     string
	planParallel int
)

func init() {
	runCmd.AddCommand(runPlanCmd)
	runPlanCmd.Flags().StringVar(&planFile, "plan", "", "Workflow plan JSON (required)")
	runPlanCmd.Flags().IntVar(&planParallel, "parallel", 3, "Max concurrent nodes")
	runPlanCmd.Flags().StringVar(&execWorkspace, "workspace", "", "Workspace for audit+evidence records (required)")
	_ = runPlanCmd.MarkFlagRequired("plan")
}

// planDoc is the JSON plan shape.
type planDoc struct {
	MaxParallel int        `json:"max_parallel"`
	Nodes       []planNode `json:"nodes"`
}

type planNode struct {
	ID           string   `json:"id"`
	Cmd          string   `json:"cmd"`
	Dependencies []string `json:"dependencies,omitempty"`
	Fallback     string   `json:"fallback,omitempty"`
	Retries      int      `json:"retries,omitempty"`
	Critical     bool     `json:"critical,omitempty"`
}

func runPlan(cmd *cobra.Command, args []string) error {
	// Stage 2 (F7): every node executes through the Action spine via the
	// intent whitelist — the CLI root is never re-entered.
	ws, err := openGovernedWorkspace(execWorkspace)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(planFile)
	if err != nil {
		return fmt.Errorf("read plan: %w", err)
	}

	var doc planDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse plan: %w", err)
	}
	if len(doc.Nodes) == 0 {
		return fmt.Errorf("plan has no nodes")
	}

	w := orchestrator.NewWorkflow()
	w.MaxParallel = planParallel
	if doc.MaxParallel > 0 {
		w.MaxParallel = doc.MaxParallel
	}

	for _, n := range doc.Nodes {
		node := n
		w.Add(&orchestrator.Node{
			ID:           node.ID,
			Dependencies: node.Dependencies,
			Fallback:     node.Fallback,
			Retries:      node.Retries,
			Critical:     node.Critical,
			Action: func(ctx context.Context, rec *orchestrator.StepRecorder) error {
				res, err := runIntent(ctx, ws, node.Cmd, "dag")
				if err != nil {
					return err
				}
				fmt.Printf("[%s] action %s status %s\n", node.ID, res.ActionID, res.Status)
				return nil
			},
		})
	}

	rec := &orchestrator.StepRecorder{}
	if err := w.Execute(context.Background(), rec); err != nil {
		return err
	}

	fmt.Println("\n=== Plan execution summary ===")
	for _, s := range rec.Steps() {
		mark := "OK  "
		switch s.Status {
		case "failed":
			mark = "FAIL"
		case "fallback":
			mark = "FB  "
		case "skipped":
			mark = "SKIP"
		}
		fmt.Printf("[%s] %-14s attempts=%d %s\n", mark, s.NodeID, s.Attempts, s.Detail)
	}
	return nil
}

// ------------------------------------------------------------ exec parallel

// execParallelCmd — distributed execution across a target list.
var execParallelCmd = &cobra.Command{
	Use:   "parallel",
	Short: "Fan a command out over many targets with a worker pool",
	Long: `Run one cloud exec command against every target in parallel.
Targets come from a file (one per line) or a comma-separated list.
The cloud backend is chosen by --backend (azure, aws).`,
	RunE: runExecParallel,
}

var (
	parBackend    string
	parTargetsF   string
	parTargetsL   string
	parCommand    string
	parConcurrent int
	parTimeout    int
	parToken      string
	parSubID      string
	parGroup      string
	parAK         string
	parSK         string
	parSessTok    string
	parRegion     string
)

func init() {
	execCmd.AddCommand(execParallelCmd)
	execParallelCmd.Flags().StringVar(&parBackend, "backend", "azure", "azure | aws")
	execParallelCmd.Flags().StringVar(&parTargetsF, "targets-file", "", "Targets file (one per line)")
	execParallelCmd.Flags().StringVar(&parTargetsL, "targets", "", "Comma-separated targets")
	execParallelCmd.Flags().StringVar(&parCommand, "cmd", "", "Command to run on each target (required)")
	execParallelCmd.Flags().IntVar(&parConcurrent, "parallel", 5, "Max concurrent workers")
	execParallelCmd.Flags().IntVar(&parTimeout, "timeout", 300, "Timeout seconds")
	execParallelCmd.Flags().StringVar(&parToken, "token", "", "Access token (azure)")
	execParallelCmd.Flags().StringVar(&parSubID, "subscription-id", "", "Azure subscription id")
	execParallelCmd.Flags().StringVar(&parGroup, "resource-group", "", "Azure resource group")
	execParallelCmd.Flags().StringVar(&parAK, "access-key", "", "AWS access key")
	execParallelCmd.Flags().StringVar(&parSK, "secret-key", "", "AWS secret key")
	execParallelCmd.Flags().StringVar(&parSessTok, "session-token", "", "AWS session token")
	execParallelCmd.Flags().StringVar(&parRegion, "region", "us-east-1", "AWS region")
	execParallelCmd.Flags().StringVar(&execWorkspace, "workspace", "", "Workspace for audit+rollback records (required)")
	_ = execParallelCmd.MarkFlagRequired("cmd")
}

func runExecParallel(cmd *cobra.Command, args []string) error {
	ws, err := openGovernedWorkspace(execWorkspace)
	if err != nil {
		return err
	}
	targets, err := exec.LoadTargets(parTargetsF, parTargetsL)
	if err != nil {
		return err
	}

	var backendRunner func(ctx context.Context, target, command string) (*types.CommandResult, error)
	switch strings.ToLower(parBackend) {
	case "azure":
		if parToken == "" || parSubID == "" || parGroup == "" {
			return fmt.Errorf("azure backend requires --token, --subscription-id, --resource-group")
		}
		hc, err := transport.NewClient("chrome", time.Duration(parTimeout)*time.Second)
		if err != nil {
			return err
		}
		azureExec := exec.NewAzureExecutor(parSubID, parToken, hc)
		backendRunner = func(ctx context.Context, target, command string) (*types.CommandResult, error) {
			return azureExec.ExecuteOnAzureVM(ctx, parGroup, target, command)
		}
	case "aws":
		if parAK == "" || parSK == "" {
			return fmt.Errorf("aws backend requires --access-key and --secret-key")
		}
		awsExec := exec.NewAWSExecutor(parRegion, parAK, parSK, parSessTok, nil)
		backendRunner = func(ctx context.Context, target, command string) (*types.CommandResult, error) {
			return awsExec.ExecuteOnEC2(ctx, target, command)
		}
	default:
		return fmt.Errorf("unknown backend %q (want azure or aws)", parBackend)
	}

	// Every target execution passes through the mutation pipeline so the
	// fan-out leaves an audit + rollback trail per target.
	runner := func(ctx context.Context, target, command string) (*types.CommandResult, error) {
		var cr *types.CommandResult
		_, err := mutation.Run(ctx, ws, &cliMutation{
			kindV:   "exec." + strings.ToLower(parBackend),
			targetV: target,
			undoV:   irreversibleShell(),
			execFn: func(ctx context.Context) (string, error) {
				r, err := backendRunner(ctx, target, command)
				cr = r
				return "", err
			},
		})
		return cr, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(parTimeout)*time.Second)
	defer cancel()

	pool := &exec.ParallelExecutor{Run: runner, Concurrency: parConcurrent}
	results := pool.RunAll(ctx, targets, parCommand)
	fmt.Print(exec.RenderResults(results))
	return nil
}

// ------------------------------------------------------------ simulate fuzz

// simulateFuzzCmd — SOC telemetry fuzzing.
var simulateFuzzCmd = &cobra.Command{
	Use:   "fuzz",
	Short: "Generate telemetry variants to test detection rule robustness",
	RunE:  runSimulateFuzz,
}

var (
	fuzzCount  int
	fuzzOut    string
	fuzzNDJSON bool
	fuzzSeed   string
)

func init() {
	simulateCmd.AddCommand(simulateFuzzCmd)
	simulateFuzzCmd.Flags().IntVar(&fuzzCount, "count", 20, "Number of variants (1-100)")
	simulateFuzzCmd.Flags().StringVar(&fuzzOut, "output", "", "Write variants to file")
	simulateFuzzCmd.Flags().BoolVar(&fuzzNDJSON, "ndjson", true, "NDJSON output (one variant per line)")
	simulateFuzzCmd.Flags().StringVar(&fuzzSeed, "seed", "", "Base telemetry JSON file (required)")
	_ = simulateFuzzCmd.MarkFlagRequired("seed")
}

func runSimulateFuzz(cmd *cobra.Command, args []string) error {
	data, err := os.ReadFile(fuzzSeed)
	if err != nil {
		return fmt.Errorf("read seed: %w", err)
	}

	var base map[string]any
	if err := json.Unmarshal(data, &base); err != nil {
		return fmt.Errorf("parse seed (need a JSON telemetry record): %w", err)
	}

	variants := validate.FuzzTelemetry(base, fuzzCount)
	fmt.Print(validate.RenderFuzzSummary(variants))

	if fuzzOut != "" {
		var payload string
		if fuzzNDJSON {
			payload, err = validate.FuzzJSON(variants)
		} else {
			var raw []byte
			raw, err = json.MarshalIndent(variants, "", "  ")
			payload = string(raw)
		}
		if err != nil {
			return err
		}
		if err := os.WriteFile(fuzzOut, []byte(payload), 0o600); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Variants written to %s\n", fuzzOut)
	}
	return nil
}

// ------------------------------------------------------------ export executive

// exportExecutiveCmd — board-ready engagement report with KPIs.
var exportExecutiveCmd = &cobra.Command{
	Use:   "executive",
	Short: "Generate the executive engagement report (KPIs + remediation)",
	RunE:  runExportExecutive,
}

var (
	execKPIWorkspace string
	execKPIPathFile  string
	execKPIOut       string
)

func init() {
	exportCmd.AddCommand(exportExecutiveCmd)
	exportExecutiveCmd.Flags().StringVar(&execKPIWorkspace, "workspace", "", "Engagement label for report titles (report inputs come from --bh-json/--actions, not workspace records)")
	exportExecutiveCmd.Flags().StringVar(&execKPIPathFile, "bh-json", "", "BloodHound path JSON to validate and include")
	exportExecutiveCmd.Flags().StringVar(&execKPIOut, "output", "", "Base output path (writes .md and .json)")
	_ = exportExecutiveCmd.MarkFlagRequired("workspace")
}

func runExportExecutive(cmd *cobra.Command, args []string) error {
	var results []*types.ValidationResult

	if execKPIPathFile != "" {
		path, err := validate.LoadPath(execKPIPathFile)
		if err != nil {
			return err
		}
		result, err := validate.NewPathValidator("", "", nil).ValidatePath(context.Background(), path)
		if err != nil {
			return err
		}
		results = append(results, result)
	}

	report := validate.Executive(execKPIWorkspace, results, nil)

	if execKPIOut == "" {
		execKPIOut = execKPIWorkspace + "-executive"
	}
	if err := report.WriteExecutive(execKPIOut); err != nil {
		return err
	}
	fmt.Print(report.Markdown())
	return nil
}

// ------------------------------------------------------------- providers

// providersCmd — official provider plugins (okta, gitlab, kubernetes).
var providersCmd = &cobra.Command{
	Use:   "providers",
	Short: "Third-party provider plugins (okta, gitlab, kubernetes)",
}

var (
	provToken  string
	provDomain string
	provTarget string
	provCmdStr string
	provLimit  int
)

func init() {
	rootCmd.AddCommand(providersCmd)

	providersCmd.PersistentFlags().StringVar(&provToken, "token", "", "API/PAT/SA token (required)")
	providersCmd.PersistentFlags().StringVar(&provDomain, "domain", "", "Provider base URL, e.g. https://org.okta.com (required)")

	providersCmd.AddCommand(providersListCmd, providersUsersCmd, providersExecCmd)

	providersExecCmd.Flags().StringVar(&provTarget, "target", "", "gitlab: project id | kubernetes: namespace/pod (required)")
	providersExecCmd.Flags().StringVar(&provCmdStr, "cmd", "", "gitlab: ref | kubernetes: command to stage (required)")
	_ = providersExecCmd.MarkFlagRequired("target")
	_ = providersExecCmd.MarkFlagRequired("cmd")
}

// providerRegistryFor returns a registry wired to the operator's domain.
func providerRegistryFor(domain, token string) *plugins.Registry {
	reg := plugins.NewRegistry()
	reg.Register(okta.New(domain, token))
	reg.Register(gitlab.New(domain, token))
	reg.Register(kubernetes.New(domain, token, ""))
	return reg
}

// providersPick resolves the active plugin by the domain suffix.
func providersPick() string {
	switch {
	case strings.Contains(provDomain, "okta"):
		return "okta"
	case strings.Contains(provDomain, "gitlab"):
		return "gitlab"
	default:
		return "kubernetes"
	}
}

var providersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered provider plugins",
	RunE: func(cmd *cobra.Command, args []string) error {
		reg := providerRegistryFor("", "")
		for _, name := range reg.Names() {
			p, _ := reg.Get(name)
			fmt.Printf("%-12s %s\n", name, p.Version())
		}
		return nil
	},
}

var providersUsersCmd = &cobra.Command{
	Use:   "users",
	Short: "okta: enumerate users",
	RunE: func(cmd *cobra.Command, args []string) error {
		prov, err := providerRegistryFor(provDomain, provToken).Provider("okta")
		if err != nil {
			return err
		}
		o, ok := prov.(interface {
			ListUsers(ctx context.Context, limit int) (any, error)
		})
		_ = o
		_ = ok
		return execOktaUsers(provToken, provDomain, provLimit)
	},
}

var providersExecCmd = &cobra.Command{
	Use:   "exec",
	Short: "Run a provider action (gitlab: pipeline, kubernetes: pod stage)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := openGovernedWorkspace(execWorkspace)
		if err != nil {
			return err
		}
		prov, err := providerRegistryFor(provDomain, provToken).Provider(providersPick())
		if err != nil {
			return err
		}
		if err := prov.ValidateToken(context.Background()); err != nil {
			return fmt.Errorf("token validation: %w", err)
		}
		res, err := func() (*sdk.Result, error) {
			var r *sdk.Result
			_, mErr := mutation.Run(context.Background(), ws, &cliMutation{
				kindV:   "providers.exec",
				targetV: providersPick() + ":" + provTarget,
				undoV:   irreversibleShell(),
				execFn: func(ctx context.Context) (string, error) {
					out, err := prov.Execute(ctx, provTarget, provCmdStr)
					r = out
					return "", err
				},
			})
			return r, mErr
		}()
		if err != nil {
			return err
		}
		fmt.Printf("Status:  %s\nExit:    %d\nOutput:  %s\n", res.Status, res.ExitCode, res.Output)
		return nil
	},
}
