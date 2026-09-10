package validate

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ATT&CK Enterprise technique IDs relevant to identity attacks.
const (
	T1078ValidAccounts      = "T1078" // Valid Accounts
	T1078CloudAccounts      = "T1078.004"
	T1550UseAltAuthMaterial = "T1550" // Use Alternate Authentication Material
	T1550WebSessionCookie   = "T1550.004"
	T1528StealAppToken      = "T1528" // Steal Application Access Token
	T1556ModifyAuthProcess  = "T1556" // Modify Authentication Process
	T1649StealFTPCreds      = "T1649" // Steal or Forge Auth Certificates
	T1110BruteForce         = "T1110" // Brute Force
	T1134AccessTokenManip   = "T1134" // Access Token Manipulation
	T1098AccountManip       = "T1098" // Account Manipulation
	T1484DomainTrustMod     = "T1484" // Domain Policy Modification
	T1098ExtraCloudRoles    = "T1098.003"
)

// tacticForTechnique maps techniques to their primary tactic.
var tacticForTechnique = map[string]string{
	T1078ValidAccounts:      "defense-evasion",
	T1078CloudAccounts:      "defense-evasion",
	T1550UseAltAuthMaterial: "defense-evasion",
	T1550WebSessionCookie:   "defense-evasion",
	T1528StealAppToken:      "credential-access",
	T1556ModifyAuthProcess:  "credential-access",
	T1649StealFTPCreds:      "credential-access",
	T1110BruteForce:         "credential-access",
	T1134AccessTokenManip:   "defense-evasion",
	T1098AccountManip:       "persistence",
	T1098ExtraCloudRoles:    "persistence",
	T1484DomainTrustMod:     "defense-evasion",
}

// techniqueScores assigns Aether's coverage evidence per technique.
var techniqueNames = map[string]string{
	T1078ValidAccounts:      "Valid Accounts",
	T1078CloudAccounts:      "Valid Accounts: Cloud Accounts",
	T1550UseAltAuthMaterial: "Use Alternate Authentication Material",
	T1550WebSessionCookie:   "Web Session Cookie (PRT replay)",
	T1528StealAppToken:      "Steal Application Access Token (IMDS/SP)",
	T1556ModifyAuthProcess:  "Modify Authentication Process (alg confusion)",
	T1649StealFTPCreds:      "Steal or Forge Authentication Certificates",
	T1110BruteForce:         "Brute Force (WS-Trust relay)",
	T1134AccessTokenManip:   "Access Token Manipulation (aud confusion)",
	T1098AccountManip:       "Account Manipulation (SP secrets)",
	T1098ExtraCloudRoles:    "Additional Cloud Roles",
	T1484DomainTrustMod:     "Domain Trust Modification (SAML strip)",
}

// actionToTechnique maps aether actions to ATT&CK techniques.
var actionToTechnique = map[string]string{
	"prt_exchange":    T1550WebSessionCookie,
	"token_refresh":   T1078CloudAccounts,
	"wstrust_relay":   T1110BruteForce,
	"fido2_downgrade": T1110BruteForce,
	"saml_forge":      T1550UseAltAuthMaterial,
	"vm_runcommand":   T1078ValidAccounts,
	"ssm_runcommand":  T1078ValidAccounts,
	"ci_dispatch":     T1078ValidAccounts,
	"imds_hijack":     T1528StealAppToken,
	"sp_hijack":       T1098AccountManip,
	"cloudkerberos":   T1550UseAltAuthMaterial,
}

// NavigatorLayer is the MITRE ATT&CK Navigator layer JSON.
type NavigatorLayer struct {
	Name        string         `json:"name"`
	Version     string         `json:"version"`
	Domain      string         `json:"domain"`
	Description string         `json:"description"`
	Filters     json.RawMessage `json:"filters,omitempty"`
	Sorting     int            `json:"sorting"`
	Layout      string         `json:"layout"`
	Techniques  []NavigatorTech `json:"techniques"`
	Legend      json.RawMessage `json:"legend,omitempty"`
}

// NavigatorTech is one technique annotation.
type NavigatorTech struct {
	TechniqueID string  `json:"techniqueID"`
	Tactic      string  `json:"tactic"`
	Color       string  `json:"color"`
	Comment     string  `json:"comment,omitempty"`
	Enabled     bool    `json:"enabled"`
	Score       float64 `json:"score,omitempty"`
}

// BuildNavigatorLayer generates an ATT&CK Navigator layer from the
// actions performed/validated during an engagement. Each exercised
// technique is colored by coverage: green = exercised, orange = tested
// in simulation.
func BuildNavigatorLayer(workspace string, actions []string) *NavigatorLayer {
	techs := map[string]*NavigatorTech{}

	add := func(techniqueID, comment string, exercised bool) {
		tactic, ok := tacticForTechnique[techniqueID]
		if !ok {
			return
		}
		existing, seen := techs[techniqueID+"|"+tactic]
		if !seen {
			color := "#8ecae6" // tested/simulated
			score := 1.0
			if exercised {
				color = "#e63946" // exercised in engagement
				score = 3.0
			}
			existing = &NavigatorTech{
				TechniqueID: techniqueID,
				Tactic:      tactic,
				Color:       color,
				Enabled:     true,
				Score:       score,
			}
			techs[techniqueID+"|"+tactic] = existing
		}
		if comment != "" {
			if existing.Comment != "" {
				existing.Comment += "; "
			}
			existing.Comment += comment
		}
	}

	for _, a := range actions {
		techniqueID, ok := actionToTechnique[strings.ToLower(a)]
		if !ok {
			continue
		}
		name := techniqueNames[techniqueID]
		add(techniqueID, fmt.Sprintf("%s (aether: %s)", name, a), true)
	}

	layer := &NavigatorLayer{
		Name:        "Aether Engagement — " + workspace,
		Version:     "4.5",
		Domain:      "enterprise-attack",
		Description: "Techniques exercised by the Aether identity attack simulation for workspace " + workspace + ".",
		Sorting:     3,
		Layout:      "layout-side",
	}
	for _, t := range techs {
		layer.Techniques = append(layer.Techniques, *t)
	}

	// Deterministic ordering.
	for i := 0; i < len(layer.Techniques); i++ {
		for j := i + 1; j < len(layer.Techniques); j++ {
			if layer.Techniques[j].TechniqueID < layer.Techniques[i].TechniqueID {
				layer.Techniques[i], layer.Techniques[j] = layer.Techniques[j], layer.Techniques[i]
			}
		}
	}
	return layer
}

// NavigatorJSON renders the layer as Navigator-compatible JSON.
func (l *NavigatorLayer) NavigatorJSON() (string, error) {
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
