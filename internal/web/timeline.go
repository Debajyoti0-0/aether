package web

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	engine "github.com/Debajyoti0-0/aether/internal/engine/graph"
)

// Graph operation names, as they appear in the audit chain's Command field.
//
// The audit chain format is frozen (Stage 1) and is not modified. Graph
// evolution is recorded through the existing generic Append(command, result)
// entry point, so a graph operation is an ordinary signed entry whose command
// names the operation and whose result carries the JSON payload. That is what
// makes time-travel auditable: the graph at any point in time is derived from
// signed history rather than from a mutable side file.
const (
	OpAddNode    = "graph.addNode"
	OpAddEdge    = "graph.addEdge"
	OpRemoveNode = "graph.removeNode"
	OpRemoveEdge = "graph.removeEdge"
	OpCheckpoint = "graph.checkpoint"
)

// IsGraphOp reports whether a chain command names a graph operation.
func IsGraphOp(command string) bool {
	switch command {
	case OpAddNode, OpAddEdge, OpRemoveNode, OpRemoveEdge, OpCheckpoint:
		return true
	}
	return false
}

// Timeline is the reconstructed history of the graph.
type Timeline struct {
	HeadSeq      int64             `json:"head_seq"`
	HeadHash     string            `json:"head_hash"`
	OpCount      int               `json:"op_count"`
	NodeCount    int               `json:"node_count"`
	EdgeCount    int               `json:"edge_count"`
	FirstGraphOp int64             `json:"first_graph_op_seq"`
	LastGraphOp  int64             `json:"last_graph_op_seq"`
	Errors       []TimelineProblem `json:"errors,omitempty"`
}

// TimelineProblem records an entry that could not be interpreted. Surfacing
// these is essential: silently skipping an unparseable graph op would make
// time-travel quietly wrong.
type TimelineProblem struct {
	Seq     int64  `json:"seq"`
	Command string `json:"command"`
	Problem string `json:"problem"`
}

// BuildTimeline walks the chain and reports where graph history starts, how
// many operations it contains, and whether every operation was interpretable.
func BuildTimeline(entries []AuditEntry, g *engine.IdentityGraph) Timeline {
	t := Timeline{HeadHash: GenesisHash, FirstGraphOp: -1, LastGraphOp: -1}
	if len(entries) > 0 {
		t.HeadSeq = entries[len(entries)-1].Seq
		t.HeadHash = entries[len(entries)-1].Hash
	}
	if g != nil {
		t.NodeCount = len(g.Nodes)
		t.EdgeCount = len(g.Edges)
	}
	for _, e := range entries {
		if !IsGraphOp(e.Command) {
			continue
		}
		t.OpCount++
		if t.FirstGraphOp < 0 {
			t.FirstGraphOp = e.Seq
		}
		t.LastGraphOp = e.Seq
		if !validOpPayload(e.Command, e.Result) {
			t.Errors = append(t.Errors, TimelineProblem{
				Seq: e.Seq, Command: e.Command, Problem: "payload is not valid JSON for this operation",
			})
		}
	}
	return t
}

// validOpPayload checks that an operation's result is decodable for its command.
func validOpPayload(command, result string) bool {
	switch command {
	case OpAddNode, OpRemoveNode:
		var n engine.GraphNode
		return json.Unmarshal([]byte(result), &n) == nil && n.ID != ""
	case OpAddEdge, OpRemoveEdge:
		var e engine.GraphEdge
		return json.Unmarshal([]byte(result), &e) == nil && e.Source != "" && e.Target != ""
	case OpCheckpoint:
		var g engine.IdentityGraph
		return json.Unmarshal([]byte(result), &g) == nil
	}
	return false
}

// StateAtSeq reconstructs the graph as it stood at audit chain entry seq.
//
// The base is the graph file the dashboard was started with (the discovered
// state), and the reconstruction applies graph operations recorded at or before
// seq. This is the honest arrangement: the dashboard is handed a graph and a
// chain, and it can only show history the chain actually attests to. If no
// operations exist, the result is the base graph unchanged and the caller is
// told so via AppliedOps, rather than the UI implying a history that does not
// exist.
func StateAtSeq(entries []AuditEntry, base *engine.IdentityGraph, seq int64) (*engine.IdentityGraph, int, error) {
	if base == nil {
		return nil, 0, fmt.Errorf("no base graph supplied")
	}
	// Deep copy so the caller's graph is never mutated.
	g := &engine.IdentityGraph{
		Nodes: make([]engine.GraphNode, len(base.Nodes)),
		Edges: make([]engine.GraphEdge, len(base.Edges)),
	}
	copy(g.Nodes, base.Nodes)
	copy(g.Edges, base.Edges)

	applied := 0
	for _, e := range entries {
		if e.Seq > seq {
			break
		}
		if !IsGraphOp(e.Command) {
			continue
		}
		switch e.Command {
		case OpAddNode:
			var n engine.GraphNode
			if err := json.Unmarshal([]byte(e.Result), &n); err != nil || n.ID == "" {
				continue
			}
			g.AddNode(n)
		case OpAddEdge:
			var ge engine.GraphEdge
			if err := json.Unmarshal([]byte(e.Result), &ge); err != nil || ge.Source == "" {
				continue
			}
			g.AddEdge(ge)
		case OpRemoveNode:
			var n engine.GraphNode
			if err := json.Unmarshal([]byte(e.Result), &n); err != nil || n.ID == "" {
				continue
			}
			g.RemoveNode(n.ID)
		case OpRemoveEdge:
			var ge engine.GraphEdge
			if err := json.Unmarshal([]byte(e.Result), &ge); err != nil {
				continue
			}
			g.RemoveEdge(ge.Source, ge.Target, ge.Type)
		case OpCheckpoint:
			// A checkpoint is a full snapshot. Adopting it is both faster and
			// more faithful than replaying every op from the base, because the
			// snapshot is the state the signer actually observed at that seq.
			var snap engine.IdentityGraph
			if err := json.Unmarshal([]byte(e.Result), &snap); err != nil {
				continue
			}
			g = &snap
		}
		applied++
	}
	return g, applied, nil
}

// NearestCheckpoint returns the highest checkpoint sequence at or below seq, or
// 0 if there is none. The time-travel UI uses it to tell the operator how much
// replay a given position requires, which is the difference between "instant"
// and "one second" in practice.
func NearestCheckpoint(entries []AuditEntry, seq int64) int64 {
	var best int64
	for _, e := range entries {
		if e.Seq > seq {
			break
		}
		if e.Command == OpCheckpoint {
			best = e.Seq
		}
	}
	return best
}

// GraphDiff is the differential between two graph states.
type GraphDiff struct {
	FromSeq     int64              `json:"from_seq"`
	ToSeq       int64              `json:"to_seq"`
	AddedNodes  []engine.GraphNode `json:"added_nodes"`
	RemovedNode []engine.GraphNode `json:"removed_nodes"`
	ModNodes    []NodeChange       `json:"modified_nodes"`
	AddedEdges  []engine.GraphEdge `json:"added_edges"`
	RemovedEdge []engine.GraphEdge `json:"removed_edges"`
	ModEdges    []EdgeChange       `json:"modified_edges"`
	// NewPathsToTarget and ClosedPathsToTarget compare the attack surface, not
	// just the object inventory. A diff that shows only added users hides the
	// case where a new edge created a path to Domain Admins, which is the whole
	// reason an operator looks at a diff.
	NewPathsToTarget    [][]string `json:"new_paths_to_target,omitempty"`
	ClosedPathsToTarget [][]string `json:"closed_paths_to_target,omitempty"`
	Target              string     `json:"target,omitempty"`
	PathStats           PathDelta  `json:"path_stats"`
	Note                string     `json:"note,omitempty"`
}

// NodeChange is one modified node.
type NodeChange struct {
	ID      string               `json:"id"`
	Before  map[string]string    `json:"before,omitempty"`
	After   map[string]string    `json:"after,omitempty"`
	Changes map[string][2]string `json:"changes"`
}

// EdgeChange is one modified edge. Edge identity is (source, target, type), so
// a "modification" is necessarily a removal plus an addition; it is reported
// when the same endpoints exist with a different type, which is the case an
// operator needs to see because it changes what the edge means.
type EdgeChange struct {
	Before engine.GraphEdge `json:"before"`
	After  engine.GraphEdge `json:"after"`
}

// PathDelta summarises the attack-surface change.
type PathDelta struct {
	From int `json:"from"`
	To   int `json:"to"`
}

// Diff compares two reconstructed graph states.
func Diff(from, to *engine.IdentityGraph, fromSeq, toSeq int64) *GraphDiff {
	d := &GraphDiff{
		FromSeq: fromSeq, ToSeq: toSeq,
		AddedNodes: []engine.GraphNode{}, RemovedNode: []engine.GraphNode{}, ModNodes: []NodeChange{},
		AddedEdges: []engine.GraphEdge{}, RemovedEdge: []engine.GraphEdge{}, ModEdges: []EdgeChange{},
	}

	fromNodes := make(map[string]engine.GraphNode, len(from.Nodes))
	for _, n := range from.Nodes {
		fromNodes[n.ID] = n
	}
	toNodes := make(map[string]engine.GraphNode, len(to.Nodes))
	for _, n := range to.Nodes {
		toNodes[n.ID] = n
	}

	ids := make([]string, 0, len(fromNodes)+len(toNodes))
	for id := range fromNodes {
		ids = append(ids, id)
	}
	for id := range toNodes {
		if _, ok := fromNodes[id]; !ok {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)

	for _, id := range ids {
		f, inFrom := fromNodes[id]
		t, inTo := toNodes[id]
		switch {
		case inFrom && !inTo:
			d.RemovedNode = append(d.RemovedNode, f)
		case !inFrom && inTo:
			d.AddedNodes = append(d.AddedNodes, t)
		default:
			if ch := nodeChanges(f, t); ch != nil {
				d.ModNodes = append(d.ModNodes, *ch)
			}
		}
	}

	fromEdges := make(map[string]engine.GraphEdge, len(from.Edges))
	for _, e := range from.Edges {
		fromEdges[EdgeKey(e.Source, e.Target, e.Type)] = e
	}
	toEdges := make(map[string]engine.GraphEdge, len(to.Edges))
	for _, e := range to.Edges {
		toEdges[EdgeKey(e.Source, e.Target, e.Type)] = e
	}
	ekeys := make([]string, 0, len(fromEdges)+len(toEdges))
	for k := range fromEdges {
		ekeys = append(ekeys, k)
	}
	for k := range toEdges {
		if _, ok := fromEdges[k]; !ok {
			ekeys = append(ekeys, k)
		}
	}
	sort.Strings(ekeys)
	for _, k := range ekeys {
		f, inFrom := fromEdges[k]
		t, inTo := toEdges[k]
		switch {
		case inFrom && !inTo:
			d.RemovedEdge = append(d.RemovedEdge, f)
		case !inFrom && inTo:
			d.AddedEdges = append(d.AddedEdges, t)
		default:
			if f.Weight != t.Weight {
				d.ModEdges = append(d.ModEdges, EdgeChange{Before: f, After: t})
			}
		}
	}
	return d
}

func nodeChanges(f, t engine.GraphNode) *NodeChange {
	if f.Label == t.Label && f.Type == t.Type && f.Provider == t.Provider && sameProps(f.Props, t.Props) {
		return nil
	}
	nc := NodeChange{ID: t.ID, Before: f.Props, After: t.Props, Changes: map[string][2]string{}}
	set := func(k, a, b string) {
		if a != b {
			nc.Changes[k] = [2]string{a, b}
		}
	}
	set("label", f.Label, t.Label)
	set("type", f.Type, t.Type)
	set("provider", f.Provider, t.Provider)
	keys := make([]string, 0, len(f.Props)+len(t.Props))
	seen := map[string]bool{}
	for k := range f.Props {
		keys = append(keys, k)
		seen[k] = true
	}
	for k := range t.Props {
		if !seen[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		set("prop."+k, f.Props[k], t.Props[k])
	}
	return &nc
}

func sameProps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// DiffPaths computes the attack-surface delta between two states for a target.
func DiffPaths(d *GraphDiff, from, to *engine.IdentityGraph, target string, opts ChokePointOptions) {
	if target == "" {
		d.Note = "no target supplied, so no attack-path comparison was performed"
		return
	}
	d.Target = target
	opts = opts.withDefaults()

	fromRes, errFrom := ChokePoints(from, target, opts)
	toRes, errTo := ChokePoints(to, target, opts)
	if errFrom != nil || errTo != nil {
		d.Note = fmt.Sprintf("attack-path comparison unavailable: from=%v to=%v", errFrom, errTo)
		return
	}
	d.PathStats = PathDelta{From: fromRes.TotalPaths, To: toRes.TotalPaths}

	fromSet := make(map[string]struct{}, len(fromRes.PathsToTarget))
	for _, p := range fromRes.PathsToTarget {
		fromSet[strings.Join(p, " → ")] = struct{}{}
	}
	for _, p := range toRes.PathsToTarget {
		if _, ok := fromSet[strings.Join(p, " → ")]; !ok {
			d.NewPathsToTarget = append(d.NewPathsToTarget, p)
		}
	}
	toSet := make(map[string]struct{}, len(toRes.PathsToTarget))
	for _, p := range toRes.PathsToTarget {
		toSet[strings.Join(p, " → ")] = struct{}{}
	}
	for _, p := range fromRes.PathsToTarget {
		if _, ok := toSet[strings.Join(p, " → ")]; !ok {
			d.ClosedPathsToTarget = append(d.ClosedPathsToTarget, p)
		}
	}
	if fromRes.Truncated || toRes.Truncated {
		d.Note = "path comparison used a bounded sample; see choke-point Truncated flags"
	}
}
