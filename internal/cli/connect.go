package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/api"
	"github.com/Debajyoti0-0/aether/internal/observability"
	"github.com/Debajyoti0-0/aether/internal/workspace"
)

// connectCmd implements `aether connect` (operator → teamserver).
var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to the teamserver as an authenticated operator",
	RunE:  runConnect,
}

// serveCmd implements the teamserver (`aether serve`).
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the mTLS teamserver (cert-bound operators, capability authz, spine dispatch)",
	RunE:  runServe,
}

var (
	tsAddr        string
	tsWorkspace   string
	tsCommand     string
	tsListenAddr  string
	tsInsecure    bool
	tsInsecureAck bool
	tsPassphrase  string
	tsWorkspaceOp *workspace.Workspace

	tsServerCert string
	tsServerKey  string
	tsCACert     string
	tsOpCert     string
	tsOpKey      string
	tsServerCA   string
	tsOpDir      string
	tsRevoked    string
	tsMaxConns   int

	// Observability
	tsMetricsAddr string
	tsMetricsPath string
	tsHealthPath  string
	tsReadyPath   string
)

func init() {
	rootCmd.AddCommand(connectCmd, serveCmd)

	connectCmd.Flags().StringVar(&tsAddr, "server", "127.0.0.1:7788", "Teamserver address")
	connectCmd.Flags().StringVar(&tsWorkspace, "workspace", "", "Workspace to sync")
	connectCmd.Flags().StringVar(&tsCommand, "exec", "", "Execute one command on the teamserver and exit")
	connectCmd.Flags().StringVar(&tsOpCert, "operator-cert", "", "Operator client certificate (PEM; from 'serve cert issue')")
	connectCmd.Flags().StringVar(&tsOpKey, "operator-key", "", "Operator client key (PEM)")
	connectCmd.Flags().StringVar(&tsServerCA, "server-ca", "", "Teamserver CA certificate to verify the server (default: <operator-cert dir>/teamserver-ca.crt)")
	connectCmd.Flags().BoolVar(&tsInsecure, "insecure", false, "Skip server cert verification (research only)")
	connectCmd.Flags().BoolVar(&tsInsecureAck, "i-know-what-im-doing", false, "Required confirmation alongside --insecure")

	serveCmd.Flags().StringVar(&tsListenAddr, "listen", "127.0.0.1:7788", "Listen address")
	serveCmd.Flags().StringVar(&tsWorkspace, "workspace", "", "Workspace to attach (remote commands execute through the spine)")
	serveCmd.Flags().StringVar(&tsPassphrase, "passphrase", "", "Workspace passphrase (or AETHER_PASSPHRASE env); required with --workspace")
	serveCmd.Flags().StringVar(&tsServerCert, "server-cert", "", "Server certificate PEM (required; from 'serve cert init')")
	serveCmd.Flags().StringVar(&tsServerKey, "server-key", "", "Server key PEM (required)")
	serveCmd.Flags().StringVar(&tsCACert, "ca-cert", "", "CA certificate PEM used to verify operators (required)")
	serveCmd.Flags().StringVar(&tsOpDir, "operators-dir", "", "Operator capability directory (default: <ca-cert dir>/operators)")
	serveCmd.Flags().StringVar(&tsRevoked, "revoked", "", "Revocation list path (default: <ca-cert dir>/revoked.txt)")
	serveCmd.Flags().IntVar(&tsMaxConns, "max-conns", 32, "Maximum concurrent connections")

	// Observability flags
	serveCmd.Flags().StringVar(&tsMetricsAddr, "metrics-addr", "", "Metrics/health listen address (e.g., 127.0.0.1:9090); empty disables")
	serveCmd.Flags().StringVar(&tsMetricsPath, "metrics-path", "/metrics", "Metrics endpoint path")
	serveCmd.Flags().StringVar(&tsHealthPath, "health-path", "/healthz", "Health endpoint path")
	serveCmd.Flags().StringVar(&tsReadyPath, "ready-path", "/readyz", "Readiness endpoint path")
}

// runConnect dials the teamserver with an operator certificate issued
// by the teamserver CA. Self-signed fallbacks are gone (Stage 3, T1):
// an operator certificate is mandatory.
func runConnect(cmd *cobra.Command, args []string) error {
	if tsOpCert == "" || tsOpKey == "" {
		return fmt.Errorf("operator credentials are required: pass --operator-cert and --operator-key (obtain them with 'aether serve cert issue --operator <name>')")
	}
	if tsInsecure {
		if !tsInsecureAck {
			return fmt.Errorf("--insecure disables server certificate verification and must be confirmed with --i-know-what-im-doing")
		}
		fmt.Fprintln(os.Stderr, "WARNING: TLS server verification DISABLED (--insecure). The server identity is NOT authenticated. Research use only.")
	}

	serverCA := tsServerCA
	if serverCA == "" && !tsInsecure {
		serverCA = filepath.Join(filepath.Dir(tsOpCert), "teamserver-ca.crt")
	}

	pair, cas, err := api.LoadOperatorTLS(tsOpCert, tsOpKey, serverCA)
	if err != nil {
		return err
	}
	client, err := api.Dial(tsAddr, pair, cas, tsInsecure)
	if err != nil {
		return err
	}
	defer client.Close()
	fmt.Fprintf(os.Stderr, "Connected to teamserver %s (authenticated operator)\n", tsAddr)

	// Stream workspace updates when requested.
	if tsWorkspace != "" && tsCommand == "" {
		updates, err := client.StreamWorkspace(tsWorkspace)
		if err != nil {
			return err
		}
		ctx, stop := signalContext()
		defer stop()

		fmt.Fprintf(os.Stderr, "Streaming workspace %q (ctrl-c to stop)\n", tsWorkspace)
		for {
			select {
			case <-ctx.Done():
				return nil
			case ev, ok := <-updates:
				if !ok {
					return nil
				}
				fmt.Printf("[%d] %s: %s\n", ev.Timestamp, ev.Kind, ev.Payload)
			}
		}
	}

	if tsCommand != "" {
		resp, err := client.ExecuteCommand(&api.CommandRequest{
			WorkspaceID: tsWorkspace,
			CommandLine: tsCommand,
		})
		if err != nil {
			return err
		}
		fmt.Println(resp.Output)
		if resp.Error != "" {
			fmt.Fprintf(os.Stderr, "error: %s\n", resp.Error)
		}
		return nil
	}

	return fmt.Errorf("specify --exec or --workspace")
}

// runServe starts the teamserver. Refuse-to-start (T1): all four PKI
// files must exist — there is no self-signed fallback.
func runServe(cmd *cobra.Command, args []string) error {
	if tsServerCert == "" || tsServerKey == "" || tsCACert == "" {
		return fmt.Errorf(
			"teamserver PKI is required: pass --server-cert, --server-key and --ca-cert " +
				"(bootstrap with 'aether serve cert init', then issue operator certificates with 'aether serve cert issue')")
	}

	// Fail closed: the teamserver is network-exposed. It must never
	// open a workspace in keyless (empty-passphrase) mode.
	var ws *workspace.Workspace
	if tsWorkspace != "" {
		pass := tsPassphrase
		if pass == "" {
			pass = os.Getenv("AETHER_PASSPHRASE")
		}
		if pass == "" {
			return fmt.Errorf("teamserver workspace requires --passphrase or AETHER_PASSPHRASE; " +
				"keyless mode is not permitted for network-exposed workspaces")
		}
		w, err := workspace.Open(tsWorkspace, pass)
		if err != nil {
			return fmt.Errorf("attach workspace %q: %w", tsWorkspace, err)
		}
		defer w.Close()
		tsWorkspaceOp = w
		ws = w
	}

	srvCert, clientCAs, err := api.LoadServerTLS(tsServerCert, tsServerKey, tsCACert)
	if err != nil {
		return err
	}

	caDir := filepath.Dir(tsCACert)
	operatorsDir := tsOpDir
	if operatorsDir == "" {
		operatorsDir = filepath.Join(caDir, "operators")
	}
	revokedPath := tsRevoked
	if revokedPath == "" {
		revokedPath = filepath.Join(caDir, "revoked.txt")
	}
	revokedData, err := os.ReadFile(revokedPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	revoked := api.LoadRevocationList(revokedData)

	srv, err := api.NewTeamserver(tsListenAddr, srvCert, clientCAs, operatorsDir, revoked, serveCommandRunner)
	if err != nil {
		return err
	}
	srv.SetMaxConnections(tsMaxConns)

	// Start observability server if enabled
	var obsServer *observability.ObservabilityServer
	if tsMetricsAddr != "" {
		obsConfig := observability.DefaultConfig()
		obsConfig.Enabled = true
		obsConfig.ListenAddress = tsMetricsAddr
		obsConfig.ReadTimeout = 10 * time.Second
		obsConfig.WriteTimeout = 10 * time.Second
		obsConfig.IdleTimeout = 60 * time.Second
		obsConfig.ShutdownGrace = 5 * time.Second

		obsServer = observability.NewObservabilityServer(
			ws,
			"", // audit key
			observability.Config{
				Enabled:       true,
				ListenAddress: tsMetricsAddr,
				ReadTimeout:   10 * time.Second,
				WriteTimeout:  10 * time.Second,
				IdleTimeout:   60 * time.Second,
				ShutdownGrace: 5 * time.Second,
			},
		)
		if err := obsServer.Start(); err != nil {
			return fmt.Errorf("start observability server: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Observability server listening on %s (metrics: %s, health: %s, ready: %s)\n",
			tsMetricsAddr, "/metrics", "/healthz", "/readyz")

		// Start system metrics updater
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		obsServer.StartSystemMetricsUpdater(ctx, 15*time.Second)
	}

	fmt.Fprintf(os.Stderr, "Teamserver listening on %s (mTLS: operators verified against %s; revocation: %s)\n",
		tsListenAddr, tsCACert, revokedPath)

	ctx, stop := signalContext()
	defer stop()
	go func() {
		<-ctx.Done()
		_ = srv.Close()
		if obsServer != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = obsServer.Stop(shutdownCtx)
		}
	}()

	return srv.Serve()
}
