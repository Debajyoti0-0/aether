package validate

import (
	"fmt"
	"strings"
)

// Profile is a pre-configured engagement preset: one selection sets
// OPSEC thresholds, pacing, logging, and risk posture.
type Profile struct {
	Name             string `json:"name"`
	Description      string `json:"description"`
	MaxRiskThreshold int    `json:"max_risk_threshold"`
	LowSlow          bool   `json:"low_slow"`
	BrowserPreset    string `json:"browser_preset"`
	AuditLogging     bool   `json:"audit_logging"`
	AutoRollback     bool   `json:"auto_rollback"`
	StealthMode      bool   `json:"stealth_mode"`
}

// BuiltInProfiles are the shipped engagement presets.
var BuiltInProfiles = map[string]Profile{
	"red-team": {
		Name:             "red-team",
		Description:      "Standard offensive engagement: balanced OPSEC, rollback on, audit on",
		MaxRiskThreshold: 60,
		LowSlow:          true,
		BrowserPreset:    "chrome",
		AuditLogging:     true,
		AutoRollback:     true,
		StealthMode:      true,
	},
	"banking": {
		Name:             "banking",
		Description:      "Regulated client: strict risk ceiling, full audit, cautious pacing",
		MaxRiskThreshold: 30,
		LowSlow:          true,
		BrowserPreset:    "edge",
		AuditLogging:     true,
		AutoRollback:     true,
		StealthMode:      true,
	},
	"incident-response": {
		Name:             "incident-response",
		Description:      "Full visibility mode: audit everything, no stealth, wide risk ceiling",
		MaxRiskThreshold: 90,
		LowSlow:          false,
		BrowserPreset:    "chrome",
		AuditLogging:     true,
		AutoRollback:     false,
		StealthMode:      false,
	},
	"purple-team": {
		Name:             "purple-team",
		Description:      "Detection validation: simulate/stream to SIEM, verbose detection output",
		MaxRiskThreshold: 50,
		LowSlow:          false,
		BrowserPreset:    "firefox",
		AuditLogging:     true,
		AutoRollback:     true,
		StealthMode:      false,
	},
	"lab": {
		Name:             "lab",
		Description:      "Internal lab: permissive, fast, no audit overhead",
		MaxRiskThreshold: 100,
		LowSlow:          false,
		BrowserPreset:    "chrome",
		AuditLogging:     false,
		AutoRollback:     false,
		StealthMode:      false,
	},
}

// GetProfile resolves a profile by name (case-insensitive).
func GetProfile(name string) (*Profile, error) {
	for k, p := range BuiltInProfiles {
		if equalFold(k, name) {
			cp := p
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("unknown profile %q (want one of: %s)", name, profileNames())
}

func profileNames() string {
	names := make([]string, 0, len(BuiltInProfiles))
	for k := range BuiltInProfiles {
		names = append(names, k)
	}
	// deterministic
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	return strings.Join(names, ", ")
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// RenderProfiles prints all built-in profiles.
func RenderProfiles() string {
	var b strings.Builder
	b.WriteString("NAME              RISK  LOW-SLOW  AUDIT  ROLLBACK  STEALTH\n")
	for _, name := range []string{"banking", "incident-response", "lab", "purple-team", "red-team"} {
		p := BuiltInProfiles[name]
		fmt.Fprintf(&b, "%-17s %-5d %-9t %-6t %-9t %t\n",
			p.Name, p.MaxRiskThreshold, p.LowSlow, p.AuditLogging, p.AutoRollback, p.StealthMode)
	}
	return b.String()
}
