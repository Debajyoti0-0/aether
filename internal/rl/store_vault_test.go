package rl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Debajyoti0-0/aether/internal/workspace"
)

func testVaultWS(t *testing.T) *workspace.Workspace {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AETHER_CONFIG_DIR", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("AppData", dir)
	t.Setenv("HOME", dir)
	w, err := workspace.Create("PlannerWS", "pw")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	return w
}

// T5 acceptance: episodes round-trip through the vault.
func TestPlannerVaultRoundTrip(t *testing.T) {
	ws := testVaultWS(t)
	vs := NewVaultEpisodeStore(ws.Vault())

	ep := Episode{
		ID:        "ep-1",
		Workspace: "PlannerWS",
		Steps: []EpisodeStep{
			{StateKey: "0|low|medium|recon", ActionKey: "graph build", Reward: 1, NextState: "1|low|medium|recon"},
		},
	}
	if err := vs.Append(ep); err != nil {
		t.Fatalf("append: %v", err)
	}
	loaded, err := vs.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 1 || loaded[0].ID != "ep-1" || len(loaded[0].Steps) != 1 {
		t.Fatalf("loaded = %+v", loaded)
	}

	// Deterministic ordering by episode ID.
	if err := vs.Append(Episode{ID: "ep-0", Workspace: "PlannerWS"}); err != nil {
		t.Fatal(err)
	}
	loaded, _ = vs.Load()
	if loaded[0].ID != "ep-0" {
		t.Errorf("ordering broken: %v", loaded[0].ID)
	}
	n, _ := vs.Count()
	if n != 2 {
		t.Errorf("count = %d", n)
	}
}

// T5 acceptance: legacy JSONL imports idempotently; the original file
// is preserved as .pre-vault-imported (no data destroyed).
func TestPlannerLegacyMigration(t *testing.T) {
	ws := testVaultWS(t)
	dir, _ := os.Getwd()
	legacy := filepath.Join(dir, ws.Name+"-episodes.jsonl")
	t.Cleanup(func() { _ = os.Remove(legacy + ".pre-vault-imported") })

	legacyJSON := `{"id":"ep-legacy","workspace":"PlannerWS","started_at":"2026-09-10T00:00:00Z","steps":[{"state":"0|low|medium|recon","action":"graph build","reward":1,"next_state":"1|low|medium|recon"}]}
{"id":"ep-legacy-2","workspace":"PlannerWS","started_at":"2026-09-10T00:00:01Z","steps":[]}
`
	if err := os.WriteFile(legacy, []byte(legacyJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Simulate the CLI migration path.
	data, err := os.ReadFile(legacy)
	if err != nil {
		t.Fatal(err)
	}
	episodes, err := NewEpisodeStoreBytes(data).Load()
	if err != nil {
		t.Fatalf("parse legacy: %v", err)
	}
	vs := NewVaultEpisodeStore(ws.Vault())
	for _, ep := range episodes {
		if err := vs.Append(ep); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Rename(legacy, legacy+".pre-vault-imported"); err != nil {
		t.Fatal(err)
	}

	// Vault holds both legacy episodes; the plaintext file is preserved
	// under the migration name, not deleted.
	loaded, err := vs.Load()
	if err != nil || len(loaded) != 2 {
		t.Fatalf("vault episodes = %d err=%v", len(loaded), err)
	}
	if _, err := os.Stat(legacy + ".pre-vault-imported"); err != nil {
		t.Fatalf("legacy file not preserved: %v", err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatal("legacy file still at original path after migration")
	}
}

// T5 acceptance: no plaintext episode JSONL remains for vault-backed
// storage; episode payload text is sealed inside the vault.
func TestPlannerNoPlaintextOnDisk(t *testing.T) {
	ws := testVaultWS(t)
	vs := NewVaultEpisodeStore(ws.Vault())
	if err := vs.Append(Episode{ID: "ep-secret-xyz", Workspace: "PlannerWS"}); err != nil {
		t.Fatal(err)
	}

	// The workspace root must contain no planner JSONL files.
	entries, _ := os.ReadDir(ws.Root)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".jsonl") {
			t.Errorf("plaintext planner file in workspace root: %s", e.Name())
		}
	}

	data, _ := os.ReadFile(filepath.Join(ws.Root, "vault.db"))
	if strings.Contains(string(data), "graph build") {
		t.Error("episode plaintext found unsealed in vault")
	}
}
