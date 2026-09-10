package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/api"
	"github.com/Debajyoti0-0/aether/internal/engine/graph"
	"github.com/Debajyoti0-0/aether/internal/engine/mutation"
	"github.com/Debajyoti0-0/aether/internal/engine/validate"
	"github.com/Debajyoti0-0/aether/internal/engine/watch"
	"github.com/Debajyoti0-0/aether/internal/store"
	"github.com/Debajyoti0-0/aether/internal/types"
	"github.com/Debajyoti0-0/aether/internal/workspace"
	"github.com/Debajyoti0-0/aether/pkg/plugins"
	"github.com/Debajyoti0-0/aether/pkg/plugins/gcp"
	"github.com/Debajyoti0-0/aether/pkg/plugins/sdk"
)

// workspaceOpen opens a workspace with the operator's passphrase env.
func workspaceOpen(name string) (*workspace.Workspace, error) {
	return workspace.Open(name, os.Getenv("AETHER_PASSPHRASE"))
}

// ---------------------------------------------------------------- watch / autopilot

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Continuously monitor the environment for new attack paths",
	Long: `Poll the environment (graph refresh via --seed graph or provider
export), diff snapshots, and report newly exploitable paths. With
--autopilot, qualifying paths are executed automatically and the
operator is notified through the teamserver.`,
	RunE: runWatch,
}

var (
	watchWorkspace  string
	watchSeedFile   string
	watchInterval   time.Duration
	watchAutopilot  bool
	watchMaxRisk    int
)

func init() {
	// watchCmd is registered in the batch init below (was duplicated here).
	watchCmd.Flags().StringVar(&watchWorkspace, "workspace", "", "Workspace name (required)")
	watchCmd.Flags().StringVar(&watchSeedFile, "seed-graph", "", "Graph JSON to watch (static seed; live providers via future hooks)")
	watchCmd.Flags().DurationVar(&watchInterval, "interval", 5*time.Minute, "Poll interval")
	watchCmd.Flags().BoolVar(&watchAutopilot, "autopilot", false, "Automatically execute new paths under the risk ceiling")
	watchCmd.Flags().IntVar(&watchMaxRisk, "max-risk", 40, "Autopilot risk ceiling")
	_ = watchCmd.MarkFlagRequired("workspace")
}

func runWatch(cmd *cobra.Command, args []string) error {
	ws, err := workspaceOpen(watchWorkspace)
	if err != nil {
		return err
	}

	var poll watch.Poller
	if watchSeedFile != "" {
		// Static-seed mode: watch a graph file that an external feed
		// (or another aether process) keeps refreshing.
		poll = func(ctx context.Context) (*graph.IdentityGraph, error) {
			return graph.LoadGraph(watchSeedFile)
		}
	} else {
		return fmt.Errorf("provide --seed-graph (live provider polling lands with the provider refresh hooks)")
	}

	// Autopilot executes through the in-process command tree.
	w := watch.NewWatcher(watchInterval, watchMaxRisk, poll)
	w.Autopilot = watchAutopilot
	if watchAutopilot {
		w.Execute = func(ctx context.Context, path []string) error {
			engine := graph.NewGraphEngine(mustLoadGraph(watchSeedFile), watchMaxRisk)
			run, err := engine.QualifyPath(path)
			if err != nil {
				return err
			}
			for _, step := range run.Runbook {
				fields := strings.Fields(strings.TrimPrefix(step, "aether "))
				if len(fields) == 0 {
					continue
				}
				root := NewRootCommand()
				root.SetArgs(fields)
				if err := root.Execute(); err != nil {
					return err
				}
			}
			return nil
		}
	}

	w.Notify = func(e watch.AutopilotEvent) {
		data, _ := json.Marshal(e)
		fmt.Println(string(data))
		_ = ws.LogEvent("watch_event", string(data))
	}

	fmt.Fprintf(os.Stderr, "Watching %s (interval %s, autopilot=%t, max-risk=%d)\n",
		watchWorkspace, watchInterval, watchAutopilot, watchMaxRisk)

	ctx, cancel := signalContext()
	defer cancel()
	return w.Run(ctx)
}

func mustLoadGraph(path string) *graph.IdentityGraph {
	g, err := graph.LoadGraph(path)
	if err != nil {
		return &graph.IdentityGraph{}
	}
	return g
}

// ---------------------------------------------------------------- dashboard

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Token-authenticated web dashboard: graph view + activity stream",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := graph.LoadGraph(dashGraphFile)
		if err != nil {
			return err
		}

		html, err := graph.Visualize(g, "Aether Dashboard — "+dashWorkspace)
		if err != nil {
			return err
		}

		d, err := api.NewDashboard()
		if err != nil {
			return err
		}
		ws, err := workspaceOpen(dashWorkspace)
		if err == nil {
			if events, err := ws.Events(); err == nil {
				for _, ev := range events {
					d.Publish(api.DashboardEvent{Time: ev.Time, Kind: ev.Kind, Detail: ev.Detail})
				}
			}
		}

		addr := fmt.Sprintf("%s:%s", dashBind, strings.TrimPrefix(dashPort, ":"))
		withTLS := dashTLSCert != "" && dashTLSKey != ""
		if (dashTLSCert == "") != (dashTLSKey == "") {
			return fmt.Errorf("--tls-cert and --tls-key must be provided together")
		}
		// Fail closed: engagement data must not be exposed over plain
		// HTTP on a non-loopback interface.
		if !api.IsLoopback(addr) && !withTLS {
			return fmt.Errorf("refusing to bind dashboard to non-loopback address %s without TLS; pass --tls-cert/--tls-key or use the default 127.0.0.1 bind", addr)
		}

		fmt.Fprintf(os.Stdout, "Dashboard token: %s\n", d.Token())
		fmt.Fprintf(os.Stdout, "Pass the token via ?token=, the X-Aether-Token header, or 'Authorization: Bearer <token>'.\n")
		fmt.Fprintf(os.Stderr, "Dashboard: %s://%s/?token=%s (workspace %s: %d nodes, %d edges)\n",
			map[bool]string{true: "https", false: "http"}[withTLS], addr, d.Token(), dashWorkspace, len(g.Nodes), len(g.Edges))

		handler := d.Handler(html, g)
		if withTLS {
			return d.ServeTLS(addr, dashTLSCert, dashTLSKey, handler)
		}
		return d.Serve(addr, handler)
	},
}

var (
	dashWorkspace string
	dashGraphFile string
	dashPort      string
	dashBind      string
	dashTLSCert   string
	dashTLSKey    string
)

// ---------------------------------------------------------------- audit

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Tamper-evident audit trail (signed hash chain)",
}

var auditVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify the workspace audit chain integrity",
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := workspaceOpen(auditWorkspace)
		if err != nil {
			return err
		}
		l, err := store.New(
			ws.Root+string(os.PathSeparator)+"db"+string(os.PathSeparator)+"audit.jsonl",
			ws.Root+string(os.PathSeparator)+"db"+string(os.PathSeparator)+"audit.key")
		if err != nil {
			return err
		}
		res, err := l.Verify()
		if err != nil {
			return err
		}
		data, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(data))
		if !res.ValidAll {
			return fmt.Errorf("audit trail integrity: TAMPERED")
		}
		fmt.Printf("Audit trail integrity: VERIFIED (%d entries, 0 tampered)\n", res.Total)
		return nil
	},
}

var auditRecordCmd = &cobra.Command{
	Use:   "record",
	Short: "Record a command into the signed audit chain",
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := workspaceOpen(auditWorkspace)
		if err != nil {
			return err
		}
		l, err := store.New(
			ws.Root+string(os.PathSeparator)+"db"+string(os.PathSeparator)+"audit.jsonl",
			ws.Root+string(os.PathSeparator)+"db"+string(os.PathSeparator)+"audit.key")
		if err != nil {
			return err
		}
		e, err := l.Append(auditCmdStr, auditResult)
		if err != nil {
			return err
		}
		fmt.Printf("Recorded seq %d (hash %s...)\n", e.Seq, e.Hash[:16])
		return nil
	},
}

var exportAuditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Export the signed audit trail as JSONL",
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := workspaceOpen(auditWorkspace)
		if err != nil {
			return err
		}
		l, err := store.New(
			ws.Root+string(os.PathSeparator)+"db"+string(os.PathSeparator)+"audit.jsonl",
			ws.Root+string(os.PathSeparator)+"db"+string(os.PathSeparator)+"audit.key")
		if err != nil {
			return err
		}
		if auditOut == "" {
			auditOut = ws.Name + "-audit.jsonl"
		}
		if err := store.ExportFile(l, auditOut); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Signed audit trail exported to %s\n", auditOut)
		return nil
	},
}

var (
	auditWorkspace string
	auditCmdStr    string
	auditResult    string
	auditOut       string
)

// ---------------------------------------------------------------- plugins registry

var pluginsCmd2 = &cobra.Command{
	Use:   "plugins",
	Short: "Plugin registry: search and install community plugins",
}

var pluginsSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search the remote plugin registry",
	RunE: func(cmd *cobra.Command, args []string) error {
		reg := plugins.NewRemoteRegistry(pluginsIndex, "")
		results, err := reg.Search(context.Background(), pluginsQuery)
		if err != nil {
			return err
		}
		fmt.Print(plugins.RenderSearch(results))
		return nil
	},
}

var pluginsInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install a plugin from the registry (checksum-verified)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := openGovernedWorkspace(execWorkspace)
		if err != nil {
			return err
		}
		reg := plugins.NewRemoteRegistry(pluginsIndex, "")

		results, err := reg.Search(context.Background(), pluginsInstallName)
		if err != nil {
			return err
		}
		if len(results) == 0 {
			return fmt.Errorf("plugin %q not found in registry", pluginsInstallName)
		}
		manifest := results[0]

		var installedPath string
		_, err = mutation.Run(context.Background(), ws, &cliMutation{
			kindV:   "plugins.install",
			targetV: manifest.Name + "@" + manifest.Version,
			undoV: &mutation.UndoSpec{
				Provider: "plugins",
				Op:       "uninstall",
				Args:     map[string]string{"name": manifest.Name, "version": manifest.Version},
				Detail:   "remove the installed plugin manifest + artifact files",
			},
			execFn: func(ctx context.Context) (string, error) {
				p, err := reg.Install(ctx, manifest)
				installedPath = p
				return "", err
			},
			afterFn: func() ([]byte, error) {
				if installedPath == "" {
					return nil, mutation.ErrStateUnavailable
				}
				return []byte(fmt.Sprintf(`{"installed_path":%q,"name":%q,"version":%q}`,
					installedPath, manifest.Name, manifest.Version)), nil
			},
		})
		if err != nil {
			return err
		}
		fmt.Printf("Installed %s v%s → %s (SHA-256 verified if manifest declared a checksum)\n", manifest.Name, manifest.Version, installedPath)
		return nil
	},
}

var pluginsInstalledCmd = &cobra.Command{
	Use:   "installed",
	Short: "List locally installed plugins",
	RunE: func(cmd *cobra.Command, args []string) error {
		reg := plugins.NewRemoteRegistry("", "")
		installed, err := reg.Installed()
		if err != nil {
			return err
		}
		if len(installed) == 0 {
			fmt.Println("No plugins installed.")
			return nil
		}
		fmt.Print(plugins.RenderSearch(installed))
		return nil
	},
}

var (
	pluginsIndex        string
	pluginsQuery        string
	pluginsInstallName  string
)

// ---------------------------------------------------------------- exec gcp

var execGCPCmd = &cobra.Command{
	Use:   "gcp",
	Short: "GCP Compute Engine exec staging (OS Login / serial console path)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := openGovernedWorkspace(execWorkspace)
		if err != nil {
			return err
		}
		reg := plugins.NewRegistry()
		reg.Register(gcp.New(gcpProject, gcpToken))

		prov, err := reg.Provider("gcp")
		if err != nil {
			return err
		}
		if err := prov.ValidateToken(context.Background()); err != nil {
			return fmt.Errorf("token validation: %w", err)
		}
		res, err := func() (*sdk.Result, error) {
			var r *sdk.Result
			_, mErr := mutation.Run(context.Background(), ws, &cliMutation{
				kindV:   "exec.gcp",
				targetV: gcpProject + "/" + gcpTarget,
				// Precautionary record: the current GCP path only stages
				// execution, but any future live-exec wiring inherits the
				// governance boundary automatically.
				undoV: irreversibleShell(),
				execFn: func(ctx context.Context) (string, error) {
					out, err := prov.Execute(ctx, gcpTarget, gcpCmdStr)
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

var (
	gcpProject string
	gcpToken   string
	gcpTarget  string
	gcpCmdStr  string
)

// ---------------------------------------------------------------- export executive --pdf

var exportPDFCmd = &cobra.Command{
	Use:   "pdf",
	Short: "Export the executive report as a PDF",
	RunE: func(cmd *cobra.Command, args []string) error {
		var results []*types.ValidationResult

		if pdfPathFile != "" {
			path, err := validate.LoadPath(pdfPathFile)
			if err != nil {
				return err
			}
			result, err := validate.NewPathValidator("", "", nil).ValidatePath(context.Background(), path)
			if err != nil {
				return err
			}
			results = append(results, result)
		}

		var actions []string
		if pdfActionsStr != "" {
			actions = splitComma(pdfActionsStr)
		}

		report := validate.Executive(pdfWorkspace, results, nil, actions...)
		if pdfOut == "" {
			pdfOut = pdfWorkspace + "-report.pdf"
		}
		data, err := report.PDF()
		if err != nil {
			return err
		}
		if err := os.WriteFile(pdfOut, data, 0o600); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "PDF report written to %s (%d bytes)\n", pdfOut, len(data))
		return nil
	},
}

var (
	pdfWorkspace  string
	pdfPathFile   string
	pdfOut        string
	pdfActionsStr string
)

func init() {
	rootCmd.AddCommand(watchCmd, dashboardCmd, auditCmd, pluginsCmd2)
	auditCmd.AddCommand(auditVerifyCmd, auditRecordCmd)
	exportCmd.AddCommand(exportAuditCmd)
	pluginsCmd2.AddCommand(pluginsSearchCmd, pluginsInstallCmd, pluginsInstalledCmd)
	execCmd.AddCommand(execGCPCmd)
	exportCmd.AddCommand(exportPDFCmd)

	dashboardCmd.Flags().StringVar(&dashWorkspace, "workspace", "", "Workspace name (required)")
	dashboardCmd.Flags().StringVar(&dashGraphFile, "graph", "", "Graph JSON file (required)")
	dashboardCmd.Flags().StringVar(&dashPort, "port", "8080", "Listen port")
	dashboardCmd.Flags().StringVar(&dashBind, "bind", "127.0.0.1", "Bind address (default 127.0.0.1; non-loopback requires TLS)")
	dashboardCmd.Flags().StringVar(&dashTLSCert, "tls-cert", "", "TLS certificate (required for non-loopback binds)")
	dashboardCmd.Flags().StringVar(&dashTLSKey, "tls-key", "", "TLS key (required for non-loopback binds)")
	_ = dashboardCmd.MarkFlagRequired("workspace")
	_ = dashboardCmd.MarkFlagRequired("graph")

	auditVerifyCmd.Flags().StringVar(&auditWorkspace, "workspace", "", "Workspace name (required)")
	auditRecordCmd.Flags().StringVar(&auditWorkspace, "workspace", "", "Workspace name (required)")
	auditRecordCmd.Flags().StringVar(&auditCmdStr, "cmd", "", "Command to record (required)")
	auditRecordCmd.Flags().StringVar(&auditResult, "result", "", "Result detail")
	exportAuditCmd.Flags().StringVar(&auditWorkspace, "workspace", "", "Workspace name (required)")
	exportAuditCmd.Flags().StringVar(&auditOut, "output", "", "Output JSONL file")
	for _, c := range []*cobra.Command{auditVerifyCmd, auditRecordCmd, exportAuditCmd} {
		_ = c.MarkFlagRequired("workspace")
	}
	_ = auditRecordCmd.MarkFlagRequired("cmd")

	pluginsCmd2.PersistentFlags().StringVar(&pluginsIndex, "index", "https://raw.githubusercontent.com/Debajyoti0-0/aether-plugins/main/index.json", "Registry index URL")
	pluginsSearchCmd.Flags().StringVar(&pluginsQuery, "query", "", "Search substring")
	pluginsInstallCmd.Flags().StringVar(&pluginsInstallName, "name", "", "Plugin name (required)")
	pluginsInstallCmd.Flags().StringVar(&execWorkspace, "workspace", "", "Workspace for audit+rollback records (required)")
	_ = pluginsInstallCmd.MarkFlagRequired("name")

	execGCPCmd.Flags().StringVar(&gcpProject, "project", "", "GCP project id (required)")
	execGCPCmd.Flags().StringVar(&gcpToken, "token", "", "OAuth2 access token (required)")
	execGCPCmd.Flags().StringVar(&gcpTarget, "target", "", "zone/instance (required)")
	execGCPCmd.Flags().StringVar(&gcpCmdStr, "cmd", "", "Command to stage (required)")
	execGCPCmd.Flags().StringVar(&execWorkspace, "workspace", "", "Workspace for audit+rollback records (required)")
	_ = execGCPCmd.MarkFlagRequired("project")
	_ = execGCPCmd.MarkFlagRequired("token")
	_ = execGCPCmd.MarkFlagRequired("target")
	_ = execGCPCmd.MarkFlagRequired("cmd")

	exportPDFCmd.Flags().StringVar(&pdfWorkspace, "workspace", "", "Engagement label for the PDF title (report inputs come from --bh-json)")
	exportPDFCmd.Flags().StringVar(&pdfPathFile, "bh-json", "", "BloodHound path JSON to include")
	exportPDFCmd.Flags().StringVar(&pdfOut, "output", "", "Output PDF file")
	_ = exportPDFCmd.MarkFlagRequired("workspace")
}
