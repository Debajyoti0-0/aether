package ad

import (
	"fmt"
	"strings"

	"github.com/Debajyoti0-0/aether/internal/cli"
	"github.com/Debajyoti0-0/aether/internal/engine/ad/acl"
	ldapengine "github.com/Debajyoti0-0/aether/internal/engine/ad/ldap"
	ldapproto "github.com/Debajyoti0-0/aether/internal/protocol/ldap"
	"github.com/spf13/cobra"
)

func init() {
	cli.RegisterModule(&ldapModule{})
}

func domainToBaseDN(domain string) string {
	var parts []string
	for _, part := range strings.Split(domain, ".") {
		parts = append(parts, "DC="+part)
	}
	return strings.Join(parts, ",")
}

func createLDAPEngine(host string, port int, useTLS, startTLS bool) *ldapengine.Engine {
	if startTLS {
		return ldapengine.NewEngineWithStartTLS(host, port)
	}
	return ldapengine.NewEngine(host, port, useTLS)
}

// requireBindArgs validates the optional positional credential pair
// (<username> <password>) used by the ldap enum/acl/path commands.
// Without this check, args[0]/args[1] indexing panics when arguments are
// omitted (Stage 46g live defect: "index out of range [0] with length 0").
func requireBindArgs(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("bind credentials required: expected exactly 2 positional arguments <username> <password>, got %d", len(args))
	}
	return nil
}

type ldapModule struct{}

func (m *ldapModule) Name() string { return "ad.ldap" }

func (m *ldapModule) Commands() []*cobra.Command {
	return []*cobra.Command{
		newLDAPCmd(),
	}
}

func newLDAPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ldap",
		Short: "LDAP enumeration and ACL analysis",
		Long:  `LDAP operations including directory enumeration, security descriptor analysis, and ACL path finding.`,
	}

	rejectUnknownSubcommand(cmd)

	cmd.AddCommand(newLDAPBindCmd())
	cmd.AddCommand(newLDAPEnumCmd())
	cmd.AddCommand(newLDAPACLCmd())
	cmd.AddCommand(newLDAPPathCmd())
	cmd.AddCommand(newLDAPRootDSECmd())

	return cmd
}

func newLDAPBindCmd() *cobra.Command {
	var (
		host      string
		port      int
		useTLS    bool
		startTLS  bool
		bindDN    string
		bindPass  string
		workspace string
	)

	cmd := &cobra.Command{
		Use:   "bind",
		Short: "Test LDAP bind/connection",
		Long:  `Test LDAP connection and authentication against a domain controller.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := cli.OpenGovernedWorkspace(cli.GetWorkspaceFlag(cmd))
			if err != nil {
				return err
			}

			e := ldapengine.NewEngine(host, port, useTLS)
			if startTLS {
				e = ldapengine.NewEngineWithStartTLS(host, port)
			}
			e.SetTimeout(30 * 1000 * 1000 * 1000) // 30 seconds

			if err := e.Connect(cmd.Context()); err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer e.Close()

			if startTLS {
				if err := e.StartTLS(cmd.Context()); err != nil {
					return fmt.Errorf("starttls: %w", err)
				}
			}

			if err := e.Bind(cmd.Context(), bindDN, bindPass); err != nil {
				return fmt.Errorf("bind: %w", err)
			}

			fmt.Printf("Successfully bound as %s\n", bindDN)

			if cli.IsJSONOutput(cmd) {
				return cli.PrintJSON(map[string]any{
					"host":   host,
					"port":   port,
					"binddn": bindDN,
					"status": "success",
				})
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&host, "host", "", "LDAP server hostname or IP")
	cmd.Flags().IntVar(&port, "port", 389, "LDAP server port")
	cmd.Flags().BoolVar(&useTLS, "tls", false, "Use LDAPS (TLS)")
	cmd.Flags().BoolVar(&startTLS, "starttls", false, "Use StartTLS")
	cmd.Flags().StringVar(&bindDN, "bind-dn", "", "Bind DN")
	cmd.Flags().StringVar(&bindPass, "bind-pass", "", "Bind password")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace name (required for governance)")

	cmd.MarkFlagRequired("host")
	cmd.MarkFlagRequired("bind-dn")
	cmd.MarkFlagRequired("bind-pass")
	cmd.MarkFlagRequired("workspace")

	return cmd
}

func newLDAPEnumCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enum",
		Short: "LDAP directory enumeration",
		Long:  `Enumerate objects in the directory (users, groups, computers, OUs, SPNs).`,
	}

	rejectUnknownSubcommand(cmd)

	cmd.AddCommand(newLDAPEnumUsersCmd())
	cmd.AddCommand(newLDAPEnumGroupsCmd())
	cmd.AddCommand(newLDAPEnumComputersCmd())
	cmd.AddCommand(newLDAPEnumOUsCmd())
	cmd.AddCommand(newLDAPEnumSPNCmd())
	cmd.AddCommand(newLDAPEnumAllCmd())

	return cmd
}

func newLDAPEnumUsersCmd() *cobra.Command {
	var (
		domain    string
		dc        string
		filter    string
		attrs     string
		limit     int
		workspace string
		useTLS    bool
		startTLS  bool
		port      int
		engFile   string
	)

	cmd := &cobra.Command{
		Use:   "users",
		Short: "Enumerate user objects",
		Long:  `Enumerate user objects in the directory.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireEngagementScope(engFile, domain, dc, ldapCapability(cmd)); err != nil {
				return err
			}

			_, err := cli.OpenGovernedWorkspace(cli.GetWorkspaceFlag(cmd))
			if err != nil {
				return err
			}

			e := createLDAPEngine(dc, port, useTLS, startTLS)
			if err := e.Connect(cmd.Context()); err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer e.Close()

			if startTLS {
				if err := e.StartTLS(cmd.Context()); err != nil {
					return fmt.Errorf("starttls: %w", err)
				}
			}

			if err := requireBindArgs(args); err != nil {
				return err
			}
			e.SetBindCredentials(fmt.Sprintf("%s@%s", args[0], domain), args[1])
			e.SetBaseDN(domainToBaseDN(domain))
			if err := e.Bind(cmd.Context(), e.BindDN, e.BindPass); err != nil {
				return fmt.Errorf("bind: %w", err)
			}

			aclEngine := acl.NewEngine(e)

			attrsList := []string{}
			if attrs != "" {
				attrsList = strings.Split(attrs, ",")
			}

			baseDN := domainToBaseDN(domain)
			principals, err := aclEngine.EnumerateUsers(cmd.Context(), acl.EnumerateObjectsOptions{
				ObjectClass: "user",
				Filter:      filter,
				Attributes:  attrsList,
				SizeLimit:   limit,
				BaseDN:      baseDN,
			})
			if err != nil {
				return fmt.Errorf("enumerate users: %w", err)
			}

			fmt.Printf("Enumerated %d users\n", len(principals))
			if cli.IsJSONOutput(cmd) {
				return cli.PrintJSON(map[string]any{
					"count": len(principals),
					"users": principals,
				})
			}

			for _, p := range principals {
				sid := ""
				if p.SID != nil {
					sid = p.SID.String()
				}
				fmt.Printf("  %s (%s) SID=%s\n", p.SamAccountName, p.DisplayName, sid)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&domain, "domain", "", "Target domain (FQDN)")
	cmd.Flags().StringVar(&dc, "dc", "", "Domain controller hostname or IP")
	cmd.Flags().IntVar(&port, "port", 389, "LDAP server port")
	cmd.Flags().BoolVar(&useTLS, "tls", false, "Use LDAPS (TLS)")
	cmd.Flags().BoolVar(&startTLS, "starttls", false, "Use StartTLS")
	cmd.Flags().StringVar(&filter, "filter", "(&(objectClass=user)(!(objectClass=computer)))", "LDAP filter")
	cmd.Flags().StringVar(&attrs, "attrs", "", "Comma-separated attributes to retrieve")
	cmd.Flags().IntVar(&limit, "limit", 1000, "Maximum results")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace name (required for governance)")

	bindEngagementFlag(cmd, &engFile)

	cmd.MarkFlagRequired("domain")
	cmd.MarkFlagRequired("dc")
	cmd.MarkFlagRequired("workspace")

	return cmd
}

func newLDAPEnumGroupsCmd() *cobra.Command {
	var (
		domain    string
		dc        string
		filter    string
		attrs     string
		limit     int
		workspace string
		useTLS    bool
		startTLS  bool
		port      int
		engFile   string
	)

	cmd := &cobra.Command{
		Use:   "groups",
		Short: "Enumerate group objects",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireEngagementScope(engFile, domain, dc, ldapCapability(cmd)); err != nil {
				return err
			}

			_, err := cli.OpenGovernedWorkspace(cli.GetWorkspaceFlag(cmd))
			if err != nil {
				return err
			}

			e := createLDAPEngine(dc, port, useTLS, startTLS)
			if err := e.Connect(cmd.Context()); err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer e.Close()

			if startTLS {
				if err := e.StartTLS(cmd.Context()); err != nil {
					return fmt.Errorf("starttls: %w", err)
				}
			}

			if err := requireBindArgs(args); err != nil {
				return err
			}
			e.SetBindCredentials(fmt.Sprintf("%s@%s", args[0], domain), args[1])
			e.SetBaseDN(domainToBaseDN(domain))
			if err := e.Bind(cmd.Context(), e.BindDN, e.BindPass); err != nil {
				return fmt.Errorf("bind: %w", err)
			}

			aclEngine := acl.NewEngine(e)

			attrsList := []string{}
			if attrs != "" {
				attrsList = strings.Split(attrs, ",")
			}

			baseDN := domainToBaseDN(domain)
			principals, err := aclEngine.EnumerateGroups(cmd.Context(), acl.EnumerateObjectsOptions{
				ObjectClass: "group",
				Filter:      filter,
				Attributes:  attrsList,
				SizeLimit:   limit,
				BaseDN:      baseDN,
			})
			if err != nil {
				return fmt.Errorf("enumerate groups: %w", err)
			}

			fmt.Printf("Enumerated %d groups\n", len(principals))
			if cli.IsJSONOutput(cmd) {
				return cli.PrintJSON(map[string]any{
					"count":  len(principals),
					"groups": principals,
				})
			}

			for _, p := range principals {
				sid := ""
				if p.SID != nil {
					sid = p.SID.String()
				}
				fmt.Printf("  %s (%s) SID=%s\n", p.SamAccountName, p.DisplayName, sid)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&domain, "domain", "", "Target domain (FQDN)")
	cmd.Flags().StringVar(&dc, "dc", "", "Domain controller hostname or IP")
	cmd.Flags().IntVar(&port, "port", 389, "LDAP server port")
	cmd.Flags().BoolVar(&useTLS, "tls", false, "Use LDAPS (TLS)")
	cmd.Flags().BoolVar(&startTLS, "starttls", false, "Use StartTLS")
	cmd.Flags().StringVar(&filter, "filter", "(objectClass=group)", "LDAP filter")
	cmd.Flags().StringVar(&attrs, "attrs", "", "Comma-separated attributes to retrieve")
	cmd.Flags().IntVar(&limit, "limit", 1000, "Maximum results")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace name (required for governance)")

	bindEngagementFlag(cmd, &engFile)

	cmd.MarkFlagRequired("domain")
	cmd.MarkFlagRequired("dc")
	cmd.MarkFlagRequired("workspace")

	return cmd
}

func newLDAPEnumComputersCmd() *cobra.Command {
	var (
		domain    string
		dc        string
		filter    string
		attrs     string
		limit     int
		workspace string
		useTLS    bool
		startTLS  bool
		port      int
		engFile   string
	)

	cmd := &cobra.Command{
		Use:   "computers",
		Short: "Enumerate computer objects",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireEngagementScope(engFile, domain, dc, ldapCapability(cmd)); err != nil {
				return err
			}

			_, err := cli.OpenGovernedWorkspace(cli.GetWorkspaceFlag(cmd))
			if err != nil {
				return err
			}

			e := createLDAPEngine(dc, port, useTLS, startTLS)
			if err := e.Connect(cmd.Context()); err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer e.Close()

			if startTLS {
				if err := e.StartTLS(cmd.Context()); err != nil {
					return fmt.Errorf("starttls: %w", err)
				}
			}

			if err := requireBindArgs(args); err != nil {
				return err
			}
			e.SetBindCredentials(fmt.Sprintf("%s@%s", args[0], domain), args[1])
			e.SetBaseDN(domainToBaseDN(domain))
			if err := e.Bind(cmd.Context(), e.BindDN, e.BindPass); err != nil {
				return fmt.Errorf("bind: %w", err)
			}

			aclEngine := acl.NewEngine(e)

			attrsList := []string{}
			if attrs != "" {
				attrsList = strings.Split(attrs, ",")
			}

			baseDN := domainToBaseDN(domain)
			principals, err := aclEngine.EnumerateComputers(cmd.Context(), acl.EnumerateObjectsOptions{
				ObjectClass: "computer",
				Filter:      filter,
				Attributes:  attrsList,
				SizeLimit:   limit,
				BaseDN:      baseDN,
			})
			if err != nil {
				return fmt.Errorf("enumerate computers: %w", err)
			}

			fmt.Printf("Enumerated %d computers\n", len(principals))
			if cli.IsJSONOutput(cmd) {
				return cli.PrintJSON(map[string]any{
					"count":     len(principals),
					"computers": principals,
				})
			}

			for _, p := range principals {
				sid := ""
				if p.SID != nil {
					sid = p.SID.String()
				}
				fmt.Printf("  %s (%s) SID=%s\n", p.SamAccountName, p.DisplayName, sid)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&domain, "domain", "", "Target domain (FQDN)")
	cmd.Flags().StringVar(&dc, "dc", "", "Domain controller hostname or IP")
	cmd.Flags().IntVar(&port, "port", 389, "LDAP server port")
	cmd.Flags().BoolVar(&useTLS, "tls", false, "Use LDAPS (TLS)")
	cmd.Flags().BoolVar(&startTLS, "starttls", false, "Use StartTLS")
	cmd.Flags().StringVar(&filter, "filter", "(objectClass=computer)", "LDAP filter")
	cmd.Flags().StringVar(&attrs, "attrs", "", "Comma-separated attributes to retrieve")
	cmd.Flags().IntVar(&limit, "limit", 1000, "Maximum results")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace name (required for governance)")

	bindEngagementFlag(cmd, &engFile)

	cmd.MarkFlagRequired("domain")
	cmd.MarkFlagRequired("dc")
	cmd.MarkFlagRequired("workspace")

	return cmd
}

func newLDAPEnumOUsCmd() *cobra.Command {
	var (
		domain    string
		dc        string
		filter    string
		attrs     string
		limit     int
		workspace string
		useTLS    bool
		startTLS  bool
		port      int
		engFile   string
	)

	cmd := &cobra.Command{
		Use:   "ous",
		Short: "Enumerate organizational units",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireEngagementScope(engFile, domain, dc, ldapCapability(cmd)); err != nil {
				return err
			}

			_, err := cli.OpenGovernedWorkspace(cli.GetWorkspaceFlag(cmd))
			if err != nil {
				return err
			}

			e := createLDAPEngine(dc, port, useTLS, startTLS)
			if err := e.Connect(cmd.Context()); err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer e.Close()

			if startTLS {
				if err := e.StartTLS(cmd.Context()); err != nil {
					return fmt.Errorf("starttls: %w", err)
				}
			}

			if err := requireBindArgs(args); err != nil {
				return err
			}
			e.SetBindCredentials(fmt.Sprintf("%s@%s", args[0], domain), args[1])
			e.SetBaseDN(domainToBaseDN(domain))
			if err := e.Bind(cmd.Context(), e.BindDN, e.BindPass); err != nil {
				return fmt.Errorf("bind: %w", err)
			}

			aclEngine := acl.NewEngine(e)

			attrsList := []string{}
			if attrs != "" {
				attrsList = strings.Split(attrs, ",")
			}

			baseDN := domainToBaseDN(domain)
			principals, err := aclEngine.EnumerateOUs(cmd.Context(), acl.EnumerateObjectsOptions{
				ObjectClass: "organizationalUnit",
				Filter:      filter,
				Attributes:  attrsList,
				SizeLimit:   limit,
				BaseDN:      baseDN,
			})
			if err != nil {
				return fmt.Errorf("enumerate OUs: %w", err)
			}

			fmt.Printf("Enumerated %d OUs\n", len(principals))
			if cli.IsJSONOutput(cmd) {
				return cli.PrintJSON(map[string]any{
					"count": len(principals),
					"ous":   principals,
				})
			}

			for _, p := range principals {
				sid := ""
				if p.SID != nil {
					sid = p.SID.String()
				}
				fmt.Printf("  %s SID=%s\n", p.SamAccountName, sid)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&domain, "domain", "", "Target domain (FQDN)")
	cmd.Flags().StringVar(&dc, "dc", "", "Domain controller hostname or IP")
	cmd.Flags().IntVar(&port, "port", 389, "LDAP server port")
	cmd.Flags().BoolVar(&useTLS, "tls", false, "Use LDAPS (TLS)")
	cmd.Flags().BoolVar(&startTLS, "starttls", false, "Use StartTLS")
	cmd.Flags().StringVar(&filter, "filter", "(objectClass=organizationalUnit)", "LDAP filter")
	cmd.Flags().StringVar(&attrs, "attrs", "", "Comma-separated attributes to retrieve")
	cmd.Flags().IntVar(&limit, "limit", 1000, "Maximum results")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace name (required for governance)")

	bindEngagementFlag(cmd, &engFile)

	cmd.MarkFlagRequired("domain")
	cmd.MarkFlagRequired("dc")
	cmd.MarkFlagRequired("workspace")

	return cmd
}

func newLDAPEnumSPNCmd() *cobra.Command {
	var (
		domain    string
		dc        string
		limit     int
		workspace string
		useTLS    bool
		startTLS  bool
		port      int
		engFile   string
	)

	cmd := &cobra.Command{
		Use:   "spns",
		Short: "Enumerate service principal names via LDAP",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireEngagementScope(engFile, domain, dc, ldapCapability(cmd)); err != nil {
				return err
			}

			_, err := cli.OpenGovernedWorkspace(cli.GetWorkspaceFlag(cmd))
			if err != nil {
				return err
			}

			e := createLDAPEngine(dc, port, useTLS, startTLS)
			if err := e.Connect(cmd.Context()); err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer e.Close()

			if startTLS {
				if err := e.StartTLS(cmd.Context()); err != nil {
					return fmt.Errorf("starttls: %w", err)
				}
			}

			if err := requireBindArgs(args); err != nil {
				return err
			}
			e.SetBindCredentials(fmt.Sprintf("%s@%s", args[0], domain), args[1])
			e.SetBaseDN(domainToBaseDN(domain))
			if err := e.Bind(cmd.Context(), e.BindDN, e.BindPass); err != nil {
				return fmt.Errorf("bind: %w", err)
			}

			result, err := e.Search(cmd.Context(), ldapengine.SearchOptions{
				BaseDN:     "",
				Scope:      2, // subtree
				Filter:     "(servicePrincipalName=*)",
				Attributes: []string{"distinguishedName", "objectSid", "sAMAccountName", "servicePrincipalName", "displayName", "objectClass"},
				SizeLimit:  limit,
			})
			if err != nil {
				return fmt.Errorf("search SPNs: %w", err)
			}

			fmt.Printf("Found %d objects with SPNs\n", len(result.Entries))
			if cli.IsJSONOutput(cmd) {
				return cli.PrintJSON(map[string]any{
					"count": len(result.Entries),
				})
			}

			for _, entry := range result.Entries {
				spns := entry.GetAttributeValues("servicePrincipalName")
				sid := ""
				if len(entry.GetAttributeRawValues("objectSid")) > 0 {
					if sidParsed, err := ldapproto.ParseSID(entry.GetAttributeRawValues("objectSid")[0]); err == nil {
						sid = sidParsed.String()
					}
				}
				fmt.Printf("  %s (%s) SID=%s\n", entry.GetFirstAttributeValue("sAMAccountName"), entry.GetFirstAttributeValue("displayName"), sid)
				for _, spn := range spns {
					fmt.Printf("    %s\n", spn)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&domain, "domain", "", "Target domain (FQDN)")
	cmd.Flags().StringVar(&dc, "dc", "", "Domain controller hostname or IP")
	cmd.Flags().IntVar(&port, "port", 389, "LDAP server port")
	cmd.Flags().BoolVar(&useTLS, "tls", false, "Use LDAPS (TLS)")
	cmd.Flags().BoolVar(&startTLS, "starttls", false, "Use StartTLS")
	cmd.Flags().IntVar(&limit, "limit", 1000, "Maximum results")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace name (required for governance)")

	bindEngagementFlag(cmd, &engFile)

	cmd.MarkFlagRequired("domain")
	cmd.MarkFlagRequired("dc")
	cmd.MarkFlagRequired("workspace")

	return cmd
}

func newLDAPEnumAllCmd() *cobra.Command {
	var (
		domain    string
		dc        string
		limit     int
		workspace string
		useTLS    bool
		startTLS  bool
		port      int
		engFile   string
	)

	cmd := &cobra.Command{
		Use:   "all",
		Short: "Enumerate all object types",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireEngagementScope(engFile, domain, dc, ldapCapability(cmd)); err != nil {
				return err
			}

			_, err := cli.OpenGovernedWorkspace(cli.GetWorkspaceFlag(cmd))
			if err != nil {
				return err
			}

			e := createLDAPEngine(dc, port, useTLS, startTLS)
			if err := e.Connect(cmd.Context()); err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer e.Close()

			if startTLS {
				if err := e.StartTLS(cmd.Context()); err != nil {
					return fmt.Errorf("starttls: %w", err)
				}
			}

			if err := requireBindArgs(args); err != nil {
				return err
			}
			e.SetBindCredentials(fmt.Sprintf("%s@%s", args[0], domain), args[1])
			e.SetBaseDN(domainToBaseDN(domain))
			if err := e.Bind(cmd.Context(), e.BindDN, e.BindPass); err != nil {
				return fmt.Errorf("bind: %w", err)
			}

			aclEngine := acl.NewEngine(e)

			baseDN := domainToBaseDN(domain)
			users, _ := aclEngine.EnumerateUsers(cmd.Context(), acl.EnumerateObjectsOptions{
				ObjectClass: "user",
				Filter:      "(&(objectClass=user)(!(objectClass=computer)))",
				SizeLimit:   limit,
				BaseDN:      baseDN,
			})
			groups, _ := aclEngine.EnumerateGroups(cmd.Context(), acl.EnumerateObjectsOptions{
				ObjectClass: "group",
				SizeLimit:   limit,
				BaseDN:      baseDN,
			})
			computers, _ := aclEngine.EnumerateComputers(cmd.Context(), acl.EnumerateObjectsOptions{
				ObjectClass: "computer",
				SizeLimit:   limit,
				BaseDN:      baseDN,
			})
			ous, _ := aclEngine.EnumerateOUs(cmd.Context(), acl.EnumerateObjectsOptions{
				ObjectClass: "organizationalUnit",
				SizeLimit:   limit,
				BaseDN:      baseDN,
			})

			if cli.IsJSONOutput(cmd) {
				return cli.PrintJSON(map[string]any{
					"users":     len(users),
					"groups":    len(groups),
					"computers": len(computers),
					"ous":       len(ous),
				})
			}

			fmt.Printf("Users: %d\n", len(users))
			fmt.Printf("Groups: %d\n", len(groups))
			fmt.Printf("Computers: %d\n", len(computers))
			fmt.Printf("OUs: %d\n", len(ous))

			return nil
		},
	}

	cmd.Flags().StringVar(&domain, "domain", "", "Target domain (FQDN)")
	cmd.Flags().StringVar(&dc, "dc", "", "Domain controller hostname or IP")
	cmd.Flags().IntVar(&port, "port", 389, "LDAP server port")
	cmd.Flags().BoolVar(&useTLS, "tls", false, "Use LDAPS (TLS)")
	cmd.Flags().BoolVar(&startTLS, "starttls", false, "Use StartTLS")
	cmd.Flags().IntVar(&limit, "limit", 1000, "Maximum results per type")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace name (required for governance)")

	bindEngagementFlag(cmd, &engFile)

	cmd.MarkFlagRequired("domain")
	cmd.MarkFlagRequired("dc")
	cmd.MarkFlagRequired("workspace")

	return cmd
}

func newLDAPRootDSECmd() *cobra.Command {
	var (
		host      string
		port      int
		useTLS    bool
		startTLS  bool
		bindDN    string
		bindPass  string
		domain    string
		workspace string
	)

	cmd := &cobra.Command{
		Use:   "rootdse",
		Short: "Query RootDSE",
		Long:  `Query the RootDSE for domain/forest configuration information.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := cli.OpenGovernedWorkspace(cli.GetWorkspaceFlag(cmd))
			if err != nil {
				return err
			}

			e := createLDAPEngine(host, port, useTLS, startTLS)
			if err := e.Connect(cmd.Context()); err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer e.Close()

			if startTLS {
				if err := e.StartTLS(cmd.Context()); err != nil {
					return fmt.Errorf("starttls: %w", err)
				}
			}

			if err := e.Bind(cmd.Context(), bindDN, bindPass); err != nil {
				return fmt.Errorf("bind: %w", err)
			}

			// RFC 4511 §5.1: the RootDSE lives at the EMPTY distinguished name.
			// Deriving a "DC=..." base from --domain is what made Stage 46g
			// return a ypservers object instead of the RootDSE.
			result, err := e.GetRootDSE(cmd.Context())
			if err != nil {
				return fmt.Errorf("get rootdse: %w", err)
			}

			if len(result.Entries) > 0 {
				entry := result.Entries[0]
				if cli.IsJSONOutput(cmd) {
					attrs := make(map[string][]string)
for _, a := range entry.Attributes {
					attrs[string(a.Type)] = make([]string, len(a.Values))
					for i, v := range a.Values {
						attrs[string(a.Type)][i] = string(v)
					}
				}
					return cli.PrintJSON(attrs)
				}

for _, a := range entry.Attributes {
				fmt.Printf("%s:", string(a.Type))
				for i, v := range a.Values {
						if i > 0 {
							fmt.Print(", ")
						}
						fmt.Printf(" %s", string(v))
					}
					fmt.Println()
				}
			}
			return nil
		},
	}

cmd.Flags().StringVar(&host, "host", "", "LDAP server hostname or IP")
	cmd.Flags().StringVar(&bindDN, "bind-dn", "", "Bind DN")
	cmd.Flags().StringVar(&bindPass, "bind-pass", "", "Bind password")
	cmd.Flags().StringVar(&domain, "domain", "", "Target domain (FQDN) for RootDSE base DN")
	cmd.Flags().IntVar(&port, "port", 389, "Port")
	cmd.Flags().BoolVar(&useTLS, "tls", false, "Use TLS")
	cmd.Flags().BoolVar(&startTLS, "starttls", false, "Use StartTLS")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace name (required for governance)")

	cmd.MarkFlagRequired("host")
	cmd.MarkFlagRequired("bind-dn")
	cmd.MarkFlagRequired("bind-pass")
	cmd.MarkFlagRequired("workspace")

	return cmd
}

func newLDAPACLCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "acl",
		Short: "Security descriptor and ACL analysis",
		Long:  `Analyze security descriptors and ACLs on directory objects.`,
	}

	rejectUnknownSubcommand(cmd)

	cmd.AddCommand(newLDAPACLGetCmd())
	cmd.AddCommand(newLDAPACLEffectiveCmd())

	return cmd
}

func newLDAPACLGetCmd() *cobra.Command {
	var (
		domain    string
		dc        string
		objectDN  string
		workspace string
		useTLS    bool
		startTLS  bool
		port      int
		engFile   string
	)

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get and analyze ACL on an object",
		Long:  `Retrieve and analyze the security descriptor (ACL) on a directory object.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireEngagementScope(engFile, domain, dc, ldapCapability(cmd)); err != nil {
				return err
			}

			_, err := cli.OpenGovernedWorkspace(cli.GetWorkspaceFlag(cmd))
			if err != nil {
				return err
			}

			e := createLDAPEngine(dc, port, useTLS, startTLS)
			if err := e.Connect(cmd.Context()); err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer e.Close()

			if startTLS {
				if err := e.StartTLS(cmd.Context()); err != nil {
					return fmt.Errorf("starttls: %w", err)
				}
			}

			if err := requireBindArgs(args); err != nil {
				return err
			}
			e.SetBindCredentials(fmt.Sprintf("%s@%s", args[0], domain), args[1])
			e.SetBaseDN(domainToBaseDN(domain))
			if err := e.Bind(cmd.Context(), e.BindDN, e.BindPass); err != nil {
				return fmt.Errorf("bind: %w", err)
			}

			aclEngine := acl.NewEngine(e)

			objACL, err := aclEngine.GetObjectACL(cmd.Context(), objectDN)
			if err != nil {
				return fmt.Errorf("get object ACL: %w", err)
			}

			if cli.IsJSONOutput(cmd) {
				return cli.PrintJSON(map[string]any{
					"object_dn":    objACL.ObjectDN,
					"object_class": objACL.ObjectClass,
					"owner":        objACL.Owner,
					"group":        objACL.Group,
					"dacl_count":   len(objACL.DACL),
					"sacl_count":   len(objACL.SACL),
				})
			}

			fmt.Printf("Object: %s (%s)\n", objACL.ObjectDN, objACL.ObjectClass)
			fmt.Printf("Owner: %s\n", objACL.Owner)
			fmt.Printf("Group: %s\n", objACL.Group)
			fmt.Printf("DACL (%d ACEs):\n", len(objACL.DACL))
			for i, ace := range objACL.DACL {
				fmt.Printf("  [%d] %s SID=%s Rights=%s\n", i, ace.Type, ace.SID, strings.Join(ace.EffectiveRights, ", "))
			}
			fmt.Printf("SACL (%d ACEs):\n", len(objACL.SACL))
			for i, ace := range objACL.SACL {
				fmt.Printf("  [%d] %s SID=%s Rights=%s\n", i, ace.Type, ace.SID, strings.Join(ace.EffectiveRights, ", "))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&domain, "domain", "", "Target domain (FQDN)")
	cmd.Flags().StringVar(&dc, "dc", "", "Domain controller hostname or IP")
	cmd.Flags().IntVar(&port, "port", 389, "LDAP server port")
	cmd.Flags().BoolVar(&useTLS, "tls", false, "Use LDAPS (TLS)")
	cmd.Flags().BoolVar(&startTLS, "starttls", false, "Use StartTLS")
	cmd.Flags().StringVar(&objectDN, "object", "", "Object DN to analyze")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace name (required for governance)")

	bindEngagementFlag(cmd, &engFile)

	cmd.MarkFlagRequired("domain")
	cmd.MarkFlagRequired("dc")
	cmd.MarkFlagRequired("object")
	cmd.MarkFlagRequired("workspace")

	return cmd
}

func newLDAPACLEffectiveCmd() *cobra.Command {
	var (
		domain    string
		dc        string
		objectDN  string
		principal string
		workspace string
		useTLS    bool
		startTLS  bool
		port      int
		engFile   string
	)

	cmd := &cobra.Command{
		Use:   "effective",
		Short: "Get effective rights for a principal on an object",
		Long:  `Calculate effective rights for a specific principal on a directory object.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireEngagementScope(engFile, domain, dc, ldapCapability(cmd)); err != nil {
				return err
			}

			_, err := cli.OpenGovernedWorkspace(cli.GetWorkspaceFlag(cmd))
			if err != nil {
				return err
			}

			e := createLDAPEngine(dc, port, useTLS, startTLS)
			if err := e.Connect(cmd.Context()); err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer e.Close()

			if startTLS {
				if err := e.StartTLS(cmd.Context()); err != nil {
					return fmt.Errorf("starttls: %w", err)
				}
			}

			if err := requireBindArgs(args); err != nil {
				return err
			}
			e.SetBindCredentials(fmt.Sprintf("%s@%s", args[0], domain), args[1])
			e.SetBaseDN(domainToBaseDN(domain))
			if err := e.Bind(cmd.Context(), e.BindDN, e.BindPass); err != nil {
				return fmt.Errorf("bind: %w", err)
			}

			aclEngine := acl.NewEngine(e)

			// Parse principal SID
			sid, err := ldapproto.ParseStringSID(principal)
			if err != nil {
				return fmt.Errorf("parse principal SID: %w", err)
			}

			rights, err := aclEngine.GetEffectiveRights(cmd.Context(), sid, objectDN)
			if err != nil {
				return fmt.Errorf("get effective rights: %w", err)
			}

			if cli.IsJSONOutput(cmd) {
				return cli.PrintJSON(map[string]any{
					"object_dn":    objectDN,
					"principal":    principal,
					"rights":       rights,
					"rights_count": len(rights),
				})
			}

			fmt.Printf("Effective rights for %s on %s:\n", principal, objectDN)
			for _, r := range rights {
				fmt.Printf("  %s\n", r)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&domain, "domain", "", "Target domain (FQDN)")
	cmd.Flags().StringVar(&dc, "dc", "", "Domain controller hostname or IP")
	cmd.Flags().IntVar(&port, "port", 389, "LDAP server port")
	cmd.Flags().BoolVar(&useTLS, "tls", false, "Use LDAPS (TLS)")
	cmd.Flags().BoolVar(&startTLS, "starttls", false, "Use StartTLS")
	cmd.Flags().StringVar(&objectDN, "object", "", "Object DN")
	cmd.Flags().StringVar(&principal, "principal", "", "Principal SID")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace name (required for governance)")

	bindEngagementFlag(cmd, &engFile)

	cmd.MarkFlagRequired("domain")
	cmd.MarkFlagRequired("dc")
	cmd.MarkFlagRequired("object")
	cmd.MarkFlagRequired("principal")
	cmd.MarkFlagRequired("workspace")

	return cmd
}

func newLDAPPathCmd() *cobra.Command {
	var (
		domain    string
		dc        string
		startUser string
		targetObj string
		maxDepth  int
		workspace string
		useTLS    bool
		startTLS  bool
		port      int
		engFile   string
	)

	cmd := &cobra.Command{
		Use:   "path",
		Short: "Find ACL-based attack paths",
		Long:  `Find ACL-based attack paths from a start principal to a target object.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireEngagementScope(engFile, domain, dc, ldapCapability(cmd)); err != nil {
				return err
			}

			_, err := cli.OpenGovernedWorkspace(cli.GetWorkspaceFlag(cmd))
			if err != nil {
				return err
			}

			e := createLDAPEngine(dc, port, useTLS, startTLS)
			if err := e.Connect(cmd.Context()); err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer e.Close()

			if startTLS {
				if err := e.StartTLS(cmd.Context()); err != nil {
					return fmt.Errorf("starttls: %w", err)
				}
			}

			if err := requireBindArgs(args); err != nil {
				return err
			}
			e.SetBindCredentials(fmt.Sprintf("%s@%s", args[0], domain), args[1])
			e.SetBaseDN(domainToBaseDN(domain))
			if err := e.Bind(cmd.Context(), e.BindDN, e.BindPass); err != nil {
				return fmt.Errorf("bind: %w", err)
			}

			aclEngine := acl.NewEngine(e)

			// Parse start user SID
			startSID, err := ldapproto.ParseStringSID(startUser)
			if err != nil {
				return fmt.Errorf("parse start user SID: %w", err)
			}

			startPrincipal := &acl.Principal{
				SID: startSID,
			}

			paths, err := aclEngine.FindACLPaths(cmd.Context(), startPrincipal, targetObj, maxDepth)
			if err != nil {
				return fmt.Errorf("find ACL paths: %w", err)
			}

			if cli.IsJSONOutput(cmd) {
				return cli.PrintJSON(map[string]any{
					"paths_count": len(paths),
					"paths":       paths,
				})
			}

			fmt.Printf("Found %d attack paths\n", len(paths))
			for i, path := range paths {
				fmt.Printf("\nPath %d (Risk Score: %d):\n", i+1, path.RiskScore)
				for j, step := range path.Path {
					fmt.Printf("  Step %d: %s -> %s (%s) Rights: %s\n",
						j+1, step.FromPrincipal, step.ToObject, step.Relation, strings.Join(step.Rights, ", "))
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&domain, "domain", "", "Target domain (FQDN)")
	cmd.Flags().StringVar(&dc, "dc", "", "Domain controller hostname or IP")
	cmd.Flags().IntVar(&port, "port", 389, "LDAP server port")
	cmd.Flags().BoolVar(&useTLS, "tls", false, "Use LDAPS (TLS)")
	cmd.Flags().BoolVar(&startTLS, "starttls", false, "Use StartTLS")
	cmd.Flags().StringVar(&startUser, "start-user", "", "Start user SID")
	cmd.Flags().StringVar(&targetObj, "target", "", "Target object DN")
	cmd.Flags().IntVar(&maxDepth, "max-depth", 5, "Maximum path depth")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace name (required for governance)")

	bindEngagementFlag(cmd, &engFile)

	cmd.MarkFlagRequired("domain")
	cmd.MarkFlagRequired("dc")
	cmd.MarkFlagRequired("start-user")
	cmd.MarkFlagRequired("target")
	cmd.MarkFlagRequired("workspace")

	return cmd
}
