package graph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGraphEndToEndContract pins the full F-010 data contract:
// provider JSON -> ingest -> serialize -> storage -> deserialize ->
// consumer -> executable runbook. Semantic correctness, not just parse success.
func TestGraphEndToEndContract(t *testing.T) {
	g := &IdentityGraph{}

	// Producer: entra ingest, per kind (users, servicePrincipals,
	// roleAssignments — each kind has its own payload shape).
	if err := g.IngestEntraJSON("users", []byte(`{"value":[
		{"id":"u1","userPrincipalName":"alice@corp.example","department":"IT"}
	]}`)); err != nil {
		t.Fatalf("ingest users: %v", err)
	}
	if err := g.IngestEntraJSON("servicePrincipals", []byte(`{"value":[
		{"id":"sp9","displayName":"automation-sp"}
	]}`)); err != nil {
		t.Fatalf("ingest sps: %v", err)
	}
	if err := g.IngestEntraJSON("roleAssignments", []byte(`{"value":[
		{"principalId":"u1","roleDefinitionId":"r-admin"}
	]}`)); err != nil {
		t.Fatalf("ingest assignments: %v", err)
	}
	// Expected shape: u1, dept:IT, sp9, role:r-admin (4 nodes);
	// u1->dept:IT (member_of), u1->role:r-admin (has_role) (2 edges).
	if len(g.Nodes) != 4 || len(g.Edges) != 2 {
		t.Fatalf("producer shape: nodes=%d edges=%d, want 4/2", len(g.Nodes), len(g.Edges))
	}

	// Serialize -> storage -> deserialize.
	path := filepath.Join(t.TempDir(), "graph.json")
	if err := g.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadGraph(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Nodes) != len(g.Nodes) || len(loaded.Edges) != len(g.Edges) {
		t.Fatalf("roundtrip shape mismatch: %d/%d vs %d/%d",
			len(loaded.Nodes), len(loaded.Edges), len(g.Nodes), len(g.Edges))
	}

	// Consumer: qualify an executable path over the loaded graph.
	eng := NewGraphEngine(loaded, 60)
	eng.MarkTokensHeld("u1")
	ep, err := eng.QualifyPath([]string{"u1", "role:r-admin"})
	if err != nil {
		t.Fatalf("qualify: %v", err)
	}
	if len(ep.Runbook) == 0 {
		t.Errorf("runbook empty for qualified path")
	}
	if ep.OPSECRisk != EdgeRisk["has_role"] {
		t.Errorf("risk = %d, want %d", ep.OPSECRisk, EdgeRisk["has_role"])
	}

	// Consumer refusal semantics: missing node, missing edge.
	if _, err := eng.QualifyPath([]string{"nope", "role:r-admin"}); err == nil || !strings.Contains(err.Error(), "unknown node") {
		t.Errorf("unknown node refusal: %v", err)
	}
}

func TestGraphRawLoadDuplicateIDsDeterministic(t *testing.T) {
	// Raw JSON with duplicate node IDs: load is deterministic (first wins),
	// which the executor's findNode relies on. Pinned as the contract.
	raw := []byte(`{"nodes":[
		{"id":"x","type":"user","provider":"entra","label":"first"},
		{"id":"x","type":"user","provider":"entra","label":"second"}
	],"edges":[]}`)
	path := filepath.Join(t.TempDir(), "dup.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	g, err := LoadGraph(path)
	if err != nil {
		t.Fatalf("load dup: %v", err)
	}
	eng := NewGraphEngine(g, 60)
	node := eng.findNode("x")
	if node == nil || node.Label != "first" {
		t.Fatalf("duplicate ID resolution = %+v, want first-wins", node)
	}
}

func TestGraphRawLoadMalformedRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte(`{"nodes":[{"id":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadGraph(path); err == nil {
		t.Fatal("malformed graph must be rejected")
	}
}

func TestGraphLargeRoundtrip(t *testing.T) {
	// Large graph: 2000 nodes / 2000 edges survive the roundtrip.
	g := &IdentityGraph{}
	for i := 0; i < 2000; i++ {
		g.AddNode(GraphNode{ID: "n" + itoa(i), Type: "user", Provider: "entra", Label: "user" + itoa(i)})
		g.AddEdge(GraphEdge{Source: "n" + itoa(i), Target: "n" + itoa((i+1)%2000), Type: "member_of", Weight: 1})
	}
	path := filepath.Join(t.TempDir(), "big.json")
	if err := g.Save(path); err != nil {
		t.Fatalf("save big: %v", err)
	}
	loaded, err := LoadGraph(path)
	if err != nil {
		t.Fatalf("load big: %v", err)
	}
	if len(loaded.Nodes) != 2000 || len(loaded.Edges) != 2000 {
		t.Fatalf("big roundtrip: nodes=%d edges=%d, want 2000/2000", len(loaded.Nodes), len(loaded.Edges))
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [8]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}
