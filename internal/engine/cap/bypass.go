package cap

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Debajyoti0-0/aether/internal/types"
)

// RenderStrategy formats a bypass strategy for terminal output.
func RenderStrategy(s *types.BypassStrategy) string {
	var b strings.Builder

	b.WriteString("=== Conditional Access Bypass Strategy ===\n")
	fmt.Fprintf(&b, "Required OS:       %s\n", s.RequiredOS)
	fmt.Fprintf(&b, "Required Browser:  %s\n", s.RequiredBrowser)
	fmt.Fprintf(&b, "Required Location: %s\n", s.RequiredLocation)
	fmt.Fprintf(&b, "MFA Required:      %t\n", s.RequiresMFA)
	fmt.Fprintf(&b, "Device Compliance: %t\n", s.RequiredDeviceCompliance)
	fmt.Fprintf(&b, "OPSEC Risk Score:  %d/100\n\n", s.RiskLevel)
	b.WriteString("Recommendations:\n")
	for _, r := range s.SpoofingRecommendations {
		fmt.Fprintf(&b, "  - %s\n", r)
	}
	return b.String()
}

// StrategyJSON renders a strategy as indented JSON.
func StrategyJSON(s *types.BypassStrategy) (string, error) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// RenderPolicies renders parsed policies as a human-readable table.
func RenderPolicies(policies []types.ConditionalAccessPolicy) string {
	var b strings.Builder
	b.WriteString("ID                                   STATE   NAME\n")
	for _, p := range policies {
		fmt.Fprintf(&b, "%-36s %-7s %s\n", truncateStr(p.ID, 36), p.State, p.DisplayName)
	}
	return b.String()
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
