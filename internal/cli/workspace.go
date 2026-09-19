package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/workspace"
)

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Engagement workspace management",
	Long:  "Create, list, report, and securely delete engagement workspaces. All tokens and evidence are AES-256-GCM encrypted at rest (Argon2id key derivation).",
}

var wsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		name := wsName
		if name == "" && len(args) > 0 {
			name = args[0]
		}
		pass := wsCreatePassphrase
		if pass == "" {
			pass = os.Getenv("AETHER_PASSPHRASE")
		}
		if pass == "" && !wsAllowKeyless {
			return fmt.Errorf("a passphrase is required: pass --passphrase or set AETHER_PASSPHRASE. " +
				"Keyless mode (no passphrase) is explicitly discouraged; if you truly need it pass --allow-empty-passphrase")
		}
		w, err := workspace.Create(name, pass)
		if err != nil {
			return err
		}
		if w.Keyless {
			fmt.Fprintln(os.Stderr, "WARNING: workspace created in KEYLESS mode; the encryption key is trivially derivable. Rekey as soon as possible.")
		}
		fmt.Printf("Workspace %q created at %s\n", w.Name, w.Root)
		return nil
	},
}

var wsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List workspaces",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		names, err := workspace.List()
		if err != nil {
			return err
		}
		if len(names) == 0 {
			fmt.Println("No workspaces. Create one: aether workspace create <name>")
			return nil
		}
		for _, n := range names {
			fmt.Println(n)
		}
		return nil
	},
}

var wsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Securely delete a workspace (shred + remove)",
	RunE: func(cmd *cobra.Command, args []string) error {
		name := wsName
		if name == "" && len(args) > 0 {
			name = args[0]
		}
		if !wsForce {
			fmt.Fprintf(os.Stderr, "This shreds and deletes workspace %q. Re-run with --force to confirm.\n", name)
			return nil
		}
		// Require passphrase for deletion to prevent unauthorized deletion
		pass := wsPassphrase
		if pass == "" {
			pass = os.Getenv("AETHER_PASSPHRASE")
		}
		if pass == "" {
			return fmt.Errorf("a passphrase is required for deletion: pass --passphrase or set AETHER_PASSPHRASE")
		}
		// Validate passphrase by opening the workspace
		w, err := workspace.Open(name, pass)
		if err != nil {
			return fmt.Errorf("invalid passphrase for workspace %q: %w", name, err)
		}
		// Close the workspace (we just needed to verify the passphrase)
		_ = w.Close()
		if err := workspace.Delete(name); err != nil {
			return err
		}
		fmt.Printf("Workspace %q securely deleted.\n", name)
		return nil
	},
}

var wsReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate the workspace engagement report",
	RunE: func(cmd *cobra.Command, args []string) error {
		name := wsName
		if name == "" && len(args) > 0 {
			name = args[0]
		}
		w, err := workspace.Open(name, passphraseOrEnv(wsPassphrase))
		if err != nil {
			return err
		}

		events, err := w.Events()
		if err != nil {
			return err
		}

		var b []byte
		header := fmt.Sprintf("# Aether Workspace Report: %s\n\n%d events\n\n", w.Name, len(events))
		b = append(b, header...)
		for _, ev := range events {
			b = append(b, fmt.Sprintf("- %s — %s: %s\n", ev.Time.Format("2006-01-02 15:04:05"), ev.Kind, ev.Detail)...)
		}

		if wsOutput != "" {
			if err := os.WriteFile(wsOutput, b, 0o600); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Report written to %s\n", wsOutput)
			return nil
		}
		os.Stdout.Write(b)
		return nil
	},
}

var wsStatsCmd = &cobra.Command{
	Use:   "info",
	Short: "Show workspace details",
	RunE: func(cmd *cobra.Command, args []string) error {
		name := wsName
		if name == "" && len(args) > 0 {
			name = args[0]
		}
		w, err := workspace.Open(name, passphraseOrEnv(wsPassphrase))
		if err != nil {
			return err
		}

		out := map[string]any{"name": w.Name, "root": w.Root}
		for _, bucket := range []string{workspace.BucketTokens, workspace.BucketIdentities, workspace.BucketEvidence} {
			keys, _ := w.ListRecords(bucket)
			out[bucket] = len(keys)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

var (
	wsName             string
	wsForce            bool
	wsPassphrase       string
	wsCreatePassphrase string
	wsAllowKeyless     bool
	wsOutput           string
)

// passphraseOrEnv returns the explicit passphrase or the
// AETHER_PASSPHRASE environment fallback.
func passphraseOrEnv(pass string) string {
	if pass == "" {
		return os.Getenv("AETHER_PASSPHRASE")
	}
	return pass
}

func init() {
	rootCmd.AddCommand(workspaceCmd)
	workspaceCmd.AddCommand(wsCreateCmd, wsListCmd, wsDeleteCmd, wsReportCmd, wsStatsCmd)

	for _, c := range []*cobra.Command{wsDeleteCmd, wsReportCmd, wsStatsCmd} {
		c.Flags().StringVar(&wsName, "workspace", "", "Workspace name (or pass as arg)")
	}
	wsCreateCmd.Flags().StringVar(&wsCreatePassphrase, "passphrase", "", "Workspace passphrase (or AETHER_PASSPHRASE env)")
	wsCreateCmd.Flags().BoolVar(&wsAllowKeyless, "allow-empty-passphrase", false, "Explicitly create a keyless workspace (strongly discouraged; emits warnings on every open)")
	wsDeleteCmd.Flags().BoolVar(&wsForce, "force", false, "Confirm deletion")
	wsDeleteCmd.Flags().StringVar(&wsPassphrase, "passphrase", "", "Workspace passphrase (or AETHER_PASSPHRASE env)")
	wsReportCmd.Flags().StringVar(&wsPassphrase, "passphrase", "", "Workspace passphrase (or AETHER_PASSPHRASE env)")
	wsReportCmd.Flags().StringVar(&wsOutput, "output", "", "Write report to file")
	wsStatsCmd.Flags().StringVar(&wsPassphrase, "passphrase", "", "Workspace passphrase (or AETHER_PASSPHRASE env)")
}
