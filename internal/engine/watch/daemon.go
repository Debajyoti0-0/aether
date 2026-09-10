package watch

import (
	"context"
	"fmt"
	"time"

	"github.com/Debajyoti0-0/aether/internal/engine/graph"
)

// GraphDiff is the delta between two graph snapshots.
type GraphDiff struct {
	NewNodes   []graph.GraphNode `json:"new_nodes,omitempty"`
	NewEdges   []graph.GraphEdge `json:"new_edges,omitempty"`
	NewPaths   [][]string        `json:"new_paths,omitempty"`
}

// HasNewPaths reports whether the diff exposes newly exploitable paths.
func (d *GraphDiff) HasNewPaths() bool {
	return len(d.NewPaths) > 0
}

// DiffGraphs compares two graph snapshots and produces a delta plus
// newly-qualified cross-provider paths through the new edges.
func DiffGraphs(prev, next *graph.IdentityGraph, engine *graph.GraphEngine, maxPaths int) (*GraphDiff, error) {
	if next == nil {
		return nil, fmt.Errorf("next snapshot is nil")
	}
	if maxPaths <= 0 {
		maxPaths = 3
	}

	diff := &GraphDiff{}

	// New nodes: present in next, absent in prev.
	prevNodes := map[string]bool{}
	if prev != nil {
		for _, n := range prev.Nodes {
			prevNodes[n.ID] = true
		}
	}
	prevEdges := map[string]bool{}
	if prev != nil {
		for _, e := range prev.Edges {
			prevEdges[e.Source+"|"+e.Target+"|"+e.Type] = true
		}
	}

	for _, n := range next.Nodes {
		if !prevNodes[n.ID] {
			diff.NewNodes = append(diff.NewNodes, n)
		}
	}
	for _, e := range next.Edges {
		if !prevEdges[e.Source+"|"+e.Target+"|"+e.Type] {
			diff.NewEdges = append(diff.NewEdges, e)
		}
	}

	if len(diff.NewEdges) == 0 {
		return diff, nil
	}

	// Qualify paths through each new edge (src -> dst).
	for _, e := range diff.NewEdges {
		if len(diff.NewPaths) >= maxPaths {
			break
		}
		paths := engine.ShortestPaths(e.Source, e.Target, 1)
		for _, p := range paths {
			if len(p) < 2 {
				continue
			}
			if _, err := engine.QualifyPath(p); err == nil {
				diff.NewPaths = append(diff.NewPaths, p)
			}
		}
	}
	return diff, nil
}

// Poller is the environment-refresh function injected by the CLI:
// it fetches a fresh graph snapshot from live providers.
type Poller func(ctx context.Context) (*graph.IdentityGraph, error)

// Notifier receives autopilot events (teamserver publish, webhook, log).
type Notifier func(event AutopilotEvent)

// AutopilotEvent is emitted when a new path is detected and (optionally)
// executed automatically.
type AutopilotEvent struct {
	Time     time.Time `json:"time"`
	Detected bool      `json:"detected"`
	Executed bool      `json:"executed"`
	Path     []string  `json:"path"`
	Labels   []string  `json:"labels,omitempty"`
	Risk     int       `json:"risk"`
	Reason   string    `json:"reason,omitempty"`
}

// PathExecutor executes one qualified path (injected runbook runner).
type PathExecutor func(ctx context.Context, path []string) error

// Watcher is the continuous-monitoring daemon: polls the environment,
// diffs graph snapshots, and optionally auto-executes new paths whose
// risk stays under the operator ceiling.
type Watcher struct {
	Interval   time.Duration
	MaxRisk    int
	Autopilot  bool
	MaxPaths   int
	Poll       Poller
	Notify     Notifier
	Execute    PathExecutor
}

// NewWatcher builds a watcher with defaults.
func NewWatcher(interval time.Duration, maxRisk int, poll Poller) *Watcher {
	return &Watcher{
		Interval: interval,
		MaxRisk:  maxRisk,
		MaxPaths: 3,
		Poll:     poll,
	}
}

// Run drives the polling loop until ctx is cancelled.
func (w *Watcher) Run(ctx context.Context) error {
	if w.Interval <= 0 {
		w.Interval = 5 * time.Minute
	}
	if w.Poll == nil {
		return fmt.Errorf("no poll function configured")
	}

	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()

	var prev *graph.IdentityGraph

	// Initial snapshot immediately; transient failures are tolerated
	// (the next tick retries).
	if err := w.tick(ctx, &prev); err != nil && w.Notify != nil {
		w.Notify(AutopilotEvent{Time: time.Now().UTC(), Reason: "initial poll error: " + err.Error()})
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := w.tick(ctx, &prev); err != nil {
				// Transient poll failures don't kill the daemon.
				if w.Notify != nil {
					w.Notify(AutopilotEvent{Time: time.Now().UTC(), Reason: "poll error: " + err.Error()})
				}
			}
		}
	}
}

func (w *Watcher) tick(ctx context.Context, prevPtr **graph.IdentityGraph) error {
	next, err := w.Poll(ctx)
	if err != nil {
		return err
	}

	engine := graph.NewGraphEngine(next, w.MaxRisk)
	diff, err := DiffGraphs(*prevPtr, next, engine, w.MaxPaths)
	if err != nil {
		return err
	}
	*prevPtr = next

	if !diff.HasNewPaths() {
		return nil
	}

	for _, p := range diff.NewPaths {
		run, err := engine.QualifyPath(p)
		if err != nil {
			continue
		}

		event := AutopilotEvent{
			Time:     time.Now().UTC(),
			Detected: true,
			Path:     p,
			Labels:   run.Path,
			Risk:     run.OPSECRisk,
		}

		switch {
		case run.OPSECRisk > w.MaxRisk:
			event.Reason = fmt.Sprintf("risk %d exceeds ceiling %d — not executed", run.OPSECRisk, w.MaxRisk)
		case !w.Autopilot:
			event.Reason = "autopilot disabled — manual execution recommended"
		default:
			if w.Execute != nil {
				event.Executed = true
				if err := w.Execute(ctx, p); err != nil {
					event.Executed = false
					event.Reason = "execution failed: " + err.Error()
				}
			} else {
				event.Reason = "no executor configured"
			}
		}

		if w.Notify != nil {
			w.Notify(event)
		}
	}
	return nil
}
