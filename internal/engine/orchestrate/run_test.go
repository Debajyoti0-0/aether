package orchestrate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Debajyoti0-0/aether/internal/transport"
	"github.com/Debajyoti0-0/aether/internal/workspace"
)

func testWorkspace(t *testing.T, name string) *workspace.Workspace {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("USERPROFILE", dir)
	t.Setenv("AppData", dir)
	t.Setenv("HOME", dir)

	w, err := workspace.Create(name, "pw")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	if err := w.LogEvent("boot", name); err != nil {
		t.Fatal(err)
	}
	return w
}

func TestRunFullChain(t *testing.T) {
	ws := testWorkspace(t, "ChainWS")

	policies := `{"value":[{"id":"p1","displayName":"MFA","state":"enabled",
	 "conditions":{"applications":{"includeApplications":["All"]},"users":{"includeUsers":["All"]}},
	 "grantControls":{"operator":"AND","builtInControls":["mfa"]}}]}`
	polPath := filepath.Join(t.TempDir(), "policies.json")
	if err := os.WriteFile(polPath, []byte(policies), 0o600); err != nil {
		t.Fatal(err)
	}

	bhFile := filepath.Join(t.TempDir(), "path.json")
	// Add matching node for g1 so validation passes.
	bhPath := `{"nodes":[{"id":"u1","label":"User"},{"id":"g1","label":"Group"}],
	  "edges":[{"source":"u1","target":"g1","type":"MemberOf"}]}`
	if err := os.WriteFile(bhFile, []byte(bhPath), 0o600); err != nil {
		t.Fatal(err)
	}

	o := &Orchestrator{
		WS:           ws,
		MaxRisk:      50,
		LowSlow:      &transport.FixedJitter{D: time.Millisecond},
		PoliciesFile: polPath,
		PathFile:     bhFile,
	}

	results, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	phases := map[string]PhaseResult{}
	for _, r := range results {
		phases[r.Phase] = r
	}

	for _, want := range []string{PhaseBypass, PhaseValidate, PhasePredict, PhaseReport} {
		if _, ok := phases[want]; !ok {
			t.Errorf("missing phase %s", want)
		}
	}
	if !phases[PhaseBypass].OK {
		t.Errorf("bypass: %s", phases[PhaseBypass].Output)
	}
	if !phases[PhaseValidate].OK {
		t.Errorf("validate: %s", phases[PhaseValidate].Output)
	}
	if !strings.Contains(phases[PhaseReport].Output, "journal: 1 events") {
		t.Errorf("report: %s", phases[PhaseReport].Output)
	}
}

func TestRunRiskGate(t *testing.T) {
	ws := testWorkspace(t, "GateWS")

	// CAP requires MFA + compliant device → risk 75 > ceiling 10.
	policies := `{"value":[{"id":"p1","displayName":"Strict","state":"enabled",
	 "conditions":{"applications":{"includeApplications":["All"]},"users":{"includeUsers":["All"]}},
	 "grantControls":{"operator":"AND","builtInControls":["mfa","compliantDevice"]}}]}`
	polPath := filepath.Join(t.TempDir(), "policies.json")
	if err := os.WriteFile(polPath, []byte(policies), 0o600); err != nil {
		t.Fatal(err)
	}

	o := &Orchestrator{
		WS:           ws,
		MaxRisk:      10,
		LowSlow:      NoJitter{},
		PoliciesFile: polPath,
	}

	results, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, r := range results {
		if r.Phase == PhaseBypass && r.OK {
			t.Fatal("bypass should fail the risk gate")
		}
	}
}

func TestRunAutoContinuesPastGate(t *testing.T) {
	ws := testWorkspace(t, "AutoWS")

	policies := `{"value":[{"id":"p1","displayName":"Strict","state":"enabled",
	 "conditions":{"applications":{"includeApplications":["All"]},"users":{"includeUsers":["All"]}},
	 "grantControls":{"operator":"AND","builtInControls":["mfa","compliantDevice"]}}]}`
	polPath := filepath.Join(t.TempDir(), "policies.json")
	if err := os.WriteFile(polPath, []byte(policies), 0o600); err != nil {
		t.Fatal(err)
	}

	o := &Orchestrator{
		WS:           ws,
		MaxRisk:      10,
		LowSlow:      NoJitter{},
		PoliciesFile: polPath,
		Auto:         true,
	}

	results, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, r := range results {
		if r.Phase == PhaseBypass && !r.OK {
			t.Fatal("auto mode should continue past the risk gate")
		}
		if r.Phase == PhaseBypass && !strings.Contains(r.Output, "auto-continue") {
			t.Errorf("expected auto-continue warning, got %q", r.Output)
		}
	}
}

func TestRunAutoUsesStoredPRT(t *testing.T) {
	ws := testWorkspace(t, "AutoPRT")

	// Store a PRT; conversion will fail against the real STS (no
	// network in tests) but the phase must attempt it and record.
	prt := map[string]string{"cookie": "0.AAAA", "tenant_id": "t", "session_key": "MDEyMzQ1Njc4OWFiY2RlZg=="}
	if err := ws.SaveRecord(workspace.BucketTokens, "prt", prt); err != nil {
		t.Fatal(err)
	}

	o := &Orchestrator{WS: ws, MaxRisk: 50, LowSlow: NoJitter{}, Auto: true}
	results, _ := o.Run(context.Background())

	for _, r := range results {
		if r.Phase != PhaseConvert {
			continue
		}
		// Either attempted (network fail) or skipped — but not silently absent.
		if r.Output == "" {
			t.Error("convert phase should record an outcome")
		}
	}
}

func TestRunMinimal(t *testing.T) {
	ws := testWorkspace(t, "MinWS")
	o := &Orchestrator{WS: ws, MaxRisk: 50, LowSlow: NoJitter{}}

	results, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("phases = %d", len(results))
	}
	// Bypass/convert/validate are no-ops without inputs.
	for _, r := range results[:3] {
		if !r.OK {
			t.Errorf("phase %s should be OK (no-op): %s", r.Phase, r.Output)
		}
	}
}
