package graph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVisualizeContainsNodesAndEdges(t *testing.T) {
	g := buildTestGraph(t)
	html, err := Visualize(g, "Test Graph")
	if err != nil {
		t.Fatalf("visualize: %v", err)
	}

	for _, want := range []string{
		"<!DOCTYPE html>",
		"alice@corp.com",
		"arn:aws:iam::1:role/EC2-Admin",
		"sa@proj.iam.gserviceaccount.com",
		`"entra"`, `"aws"`, `"gcp"`,
		"can_assume",
		"Test Graph",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("html missing %q", want)
		}
	}
}

func TestVisualizeSelfContained(t *testing.T) {
	g := buildTestGraph(t)
	html, err := Visualize(g, "")
	if err != nil {
		t.Fatalf("visualize: %v", err)
	}
	// No external resources: no http(s) script/link/font URLs, no CDN.
	for _, banned := range []string{"http://cdn", "https://cdn", "unpkg.com", "jsdelivr", "googleapis.com/js"} {
		if strings.Contains(html, banned) {
			t.Errorf("html references external resource %q", banned)
		}
	}
	if strings.Contains(html, "<script src=") {
		t.Error("html loads external scripts")
	}
}

func TestVisualizeDefaultTitle(t *testing.T) {
	g := &IdentityGraph{}
	html, err := Visualize(g, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "Aether Identity Graph") {
		t.Error("default title missing")
	}
}

func TestSaveVisualize(t *testing.T) {
	g := buildTestGraph(t)
	path := filepath.Join(t.TempDir(), "graph.html")
	if err := SaveVisualize(g, "Engagement", path); err != nil {
		t.Fatalf("save: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Engagement") {
		t.Error("saved html missing title")
	}
}
