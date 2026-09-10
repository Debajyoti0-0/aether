package orchestrator

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

func TestValidateOrdersAndDetectsCycles(t *testing.T) {
	w := NewWorkflow()
	w.Add(&Node{ID: "a", Dependencies: []string{"b"}})
	w.Add(&Node{ID: "b", Dependencies: []string{"a"}})

	if _, err := w.Validate(); err == nil {
		t.Fatal("cycle not detected")
	}

	w = NewWorkflow()
	w.Add(&Node{ID: "convert", Dependencies: []string{"ghost"}})
	if _, err := w.Validate(); err == nil {
		t.Fatal("unknown dependency not detected")
	}

	w = NewWorkflow()
	w.Add(&Node{ID: "a"})
	w.Add(&Node{ID: "b", Dependencies: []string{"a"}})
	order, err := w.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(order) != 2 || order[0] != "a" {
		t.Errorf("order = %v", order)
	}
}

func TestExecuteLinearChain(t *testing.T) {
	var order []string
	w := NewWorkflow()
	w.Add(&Node{ID: "prt", Action: func(ctx context.Context, r *StepRecorder) error {
		order = append(order, "prt")
		return nil
	}})
	w.Add(&Node{ID: "cap", Dependencies: []string{"prt"}, Action: func(ctx context.Context, r *StepRecorder) error {
		order = append(order, "cap")
		return nil
	}})
	w.Add(&Node{ID: "exec", Dependencies: []string{"cap"}, Action: func(ctx context.Context, r *StepRecorder) error {
		order = append(order, "exec")
		return nil
	}})

	if err := w.Execute(context.Background(), &StepRecorder{}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(order) != 3 || order[0] != "prt" || order[2] != "exec" {
		t.Errorf("order = %v", order)
	}
}

func TestExecuteFallbackOnFailure(t *testing.T) {
	fallbackRan := false

	w := NewWorkflow()
	w.Add(&Node{ID: "prt", Retries: 1, Action: func(ctx context.Context, r *StepRecorder) error {
		return errors.New("token protection enforced")
	}, Fallback: "refresh"})
	w.Add(&Node{ID: "refresh", Action: func(ctx context.Context, r *StepRecorder) error {
		fallbackRan = true
		return nil
	}})
	w.Add(&Node{ID: "exec", Dependencies: []string{"prt"}, Action: func(ctx context.Context, r *StepRecorder) error {
		return nil
	}})

	rec := &StepRecorder{}
	if err := w.Execute(context.Background(), rec); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !fallbackRan {
		t.Fatal("fallback did not run")
	}

	steps := rec.Steps()
	var found bool
	for _, s := range steps {
		if s.NodeID == "prt" && s.Status == "fallback" {
			found = true
			if s.Attempts != 3 { // 2 primary attempts + 1 fallback
				t.Errorf("attempts = %d, want 3", s.Attempts)
			}
		}
	}
	if !found {
		t.Errorf("fallback outcome missing: %+v", steps)
	}
}

func TestExecuteSkipsOnDependencyFailure(t *testing.T) {
	w := NewWorkflow()
	w.Add(&Node{ID: "bad", Action: func(ctx context.Context, r *StepRecorder) error {
		return errors.New("boom")
	}})
	w.Add(&Node{ID: "downstream", Dependencies: []string{"bad"}, Action: func(ctx context.Context, r *StepRecorder) error {
		t.Error("downstream must not run")
		return nil
	}})

	rec := &StepRecorder{}
	if err := w.Execute(context.Background(), rec); err != nil {
		t.Fatalf("execute: %v", err)
	}

	for _, s := range rec.Steps() {
		if s.NodeID == "downstream" && s.Status != "skipped" {
			t.Errorf("downstream status = %q, want skipped", s.Status)
		}
	}
}

func TestExecuteCriticalAborts(t *testing.T) {
	w := NewWorkflow()
	w.Add(&Node{ID: "core", Critical: true, Action: func(ctx context.Context, r *StepRecorder) error {
		return errors.New("fatal")
	}})
	w.Add(&Node{ID: "other", Action: func(ctx context.Context, r *StepRecorder) error {
		return nil
	}})

	if err := w.Execute(context.Background(), &StepRecorder{}); err == nil {
		t.Fatal("critical failure should abort workflow")
	}
}

func TestExecuteRetries(t *testing.T) {
	var calls int32
	w := NewWorkflow()
	w.Add(&Node{ID: "flaky", Retries: 2, Action: func(ctx context.Context, r *StepRecorder) error {
		if atomic.AddInt32(&calls, 1) < 3 {
			return errors.New("transient")
		}
		return nil
	}})

	rec := &StepRecorder{}
	if err := w.Execute(context.Background(), rec); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("calls = %d, want 3", got)
	}
	for _, s := range rec.Steps() {
		if s.NodeID == "flaky" && s.Attempts != 3 {
			t.Errorf("attempts = %d", s.Attempts)
		}
	}
}

func TestExecuteParallelRespectsLimit(t *testing.T) {
	var concurrent, peak int32

	w := NewWorkflow()
	w.MaxParallel = 2
	for _, id := range []string{"n1", "n2", "n3", "n4"} {
		node := id
		w.Add(&Node{ID: node, Action: func(ctx context.Context, r *StepRecorder) error {
			cur := atomic.AddInt32(&concurrent, 1)
			for {
				p := atomic.LoadInt32(&peak)
				if cur <= p || atomic.CompareAndSwapInt32(&peak, p, cur) {
					break
				}
			}
			for atomic.LoadInt32(&concurrent) > 0 {
				break
			}
			atomic.AddInt32(&concurrent, -1)
			return nil
		}})
		_ = node
	}

	if err := w.Execute(context.Background(), &StepRecorder{}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if peak > 2 {
		t.Errorf("peak concurrency = %d, want <= 2", peak)
	}
}

func TestFallbackCycleRejected(t *testing.T) {
	w := NewWorkflow()
	w.Add(&Node{ID: "a", Fallback: "a"})
	if _, err := w.Validate(); err == nil {
		t.Fatal("self-fallback should be rejected")
	}

	w = NewWorkflow()
	w.Add(&Node{ID: "a", Fallback: "ghost"})
	if _, err := w.Validate(); err == nil {
		t.Fatal("unknown fallback should be rejected")
	}
}

func TestPrintAttachesToLastStep(t *testing.T) {
	rec := &StepRecorder{}
	rec.Record("n1", "ok", "", 1)
	rec.Print("detail: %d", 42)

	steps := rec.Steps()
	if !fmtSprintContains(steps[0].Detail, "detail: 42") {
		t.Errorf("detail = %q", steps[0].Detail)
	}
}

func fmtSprintContains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
