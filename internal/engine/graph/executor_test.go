package graph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const usersJSON = `{"value":[
  {"id":"u1","userPrincipalName":"alice@corp.com","department":"IT"},
  {"id":"u2","userPrincipalName":"bob@corp.com"}
]}`

const spJSON = `{"value":[{"id":"sp1","displayName":"CI-Runner"}]}`

const rolesJSON = `{"value":[
  {"principalId":"u1","roleDefinitionId":"role-global-admin"},
  {"principalId":"sp1","roleDefinitionId":"role-exchange"}
]}`

const awsJSON = `{
  "users": [{"userName":"deploy","arn":"arn:aws:iam::1:user/deploy","groups":["Admins"]}],
  "roles": [{"roleName":"EC2-Admin","arn":"arn:aws:iam::1:role/EC2-Admin","assumedBy":[{"principal":"arn:aws:iam::1:user/deploy"}]}]
}`

const gcpJSON = `{
  "serviceAccounts": [{"email":"sa@proj.iam.gserviceaccount.com"}],
  "bindings": [{"role":"roles/editor","members":["sa@proj.iam.gserviceaccount.com"]}]
}`

func buildTestGraph(t *testing.T) *IdentityGraph {
	t.Helper()
	g := &IdentityGraph{}
	if err := g.IngestEntraJSON("users", []byte(usersJSON)); err != nil {
		t.Fatal(err)
	}
	if err := g.IngestEntraJSON("servicePrincipals", []byte(spJSON)); err != nil {
		t.Fatal(err)
	}
	if err := g.IngestEntraJSON("roleAssignments", []byte(rolesJSON)); err != nil {
		t.Fatal(err)
	}
	if err := g.IngestAWSJSON([]byte(awsJSON)); err != nil {
		t.Fatal(err)
	}
	if err := g.IngestGCPJSON([]byte(gcpJSON)); err != nil {
		t.Fatal(err)
	}
	return g
}

func TestIngestEntra(t *testing.T) {
	g := buildTestGraph(t)
	if len(g.Nodes) < 7 {
		t.Errorf("nodes = %d, want >= 7", len(g.Nodes))
	}
	// alice has has_role edge to role:role-global-admin
	found := false
	for _, e := range g.Edges {
		if e.Source == "u1" && e.Target == "role:role-global-admin" && e.Type == "has_role" {
			found = true
		}
	}
	if !found {
		t.Error("role edge missing")
	}
}

func TestIngestAWSAndGCP(t *testing.T) {
	g := buildTestGraph(t)
	foundAssume, foundGCP := false, false
	for _, e := range g.Edges {
		if e.Type == "can_assume" {
			foundAssume = true
		}
		if strings.HasPrefix(e.Target, "gcp-role:") {
			foundGCP = true
		}
	}
	if !foundAssume || !foundGCP {
		t.Errorf("assume=%v gcp=%v", foundAssume, foundGCP)
	}
}

func TestIngestUnknownKind(t *testing.T) {
	g := &IdentityGraph{}
	if err := g.IngestEntraJSON("bogus", []byte("{}")); err == nil {
		t.Error("expected error")
	}
}

func TestIngestMalformed(t *testing.T) {
	g := &IdentityGraph{}
	if err := g.IngestAWSJSON([]byte("not json")); err == nil {
		t.Error("expected error")
	}
	if err := g.IngestGCPJSON([]byte("not json")); err == nil {
		t.Error("expected error")
	}
}

func TestMergeDedup(t *testing.T) {
	g1 := &IdentityGraph{}
	g1.AddNode(GraphNode{ID: "a", Type: "user", Label: "a"})
	g1.AddNode(GraphNode{ID: "a", Type: "user", Label: "a-labeled"})

	g2 := &IdentityGraph{}
	g2.AddNode(GraphNode{ID: "b"})
	g2.AddEdge(GraphEdge{Source: "a", Target: "b", Type: "member_of"})
	g2.AddEdge(GraphEdge{Source: "a", Target: "b", Type: "member_of"})

	g1.Merge(g2)
	if len(g1.Nodes) != 2 {
		t.Errorf("nodes = %d", len(g1.Nodes))
	}
	if len(g1.Edges) != 1 {
		t.Errorf("edges = %d (dedup failed)", len(g1.Edges))
	}
	// First non-empty label wins on upsert.
	if g1.Nodes[0].Label != "a" {
		t.Errorf("label = %q", g1.Nodes[0].Label)
	}
}

func TestSaveLoadGraph(t *testing.T) {
	g := buildTestGraph(t)
	path := filepath.Join(t.TempDir(), "graph.json")
	if err := g.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadGraph(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Nodes) != len(g.Nodes) || len(loaded.Edges) != len(g.Edges) {
		t.Errorf("mismatch after roundtrip")
	}
	if _, err := LoadGraph(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Error("missing file should fail")
	}
}

func TestQualifyPath(t *testing.T) {
	g := buildTestGraph(t)
	e := NewGraphEngine(g, 60)
	e.MarkTokensHeld("u1")

	run, err := e.QualifyPath([]string{"u1", "role:role-global-admin"})
	if err != nil {
		t.Fatalf("qualify: %v", err)
	}
	if len(run.Runbook) != 1 {
		t.Fatalf("runbook = %v", run.Runbook)
	}
	if !strings.Contains(run.Runbook[0], "aether exec azure") {
		t.Errorf("runbook cmd = %q", run.Runbook[0])
	}
	if run.OPSECRisk != 30 {
		t.Errorf("risk = %d", run.OPSECRisk)
	}
	if len(run.Notes) != 0 {
		t.Errorf("notes = %v (tokens held)", run.Notes)
	}
}

func TestQualifyPathNoTokens(t *testing.T) {
	g := buildTestGraph(t)
	e := NewGraphEngine(g, 60)

	run, err := e.QualifyPath([]string{"u1", "role:role-global-admin"})
	if err != nil {
		t.Fatalf("qualify: %v", err)
	}
	if len(run.Notes) == 0 {
		t.Error("expected no-token note")
	}
}

func TestQualifyPathUnknown(t *testing.T) {
	g := buildTestGraph(t)
	e := NewGraphEngine(g, 60)

	if _, err := e.QualifyPath([]string{"ghost"}); err == nil {
		t.Error("single node should fail")
	}
	if _, err := e.QualifyPath([]string{"u1", "ghost"}); err == nil {
		t.Error("unknown node should fail")
	}
	// u2 has no outgoing edge to sp1
	if _, err := e.QualifyPath([]string{"u2", "sp1"}); err == nil {
		t.Error("missing edge should fail")
	}
}

func TestShortestPaths(t *testing.T) {
	g := buildTestGraph(t)
	e := NewGraphEngine(g, 60)

	// deploy -> EC2-Admin (can_assume)
	paths := e.ShortestPaths("arn:aws:iam::1:user/deploy", "arn:aws:iam::1:role/EC2-Admin", 3)
	if len(paths) == 0 {
		t.Fatal("no paths found")
	}
	if len(paths[0]) != 2 {
		t.Errorf("path len = %d", len(paths[0]))
	}
}

func TestRenderRunbook(t *testing.T) {
	g := buildTestGraph(t)
	e := NewGraphEngine(g, 60)
	e.MarkTokensHeld("u1")

	run, err := e.QualifyPath([]string{"u1", "role:role-global-admin"})
	if err != nil {
		t.Fatal(err)
	}
	out := RenderRunbook(run)
	if !strings.Contains(out, "Executable Runbook") || !strings.Contains(out, "aether exec azure") {
		t.Errorf("rendered = %q", out)
	}
}

func TestStats(t *testing.T) {
	g := buildTestGraph(t)
	stats := g.Stats()
	if !strings.Contains(stats, "entra") || !strings.Contains(stats, "aws") || !strings.Contains(stats, "gcp") {
		t.Errorf("stats = %q", stats)
	}
	_ = os.Getpid
}
