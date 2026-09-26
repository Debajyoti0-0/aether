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
