package watch

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Debajyoti0-0/aether/internal/engine/graph"
)

func watchGraph() *graph.IdentityGraph {
	g := &graph.IdentityGraph{}
	g.AddNode(graph.GraphNode{ID: "u1", Type: "user", Provider: "entra", Label: "admin@corp.com"})
	g.AddNode(graph.GraphNode{ID: "aws-role", Type: "aws_role", Provider: "aws", Label: "EC2-Admin"})
	g.AddEdge(graph.GraphEdge{Source: "u1", Target: "aws-role", Type: "can_assume"})
	return g
}

func TestDiffGraphsDetectsNewEdges(t *testing.T) {
	prev := watchGraph()

	next := watchGraph()
	next.AddNode(graph.GraphNode{ID: "gcp-sa", Type: "gcp_sa", Provider: "gcp", Label: "sa@proj"})
	next.AddEdge(graph.GraphEdge{Source: "aws-role", Target: "gcp-sa", Type: "can_assume"})

	engine := graph.NewGraphEngine(next, 60)
	diff, err := DiffGraphs(prev, next, engine, 3)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}

	if len(diff.NewNodes) != 1 || diff.NewNodes[0].ID != "gcp-sa" {
		t.Errorf("new nodes = %+v", diff.NewNodes)
	}
	if len(diff.NewEdges) != 1 {
		t.Errorf("new edges = %+v", diff.NewEdges)
	}
	if !diff.HasNewPaths() {
		t.Fatal("no new paths detected")
	}
}

func TestDiffGraphsNoChange(t *testing.T) {
	g := watchGraph()
	engine := graph.NewGraphEngine(g, 60)

	diff, err := DiffGraphs(g, watchGraph(), engine, 3)
	if err != nil {
		t.Fatal(err)
	}
	if diff.HasNewPaths() || len(diff.NewNodes) > 0 || len(diff.NewEdges) > 0 {
		t.Errorf("identical graphs should produce empty diff: %+v", diff)
	}
}

func TestDiffGraphsNilPrevious(t *testing.T) {
	g := watchGraph()
	engine := graph.NewGraphEngine(g, 60)

	diff, err := DiffGraphs(nil, g, engine, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.NewNodes) != 2 || len(diff.NewEdges) != 1 {
		t.Errorf("first snapshot diff = %+v", diff)
	}
	// First snapshot yields no "paths" through qualification (single
	// edge qualifies fine, so paths may exist) — just ensure no error.
}

func TestDiffNilNext(t *testing.T) {
	engine := graph.NewGraphEngine(watchGraph(), 60)
	if _, err := DiffGraphs(nil, nil, engine, 3); err == nil {
		t.Error("nil next should fail")
	}
}

func TestWatcherAutopilotExecutes(t *testing.T) {
	var executed int32

	w := NewWatcher(10*time.Millisecond, 60, func(ctx context.Context) (*graph.IdentityGraph, error) {
		return watchGraph(), nil
	})
	w.Autopilot = true
	w.Execute = func(ctx context.Context, path []string) error {
		atomic.AddInt32(&executed, 1)
		return nil
	}

	var events []AutopilotEvent
	w.Notify = func(e AutopilotEvent) {
		events = append(events, e)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_ = w.Run(ctx)

	if atomic.LoadInt32(&executed) == 0 {
		t.Fatal("autopilot did not execute the detected path")
	}
	found := false
	for _, e := range events {
		if e.Detected && e.Executed && len(e.Path) == 2 {
			found = true
		}
	}
	if !found {
		t.Errorf("no executed event: %+v", events)
	}
}

func TestWatcherRiskGateBlocksExecution(t *testing.T) {
	// can_assume risk = 20; set ceiling to 10 so the gate blocks.
	w := NewWatcher(10*time.Millisecond, 10, func(ctx context.Context) (*graph.IdentityGraph, error) {
		return watchGraph(), nil
	})
	w.Autopilot = true
	w.Execute = func(ctx context.Context, path []string) error {
		t.Error("execution must be blocked by risk gate")
		return nil
	}

	var events []AutopilotEvent
	w.Notify = func(e AutopilotEvent) { events = append(events, e) }

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	_ = w.Run(ctx)

	blocked := false
	for _, e := range events {
		if e.Detected && !e.Executed && e.Reason != "" {
			blocked = true
		}
	}
	if !blocked {
		t.Errorf("risk gate did not block: %+v", events)
	}
}

func TestWatcherManualModeDetectsOnly(t *testing.T) {
	w := NewWatcher(10*time.Millisecond, 60, func(ctx context.Context) (*graph.IdentityGraph, error) {
		return watchGraph(), nil
	})
	w.Autopilot = false
	w.Execute = func(ctx context.Context, path []string) error {
		t.Error("execution must not run with autopilot disabled")
		return nil
	}

	var events []AutopilotEvent
	w.Notify = func(e AutopilotEvent) { events = append(events, e) }

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	_ = w.Run(ctx)

	detected := 0
	for _, e := range events {
		if e.Detected && !e.Executed {
			detected++
		}
	}
	if detected == 0 {
		t.Error("watch mode should still detect paths")
	}
}

func TestWatcherSurvivesPollErrors(t *testing.T) {
	calls := int32(0)
	w := NewWatcher(10*time.Millisecond, 60, func(ctx context.Context) (*graph.IdentityGraph, error) {
		if atomic.AddInt32(&calls, 1) <= 2 {
			return nil, errors.New("network flap")
		}
		return watchGraph(), nil
	})
	w.Autopilot = false

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := w.Run(ctx); err != nil {
		t.Fatalf("watcher died on poll error: %v", err)
	}
}

func TestWatcherNoPoll(t *testing.T) {
	w := NewWatcher(time.Millisecond, 60, nil)
	if err := w.Run(context.Background()); err == nil {
		t.Error("missing poller should fail fast")
	}
}
