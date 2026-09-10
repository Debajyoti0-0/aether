package graph

import (
	"fmt"
	"sort"
	"strings"
)

// CorrelatedPath is one cross-provider attack path discovered by
// Correlate: its start and end nodes live in different clouds.
type CorrelatedPath struct {
	FromProvider string   `json:"from_provider"`
	ToProvider   string   `json:"to_provider"`
	Path         []string `json:"path"`     // node IDs
	Labels       []string `json:"labels"`   // node labels
	Runbook      []string `json:"runbook"`  // aether commands
	OPSECRisk    int      `json:"opsec_risk"`
}

// Correlate discovers cross-cloud attack paths: it walks the identity
// graph for every (providerA, providerB) pair and qualifies the
// shortest path between them as an executable runbook.
func (e *GraphEngine) Correlate(maxPaths int) ([]CorrelatedPath, error) {
	if maxPaths <= 0 {
		maxPaths = 3
	}

	// Group node IDs by provider.
	byProvider := map[string][]string{}
	for _, n := range e.Graph.Nodes {
		byProvider[n.Provider] = append(byProvider[n.Provider], n.ID)
	}

	providers := make([]string, 0, len(byProvider))
	for p := range byProvider {
		providers = append(providers, p)
	}
	sort.Strings(providers)

	var out []CorrelatedPath

	for i := 0; i < len(providers); i++ {
		for j := 0; j < len(providers); j++ {
			if i == j {
				continue
			}
			a, b := providers[i], providers[j]

			// Try each seed pair in this direction, bounded by maxPaths.
			found := 0
			for _, src := range byProvider[a] {
				if found >= maxPaths {
					break
				}
				for _, dst := range byProvider[b] {
					if found >= maxPaths {
						break
					}
					paths := e.ShortestPaths(src, dst, maxPaths-found)
					for _, p := range paths {
						if !crossesProviders(e.Graph, p) {
							continue
						}
						run, err := e.QualifyPath(p)
						if err != nil {
							continue
						}
						out = append(out, CorrelatedPath{
							FromProvider: a,
							ToProvider:   b,
							Path:         p,
							Labels:       run.Path,
							Runbook:      run.Runbook,
							OPSECRisk:    run.OPSECRisk,
						})
						found++
						if found >= maxPaths {
							break
						}
					}
				}
			}
		}
	}

	// Deterministic output.
	sort.Slice(out, func(x, y int) bool {
		return fmt.Sprint(out[x].FromProvider, out[x].ToProvider, out[x].Path) <
			fmt.Sprint(out[y].FromProvider, out[y].ToProvider, out[y].Path)
	})
	return out, nil
}

// crossesProviders reports whether a node-ID path spans >1 provider.
func crossesProviders(g *IdentityGraph, path []string) bool {
	providers := map[string]bool{}
	for _, id := range path {
		for i := range g.Nodes {
			if g.Nodes[i].ID == id {
				providers[g.Nodes[i].Provider] = true
				break
			}
		}
	}
	return len(providers) > 1
}

// RenderCorrelated renders correlated paths for the terminal.
func RenderCorrelated(paths []CorrelatedPath) string {
	if len(paths) == 0 {
		return "No cross-provider paths found in graph.\n"
	}
	var b strings.Builder
	b.WriteString("=== Cross-Provider Attack Paths ===\n\n")
	for i, p := range paths {
		fmt.Fprintf(&b, "%d. %s → %s (risk %d/100)\n", i+1, p.FromProvider, p.ToProvider, p.OPSECRisk)
		fmt.Fprintf(&b, "   %s\n", strings.Join(p.Labels, " -> "))
		for _, cmd := range p.Runbook {
			fmt.Fprintf(&b, "   $ %s\n", cmd)
		}
		b.WriteString("\n")
	}
	return b.String()
}
