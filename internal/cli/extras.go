package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/engine/validate"
	"github.com/Debajyoti0-0/aether/internal/transport"
	"github.com/Debajyoti0-0/aether/internal/workspace"
)

// workspaceRekeyCmd — rotate the workspace passphrase (and migrate
// legacy/keyless workspaces onto the keyed layout).
var workspaceRekeyCmd = &cobra.Command{
	Use:   "rekey",
	Short: "Rotate the workspace encryption passphrase",
	Long:  "Re-encrypt every stored record (tokens, identities, evidence, journal) under a new passphrase. The old passphrase must decrypt the existing records. Migrating a KEYLESS (or pre-Stage-1 legacy) workspace requires --confirm-keyless when the old passphrase is empty.",
	RunE: func(cmd *cobra.Command, args []string) error {
		name := rekeyWorkspace
		if name == "" && len(args) > 0 {
			name = args[0]
		}
		oldPass := rekeyOld
		if oldPass == "" {
			oldPass = os.Getenv("AETHER_PASSPHRASE")
		}
		newPass := rekeyNew
		if newPass == "" {
			newPass = os.Getenv("AETHER_NEW_PASSPHRASE")
		}
		if oldPass == "" && !rekeyConfirmKeyless {
			return fmt.Errorf("migrating a keyless or legacy workspace (empty old passphrase) requires the explicit --confirm-keyless flag")
		}
		if newPass == "" {
			return fmt.Errorf("a new passphrase is required: pass --new-passphrase or set AETHER_NEW_PASSPHRASE")
		}

		w, err := workspace.OpenForMigration(name, oldPass)
		if err != nil {
			return err
		}

		n, err := w.Rekey(oldPass, newPass)
		if err != nil {
			return err
		}
		if err := w.LogEvent("rekeyed", fmt.Sprintf("%d records rotated", n)); err != nil {
			return err
		}
		fmt.Printf("Rekeyed workspace %q: %d records + journal rotated. Store the new passphrase safely.\n", name, n)
		return nil
	},
}

var (
	rekeyWorkspace      string
	rekeyOld            string
	rekeyNew            string
	rekeyConfirmKeyless bool
)

func init() {
	workspaceCmd.AddCommand(workspaceRekeyCmd)
	workspaceRekeyCmd.Flags().StringVar(&rekeyWorkspace, "workspace", "", "Workspace name (or pass as arg)")
	workspaceRekeyCmd.Flags().StringVar(&rekeyOld, "old-passphrase", "", "Current passphrase (or AETHER_PASSPHRASE env); empty only for keyless/legacy migration")
	workspaceRekeyCmd.Flags().StringVar(&rekeyNew, "new-passphrase", "", "New passphrase (or AETHER_NEW_PASSPHRASE env)")
	workspaceRekeyCmd.Flags().BoolVar(&rekeyConfirmKeyless, "confirm-keyless", false, "Required when migrating a keyless/legacy workspace (empty old passphrase)")
}

// validateStealthCmd — stealth posture validation.
var validateStealthCmd = &cobra.Command{
	Use:   "stealth",
	Short: "Validate the stealth posture of a planned operation",
	Long: `Score an operation's stealth posture: jitter pacing, TLS fingerprint
matching, SOC signal exposure, and timing. Produces an actionable
checklist before execution.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var findings []string
		score := 100

		// Jitter pacing.
		if valLowSlow {
			findings = append(findings, "OK   --low-slow enabled: 1-5s human-paced jitter between operations")
		} else {
			score -= 25
			findings = append(findings, "GAP  --low-slow disabled: operations run at machine speed (burst detection risk)")
		}

		// TLS fingerprint.
		if valPreset == "" || transportBrowserPreset(valPreset) {
			findings = append(findings, fmt.Sprintf("OK   TLS fingerprint preset %q (matches a real browser stack)", valPreset))
		} else {
			score -= 15
			findings = append(findings, fmt.Sprintf("GAP  unknown TLS preset %q — fallback may look non-standard", valPreset))
		}

		// SOC exposure.
		p := validate.NewPredictor()
		env := &validate.Environment{
			UnknownLocation: valStealthUnknownLoc,
			NewDevice:       valStealthNewDevice,
			TimeOfDay:       valTimeOfDay,
		}
		highSignals := 0
		for _, s := range p.Simulate(valStealthAction, env) {
			if strings.EqualFold(s.Probability, "high") {
				highSignals++
			}
		}
		if highSignals == 0 {
			findings = append(findings, "OK   no high-probability SOC signals predicted for this action")
		} else {
			score -= 20 * highSignals
			findings = append(findings, fmt.Sprintf("GAP  %d high-probability SOC signal(s) predicted — run 'aether validate soc' for evasion tips", highSignals))
		}

		// Timing.
		if valTimeOfDay == "off_hours" {
			findings = append(findings, "OK   off-hours execution window")
		} else {
			score -= 10
			findings = append(findings, "GAP  business-hours execution: admin/alert activity is elevated")
		}

		if score < 0 {
			score = 0
		}
		verdict := "GOOD"
		if score < 50 {
			verdict = "POOR"
		} else if score < 80 {
			verdict = "FAIR"
		}

		fmt.Printf("=== Stealth Posture: %s (%d/100) ===\n\n", verdict, score)
		for _, f := range findings {
			fmt.Printf("  %s\n", f)
		}
		return nil
	},
}

var (
	valStealthAction      string
	valStealthUnknownLoc  bool
	valStealthNewDevice   bool
	valLowSlow            bool
	valPreset             string
)

func transportBrowserPreset(p string) bool {
	switch p {
	case "chrome", "edge", "firefox":
		return true
	}
	return false
}

// _ keeps the transport Jitter types linked for the checklist text.
var _ transport.Jitter = transport.NoJitter{}

func init() {
	validateCmd.AddCommand(validateStealthCmd)
	validateStealthCmd.Flags().StringVar(&valStealthAction, "action", "", "Action e.g. prt_exchange (required)")
	validateStealthCmd.Flags().BoolVar(&valLowSlow, "low-slow", false, "Jitter pacing will be enabled")
	validateStealthCmd.Flags().StringVar(&valPreset, "browser-preset", "chrome", "TLS preset that will be used")
	validateStealthCmd.Flags().BoolVar(&valStealthUnknownLoc, "unknown-location", false, "Operating from an unfamiliar location")
	validateStealthCmd.Flags().BoolVar(&valStealthNewDevice, "new-device", false, "Operating from a new device")
	validateStealthCmd.Flags().StringVar(&valTimeOfDay, "time-of-day", "business_hours", "business_hours or off_hours")
	_ = validateStealthCmd.MarkFlagRequired("action")
}
