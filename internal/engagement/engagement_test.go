package engagement

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// windowAround returns an engagement whose time window brackets now.
func windowAround(now time.Time) TimeWindow {
	return TimeWindow{
		Start: now.Add(-time.Hour).Format(time.RFC3339),
		End:   now.Add(time.Hour).Format(time.RFC3339),
	}
}

func baseEngagement() *Engagement {
	return &Engagement{
		EngagementID:           "test-001",
		AuthorizedDomains:      []string{"example.com"},
		AuthorizedDCs:          []string{"dc01.example.com"},
		AuthorizedCapabilities: []string{CapEnumRead, CapKerbRoast, CapKerbTGT},
		TimeWindow:             windowAround(time.Now().UTC()),
		Operator:               "bob",
	}
}

func TestIsAuthorizedAllowsInScopeOperation(t *testing.T) {
	e := baseEngagement()
	if err := e.IsAuthorized("example.com", "dc01.example.com", CapEnumRead); err != nil {
		t.Fatalf("in-scope operation refused: %v", err)
	}
}

func TestIsAuthorizedComparesDomainAndDCCaseInsensitively(t *testing.T) {
	e := baseEngagement()
	// DNS names are case-insensitive; an engagement naming EXAMPLE.COM must
	// authorize a target the operator typed in lower case, and the reverse.
	for _, domain := range []string{"example.com", "EXAMPLE.COM", "ExAmPlE.cOm"} {
		if err := e.IsAuthorized(domain, "dc01.example.com", CapEnumRead); err != nil {
			t.Errorf("domain %q refused, want authorized: %v", domain, err)
		}
	}
	for _, dc := range []string{"dc01.example.com", "DC01.EXAMPLE.COM"} {
		if err := e.IsAuthorized("example.com", dc, CapEnumRead); err != nil {
			t.Errorf("dc %q refused, want authorized: %v", dc, err)
		}
	}
}

func TestIsAuthorizedRefusesOutOfScope(t *testing.T) {
	cases := []struct {
		name       string
		domain     string
		dc         string
		capability string
		wantSubstr string
	}{
		{"unauthorized domain", "evil.com", "dc01.example.com", CapEnumRead, "is not authorized"},
		{"unauthorized dc", "example.com", "dc02.evil.com", CapEnumRead, "is not authorized"},
		{"unauthorized capability", "example.com", "dc01.example.com", "ad.cs.enroll", "is not authorized"},
		{"capability is case-sensitive", "example.com", "dc01.example.com", "AD.ENUM.READ", "is not authorized"},
		{"suffix is not a subdomain grant", "example.com.evil.com", "dc01.example.com", CapEnumRead, "is not authorized"},
		{"prefix is not a domain grant", "notexample.com", "dc01.example.com", CapEnumRead, "is not authorized"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := baseEngagement().IsAuthorized(tc.domain, tc.dc, tc.capability)
			if err == nil {
				t.Fatalf("IsAuthorized(%q, %q, %q) = nil, want refusal", tc.domain, tc.dc, tc.capability)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("error %q does not mention %q", err, tc.wantSubstr)
			}
		})
	}
}

func TestIsAuthorizedRefusesOutsideTimeWindow(t *testing.T) {
	now := time.Now().UTC()

	closed := baseEngagement()
	closed.TimeWindow = TimeWindow{
		Start: now.Add(-2 * time.Hour).Format(time.RFC3339),
		End:   now.Add(-time.Hour).Format(time.RFC3339),
	}
	if err := closed.IsAuthorized("example.com", "dc01.example.com", CapEnumRead); err == nil {
		t.Error("operation after the window closed was authorized")
	} else if !strings.Contains(err.Error(), "after the engagement time window") {
		t.Errorf("unexpected error for a closed window: %v", err)
	}

	future := baseEngagement()
	future.TimeWindow = TimeWindow{
		Start: now.Add(time.Hour).Format(time.RFC3339),
		End:   now.Add(2 * time.Hour).Format(time.RFC3339),
	}
	if err := future.IsAuthorized("example.com", "dc01.example.com", CapEnumRead); err == nil {
		t.Error("operation before the window opened was authorized")
	} else if !strings.Contains(err.Error(), "before the engagement time window") {
		t.Errorf("unexpected error for a future window: %v", err)
	}
}

func TestIsAuthorizedRefusesMalformedWindow(t *testing.T) {
	// A malformed bound must refuse, never default to an open window.
	badStart := baseEngagement()
	badStart.TimeWindow = TimeWindow{Start: "not-a-time", End: time.Now().Add(time.Hour).Format(time.RFC3339)}
	if err := badStart.IsAuthorized("example.com", "dc01.example.com", CapEnumRead); err == nil {
		t.Error("malformed start time was accepted")
	}

	badEnd := baseEngagement()
	badEnd.TimeWindow = TimeWindow{Start: time.Now().Add(-time.Hour).Format(time.RFC3339), End: "01/01/2026"}
	if err := badEnd.IsAuthorized("example.com", "dc01.example.com", CapEnumRead); err == nil {
		t.Error("malformed end time was accepted")
	}

	empty := baseEngagement()
	now := time.Now().UTC().Format(time.RFC3339)
	empty.TimeWindow = TimeWindow{Start: now, End: now}
	if err := empty.IsAuthorized("example.com", "dc01.example.com", CapEnumRead); err == nil {
		t.Error("empty time window (start == end) was accepted")
	}

	reversed := baseEngagement()
	reversed.TimeWindow = TimeWindow{
		Start: now,
		End:   time.Now().Add(-time.Hour).Format(time.RFC3339),
	}
	if err := reversed.IsAuthorized("example.com", "dc01.example.com", CapEnumRead); err == nil {
		t.Error("reversed time window was accepted")
	}
}

func TestIsAuthorizedOnNilEngagementRefuses(t *testing.T) {
	var e *Engagement
	if err := e.IsAuthorized("example.com", "dc01.example.com", CapEnumRead); err == nil {
		t.Error("nil engagement authorized an operation; scope enforcement must fail closed")
	}
}

func writeFile(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

const validDoc = `{
  "engagement_id": "test-001",
  "authorized_domains": ["example.com"],
  "authorized_dcs": ["dc01.example.com"],
  "authorized_capabilities": ["ad.enum.read"],
  "time_window": {"start": "2020-01-01T00:00:00Z", "end": "2999-01-01T00:00:00Z"},
  "operator": "bob"
}`

func TestLoadAcceptsValidDocument(t *testing.T) {
	e, err := Load(writeFile(t, "engagement.json", validDoc))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if e.EngagementID != "test-001" {
		t.Errorf("engagement_id = %q, want test-001", e.EngagementID)
	}
	if err := e.IsAuthorized("example.com", "dc01.example.com", CapEnumRead); err != nil {
		t.Errorf("loaded engagement refused an in-scope operation: %v", err)
	}
}

func TestLoadRefusesUnusableDocuments(t *testing.T) {
	cases := []struct {
		name string
		path string
		body string
	}{
		{"empty path", "", ""},
		{"whitespace path", "   ", ""},
		{"missing file", filepath.Join(t.TempDir(), "absent.json"), ""},
		{"malformed json", "bad.json", `{"engagement_id": `},
		{"not an object", "arr.json", `["example.com"]`},
		{"no engagement id", "noid.json", `{"authorized_domains":["example.com"],"authorized_dcs":["dc01.example.com"],"authorized_capabilities":["ad.enum.read"],"time_window":{"start":"2020-01-01T00:00:00Z","end":"2999-01-01T00:00:00Z"}}`},
		{"no domains", "nodom.json", `{"engagement_id":"e","authorized_dcs":["dc01.example.com"],"authorized_capabilities":["ad.enum.read"],"time_window":{"start":"2020-01-01T00:00:00Z","end":"2999-01-01T00:00:00Z"}}`},
		{"no dcs", "nodc.json", `{"engagement_id":"e","authorized_domains":["example.com"],"authorized_capabilities":["ad.enum.read"],"time_window":{"start":"2020-01-01T00:00:00Z","end":"2999-01-01T00:00:00Z"}}`},
		{"no capabilities", "nocap.json", `{"engagement_id":"e","authorized_domains":["example.com"],"authorized_dcs":["dc01.example.com"],"time_window":{"start":"2020-01-01T00:00:00Z","end":"2999-01-01T00:00:00Z"}}`},
		{"bad start time", "badstart.json", `{"engagement_id":"e","authorized_domains":["example.com"],"authorized_dcs":["dc01.example.com"],"authorized_capabilities":["ad.enum.read"],"time_window":{"start":"nope","end":"2999-01-01T00:00:00Z"}}`},
		{"bad end time", "badend.json", `{"engagement_id":"e","authorized_domains":["example.com"],"authorized_dcs":["dc01.example.com"],"authorized_capabilities":["ad.enum.read"],"time_window":{"start":"2020-01-01T00:00:00Z","end":"nope"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Load(tc.path); err == nil {
				t.Fatal("Load accepted an unusable engagement document")
			}
		})
	}
}
