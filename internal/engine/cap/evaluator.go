package cap

import (
	"fmt"
	"strings"

	"github.com/Debajyoti0-0/aether/internal/types"
)

// Evaluator evaluates Conditional Access policies offline to determine
// what an operator must satisfy (or spoof) before acting.
type Evaluator struct {
	Policies []types.ConditionalAccessPolicy
}

// NewEvaluator builds an evaluator over a set of policies.
func NewEvaluator(policies []types.ConditionalAccessPolicy) *Evaluator {
	return &Evaluator{Policies: policies}
}

// Evaluate finds the strictest requirements that apply to a user+app
// pair and produces a bypass strategy.
func (e *Evaluator) Evaluate(targetUser, targetApp string) (*types.BypassStrategy, error) {
	applicable := e.applicable(targetUser, targetApp)

	strategy := &types.BypassStrategy{
		RequiredOS:       "any",
		RequiredLocation: "trusted location (named)",
		RequiredBrowser:  "any",
	}

	if len(applicable) == 0 {
		strategy.SpoofingRecommendations = []string{
			"No enabled policy applies to this user/app pair.",
			"Standard token usage should succeed; keep risk low to avoid sign-in risk policies.",
		}
		strategy.RiskLevel = 10
		return strategy, nil
	}

	reqs := aggregateRequirements(applicable)

	strategy.RequiresMFA = reqs.mfa
	strategy.RequiredDeviceCompliance = reqs.compliantDevice
	strategy.RiskLevel = riskFor(reqs)

	if reqs.platforms.Len() > 0 {
		strategy.RequiredOS = strings.Join(reqs.platforms.Slice(), ", ")
	}
	if reqs.excludePlatforms.Len() > 0 {
		strategy.SpoofingRecommendations = append(strategy.SpoofingRecommendations,
			fmt.Sprintf("Avoid platforms: %s (explicitly excluded)", strings.Join(reqs.excludePlatforms.Slice(), ", ")))
	}
	if reqs.browsers.Len() > 0 {
		strategy.RequiredBrowser = strings.Join(reqs.browsers.Slice(), ", ")
	}
	if reqs.excludeLocations.Len() > 0 {
		strategy.SpoofingRecommendations = append(strategy.SpoofingRecommendations,
			fmt.Sprintf("Avoid locations: %s", strings.Join(reqs.excludeLocations.Slice(), ", ")))
	}
	if reqs.excludeApps.Len() > 0 {
		strategy.SpoofingRecommendations = append(strategy.SpoofingRecommendations,
			fmt.Sprintf("Excluded apps cannot use these tokens: %s", strings.Join(reqs.excludeApps.Slice(), ", ")))
	}

	strategy.SpoofingRecommendations = append(strategy.SpoofingRecommendations, recommendations(reqs)...)

	return strategy, nil
}

// ApplicablePolicies returns enabled policies matching a user+app pair.
func (e *Evaluator) ApplicablePolicies(targetUser, targetApp string) []types.ConditionalAccessPolicy {
	return e.applicable(targetUser, targetApp)
}

type requirements struct {
	mfa             bool
	compliantDevice bool
	domainJoined    bool
	approvedApp     bool
	platforms       set
	excludePlatforms set
	browsers        set
	excludeLocations set
	excludeApps     set
}

type set map[string]bool

func (s set) Len() int      { return len(s) }
func (s set) Slice() []string {
	out := make([]string, 0, len(s))
	for k := range s {
		out = append(out, k)
	}
	return out
}

func (e *Evaluator) applicable(user, app string) []types.ConditionalAccessPolicy {
	var out []types.ConditionalAccessPolicy
	for _, p := range e.Policies {
		if !strings.EqualFold(p.State, "enabled") {
			continue
		}
		if !appliesToUser(p, user) {
			continue
		}
		if !appliesToApp(p, app) {
			continue
		}
		out = append(out, p)
	}
	return out
}

func appliesToUser(p types.ConditionalAccessPolicy, user string) bool {
	c := p.Conditions.Users
	if len(c.IncludeUsers) == 0 && len(c.IncludeGroups) == 0 && len(c.IncludeRoles) == 0 {
		return true // All users
	}
	for _, u := range c.IncludeUsers {
		if u == "All" || strings.EqualFold(u, user) {
			return true
		}
	}
	// Group/role membership cannot be resolved offline; assume included.
	return len(c.IncludeGroups) > 0 || len(c.IncludeRoles) > 0
}

func appliesToApp(p types.ConditionalAccessPolicy, app string) bool {
	c := p.Conditions.Applications
	for _, a := range c.ExcludeApplications {
		if strings.EqualFold(a, app) || a == "All" {
			return false
		}
	}
	if len(c.IncludeApplications) == 0 {
		return true
	}
	for _, a := range c.IncludeApplications {
		if a == "All" || strings.EqualFold(a, app) {
			return true
		}
	}
	return false
}

func aggregateRequirements(policies []types.ConditionalAccessPolicy) requirements {
	r := requirements{
		platforms:        set{},
		excludePlatforms: set{},
		browsers:         set{},
		excludeLocations: set{},
		excludeApps:      set{},
	}

	for _, p := range policies {
		g := p.GrantControls
		for _, ctl := range g.BuiltInControls {
			switch strings.ToLower(ctl) {
			case "mfa":
				r.mfa = true
			case "compliantdevice":
				r.compliantDevice = true
			case "domainjoineddevice":
				r.domainJoined = true
			case "approvedapplication":
				r.approvedApp = true
			}
		}

		for _, pf := range p.Conditions.Platforms.IncludePlatforms {
			if pf != "all" {
				r.platforms[pf] = true
			}
		}
		for _, pf := range p.Conditions.Platforms.ExcludePlatforms {
			r.excludePlatforms[pf] = true
		}
		for _, ca := range p.Conditions.ClientApps.IncludeClientApps {
			if strings.EqualFold(ca, "browser") {
				r.browsers["browser"] = true
			}
		}
		for _, loc := range p.Conditions.Locations.ExcludeLocations {
			r.excludeLocations[loc] = true
		}
		for _, a := range p.Conditions.Applications.ExcludeApplications {
			r.excludeApps[a] = true
		}
	}
	return r
}

func riskFor(r requirements) int {
	risk := 10
	if r.mfa {
		risk += 30
	}
	if r.compliantDevice {
		risk += 35
	}
	if r.domainJoined {
		risk += 15
	}
	if r.approvedApp {
		risk += 10
	}
	if risk > 100 {
		risk = 100
	}
	return risk
}

func recommendations(r requirements) []string {
	var out []string
	if r.mfa {
		out = append(out, "MFA is enforced: use a relayed/stolen session (PRT or refresh token) instead of interactive auth.")
	}
	if r.compliantDevice || r.domainJoined {
		out = append(out, "Device compliance is enforced: present a registered/compliant device claim (PRT with device claims).")
	}
	if r.platforms.Len() > 0 {
		out = append(out, "Spoof the device platform to: "+strings.Join(r.platforms.Slice(), ", ")+".")
	}
	if r.excludeLocations.Len() > 0 {
		out = append(out, "Source traffic from a trusted/named location or avoid excluded location ranges.")
	}
	out = append(out, "Match TLS fingerprint (JA3/JA4) to the required client to avoid client-app detection.")
	return out
}
