package validate

import (
	"fmt"
	"strings"
)

// SOCSignal is a predicted detection: which log/alert an action will
// generate and how to reduce its footprint.
type SOCSignal struct {
	EntraIDLogID    string   `json:"entra_id_log_id"`   // e.g. "50126"
	EntraIDLogName  string   `json:"entra_id_log_name"` // e.g. "Invalid username or password"
	SentinelRule    string   `json:"sentinel_rule,omitempty"`
	DefenderAlert   string   `json:"defender_alert,omitempty"`
	Probability     string   `json:"probability"` // low, medium, high
	EvasionTips     []string `json:"evasion_tips,omitempty"`
}

// Environment captures the context of a simulated action.
type Environment struct {
	HasMFA          bool   `json:"has_mfa"`
	UnknownLocation bool   `json:"unknown_location"`
	NewDevice       bool   `json:"new_device"`
	TimeOfDay       string `json:"time_of_day"` // business_hours, off_hours
	CloudTenant     string `json:"cloud_tenant,omitempty"`
}

// SignalDB is the embedded threat-model database: action → signals.
// Pre-seeded from public Entra ID sign-in error code documentation.
var SignalDB = map[string][]SOCSignal{
	"token_refresh": {
		{EntraIDLogID: "50173", EntraIDLogName: "Fresh token requested", Probability: "low",
			EvasionTips: []string{"Refresh from the same IP/UA as the original grant when possible"}},
	},
	"prt_exchange": {
		{EntraIDLogID: "50177", EntraIDLogName: "User was not able to onboard to persistent browser session", Probability: "medium",
			SentinelRule: "Anomalous sign-in from unfamiliar device",
			EvasionTips: []string{"Present the original device_id claims", "Match JA3/JA4 of the Windows broker"}},
		{EntraIDLogID: "53003", EntraIDLogName: "Blocked by Conditional Access", Probability: "medium",
			EvasionTips: []string{"Run `aether cap evaluate` before attempting"}},
	},
	"wstrust_relay": {
		{EntraIDLogID: "50126", EntraIDLogName: "Invalid username or password", Probability: "low",
			EvasionTips: []string{"Rotate to valid credentials before relay"}},
		{EntraIDLogID: "50074", EntraIDLogName: "User did not pass the MFA challenge", Probability: "high",
			SentinelRule: "MFA challenge skipped via legacy authentication",
			EvasionTips: []string{"Target usernamemixed endpoints only", "Avoid password spray pacing patterns"}},
	},
	"fido2_downgrade": {
		{EntraIDLogID: "50097", EntraIDLogName: "Device authentication required", Probability: "medium",
			EvasionTips: []string{"Only attempt on federated trusts with legacy WS-Trust enabled"}},
	},
	"saml_forge": {
		{EntraIDLogID: "50158", EntraIDLogName: "External security token not validated", Probability: "high",
			SentinelRule: "Suspicious SAML token usage",
			EvasionTips: []string{"Keep assertion lifetimes short", "Match the signing cert of the trusted IdP"}},
	},
	"vm_runcommand": {
		{EntraIDLogID: "", DefenderAlert: "RunCommand executed on virtual machine", Probability: "high",
			SentinelRule: "Azure RunCommand anomaly",
			EvasionTips: []string{"Prefer existing scheduled tasks/service persistence over repeated RunCommand", "Avoid base64-encoded payloads"}},
	},
	"ssm_runcommand": {
		{EntraIDLogID: "", DefenderAlert: "", Probability: "high",
			SentinelRule: "AWS SSM document execution outside baseline",
			EvasionTips: []string{"Use AWS-RunShellScript sparingly; check CloudTrail baselining"}},
	},
	"ci_dispatch": {
		{EntraIDLogID: "", SentinelRule: "Suspicious GitHub workflow dispatch", Probability: "medium",
			EvasionTips: []string{"Dispatch on low-traffic branches only"}},
	},
	"imds_hijack": {
		{EntraIDLogID: "", DefenderAlert: "Managed identity token requested from unexpected process", Probability: "medium",
			EvasionTips: []string{"Query only needed resources", "Cache tokens instead of re-querying IMDS"}},
	},
	"sp_hijack": {
		{EntraIDLogID: "50175", EntraIDLogName: "New client credential created", Probability: "high",
			SentinelRule: "New secret added to service principal",
			EvasionTips: []string{"Use short-lived credentials and rotate them out quickly"}},
	},
	"policy_export": {
		{EntraIDLogID: "", Probability: "low",
			EvasionTips: []string{"Read-only Graph calls; low signal"}},
	},
	"path_validation": {
		{EntraIDLogID: "", Probability: "low",
			EvasionTips: []string{"Structural validation makes no tenant calls"}},
	},
	"cloudkerberos": {
		{EntraIDLogID: "50142", EntraIDLogName: "Cloud Kerberos ticket provisioning", Probability: "high",
			SentinelRule: "Unusual Kerberos ticket grant via cloud KDC",
			EvasionTips: []string{"Limit to one TGT request per session", "Use the compromised user's own device context"}},
	},
}

// Predictor simulates SOC detection for actions before execution.
type Predictor struct {
	DB map[string][]SOCSignal
}

// NewPredictor builds a predictor over the embedded signal DB.
func NewPredictor() *Predictor {
	return &Predictor{DB: SignalDB}
}

// Simulate returns the predicted signals for an action in an environment.
func (p *Predictor) Simulate(action string, env *Environment) []SOCSignal {
	signals, ok := p.DB[strings.ToLower(action)]
	if !ok {
		return nil
	}

	out := make([]SOCSignal, len(signals))
	copy(out, signals)

	// Adjust probability based on environment.
	for i := range out {
		out[i].Probability = adjustProbability(out[i].Probability, env)
	}
	return out
}

func adjustProbability(base string, env *Environment) string {
	if env == nil {
		return base
	}
	known := map[string]int{"low": 0, "medium": 1, "high": 2}
	score := known[base]
	if env.UnknownLocation {
		score++
	}
	if env.NewDevice {
		score++
	}
	if env.TimeOfDay == "off_hours" {
		score++
	}
	switch {
	case score >= 4:
		return "high"
	case score >= 2:
		return "medium"
	default:
		return "low"
	}
}

// RenderSignals formats predicted signals for terminal output.
func RenderSignals(action string, signals []SOCSignal) string {
	if len(signals) == 0 {
		return fmt.Sprintf("No known SOC signals for action %q.\n", action)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "=== SOC Signal Prediction: %s ===\n", action)
	for _, s := range signals {
		if s.EntraIDLogID != "" {
			fmt.Fprintf(&b, "  Entra ID log %s (%s) — probability %s\n", s.EntraIDLogID, s.EntraIDLogName, s.Probability)
		}
		if s.SentinelRule != "" {
			fmt.Fprintf(&b, "  Sentinel rule: %s\n", s.SentinelRule)
		}
		if s.DefenderAlert != "" {
			fmt.Fprintf(&b, "  Defender alert: %s\n", s.DefenderAlert)
		}
		for _, tip := range s.EvasionTips {
			fmt.Fprintf(&b, "    tip: %s\n", tip)
		}
	}
	return b.String()
}
