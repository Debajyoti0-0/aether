package validate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

const samplePath = `{
  "nodes": [
    {"id": "u1", "label": "User", "properties": {"name": "user@example.com"}},
    {"id": "g1", "label": "Group"},
    {"id": "c1", "label": "Computer"}
  ],
  "edges": [
    {"source": "u1", "target": "g1", "type": "MemberOf"},
    {"source": "g1", "target": "c1", "type": "AdminTo"}
  ]
}`

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "path.json")
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestLoadPath(t *testing.T) {
	p, err := LoadPath(writeTemp(t, samplePath))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(p.Nodes) != 3 || len(p.Edges) != 2 {
		t.Errorf("nodes=%d edges=%d", len(p.Nodes), len(p.Edges))
	}
}

func TestLoadPathEmpty(t *testing.T) {
	if _, err := LoadPath(writeTemp(t, `{"nodes":[],"edges":[]}`)); err == nil {
		t.Error("expected error for no edges")
	}
}

func TestValidatePath(t *testing.T) {
	p, err := LoadPath(writeTemp(t, samplePath))
	if err != nil {
		t.Fatal(err)
	}

	v := NewPathValidator("", "", nil)
	result, err := v.ValidatePath(context.Background(), p)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	if !result.IsValid {
		t.Error("path should be valid")
	}
	if len(result.Steps) != 2 {
		t.Fatalf("steps = %d", len(result.Steps))
	}
	// MemberOf=5, AdminTo=40 → average 22
	if result.OverallRisk != 22 {
		t.Errorf("overall risk = %d, want 22", result.OverallRisk)
	}
}

func TestValidatePathMissingNode(t *testing.T) {
	broken := `{"nodes":[{"id":"u1","label":"User"}],"edges":[{"source":"u1","target":"ghost","type":"AdminTo"}]}`
	p, err := LoadPath(writeTemp(t, broken))
	if err != nil {
		t.Fatal(err)
	}

	v := NewPathValidator("", "", nil)
	result, err := v.ValidatePath(context.Background(), p)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if result.IsValid {
		t.Error("path with missing node should be invalid")
	}
}
