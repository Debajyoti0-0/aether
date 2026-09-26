package ad

import (
	"fmt"
	"os"
	"strings"

	"github.com/Debajyoti0-0/aether/internal/cli"
	"github.com/Debajyoti0-0/aether/internal/engagement"
	"github.com/Debajyoti0-0/aether/internal/engine/ad/kerberos"
	"github.com/Debajyoti0-0/aether/internal/engine/mutation"
	"github.com/spf13/cobra"
)

func newEnumCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enum",
		Short: "Active Directory enumeration",
		Long:  "Enumerate users, AS-REP roastable accounts, and SPNs via Kerberos/LDAP",
	}

	cmd.AddCommand(newEnumUsersCmd())
	cmd.AddCommand(newEnumASREPCmd())
	cmd.AddCommand(newEnumSPNCmd())

	return cmd
}

func newEnumUsersCmd() *cobra.Command {
	var (
		domain    string
		dc        string
		userlist  string
		workspace string
		engFile   string
	)

	cmd := &cobra.Command{
		Use:   "users",
		Short: "Enumerate valid usernames via Kerberos AS-REQ",
		Long: `Enumerate valid usernames by sending AS-REQ requests to the KDC.
Accounts that exist will return PREAUTH_REQUIRED or AS-REP.
Accounts that don't exist will return KDC_ERR_C_PRINCIPAL_UNKNOWN.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireEngagementScope(engFile, domain, dc, engagement.CapEnumRead); err != nil {
				return err
			}

			ws, err := cli.OpenGovernedWorkspace(cli.GetWorkspaceFlag(cmd))
			if err != nil {
				return err
			}

			usernames := []string{}
			if userlist != "" {
				data, err := os.ReadFile(userlist)
				if err != nil {
					return fmt.Errorf("read userlist: %w", err)
				}
				// Split into LINES, not runes: `for _, line := range string(data)`
				// iterates code points, turning every character into a "username".
				for _, line := range strings.Split(string(data), "\n") {
					line = strings.TrimSpace(line)
					if line != "" {
						usernames = append(usernames, line)
					}
				}
			} else if len(args) > 0 {
				usernames = args
			}

			if len(usernames) == 0 {
				return fmt.Errorf("usernames required via args or --userlist")
			}

			mut := &kerberos.EnumUsersMutation{
				Domain:    domain,
				DC:        dc,
				Usernames: usernames,
			}

			res, err := mutation.Run(cmd.Context(), ws, mut)
			if err != nil {
				return err
			}

			fmt.Printf("Action: %s\nStatus: %s\n", res.ActionID, res.Status)
			if cli.IsJSONOutput(cmd) {
				return cli.PrintJSON(map[string]any{
					"domain": domain,
					"dc":     dc,
					"count":  len(mut.Results),
				})
			}
			fmt.Printf("Enumerated %d users\n", len(mut.Results))
			for _, r := range mut.Results {
				if r.Error != "" {
					fmt.Printf("  %s: %s\n", r.Username, r.Error)
				} else {
					fmt.Printf("  %s (%s) preauth=%v\n", r.Username, r.Principal, r.PreauthRequired)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&domain, "domain", "", "Target domain (FQDN)")
	cmd.Flags().StringVar(&dc, "dc", "", "Domain controller hostname or IP")
	cmd.Flags().StringVar(&userlist, "userlist", "", "Path to file with usernames (one per line)")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace name (required for governance)")
	bindEngagementFlag(cmd, &engFile)

	cmd.MarkFlagRequired("domain")
	cmd.MarkFlagRequired("dc")
	cmd.MarkFlagRequired("workspace")

	return cmd
}

func newEnumASREPCmd() *cobra.Command {
	var (
		domain    string
		dc        string
		userlist  string
		workspace string
		engFile   string
	)

	cmd := &cobra.Command{
		Use:   "asrep",
		Short: "Enumerate AS-REP roastable accounts (no pre-auth)",
		Long: `Enumerate accounts that do not require Kerberos pre-authentication.
These accounts are vulnerable to AS-REP roasting.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireEngagementScope(engFile, domain, dc, engagement.CapEnumRead); err != nil {
				return err
			}

			ws, err := cli.OpenGovernedWorkspace(cli.GetWorkspaceFlag(cmd))
			if err != nil {
				return err
			}

			usernames := []string{}
			if userlist != "" {
				data, err := os.ReadFile(userlist)
				if err != nil {
					return fmt.Errorf("read userlist: %w", err)
				}
				// Split into LINES, not runes (see EnumUsers fix rationale).
				for _, line := range strings.Split(string(data), "\n") {
					line = strings.TrimSpace(line)
					if line != "" {
						usernames = append(usernames, line)
					}
				}
			} else if len(args) > 0 {
				usernames = args
			}

			if len(usernames) == 0 {
				return fmt.Errorf("usernames required via args or --userlist")
			}

			mut := &kerberos.EnumASREPMutation{
				Domain: domain,
				DC:     dc,
				Users:  usernames,
			}

			res, err := mutation.Run(cmd.Context(), ws, mut)
			if err != nil {
				return err
			}

			fmt.Printf("Action: %s\nStatus: %s\n", res.ActionID, res.Status)
			if cli.IsJSONOutput(cmd) {
				return cli.PrintJSON(map[string]any{
					"domain": domain,
					"dc":     dc,
					"count":  len(mut.Results),
				})
			}
			fmt.Printf("Found %d AS-REP roastable accounts\n", len(mut.Results))
			for _, r := range mut.Results {
				fmt.Printf("  %s (%s)\n", r.Username, r.Principal)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&domain, "domain", "", "Target domain (FQDN)")
	cmd.Flags().StringVar(&dc, "dc", "", "Domain controller hostname or IP")
	cmd.Flags().StringVar(&userlist, "userlist", "", "Path to file with usernames (one per line)")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace name (required for governance)")
	bindEngagementFlag(cmd, &engFile)

	cmd.MarkFlagRequired("domain")
	cmd.MarkFlagRequired("dc")
	cmd.MarkFlagRequired("workspace")

	return cmd
}

func newEnumSPNCmd() *cobra.Command {
	var (
		domain    string
		dc        string
		workspace string
		engFile   string
	)

	cmd := &cobra.Command{
		Use:   "spn",
		Short: "Enumerate Service Principal Names",
		Long:  `Enumerate SPN-registered accounts via LDAP.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireEngagementScope(engFile, domain, dc, engagement.CapEnumRead); err != nil {
				return err
			}

			ws, err := cli.OpenGovernedWorkspace(cli.GetWorkspaceFlag(cmd))
			if err != nil {
				return err
			}

			mut := &kerberos.EnumSPNMutation{
				Domain: domain,
				DC:     dc,
			}

			res, err := mutation.Run(cmd.Context(), ws, mut)
			if err != nil {
				return err
			}

			fmt.Printf("Action: %s\nStatus: %s\n", res.ActionID, res.Status)
			if cli.IsJSONOutput(cmd) {
				return cli.PrintJSON(map[string]any{
					"domain": domain,
					"dc":     dc,
					"count":  len(mut.Results),
				})
			}
			fmt.Printf("Enumerated %d SPNs\n", len(mut.Results))
			for _, r := range mut.Results {
				fmt.Printf("  %s (%s)\n", r.ServicePrincipal, r.Account)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&domain, "domain", "", "Target domain (FQDN)")
	cmd.Flags().StringVar(&dc, "dc", "", "Domain controller hostname or IP")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace name (required for governance)")
	bindEngagementFlag(cmd, &engFile)

	cmd.MarkFlagRequired("domain")
	cmd.MarkFlagRequired("dc")
	cmd.MarkFlagRequired("workspace")

	return cmd
}
