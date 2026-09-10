package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/api"
	"github.com/Debajyoti0-0/aether/internal/workspace"
)

// connectCmd implements `aether connect` (operator → teamserver).
var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to the teamserver as an operator",
	RunE:  runConnect,
}

// serveCmd implements the teamserver (`aether serve`).
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the mTLS teamserver for multi-operator sync",
	RunE:  runServe,
}

var (
	tsAddr       string
	tsWorkspace  string
	tsCommand    string
	tsListenAddr string
	tsInsecure   bool
)

func init() {
	rootCmd.AddCommand(connectCmd, serveCmd)

	connectCmd.Flags().StringVar(&tsAddr, "server", "127.0.0.1:7788", "Teamserver address")
	connectCmd.Flags().StringVar(&tsWorkspace, "workspace", "", "Workspace to sync")
	connectCmd.Flags().StringVar(&tsCommand, "exec", "", "Execute one command on the teamserver and exit")
	connectCmd.Flags().BoolVar(&tsInsecure, "insecure", false, "Skip server cert verification (self-signed)")

	serveCmd.Flags().StringVar(&tsListenAddr, "listen", "127.0.0.1:7788", "Listen address")
}

// apiDial connects with a generated operator certificate. Production
// deployments should pin the server CA.
func apiDial(addr string, insecure bool) (*api.TeamClient, error) {
	cert, err := api.GenerateClientCert("cli-operator")
	if err != nil {
		return nil, fmt.Errorf("generate operator cert: %w", err)
	}
	return api.Dial(addr, cert, nil, insecure)
}

func runConnect(cmd *cobra.Command, args []string) error {
	client, err := apiDial(tsAddr, tsInsecure)
	if err != nil {
		return err
	}
	defer client.Close()
	fmt.Fprintf(os.Stderr, "Connected to teamserver %s\n", tsAddr)

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
			Operator:    "cli",
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

func runServe(cmd *cobra.Command, args []string) error {
	cert, err := api.GenerateServerCert([]string{"127.0.0.1", "localhost"})
	if err != nil {
		return err
	}

	srv, err := api.NewTeamserver(tsListenAddr, cert, func(req *api.CommandRequest) (*api.CommandResponse, error) {
		if req.WorkspaceID != "" && workspace.Exists(req.WorkspaceID) {
			if w, err := workspace.Open(req.WorkspaceID, ""); err == nil {
				_ = w.LogEvent("remote_command", req.CommandLine)
			}
		}
		return &api.CommandResponse{OK: true, Output: "queued: " + req.CommandLine}, nil
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Teamserver listening on %s (mTLS, self-signed — rotate for prod)\n", tsListenAddr)

	ctx, stop := signalContext()
	defer stop()
	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()

	return srv.Serve()
}
