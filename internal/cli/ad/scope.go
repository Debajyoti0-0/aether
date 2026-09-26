package ad

import (
	"github.com/Debajyoti0-0/aether/internal/engagement"
	"github.com/spf13/cobra"
)

// bindEngagementFlag adds the mandatory --engagement flag to an offensive
// command. The flag is required: scope enforcement is fail-closed, so there
// is deliberately no "unscoped" mode.
func bindEngagementFlag(cmd *cobra.Command, target *string) {
	cmd.Flags().StringVar(target, "engagement", "",
		"Rules-of-engagement JSON file authorizing this domain, domain controller and capability (required)")
	cmd.MarkFlagRequired("engagement")
}

// requireEngagementScope refuses the operation unless the engagement
// authorizes the requested domain, domain controller and capability at this
// instant.
//
// It is called at the very top of each offensive command's RunE, before the
// governed workspace is opened and before any network traffic, so a
// refused operation leaves no audit entry, no workspace mutation and no
// packet on the wire.
func requireEngagementScope(path, domain, dc, capability string) error {
	e, err := engagement.Load(path)
	if err != nil {
		return err
	}
	return e.IsAuthorized(domain, dc, capability)
}

// ldapCapability maps an LDAP subcommand to the engagement capability it
// requires. ACL and rights analysis are separated from plain enumeration.
//
// The lookup keys off the leaf command name because the LDAP tree is mounted
// at two command paths (aether ldap ... and aether ad ldap ...), so the full
// command path is not stable. Within the LDAP subtree the leaf names are
// unambiguous: users/groups/computers/ous/spns/all enumerate, while
// get/effective/path analyse security descriptors.
//
// An unrecognised leaf resolves to the stronger ACL capability rather than
// the weaker read capability, so a subcommand added later cannot silently
// inherit a broader engagement than it needs.
// rejectUnknownSubcommand makes a grouping command refuse an unrecognised leaf
// instead of quietly printing its own help and exiting 0.
//
// Setting Args alone cannot do this. cobra checks Runnable() and returns
// flag.ErrHelp before it validates arguments at all (cobra v1.8.0 command.go,
// the Runnable check precedes ValidateArgs), so a command with no RunE never
// reaches its validator. Making the group runnable is what lets the validation
// happen: an unexpected argument is then rejected, while no argument still
// falls through to the help the group was going to print anyway.
//
// Without this, `aether ad ldap acl path` exits 0 having done nothing, so an
// operator who asked for an analysis that never ran is told it succeeded.
func rejectUnknownSubcommand(cmd *cobra.Command) {
	cmd.Args = cobra.NoArgs
	cmd.RunE = func(c *cobra.Command, args []string) error { return c.Help() }
}

func ldapCapability(cmd *cobra.Command) string {
	switch cmd.Name() {
	case "users", "groups", "computers", "ous", "spns", "all":
		return engagement.CapLDAPRead
	default:
		return engagement.CapLDAPACL
	}
}
