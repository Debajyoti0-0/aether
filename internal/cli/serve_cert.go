package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/api"
)

// Teamserver PKI commands (Stage 3, T1):
//
//	aether serve cert init                     — CA + server cert
//	aether serve cert issue --operator alice   — client cert + capabilities
//	aether serve cert revoke --operator alice  — revoke (revoked.txt)
//
// All key material is written 0600 under the teamserver root.

var (
	tsCertDir    string
	tsCertOperator string
	tsCertCaps   string
	tsCertDays   int
	tsCertHosts  string
)

func init() {
	serveCmd.AddCommand(serveCertCmd)
	serveCertCmd.AddCommand(serveCertInitCmd, serveCertIssueCmd, serveCertRevokeCmd)

	for _, c := range []*cobra.Command{serveCertInitCmd, serveCertIssueCmd, serveCertRevokeCmd} {
		c.Flags().StringVar(&tsCertDir, "dir", defaultTeamserverDir(), "Teamserver PKI directory")
	}
	serveCertIssueCmd.Flags().StringVar(&tsCertOperator, "operator", "", "Operator name (required)")
	serveCertIssueCmd.Flags().StringVar(&tsCertCaps, "caps", "", "Comma-separated execute capabilities to grant (e.g. exec.azure,exec.aws); read caps are granted by default")
	serveCertIssueCmd.Flags().IntVar(&tsCertDays, "days", 365, "Certificate validity in days")
	_ = serveCertIssueCmd.MarkFlagRequired("operator")
	// Charter fix (charter fix H1): revoke reads tsCertOperator but never
	// registered the flag — revocation via CLI was unusable.
	serveCertRevokeCmd.Flags().StringVar(&tsCertOperator, "operator", "", "Operator name to revoke (required)")
	_ = serveCertRevokeCmd.MarkFlagRequired("operator")
	serveCertInitCmd.Flags().StringVar(&tsCertHosts, "extra-hosts", "", "Extra SAN hosts for the server cert (comma-separated)")
}

func defaultTeamserverDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "teamserver"
	}
	return filepath.Join(dir, "aether", "teamserver")
}

var serveCertCmd = &cobra.Command{
	Use:   "cert",
	Short: "Teamserver PKI management (CA, operator certificates, revocation)",
}

var serveCertInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate the teamserver CA and server certificate",
	RunE: func(cmd *cobra.Command, args []string) error {
		var extra []string
		if tsCertHosts != "" {
			for _, h := range splitComma(tsCertHosts) {
				if h != "" {
					extra = append(extra, h)
				}
			}
		}
		paths, err := api.InitCA(tsCertDir, extra)
		if err != nil {
			return err
		}
		fmt.Printf("Teamserver PKI initialized in %s\n", tsCertDir)
		fmt.Printf("  CA:     %s\n  Server: %s\n", paths.CACert, paths.SrvCert)
		fmt.Println("Issue operator certificates with: aether serve cert issue --operator <name>")
		return nil
	},
}

var serveCertIssueCmd = &cobra.Command{
	Use:   "issue",
	Short: "Issue an operator client certificate (+ capability file)",
	RunE: func(cmd *cobra.Command, args []string) error {
		certPEM, keyPEM, err := api.IssueOperatorCert(
			filepath.Join(tsCertDir, "teamserver-ca.crt"),
			filepath.Join(tsCertDir, "teamserver-ca.key"),
			tsCertOperator, tsCertDays)
		if err != nil {
			return err
		}
		opDir := filepath.Join(tsCertDir, "operators", tsCertOperator)
		if err := os.MkdirAll(opDir, 0o700); err != nil {
			return err
		}
		certPath := filepath.Join(opDir, tsCertOperator+".crt")
		keyPath := filepath.Join(opDir, tsCertOperator+".key")
		if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
			return err
		}
		if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
			return err
		}

		// Capability file: defaults (read-only) + granted execute caps.
		caps := []string{api.CapReadAudit, api.CapReadEvents, api.CapReadGraph, api.CapReadWorkspace}
		caps = append(caps, splitComma(tsCertCaps)...)
		if err := api.WriteOperatorCaps(filepath.Join(tsCertDir, "operators"), tsCertOperator, caps); err != nil {
			return err
		}

		fmt.Printf("Issued operator certificate for %q (valid %d days)\n", tsCertOperator, tsCertDays)
		fmt.Printf("  cert: %s\n  key:  %s\n", certPath, keyPath)
		fmt.Printf("  capabilities: %v\n", caps)
		return nil
	},
}

var serveCertRevokeCmd = &cobra.Command{
	Use:   "revoke",
	Short: "Revoke an operator (connections fail closed)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateOperatorNameRevocation(tsCertOperator); err != nil {
			return err
		}
		path := filepath.Join(tsCertDir, "revoked.txt")
		// Stage 42 finding (S42-1): revoke appended to revoked.txt without
		// ensuring the PKI directory exists — a fresh --dir made revocation
		// fail with "cannot find the path" after flag parsing succeeded.
		if err := os.MkdirAll(tsCertDir, 0o700); err != nil {
			return err
		}
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := fmt.Fprintf(f, "%s\n", tsCertOperator); err != nil {
			return err
		}
		fmt.Printf("Operator %q revoked (appended to %s)\n", tsCertOperator, path)
		return nil
	},
}

func validateOperatorNameRevocation(name string) error {
	if name == "" || filepath.Base(name) != name {
		return fmt.Errorf("invalid operator name %q", name)
	}
	return nil
}
