// Package engagement implements rules-of-engagement scope enforcement.
//
// An engagement file declares the domains, domain controllers, capabilities
// and a time window that an operator is authorized to exercise. Offensive
// Active Directory commands load one and are refused unless the requested
// (domain, dc, capability) triple falls inside the declared scope and the
// current time falls inside the window.
//
// This control was recovered during Stage 53R from the C:\dev\aether
// development line, where it lived in package cli and was wired into the
// enum, roast and tgt commands. It is promoted to its own package here so
// that it is testable independently of the command layer and reusable by
// any future command that needs scope enforcement.
//
// The recovered implementation compared domains and domain controllers with
// exact string equality. DNS names are case-insensitive, so an engagement
// naming "EXAMPLE.COM" must authorize a target typed as "example.com" and
// vice versa. Comparison is therefore case-insensitive. This weakens
// nothing: every value that is not authorized is still refused.
package engagement

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// Engagement is the on-disk rules-of-engagement document.
type Engagement struct {
	EngagementID           string     `json:"engagement_id"`
	AuthorizedDomains      []string   `json:"authorized_domains"`
	AuthorizedDCs          []string   `json:"authorized_dcs"`
	AuthorizedCapabilities []string   `json:"authorized_capabilities"`
	TimeWindow             TimeWindow `json:"time_window"`
	Operator               string     `json:"operator"`
}

// TimeWindow is the inclusive-exclusive interval during which the
// engagement is valid. Both bounds are RFC 3339 timestamps.
type TimeWindow struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// Capability identifiers used by the Active Directory command surface.
// These strings are the contract between an engagement file and the
// commands that consult it; they are matched exactly (capabilities are
// identifiers, not names, and are therefore case-sensitive).
const (
	CapEnumRead  = "ad.enum.read"
	CapKerbRoast = "ad.kerberos.roast"
	CapKerbTGT   = "ad.kerberos.tgt"
)

// IsAuthorized reports whether the engagement permits the given capability
// against the given domain and domain controller at this instant. It
// returns a descriptive error for every refusal so that the CLI can tell
// the operator precisely which clause of the engagement was not met.
//
// All checks are fail-closed: an engagement that cannot be parsed, is
// missing a window, or carries a malformed timestamp refuses the operation.
func (e *Engagement) IsAuthorized(domain, dc, capability string) error {
	if e == nil {
		return errors.New("engagement scope is not loaded")
	}

	if !containsFold(e.AuthorizedDomains, domain) {
		return fmt.Errorf("domain %q is not authorized in engagement %s", domain, e.describe())
	}
	if !containsFold(e.AuthorizedDCs, dc) {
		return fmt.Errorf("domain controller %q is not authorized in engagement %s", dc, e.describe())
	}
	if !contains(e.AuthorizedCapabilities, capability) {
		return fmt.Errorf("capability %q is not authorized in engagement %s", capability, e.describe())
	}

	now := time.Now().UTC()
	if err := e.checkWindow(now); err != nil {
		return err
	}
	return nil
}

// checkWindow refuses the operation unless now is inside the declared
// window. A malformed bound refuses rather than defaulting to open.
func (e *Engagement) checkWindow(now time.Time) error {
	start, err := time.Parse(time.RFC3339, e.TimeWindow.Start)
	if err != nil {
		return fmt.Errorf("engagement %s has an invalid start time %q: %w", e.describe(), e.TimeWindow.Start, err)
	}
	end, err := time.Parse(time.RFC3339, e.TimeWindow.End)
	if err != nil {
		return fmt.Errorf("engagement %s has an invalid end time %q: %w", e.describe(), e.TimeWindow.End, err)
	}
	if !start.Before(end) {
		return fmt.Errorf("engagement %s has an empty time window: start %s is not before end %s",
			e.describe(), start.Format(time.RFC3339), end.Format(time.RFC3339))
	}
	if now.Before(start) {
		return fmt.Errorf("operation is before the engagement time window (opens %s)", start.Format(time.RFC3339))
	}
	if !now.Before(end) {
		return fmt.Errorf("operation is after the engagement time window (closed %s)", end.Format(time.RFC3339))
	}
	return nil
}

// describe names the engagement for error messages, tolerating an
// otherwise-valid document that omits the identifier.
func (e *Engagement) describe() string {
	if e.EngagementID == "" {
		return "(unnamed)"
	}
	return e.EngagementID
}

// contains reports whether needle appears in haystack, exactly.
func contains(haystack []string, needle string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

// containsFold reports whether needle appears in haystack, comparing ASCII
// case-insensitively. DNS domain names and hostnames are case-insensitive
// by definition, so authorization must be too.
func containsFold(haystack []string, needle string) bool {
	for _, v := range haystack {
		if strings.EqualFold(v, needle) {
			return true
		}
	}
	return false
}

// Load reads and parses an engagement document. An empty path is an error:
// scope enforcement is fail-closed, so "no engagement" is never a wildcard.
func Load(path string) (*Engagement, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("engagement file required: refusing the operation without a rules-of-engagement scope")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read engagement file: %w", err)
	}
	var e Engagement
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("parse engagement file %s: %w", path, err)
	}
	if e.EngagementID == "" {
		return nil, fmt.Errorf("engagement file %s has no engagement_id", path)
	}
	if len(e.AuthorizedDomains) == 0 {
		return nil, fmt.Errorf("engagement file %s authorizes no domains", path)
	}
	if len(e.AuthorizedDCs) == 0 {
		return nil, fmt.Errorf("engagement file %s authorizes no domain controllers", path)
	}
	if len(e.AuthorizedCapabilities) == 0 {
		return nil, fmt.Errorf("engagement file %s authorizes no capabilities", path)
	}
	// Validate the window eagerly so a malformed window is reported at load
	// time rather than at the first command that happens to run.
	if _, err := time.Parse(time.RFC3339, e.TimeWindow.Start); err != nil {
		return nil, fmt.Errorf("engagement file %s has an invalid start time %q: %w", path, e.TimeWindow.Start, err)
	}
	if _, err := time.Parse(time.RFC3339, e.TimeWindow.End); err != nil {
		return nil, fmt.Errorf("engagement file %s has an invalid end time %q: %w", path, e.TimeWindow.End, err)
	}
	return &e, nil
}
