// Command stage52-seed writes a signed audit chain and an identity graph for the
// Stage 52 evidence run.
//
// It is evidence tooling, not a product command. The dashboard reads a plain
// chain-plus-key pair (internal/web.Config), and no shipped CLI subcommand
// appends to one: `aether audit record` writes into an encrypted workspace
// vault, and `aether export audit` emits the chain without the signing key. So
// the chain the dashboard is pointed at is produced here with the same
// internal/store the product itself signs with, and the evidence records that
// fact rather than implying a shipped command produced it.
package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	engine "github.com/Debajyoti0-0/aether/internal/engine/graph"
	"github.com/Debajyoti0-0/aether/internal/store"
)

const (
	opAddNode = "graph.addNode"
	opAddEdge = "graph.addEdge"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: stage52-seed <dir>")
		os.Exit(2)
	}
	dir := os.Args[1]
	if err := os.MkdirAll(dir, 0o700); err != nil {
		fail(err)
	}

	l, err := store.New(filepath.Join(dir, "aether-audit.jsonl"), filepath.Join(dir, "aether-audit.key"))
	if err != nil {
		fail(err)
	}

	// A small but non-trivial graph: two privilege tiers and a service account
	// that reaches both, so choke-point analysis has something to find and time
	// travel has more than one distinct state to move between.
	type node struct{ id, label, kind, provider string }
	nodes := []node{
		{"U-1", "alice", "user", "entra"},
		{"U-2", "bob", "user", "entra"},
		{"G-1", "Tier1", "group", "entra"},
		{"G-2", "Domain Admins", "group", "entra"},
		{"S-1", "svc-backup", "servicePrincipal", "entra"},
	}
	type edge struct {
		from, to, kind string
		weight         int
	}
	edges := []edge{
		{"U-1", "G-1", "memberOf", 10},
		{"U-2", "G-1", "memberOf", 10},
		{"S-1", "G-1", "memberOf", 10},
		{"G-1", "G-2", "adminOf", 60},
		{"S-1", "G-2", "adminOf", 80},
	}

	type step struct{ command, result string }
	steps := []step{{"workspace.create", `{"workspace":"stage52-evidence"}`}}
	for _, n := range nodes {
		steps = append(steps, step{opAddNode,
			fmt.Sprintf(`{"id":%q,"label":%q,"type":%q,"provider":%q}`, n.id, n.label, n.kind, n.provider)})
	}
	for _, e := range edges {
		steps = append(steps, step{opAddEdge,
			fmt.Sprintf(`{"source":%q,"target":%q,"type":%q,"weight":%d}`, e.from, e.to, e.kind, e.weight)})
	}
	for _, s := range steps {
		if _, err := l.Append(s.command, s.result); err != nil {
			fail(err)
		}
	}

	g := &engine.IdentityGraph{}
	for _, n := range nodes {
		g.Nodes = append(g.Nodes, engine.GraphNode{ID: n.id, Label: n.label, Type: n.kind, Provider: n.provider})
	}
	for _, e := range edges {
		g.Edges = append(g.Edges, engine.GraphEdge{Source: e.from, Target: e.to, Type: e.kind, Weight: e.weight})
	}
	if err := g.Save(filepath.Join(dir, "aether-graph.json")); err != nil {
		fail(err)
	}

	res, err := l.Verify()
	if err != nil {
		fail(err)
	}
	fmt.Printf("chain entries: %d, valid: %d, valid_all: %v\n", res.Total, res.Valid, res.ValidAll)
	fmt.Printf("graph: %d nodes, %d edges\n", len(g.Nodes), len(g.Edges))
	fmt.Printf("public key fingerprint: %s\n", fingerprint(l.PublicKey()))
}

func fingerprint(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	hexed := hex.EncodeToString(sum[:])
	var groups []string
	for i := 0; i < 4; i++ {
		groups = append(groups, hexed[i*8:(i+1)*8])
	}
	return strings.Join(groups, "-")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "stage52-seed:", err)
	os.Exit(1)
}
