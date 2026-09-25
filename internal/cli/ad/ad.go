package ad

import (
	"github.com/Debajyoti0-0/aether/internal/cli"
	"github.com/spf13/cobra"
)

func init() {
	cli.RegisterModule(&module{})
}

type module struct{}

func (m *module) Name() string { return "ad" }

func (m *module) Commands() []*cobra.Command {
	return []*cobra.Command{
		newADCmd(),
	}
}

func newADCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ad",
		Short: "Active Directory operations (Kerberos, LDAP, AD CS)",
		Long:  `Active Directory operations including Kerberos enumeration, roasting, TGT acquisition, and ccache management.`,
	}

	cmd.AddCommand(newEnumCmd())
	cmd.AddCommand(newKerberoastCmd())
	cmd.AddCommand(newASREPRoastCmd())
	cmd.AddCommand(newTGTCmd())
	cmd.AddCommand(newCCacheCmd())
	cmd.AddCommand(newLDAPCmd())

	return cmd
}