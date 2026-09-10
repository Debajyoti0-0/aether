package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/engine/relay"
	"github.com/Debajyoti0-0/aether/internal/protocol/oauth2"
	"github.com/Debajyoti0-0/aether/internal/protocol/wstrust"
)

// relaySTSClient builds the WS-Trust client with the selected preset.
func relaySTSClient() (*wstrust.Client, error) {
	return wstrust.NewClient(relaySTSEndpt, relayPreset, time.Duration(relayTimeout)*time.Second)
}

var relayCmd = &cobra.Command{
	Use:   "relay",
	Short: "Authentication relay operations",
	Long:  "Relay authentication flows (WS-Trust/SAML, device code) for authorized MFA testing.",
}

var relayMFACmd = &cobra.Command{
	Use:   "mfa",
	Short: "WS-Trust/SAML relay to OAuth tokens",
	RunE:  runRelayMFA,
}

var relayDeviceCodeCmd = &cobra.Command{
	Use:   "devicecode",
	Short: "Run a device code flow and poll for tokens",
	RunE:  runRelayDeviceCode,
}

var relayStretchCmd = &cobra.Command{
	Use:   "stretch",
	Short: "Keep a session alive via continuous refresh",
	RunE:  runRelayStretch,
}

var (
	relayUsername  string
	relayPassword  string
	relaySTSEndpt  string
	relayTenant    string
	relayClientID  string
	relayAppliesTo string
	relayOutput    string
	relayPreset    string
	relayTimeout   int
	relayMaxRounds int
	relayRefresh   string
)

func init() {
	rootCmd.AddCommand(relayCmd)
	relayCmd.AddCommand(relayMFACmd, relayDeviceCodeCmd, relayStretchCmd)

	relayMFACmd.Flags().StringVar(&relayUsername, "username", "", "Username (user@domain)")
	relayMFACmd.Flags().StringVar(&relayPassword, "password", "", "Password (or AETHER_PASSWORD env)")
	relayMFACmd.Flags().StringVar(&relaySTSEndpt, "sts-endpoint", "", "WS-Trust STS endpoint (required)")
	relayMFACmd.Flags().StringVar(&relayTenant, "tenant", "", "Tenant id or domain (required)")
	relayMFACmd.Flags().StringVar(&relayClientID, "client-id", "1950a258-227b-4e31-a9cf-717495945fc2", "Public client id")
	relayMFACmd.Flags().StringVar(&relayAppliesTo, "applies-to", relay.AppliesToMicrosoftOnline, "AppliesTo realm")
	relayMFACmd.Flags().StringVar(&relayOutput, "output", "", "Output file for tokens (JSON)")
	relayMFACmd.Flags().StringVar(&relayPreset, "browser-preset", "chrome", "TLS fingerprint preset")
	relayMFACmd.Flags().IntVar(&relayTimeout, "timeout", 30, "Request timeout seconds")
	_ = relayMFACmd.MarkFlagRequired("sts-endpoint")
	_ = relayMFACmd.MarkFlagRequired("tenant")

	relayDeviceCodeCmd.Flags().StringVar(&relayTenant, "tenant", "", "Tenant id or domain (required)")
	relayDeviceCodeCmd.Flags().StringVar(&relayClientID, "client-id", "1950a258-227b-4e31-a9cf-717495945fc2", "Public client id")
	relayDeviceCodeCmd.Flags().IntVar(&relayTimeout, "timeout", 300, "Polling timeout seconds")
	_ = relayDeviceCodeCmd.MarkFlagRequired("tenant")

	relayStretchCmd.Flags().StringVar(&relayTenant, "tenant", "", "Tenant id or domain (required)")
	relayStretchCmd.Flags().StringVar(&relayClientID, "client-id", "1950a258-227b-4e31-a9cf-717495945fc2", "Public client id")
	relayStretchCmd.Flags().StringVar(&relayRefresh, "refresh-token", "", "Refresh token to stretch (required)")
	relayStretchCmd.Flags().IntVar(&relayMaxRounds, "max-refresh", 10, "Maximum refresh rounds")
	relayStretchCmd.Flags().StringVar(&relayOutput, "output", "", "Output file for tokens (JSON)")
	_ = relayStretchCmd.MarkFlagRequired("tenant")
	_ = relayStretchCmd.MarkFlagRequired("refresh-token")
}

func runRelayMFA(cmd *cobra.Command, args []string) error {
	pw := relayPassword
	if pw == "" {
		pw = os.Getenv("AETHER_PASSWORD")
	}
	if pw == "" {
		return fmt.Errorf("password required via --password or AETHER_PASSWORD")
	}

	sts, err := relaySTSClient()
	if err != nil {
		return err
	}

	oauth, err := oauth2.NewClient(relayPreset, time.Duration(relayTimeout)*time.Second)
	if err != nil {
		return err
	}

	r := &relay.WSTrustRelay{STS: sts, OAuth: oauth, Tenant: relayTenant, ClientID: relayClientID}
	result, err := r.Relay(context.Background(), relayUsername, pw, relayAppliesTo)
	if err != nil {
		if result != nil && result.Tokens == nil && result.AssertionRaw != "" {
			fmt.Fprintln(os.Stderr, "ws-trust succeeded but token exchange failed:", err)
			fmt.Fprintln(os.Stderr, "assertion saved for manual exchange")
			if relayOutput != "" {
				return os.WriteFile(relayOutput, []byte(result.AssertionRaw), 0o600)
			}
		}
		return err
	}

	if err := saveTokens(result.Tokens, relayOutput); err != nil {
		return err
	}
	return printTokens(result.Tokens)
}

func runRelayDeviceCode(cmd *cobra.Command, args []string) error {
	oauth, err := oauth2.NewClient(relayPreset, 30*time.Second)
	if err != nil {
		return err
	}

	d := &relay.DeviceCodeRelay{OAuth: oauth, Tenant: relayTenant}
	dc, err := d.Start(context.Background(), relayClientID, "")
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Verification URL: %s\nCode: %s\nWaiting for authorization...\n",
		dc.VerificationURI, dc.UserCode)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(relayTimeout)*time.Second)
	defer cancel()

	tokens, err := d.Poll(ctx, relayClientID, dc)
	if err != nil {
		return err
	}
	return printTokens(tokens)
}

func runRelayStretch(cmd *cobra.Command, args []string) error {
	oauth, err := oauth2.NewClient(relayPreset, 30*time.Second)
	if err != nil {
		return err
	}

	s := relay.NewSessionStretcher(oauth, relayTenant, relayClientID)
	s.MaxRefresh = relayMaxRounds

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if _, err := s.Stretch(ctx, relayRefresh); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Session stretched; refresh round %d\n", s.RefreshCount())

	tokens, err := s.Maintain(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "maintenance stopped:", err)
	}
	if err := saveTokens(tokens, relayOutput); err != nil {
		return err
	}
	return printTokens(tokens)
}
