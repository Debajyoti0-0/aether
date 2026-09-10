package graph

import (
	"fmt"
	"sort"
	"strings"
)

// CloudChain is an attack path that hops through 3+ distinct providers
// (e.g., entra → aws → gcp), discovered by SynthesizeChains.
type CloudChain struct {
	Providers []string `json:"providers"` // ordered, unique
	Path      []string `json:"path"`      // node IDs
	Labels    []string `json:"labels"`
	Runbook   []string `json:"runbook"`
	OPSECRisk int      `json:"opsec_risk"`
}

// SynthesizeChains finds attack paths that traverse at least minHops+1
// distinct providers (minHops = minimum provider transitions). It uses
// a provider-aware DFS that is cycle-safe and bounded by maxChains.
func (e *GraphEngine) SynthesizeChains(minHops, maxChains int) ([]CloudChain, error) {
	if minHops < 1 {
		minHops = 2
	}
	if maxChains <= 0 {
		maxChains = 10
	}

	// Group nodes by provider.
	byProvider := map[string][]string{}
	for _, n := range e.Graph.Nodes {
		byProvider[n.Provider] = append(byProvider[n.Provider], n.ID)
	}
	providers := make([]string, 0, len(byProvider))
	for p := range byProvider {
		providers = append(providers, p)
	}
	sort.Strings(providers)
	if len(providers) < minHops+1 {
		return nil, fmt.Errorf("graph has %d providers; need at least %d for a %d-hop chain",
			len(providers), minHops+1, minHops)
	}

	// Build adjacency with provider lookup.
	providerOf := map[string]string{}
	for _, n := range e.Graph.Nodes {
		providerOf[n.ID] = n.Provider
	}
	adjacency := map[string][]string{}
	for _, edge := range e.Graph.Edges {
		adjacency[edge.Source] = append(adjacency[edge.Source], edge.Target)
	}

	var chains []CloudChain

	// DFS from every node in every provider: a chain may start in any
	// cloud (e.g., entra → aws → gcp starts in entra, not "aws").
	for _, p := range providers {
		for _, start := range byProvider[p] {
			if len(chains) >= maxChains {
				break
			}
			visited := map[string]bool{start: true}
			chain := []string{start}
			seenProviders := map[string]bool{providerOf[start]: true}

			e.dfsChains(start, chain, visited, seenProviders, adjacency, providerOf,
				minHops, maxChains, &chains)
		}
	}

	// Deterministic ordering.
	sort.Slice(chains, func(i, j int) bool {
		return strings.Join(chains[i].Path, ">") < strings.Join(chains[j].Path, ">")
	})
	return chains, nil
}

// dfsChains recursively extends the current chain, emitting it when the
// provider diversity requirement is met.
func (e *GraphEngine) dfsChains(
	node string,
	chain []string,
	visited map[string]bool,
	seenProviders map[string]bool,
	adjacency map[string][]string,
	providerOf map[string]string,
	minHops, maxChains int,
	out *[]CloudChain,
) {
	if len(*out) >= maxChains {
		return
	}

	// Emit when diversity is satisfied.
	if len(seenProviders) >= minHops+1 {
		if run, err := e.QualifyPath(chain); err == nil {
			providersOrdered := orderedProviders(chain, providerOf)
			*out = append(*out, CloudChain{
				Providers: providersOrdered,
				Path:      append([]string{}, chain...),
				Labels:    run.Path,
				Runbook:   run.Runbook,
				OPSECRisk: run.OPSECRisk,
			})
			if len(*out) >= maxChains {
				return
			}
		}
		// Continue extending anyway — longer chains may also be valid.
	}

	for _, next := range adjacency[node] {
		if visited[next] {
			continue // cycle-safe
		}
		visited[next] = true
		chain = append(chain, next)

		newProvider := providerOf[next]
		_, had := seenProviders[newProvider]
		if !had {
			seenProviders[newProvider] = true
		}

		e.dfsChains(next, chain, visited, seenProviders, adjacency, providerOf,
			minHops, maxChains, out)

		// Backtrack.
		chain = chain[:len(chain)-1]
		if !had {
			delete(seenProviders, newProvider)
		}
		visited[next] = false

		if len(*out) >= maxChains {
			return
		}
	}
}

// orderedProviders returns the unique providers in path order.
func orderedProviders(path []string, providerOf map[string]string) []string {
	var out []string
	seen := map[string]bool{}
	for _, id := range path {
		p := providerOf[id]
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// RenderChains renders cloud chains for the terminal.
func RenderChains(chains []CloudChain) string {
	if len(chains) == 0 {
		return "No multi-provider chains found in graph.\n"
	}
	var b strings.Builder
	b.WriteString("=== Cross-Cloud Chains ===\n\n")
	for i, c := range chains {
		fmt.Fprintf(&b, "%d. %s (risk %d/100)\n", i+1, strings.Join(c.Providers, " → "), c.OPSECRisk)
		fmt.Fprintf(&b, "   %s\n", strings.Join(c.Labels, " -> "))
		for _, cmd := range c.Runbook {
			fmt.Fprintf(&b, "   $ %s\n", cmd)
		}
		b.WriteString("\n")
	}
	return b.String()
}
