package web

import (
	"fmt"
	"sort"
	"strings"

	engine "github.com/Debajyoti0-0/aether/internal/engine/graph"
)

// ChokePoint is an edge that appears in a large fraction of the enumerated
// paths to a target. Coverage is the fraction of enumerated paths containing the
// edge, so it is directly interpretable: 0.87 means "87% of the paths we
// enumerated use this edge".
type ChokePoint struct {
	Source   string  `json:"source"`
	Target   string  `json:"target"`
	Type     string  `json:"type"`
	Coverage float64 `json:"coverage"`
	Paths    int     `json:"paths"`
	Weight   int     `json:"weight,omitempty"`
	// Rank is the 1-based position in the deterministic ordering.
	Rank int `json:"rank"`
}

// Key is the stable identity of the edge, used for sorting and for the client's
// highlight set.
func (c ChokePoint) Key() string { return EdgeKey(c.Source, c.Target, c.Type) }

// EdgeKey renders a canonical, unambiguous edge identifier. The separator is a
// control character so that a label containing the separator cannot forge a
// collision between two different edges.
func EdgeKey(source, target, typ string) string {
	return source + "\x1f" + target + "\x1f" + typ
}

// ChokePointResult is the complete, deterministic answer for one analysis.
type ChokePointResult struct {
	Target          string       `json:"target"`
	TargetID        string       `json:"target_id"`
	TargetMatchedBy string       `json:"target_matched_by"`
	Threshold       float64      `json:"threshold"`
	MaxHops         int          `json:"max_hops"`
	TotalPaths      int          `json:"total_paths"`
	Sources         int          `json:"sources"`
	Truncated       bool         `json:"truncated"`
	TruncationCause string       `json:"truncation_cause,omitempty"`
	EdgeCount       int          `json:"edge_count"`
	ChokePoints     []ChokePoint `json:"choke_points"`
	// PathsToTarget is a bounded sample of the enumerated paths, for display.
	// It is a sample and is labelled as such; the counts above are exhaustive.
	PathsToTarget [][]string `json:"paths_sample,omitempty"`
	Deterministic bool       `json:"deterministic"`
}

// ChokePointOptions controls the analysis. Zero values mean "use the default".
type ChokePointOptions struct {
	// Threshold is the minimum coverage for an edge to be reported.
	Threshold float64
	// MaxHops bounds path length. Bounding is mandatory: unbounded simple-path
	// enumeration is super-exponential.
	MaxHops int
	// MaxPaths is a hard budget on enumerated paths. When the budget is
	// exhausted the result is returned with Truncated set, never silently
	// resampled. Determinism is a hard requirement, so a Monte Carlo fallback
	// is deliberately not implemented.
	MaxPaths int
	// MaxSamplePaths bounds how many full paths are returned for display.
	MaxSamplePaths int
	// MaxResults bounds how many choke points are returned.
	MaxResults int
}

const (
	defaultThreshold     = 0.5
	defaultMaxHops       = 6
	defaultMaxPaths      = 200000
	defaultMaxSamplePath = 25
	defaultMaxResults    = 50
)

func (o ChokePointOptions) withDefaults() ChokePointOptions {
	if o.Threshold <= 0 || o.Threshold > 1 {
		o.Threshold = defaultThreshold
	}
	if o.MaxHops <= 0 {
		o.MaxHops = defaultMaxHops
	}
	if o.MaxPaths <= 0 {
		o.MaxPaths = defaultMaxPaths
	}
	if o.MaxSamplePaths <= 0 {
		o.MaxSamplePaths = defaultMaxSamplePath
	}
	if o.MaxResults <= 0 {
		o.MaxResults = defaultMaxResults
	}
	return o
}

// ResolveTarget finds the node an analysis should aim at.
//
// Matching is deliberately strict and ordered, and the reason is recorded on
// the result so an operator can tell which rule fired:
//
//  1. exact node ID
//  2. exact label
//  3. case-insensitive label
//  4. case-insensitive label suffix match (e.g. "DOMAIN ADMINS@AETHER.TEST")
//
// Fuzzy and substring matching is not offered. A choke-point number computed
// against the wrong target is worse than no number, and silently matching
// "Domain Admins" to some unrelated node would produce exactly that.
func ResolveTarget(g *engine.IdentityGraph, want string) (nodeID string, matchedBy string, err error) {
	want = strings.TrimSpace(want)
	if want == "" {
		return "", "", fmt.Errorf("target is required")
	}
	for _, n := range g.Nodes {
		if n.ID == want {
			return n.ID, "id", nil
		}
	}
	for _, n := range g.Nodes {
		if n.Label == want {
			return n.ID, "label", nil
		}
	}
	lower := strings.ToLower(want)
	for _, n := range g.Nodes {
		if strings.ToLower(n.Label) == lower {
			return n.ID, "label-ci", nil
		}
	}
	for _, n := range g.Nodes {
		l := strings.ToLower(n.Label)
		if strings.HasSuffix(l, "@"+lower) {
			return n.ID, "label-suffix", nil
		}
	}
	return "", "", fmt.Errorf("target %q not found: no node has that ID or label", want)
}

// ChokePoints computes, deterministically, which edges dominate the paths from
// every source to the target.
//
// Determinism is a hard requirement, so the implementation is exact enumeration
// with explicit budgets, not sampling:
//
//   - nodes are visited in sorted ID order, so the traversal order is fixed
//   - adjacency lists are sorted, so neighbour order is fixed
//   - ties in coverage break on the edge key, so the ranking is total
//   - a budget exhaustion sets Truncated rather than switching strategy
//
// The same graph and target therefore always produce byte-identical output.
func ChokePoints(g *engine.IdentityGraph, target string, opts ChokePointOptions) (ChokePointResult, error) {
	opts = opts.withDefaults()
	res := ChokePointResult{
		Target:        target,
		Threshold:     opts.Threshold,
		MaxHops:       opts.MaxHops,
		Deterministic: true,
	}

	targetID, matchedBy, err := ResolveTarget(g, target)
	if err != nil {
		return res, err
	}
	res.TargetID = targetID
	res.TargetMatchedBy = matchedBy

	adj, outDegree, inDegree := buildAdjacency(g)

	// A source is a node with no inbound edge. If the graph is a pure cycle
	// there are none, and every node is a source instead, so the analysis still
	// produces paths rather than an empty result.
	var sources []string
	for id, deg := range inDegree {
		if deg == 0 {
			sources = append(sources, id)
		}
	}
	if len(sources) == 0 {
		sources = make([]string, 0, len(g.Nodes))
		for _, n := range g.Nodes {
			sources = append(sources, n.ID)
		}
	}
	sort.Strings(sources)
	res.Sources = len(sources)

	edgeUse := make(map[string]int)
	edgeMeta := make(map[string]engine.GraphEdge)
	var sample [][]string

	visited := make(map[string]bool, len(g.Nodes))
	current := make([]string, 0, opts.MaxHops+1)
	currentEdges := make([]engine.GraphEdge, 0, opts.MaxHops)

	var walk func(nodeID string, depth int) bool
	walk = func(nodeID string, depth int) bool {
		// Budget check. Returning true means "stop the whole walk".
		if res.TotalPaths >= opts.MaxPaths {
			res.Truncated = true
			res.TruncationCause = fmt.Sprintf(
				"path budget of %d exhausted at maxHops=%d; the graph is too dense for exact enumeration. "+
					"Raise maxHops or lower the target set. No sampling was substituted, because sampling is not deterministic.",
				opts.MaxPaths, opts.MaxHops)
			return true
		}
		if nodeID == targetID && depth > 0 {
			res.TotalPaths++
			for _, e := range currentEdges {
				k := EdgeKey(e.Source, e.Target, e.Type)
				edgeUse[k]++
				edgeMeta[k] = e
			}
			if len(sample) < opts.MaxSamplePaths {
				sample = append(sample, append([]string(nil), current...))
			}
			return false
		}
		if depth >= opts.MaxHops {
			return false
		}
		visited[nodeID] = true
		current = append(current, nodeID)
		for _, e := range adj[nodeID] {
			if visited[e.Target] {
				continue // simple paths only
			}
			currentEdges = append(currentEdges, e)
			if walk(e.Target, depth+1) {
				return true
			}
			currentEdges = currentEdges[:len(currentEdges)-1]
		}
		current = current[:len(current)-1]
		delete(visited, nodeID)
		return false
	}

	for _, s := range sources {
		if s == targetID {
			continue
		}
		if outDegree[s] == 0 {
			continue
		}
		if walk(s, 0) {
			break
		}
	}

	res.EdgeCount = len(edgeUse)
	res.PathsToTarget = sample

	if res.TotalPaths == 0 {
		res.ChokePoints = []ChokePoint{}
		res.TruncationCause = "no path from any source to the target within maxHops; the coverage figures below would be undefined, so none are reported"
		return res, nil
	}

	candidates := make([]ChokePoint, 0, len(edgeUse))
	for k, count := range edgeUse {
		cov := float64(count) / float64(res.TotalPaths)
		if cov < opts.Threshold {
			continue
		}
		e := edgeMeta[k]
		candidates = append(candidates, ChokePoint{
			Source: e.Source, Target: e.Target, Type: e.Type,
			Coverage: cov, Paths: count, Weight: e.Weight,
		})
	}

	// Total order: coverage descending, then the edge key ascending. The second
	// term removes every tie, so the ranking cannot vary between runs.
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Coverage != candidates[j].Coverage {
			return candidates[i].Coverage > candidates[j].Coverage
		}
		ki, kj := candidates[i].Key(), candidates[j].Key()
		if ki != kj {
			return ki < kj
		}
		return false
	})

	if len(candidates) > opts.MaxResults {
		if res.TruncationCause == "" {
			res.TruncationCause = fmt.Sprintf(
				"showing the top %d of %d edges above the %.2f threshold; raise maxResults to see the rest",
				opts.MaxResults, len(candidates), opts.Threshold)
		}
		candidates = candidates[:opts.MaxResults]
	}
	for i := range candidates {
		candidates[i].Rank = i + 1
	}
	res.ChokePoints = candidates
	return res, nil
}

// buildAdjacency returns sorted adjacency lists plus in/out degree maps.
func buildAdjacency(g *engine.IdentityGraph) (map[string][]engine.GraphEdge, map[string]int, map[string]int) {
	adj := make(map[string][]engine.GraphEdge, len(g.Nodes))
	out := make(map[string]int, len(g.Nodes))
	in := make(map[string]int, len(g.Nodes))
	known := make(map[string]bool, len(g.Nodes))
	for _, n := range g.Nodes {
		known[n.ID] = true
		adj[n.ID] = nil
		out[n.ID] = 0
		in[n.ID] = 0
	}
	for _, e := range g.Edges {
		// An edge referencing an unknown node would create a phantom vertex and
		// make coverage figures meaningless, so it is dropped from the analysis
		// and the caller learns about it via EdgeCount being lower than
		// len(g.Edges).
		if !known[e.Source] || !known[e.Target] {
			continue
		}
		adj[e.Source] = append(adj[e.Source], e)
		out[e.Source]++
		in[e.Target]++
	}
	for id := range adj {
		list := adj[id]
		sort.Slice(list, func(i, j int) bool {
			ki, kj := EdgeKey(list[i].Source, list[i].Target, list[i].Type), EdgeKey(list[j].Source, list[j].Target, list[j].Type)
			if ki != kj {
				return ki < kj
			}
			return list[i].Weight > list[j].Weight
		})
		adj[id] = list
	}
	return adj, out, in
}
