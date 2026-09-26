package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/api"
	"github.com/Debajyoti0-0/aether/internal/engine/graph"
	"github.com/Debajyoti0-0/aether/internal/store"
	"github.com/Debajyoti0-0/aether/internal/web"
)

// `aether dashboard view` serves the Stage 52 read-only dashboard.
//
// The pre-existing `aether dashboard` command is left exactly as it is. It is a
// token-authenticated view of a rendered graph plus a workspace event stream,
// and it is existing behaviour with existing callers; Stage 52 adds a different
// thing rather than replacing it, so it lands as a subcommand instead of a
// second top-level command with the same name.
var (
	dashViewGraphFile string
	dashViewAuditFile string
	dashViewKeyFile   string
	dashViewPort      int
	dashViewBindAll   bool
	dashViewAllowRmt  bool
	dashViewToken     string
	dashViewWorkspace string
	dashViewOperator  string
	dashViewOpsDir    string
	dashViewCert      string
	dashViewKey       string
	dashViewNoAudit   bool
)

var dashboardViewCmd = &cobra.Command{
	Use:   "view",
	Short: "Serve the read-only engagement dashboard (signed audit chain, time travel, choke points)",
	Long: `Serve the read-only operator dashboard.

The dashboard is a view, not a control plane. It has no POST, PUT, PATCH or
DELETE endpoint, and its WebSocket accepts no commands: every technique still
executes from the CLI, through the spine. What this adds is an in-browser check
of the audit chain, performed by a WebAssembly verifier that does not trust the
server serving it, plus the ability to see the identity graph as it stood at any
signed point in the chain.

It binds 127.0.0.1 by default. Binding a routable address requires both
--bind-all and --allow-remote-binding, and a capability, because the surface
exposes the whole engagement at once.`,
	RunE: runDashboardView,
}

func init() {
	dashboardCmd.AddCommand(dashboardViewCmd)

	f := dashboardViewCmd.Flags()
	f.StringVar(&dashViewGraphFile, "graph", "aether-graph.json", "Identity graph file to render")
	f.StringVar(&dashViewAuditFile, "audit", "aether-audit.jsonl", "Audit chain file to verify and replay")
	f.StringVar(&dashViewKeyFile, "audit-key", "aether-audit.key", "Ed25519 audit signing key file")
	f.IntVar(&dashViewPort, "port", 8443, "TCP port to listen on (loopback by default)")
	f.BoolVar(&dashViewBindAll, "bind-all", false, "Bind all interfaces (also requires --allow-remote-binding)")
	f.BoolVar(&dashViewAllowRmt, "allow-remote-binding", false, "Second, independent switch required to bind a non-loopback address")
	f.StringVar(&dashViewToken, "require-capability", "", "Capability token every request must present in X-Aether-Capability")
	f.StringVar(&dashViewWorkspace, "workspace", "", "Engagement/workspace name shown in the UI")
	f.StringVar(&dashViewOperator, "operator", "", "Operator identity shown in the UI, and whose capability file is checked")
	f.StringVar(&dashViewOpsDir, "operators-dir", ".", "Directory holding per-operator capability files, read when --operator is given")
	f.StringVar(&dashViewCert, "tls-cert", "", "Serve HTTPS with this certificate")
	f.StringVar(&dashViewKey, "tls-key", "", "Serve HTTPS with this key (required with --tls-cert)")
	f.BoolVar(&dashViewNoAudit, "no-audit", false, "Start without an audit chain (nothing can be verified; the UI says so)")
}

func runDashboardView(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("dashboard view takes no positional arguments, got %d", len(args))
	}
	if (dashViewCert == "") != (dashViewKey == "") {
		return errors.New("--tls-cert and --tls-key must be set together")
	}
	if err := checkDashboardViewCapability(); err != nil {
		return err
	}

	src, auditLog, err := dashboardViewSource()
	if err != nil {
		return err
	}

	srv, err := web.New(web.Config{
		Port:               dashViewPort,
		BindAll:            dashViewBindAll,
		AllowRemoteBinding: dashViewAllowRmt,
		TLSCert:            dashViewCert,
		TLSKey:             dashViewKey,
		Workspace:          dashViewWorkspace,
		Operator:           dashViewOperator,
		Capability:         dashViewToken,
	}, src, auditLog)
	if err != nil {
		return err
	}

	// Bind before announcing anything, so the operator is told the real port even
	// when they asked for an ephemeral one, and a port collision is reported as an
	// error rather than as a dashboard that silently never appeared.
	addr, err := srv.Listen()
	if err != nil {
		return err
	}

	for _, line := range dashboardViewAssets() {
		fmt.Printf("  %s\n", line)
	}
	scheme := "http"
	if dashViewCert != "" {
		scheme = "https"
	}
	fmt.Printf("Aether dashboard: %s://%s/ (read-only)\n", scheme, addr)
	switch {
	case dashViewBindAll:
		fmt.Println("  WARNING: bound to all interfaces; the engagement graph and the full audit chain are reachable from the network")
	case dashViewToken != "":
		fmt.Println("  capability-gated: every request must present X-Aether-Capability")
	default:
		fmt.Println("  loopback only; no capability required because no other host can reach it")
	}

	errc := make(chan error, 1)
	go func() {
		if dashViewCert != "" {
			errc <- srv.ServeTLS()
			return
		}
		errc <- srv.Serve()
	}()

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		// Close is what appends the dashboard.stop entry, so a dashboard that was
		// stopped with Ctrl-C is still accounted for in the chain.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Close()
		<-shutdownCtx.Done()
		return nil
	}
}

// checkDashboardViewCapability enforces the spine rule for the dashboard.
//
// dashboard.read is not in the default operator capability set: the CLI already
// needs read.graph and read.audit to do its job, whereas this dashboard opens a
// network listener that serves the whole engagement in one surface. Naming an
// operator therefore asserts an authorisation claim, and that claim is checked
// against the operator's capability file rather than taken on trust.
//
// A missing capability file is a refusal, not a pass. An operator who has never
// been issued a capability file has not been granted this capability.
func checkDashboardViewCapability() error {
	if dashViewOperator == "" {
		return nil
	}
	caps, err := api.LoadOperatorCaps(dashViewOpsDir, dashViewOperator)
	if err != nil {
		return fmt.Errorf(
			"refusing to start the dashboard for operator %q: %w. dashboard.read is not granted by default; "+
				"issue it with 'aether serve cert issue --caps dashboard.read', or start without --operator on loopback",
			dashViewOperator, err)
	}
	if !caps.Has(api.CapReadDashboard) {
		return fmt.Errorf(
			"refusing to start the dashboard for operator %q: the %s capability is not granted. "+
				"Grant it with 'aether serve cert issue --caps dashboard.read'",
			dashViewOperator, api.CapReadDashboard)
	}
	return nil
}

// dashboardViewSource opens the graph and audit chain the dashboard will serve.
//
// Both are optional, and each missing piece is reported rather than fatal: a
// dashboard over a graph with no chain can still show the graph, it simply
// cannot attest to how the graph was discovered, and the UI says exactly that.
// What is never allowed is an absent chain presented as though it were a
// verified one, which is why web.Source tracks the absence separately and every
// view surfaces it.
func dashboardViewSource() (*web.Source, *store.Log, error) {
	g, err := graph.LoadGraph(dashViewGraphFile)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, nil, fmt.Errorf("read graph %s: %w", dashViewGraphFile, err)
		}
		fmt.Fprintf(os.Stderr,
			"note: %s does not exist; the dashboard will render an empty graph and say so\n", dashViewGraphFile)
	}

	if dashViewNoAudit {
		fmt.Fprintln(os.Stderr,
			"note: --no-audit was given, so this dashboard cannot verify anything it shows")
		return web.NewSource(nil, g), nil, nil
	}

	auditLog, err := store.New(dashViewAuditFile, dashViewKeyFile)
	if err != nil {
		return nil, nil, fmt.Errorf("open audit chain %s: %w", dashViewAuditFile, err)
	}

	// Surface an existing chain's integrity before serving it, so an operator is
	// never shown a picture of a tampered engagement without being told.
	//
	// The two failures are reported separately. A bad hash or signature is a
	// forged or corrupted entry; a linkage break is a different fault, and
	// BrokenChainAt is 0 when there is none. Printing "breaks at seq 0" for a
	// chain whose only fault is a rewritten payload would be a false claim
	// about where the fault is.
	if res, err := auditLog.Verify(); err == nil && !res.ValidAll {
		fmt.Fprintf(os.Stderr,
			"WARNING: the audit chain at %s does not verify (%d of %d entries failed hash or signature checks",
			dashViewAuditFile, res.Total-res.Valid, res.Total)
		if res.BrokenChainAt > 0 {
			fmt.Fprintf(os.Stderr, "; linkage first breaks at seq %d", res.BrokenChainAt)
		} else {
			fmt.Fprint(os.Stderr, "; linkage is intact")
		}
		fmt.Fprint(os.Stderr,
			"). The dashboard will still serve it, and the browser-side verifier will reach the same verdict.\n")
	}

	return web.NewSource(auditLog, g), auditLog, nil
}

// dashboardViewAssets prints the embedded asset inventory at startup.
//
// A dashboard that silently fails to load its verifier or its layout module is
// indistinguishable from one with nothing to report, so what was actually
// embedded is stated on the console where the operator will see it.
func dashboardViewAssets() []string {
	present := web.AssetPresence()
	paths := make([]string, 0, len(present))
	for p := range present {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		marker := ""
		if strings.EqualFold(present[p], "MISSING") {
			marker = "   <-- MISSING"
		}
		out = append(out, fmt.Sprintf("asset %-40s %s%s", p, present[p], marker))
	}
	return out
}
