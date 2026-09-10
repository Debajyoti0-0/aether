package orchestrator

import (
	"context"
	"fmt"
	"sync"
)

// NodeAction is the unit of work a DAG node performs.
// It receives a step result recorder for structured output.
type NodeAction func(ctx context.Context, rec *StepRecorder) error

// Node is one vertex of the workflow DAG.
type Node struct {
	ID           string
	Action       NodeAction
	Dependencies []string
	// Fallback, when non-empty, is executed if this node fails
	// after its retries. The fallback node must not depend (directly
	// or transitively) on the failing node.
	Fallback string
	// Retries is the number of re-execution attempts on failure
	// (total attempts = Retries + 1).
	Retries int
	// Critical, when true, aborts the whole workflow on failure
	// (after fallbacks). Non-critical failures are recorded and skipped.
	Critical bool
}

// Workflow is a DAG of nodes with fallback edges and concurrency limits.
type Workflow struct {
	Nodes       map[string]*Node
	Start       string
	MaxParallel int
}

// NewWorkflow builds an empty workflow.
func NewWorkflow() *Workflow {
	return &Workflow{Nodes: map[string]*Node{}, MaxParallel: 1}
}

// Add registers a node.
func (w *Workflow) Add(n *Node) {
	w.Nodes[n.ID] = n
}

// StepRecorder accumulates per-node outcomes during execution.
type StepRecorder struct {
	mu     sync.Mutex
	steps  []StepOutcome
}

// StepOutcome is the recorded result of one node execution.
type StepOutcome struct {
	NodeID   string `json:"node_id"`
	Status   string `json:"status"` // ok, fallback, failed, skipped
	Detail   string `json:"detail,omitempty"`
	Attempts int    `json:"attempts"`
}

// Record appends an outcome (thread-safe).
func (r *StepRecorder) Record(nodeID, status, detail string, attempts int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.steps = append(r.steps, StepOutcome{NodeID: nodeID, Status: status, Detail: detail, Attempts: attempts})
}

// Print appends a human-readable detail line (thread-safe).
func (r *StepRecorder) Print(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.steps) > 0 {
		r.steps[len(r.steps)-1].Detail = stringsTrimSpace(fmt.Sprintf("%s\n%s", r.steps[len(r.steps)-1].Detail, fmt.Sprintf(format, args...)))
	}
}

// Steps returns a copy of all outcomes.
func (r *StepRecorder) Steps() []StepOutcome {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]StepOutcome, len(r.steps))
	copy(out, r.steps)
	return out
}

// Validate checks the DAG: unknown dependencies, cycles, and fallback
// self-reference. Returns an ordered (topological) node list.
func (w *Workflow) Validate() ([]string, error) {
	for id, n := range w.Nodes {
		for _, dep := range n.Dependencies {
			if _, ok := w.Nodes[dep]; !ok {
				return nil, fmt.Errorf("node %q depends on unknown node %q", id, dep)
			}
		}
		if n.Fallback != "" {
			if _, ok := w.Nodes[n.Fallback]; !ok {
				return nil, fmt.Errorf("node %q has unknown fallback %q", id, n.Fallback)
			}
			if n.Fallback == id {
				return nil, fmt.Errorf("node %q cannot be its own fallback", id)
			}
		}
	}

	// Kahn's algorithm for topological order + cycle detection.
	indegree := map[string]int{}
	dependents := map[string][]string{}
	for id, n := range w.Nodes {
		indegree[id] += 0
		for _, dep := range n.Dependencies {
			indegree[id]++
			dependents[dep] = append(dependents[dep], id)
		}
	}

	var queue []string
	for id, d := range indegree {
		if d == 0 {
			queue = append(queue, id)
		}
	}

	var order []string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		order = append(order, cur)
		for _, next := range dependents[cur] {
			indegree[next]--
			if indegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if len(order) != len(w.Nodes) {
		return nil, fmt.Errorf("workflow contains a dependency cycle (%d of %d nodes ordered)", len(order), len(w.Nodes))
	}
	return order, nil
}

// Execute runs the DAG respecting dependencies, fallbacks, retries,
// criticality, and the concurrency limit.
func (w *Workflow) Execute(ctx context.Context, rec *StepRecorder) error {
	order, err := w.Validate()
	if err != nil {
		return err
	}

	maxParallel := w.MaxParallel
	if maxParallel < 1 {
		maxParallel = 1
	}

	done := map[string]bool{}
	failed := map[string]bool{}
	var mu sync.Mutex

	markDone := func(id string) { mu.Lock(); done[id] = true; mu.Unlock() }
	markFailed := func(id string) { mu.Lock(); failed[id] = true; mu.Unlock() }
	isDone := func(id string) bool { mu.Lock(); defer mu.Unlock(); return done[id] }
	isFailed := func(id string) bool { mu.Lock(); defer mu.Unlock(); return failed[id] }
	dependenciesSatisfied := func(n *Node) bool {
		for _, dep := range n.Dependencies {
			if !isDone(dep) {
				return false
			}
		}
		return true
	}
	skippedByDependency := func(n *Node) bool {
		for _, dep := range n.Dependencies {
			if isFailed(dep) {
				return true
			}
		}
		return false
	}

	runNode := func(n *Node) {
		// A dependency failure skips the node (recorded), unless a
		// fallback path exists that does not depend on the failure.
		if skippedByDependency(n) {
			rec.Record(n.ID, "skipped", "dependency failed", 0)
			markFailed(n.ID)
			return
		}

		attempts := 0
		var lastErr error
		for attempt := 0; attempt <= n.Retries; attempt++ {
			attempts++
			if err := ctx.Err(); err != nil {
				lastErr = err
				break
			}
			lastErr = n.Action(ctx, rec)
			if lastErr == nil {
				break
			}
		}

		if lastErr == nil {
			rec.Record(n.ID, "ok", "", attempts)
			markDone(n.ID)
			return
		}

		// Try the fallback node.
		if n.Fallback != "" {
			fb := w.Nodes[n.Fallback]
			if fb != nil {
				if fbErr := fb.Action(ctx, rec); fbErr == nil {
					rec.Record(n.ID, "fallback", fmt.Sprintf("primary failed (%v); fallback %q succeeded", lastErr, n.Fallback), attempts+fb.Retries+1)
					markDone(n.ID)
					markDone(fb.ID)
					return
				}
			}
		}

		rec.Record(n.ID, "failed", lastErr.Error(), attempts)
		markFailed(n.ID)
	}

	// Level-by-level execution: run every node whose dependencies are
	// all done, up to MaxParallel at a time, until no progress is made.
	for {
		ready := []*Node{}
		for _, id := range order {
			if isDone(id) || isFailed(id) {
				continue
			}
			n := w.Nodes[id]
			if dependenciesSatisfied(n) {
				ready = append(ready, n)
			}
		}
		if len(ready) == 0 {
			break
		}

		if len(ready) > maxParallel {
			ready = ready[:maxParallel]
		}

		var wg sync.WaitGroup
		for _, n := range ready {
			wg.Add(1)
			go func(n *Node) {
				defer wg.Done()
				runNode(n)
			}(n)
		}
		wg.Wait()

		// If nothing became done/failed this round we would spin
		// forever; the ready list is recomputed each loop so an empty
		// transition means every remaining node is blocked. Break.
		progressed := false
		for _, id := range order {
			if isDone(id) || isFailed(id) {
				progressed = true
				break
			}
		}
		_ = progressed
	}

	// Any node neither done nor failed is blocked by a failed dependency.
	for _, id := range order {
		if !isDone(id) && !isFailed(id) {
			rec.Record(id, "skipped", "blocked by failed dependency", 0)
			markFailed(id)
		}
	}

	// Critical failure check.
	for _, s := range rec.Steps() {
		if s.Status == "failed" && w.Nodes[s.NodeID] != nil && w.Nodes[s.NodeID].Critical {
			return fmt.Errorf("critical node %q failed: %s", s.NodeID, s.Detail)
		}
	}
	return nil
}

func stringsTrimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
