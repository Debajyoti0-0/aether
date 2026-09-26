package ad

import (
	"fmt"
	"time"

	"github.com/Debajyoti0-0/aether/internal/cli"
	"github.com/Debajyoti0-0/aether/internal/engagement"
	"github.com/Debajyoti0-0/aether/internal/engine/ad/kerberos"
	"github.com/Debajyoti0-0/aether/internal/engine/mutation"
	"github.com/spf13/cobra"
)

func newTGTCmd() *cobra.Command {
	var (
		domain     string
		dc         string
		username   string
		password   string
		ccachePath string
		keytabPath string
		outputPath string
		workspace  string
		engFile    string
	)

	cmd := &cobra.Command{
		Use:   "tgt",
		Short: "Acquire Ticket Granting Ticket (TGT) and store in ccache",
		Long: `Authenticate to the KDC and acquire a TGT for the specified user.
Stores the resulting credentials in a ccache file for later use.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireEngagementScope(engFile, domain, dc, engagement.CapKerbTGT); err != nil {
				return err
			}

			ws, err := cli.OpenGovernedWorkspace(cli.GetWorkspaceFlag(cmd))
			if err != nil {
				return err
			}

			if username == "" {
				return fmt.Errorf("--username required")
			}

			if password == "" && ccachePath == "" && keytabPath == "" {
				return fmt.Errorf("one of --password, --ccache, or --keytab required")
			}

			mut := &kerberos.TGTMutation{
				Input: kerberos.TGTInput{
					Domain:     domain,
					DC:         dc,
					Username:   username,
					Password:   password,
					CCachePath: ccachePath,
					OutputPath: outputPath,
					KeyTabPath: keytabPath,
				},
			}

			res, err := mutation.Run(cmd.Context(), ws, mut)
			if err != nil {
				return err
			}

			fmt.Printf("Action: %s\nStatus: %s\n", res.ActionID, res.Status)
			if cli.IsJSONOutput(cmd) {
				return cli.PrintJSON(map[string]any{
					"ccache_path": mut.Output.CCachePath,
					"principal":   mut.Output.Principal,
					"realm":       mut.Output.Realm,
					"start_time":  mut.Output.StartTime,
					"end_time":    mut.Output.EndTime,
				})
			}
			fmt.Printf("TGT acquired for %s\n", mut.Output.Principal)
			fmt.Printf("CCache stored at: %s\n", mut.Output.CCachePath)
			fmt.Printf("Valid: %s to %s\n", mut.Output.StartTime.Format(time.RFC3339), mut.Output.EndTime.Format(time.RFC3339))
			return nil
		},
	}

	cmd.Flags().StringVar(&domain, "domain", "", "Target domain (FQDN)")
	cmd.Flags().StringVar(&dc, "dc", "", "Domain controller hostname or IP")
	cmd.Flags().StringVar(&username, "username", "", "Username to authenticate as")
	cmd.Flags().StringVar(&password, "password", "", "Password for the user")
	cmd.Flags().StringVar(&ccachePath, "ccache", "", "Path to existing ccache for auth")
	cmd.Flags().StringVar(&keytabPath, "keytab", "", "Path to keytab file (not yet implemented)")
	cmd.Flags().StringVar(&outputPath, "output", "", "Output ccache path (default: /tmp/<user>_<timestamp>.ccache)")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace name (required for governance)")
	bindEngagementFlag(cmd, &engFile)

	cmd.MarkFlagRequired("domain")
	cmd.MarkFlagRequired("dc")
	cmd.MarkFlagRequired("workspace")

	return cmd
}

func newCCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ccache",
		Short: "CCache management (show, convert)",
		Long:  "Inspect, convert, and manage Kerberos credential caches",
	}

	cmd.AddCommand(newCCacheShowCmd())
	cmd.AddCommand(newCCacheConvertCmd())

	return cmd
}

func newCCacheShowCmd() *cobra.Command {
	var (
		ccachePath string
		workspace  string
	)

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show contents of a ccache file",
		Long:  "Display all entries in a Kerberos credential cache file",
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := cli.OpenGovernedWorkspace(cli.GetWorkspaceFlag(cmd))
			if err != nil {
				return err
			}

			if ccachePath == "" && len(args) > 0 {
				ccachePath = args[0]
			}

			if ccachePath == "" {
				return fmt.Errorf("ccache path required")
			}

			mut := &kerberos.CCacheMutation{
				Input: kerberos.CCacheInput{
					CCachePath: ccachePath,
					Action:     "show",
				},
			}

			res, err := mutation.Run(cmd.Context(), ws, mut)
			if err != nil {
				return err
			}

			fmt.Printf("Action: %s\nStatus: %s\n", res.ActionID, res.Status)
			if cli.IsJSONOutput(cmd) {
				return cli.PrintJSON(map[string]any{
					"entries":      len(mut.Output.Entries),
					"output_path":  mut.Output.Output,
				})
			}
			fmt.Printf("CCache: %s (%d entries)\n", ccachePath, len(mut.Output.Entries))
			for i, e := range mut.Output.Entries {
				// ClientPrincipal and ServerPrincipal are already fully qualified
				// by PrincipalName.FullName, which joins the components with "/"
				// and appends "@" + realm. Appending the realm again here printed
				// "user1@AETHER.TEST@AETHER.TEST"; the slash form used here is
				// what MIT's own klist prints for the same credential.
				fmt.Printf("  [%d] %s -> %s (etype=%d, flags=0x%x, valid=%s to %s)\n",
					i, e.ClientPrincipal, e.ServerPrincipal,
					e.KeyType, e.TicketFlags, e.StartTime.Format(time.RFC3339), e.EndTime.Format(time.RFC3339))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&ccachePath, "ccache", "", "Path to ccache file")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace name (required for governance)")

	return cmd
}

func newCCacheConvertCmd() *cobra.Command {
	var (
		inputPath  string
		outputPath string
		workspace  string
	)

	cmd := &cobra.Command{
		Use:   "convert",
		Short: "Convert ccache between MIT and Heimdal formats",
		Long:  "Convert a ccache file between MIT and Heimdal formats (both use same format, this validates and rewrites)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := cli.OpenGovernedWorkspace(cli.GetWorkspaceFlag(cmd))
			if err != nil {
				return err
			}

			if inputPath == "" && len(args) > 0 {
				inputPath = args[0]
			}

			if inputPath == "" {
				return fmt.Errorf("input ccache path required")
			}

			if outputPath == "" {
				return fmt.Errorf("--output required")
			}

			mut := &kerberos.CCacheMutation{
				Input: kerberos.CCacheInput{
					CCachePath:  inputPath,
					Action:      "convert",
					OutputPath:  outputPath,
				},
			}

			res, err := mutation.Run(cmd.Context(), ws, mut)
			if err != nil {
				return err
			}

			fmt.Printf("Action: %s\nStatus: %s\n", res.ActionID, res.Status)
			if cli.IsJSONOutput(cmd) {
				return cli.PrintJSON(map[string]any{
					"input_path":   inputPath,
					"output_path":  mut.Output.Output,
					"entry_count":  len(mut.Output.Entries),
				})
			}
			fmt.Printf("Converted %s -> %s (%d entries)\n", inputPath, outputPath, len(mut.Output.Entries))
			return nil
		},
	}

	cmd.Flags().StringVar(&inputPath, "input", "", "Input ccache path")
	cmd.Flags().StringVar(&outputPath, "output", "", "Output ccache path")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace name (required for governance)")

	cmd.MarkFlagRequired("output")
	cmd.MarkFlagRequired("workspace")

	return cmd
}