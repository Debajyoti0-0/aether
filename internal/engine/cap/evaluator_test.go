package cap

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleGraphPolicies = `{
  "value": [
    {
      "id": "11111111-1111-1111-1111-111111111111",
      "displayName": "Require MFA for all",
      "state": "enabled",
      "conditions": {
        "applications": {"includeApplications": ["All"]},
        "users": {"includeUsers": ["All"]},
        "platforms": {"includePlatforms": ["windows"]},
        "clientApps": {"includeClientApps": ["browser"]}
      },
      "grantControls": {
        "operator": "AND",
        "builtInControls": ["mfa"]
      }
    },
    {
      "id": "22222222-2222-2222-2222-222222222222",
      "displayName": "Block legacy auth",
      "state": "disabled",
      "conditions": {
        "applications": {"includeApplications": ["All"]},
        "users": {"includeUsers": ["All"]}
      },
      "grantControls": {"operator": "AND", "builtInControls": ["block"]}
    }
  ]
}`

func TestParseFromGraph(t *testing.T) {
	policies, err := ParseFromGraph([]byte(sampleGraphPolicies))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(policies) != 2 {
		t.Fatalf("got %d policies, want 2", len(policies))
	}
	if policies[0].DisplayName != "Require MFA for all" {
		t.Errorf("name = %q", policies[0].DisplayName)
	}
	if len(policies[0].GrantControls.BuiltInControls) != 1 {
		t.Errorf("grant controls = %v", policies[0].GrantControls.BuiltInControls)
	}
}

func TestParseFromFileAndEvaluate(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "policies.json")
	if err := os.WriteFile(file, []byte(sampleGraphPolicies), 0o600); err != nil {
		t.Fatal(err)
	}

	policies, err := ParseFromFile(file)
	if err != nil {
		t.Fatalf("parse file failed: %v", err)
	}

	e := NewEvaluator(policies)
	strategy, err := e.Evaluate("user@example.com", "00000003-0000-0000-c000-000000000000")
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}

	if !strategy.RequiresMFA {
		t.Error("MFA should be required (enabled policy)")
	}
	if strategy.RequiredOS != "windows" {
		t.Errorf("required OS = %q, want windows", strategy.RequiredOS)
	}
	if strategy.RiskLevel < 30 {
		t.Errorf("risk level = %d, want >= 30", strategy.RiskLevel)
	}
	if len(strategy.SpoofingRecommendations) == 0 {
		t.Error("should have recommendations")
	}
}

func TestEvaluateNoPolicies(t *testing.T) {
	e := NewEvaluator(nil)
	strategy, err := e.Evaluate("u@x.com", "app")
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if strategy.RequiresMFA || strategy.RequiredDeviceCompliance {
		t.Error("no policies should mean no controls")
	}
}

func TestDisabledPolicyIgnored(t *testing.T) {
	policies, _ := ParseFromGraph([]byte(sampleGraphPolicies))
	e := NewEvaluator(policies)

	got := e.ApplicablePolicies("user@example.com", "some-app")
	if len(got) != 1 {
		t.Fatalf("applicable = %d, want 1 (disabled policy excluded)", len(got))
	}
	if got[0].ID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("wrong policy: %s", got[0].ID)
	}
}
