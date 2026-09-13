//go:build integration
// +build integration

package integration

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/Debajyoti0-0/aether/internal/engine/spine"
)

// TestIdempotencyReplaySafety verifies that duplicate requests with the same
// action ID return the cached result instead of re-executing the mutation.
func TestIdempotencyReplaySafety(t *testing.T) {
	t.Parallel()
	ws := newWorkspace(t, "idempotency-replay")

	executed := 0
	mutation := &countingMutation{
		kind:   "test.idempotent",
		target: "target-1",
		execFn: func(ctx context.Context) (string, error) {
			executed++
			return "op-" + time.Now().Format(time.RFC3339Nano), nil
		},
		undoSpec: &spine.UndoSpec{Irreversible: true},
	}

	s := spine.New(ws)

	// First execution
	action := &spine.Action{
		Kind:         mutation.kind,
		Target:       mutation.target,
		Actor:        "test",
		Mutation:     mutation,
		ApprovalMode: spine.ApprovalAuto,
	}

	result1, err := s.Run(context.Background(), action)
	if err != nil {
		t.Fatalf("first execution failed: %v", err)
	}
	if executed != 1 {
		t.Fatalf("expected 1 execution, got %d", executed)
	}

	// Second execution with same action ID (replay)
	action2 := &spine.Action{
		ID:           action.ID, // Same ID = replay
		Kind:         mutation.kind,
		Target:       mutation.target,
		Actor:        "test",
		Mutation:     mutation,
		ApprovalMode: spine.ApprovalAuto,
	}

	result2, err := s.Run(context.Background(), action2)
	if err != nil {
		t.Fatalf("replay execution failed: %v", err)
	}

	// Should return cached result, not re-execute
	if executed != 1 {
		t.Fatalf("expected 1 execution after replay, got %d", executed)
	}

	// Results should be identical
	if result1.ActionID != result2.ActionID || result1.OperationID != result2.OperationID {
		t.Fatalf("replay returned different result: %+v vs %+v", result1, result2)
	}

	// Verify idempotency record is marked completed
	data, err := ws.IdempotencyGet(action.ID)
	if err != nil {
		t.Fatalf("idempotency record not found: %v", err)
	}
	var record map[string]interface{}
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("invalid idempotency record: %v", err)
	}
	if record["status"] != "completed" {
		t.Fatalf("expected completed status, got %v", record["status"])
	}
}

// TestIdempotencyCrashRecovery verifies that pending idempotency entries
// are handled correctly after a crash (process restart).
func TestIdempotencyCrashRecovery(t *testing.T) {
	t.Parallel()
	ws := newWorkspace(t, "idempotency-crash")

	executed := 0

	// Simulate a crashed execution by manually inserting a "pending" record
	pendingRecord := map[string]interface{}{
		"request_id": "crash-action-123",
		"status":     "pending",
		"created_at": time.Now().UTC().Add(-1 * time.Hour),
		"expires_at": time.Now().UTC().Add(23 * time.Hour),
	}
	pendingData, _ := json.Marshal(pendingRecord)
	if err := ws.IdempotencyPut("crash-action-123", pendingData); err != nil {
		t.Fatalf("failed to insert pending record: %v", err)
	}

	// Now try to execute the same action - should detect pending and reject
	s := spine.New(ws)
	action := &spine.Action{
		ID:           "crash-action-123",
		Kind:         "test.crash",
		Target:       "target-crash",
		Actor:        "test",
		Mutation: &countingMutation{
			kind:   "test.crash",
			target: "target-crash",
			execFn: func(ctx context.Context) (string, error) {
				executed++
				return "op-crash", nil
			},
			undoSpec: &spine.UndoSpec{Irreversible: true},
		},
		ApprovalMode: spine.ApprovalAuto,
	}

	// Should fail because there's a pending entry
	_, err := s.Run(context.Background(), action)
	if err == nil {
		t.Fatal("expected error for pending action, got nil")
	}
	if err.Error() != `action "crash-action-123" already in progress` {
		t.Fatalf("expected 'already in progress' error, got: %v", err)
	}

	// Now simulate crash recovery by marking the pending entry as failed
	failedRecord := map[string]interface{}{
		"request_id": "crash-action-123",
		"status":     "failed",
		"created_at": time.Now().UTC().Add(-1 * time.Hour),
		"expires_at": time.Now().UTC().Add(23 * time.Hour),
	}
	failedData, _ := json.Marshal(failedRecord)
	if err := ws.IdempotencyPut("crash-action-123", failedData); err != nil {
		t.Fatalf("failed to update record: %v", err)
	}

	// Now execution should proceed (failed entry doesn't block)
	executed = 0
	_, err = s.Run(context.Background(), &spine.Action{
		ID:           "crash-action-123",
		Kind:         "test.crash",
		Target:       "target-crash",
		Actor:        "test",
		Mutation: &countingMutation{
			kind:   "test.crash",
			target: "target-crash",
			execFn: func(ctx context.Context) (string, error) {
				executed++
				return "op-crash", nil
			},
			undoSpec: &spine.UndoSpec{Irreversible: true},
		},
		ApprovalMode: spine.ApprovalAuto,
	})
	if err != nil {
		t.Fatalf("execution after failed recovery failed: %v", err)
	}
	if executed != 1 {
		t.Fatalf("expected 1 execution after failed recovery, got %d", executed)
	}

	// Verify record is now completed
	data, err := ws.IdempotencyGet("crash-action-123")
	if err != nil {
		t.Fatalf("idempotency record not found: %v", err)
	}
	var record map[string]interface{}
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("invalid record: %v", err)
	}
	if record["status"] != "completed" {
		t.Fatalf("expected completed status after recovery, got %v", record["status"])
	}
}

// TestIdempotencyAdversarialDuplicates verifies that concurrent duplicate
// requests with the same action ID are handled correctly (only one executes).
func TestIdempotencyAdversarialDuplicates(t *testing.T) {
	// t.Parallel() - disabled to isolate test
	ws := newWorkspace(t, "idempotency-adversarial")

	executed := 0
	var execMu sync.Mutex
	mutation := &countingMutation{
		kind:   "test.adversarial",
		target: "target-adv",
		execFn: func(ctx context.Context) (string, error) {
			execMu.Lock()
			executed++
			execMu.Unlock()
			time.Sleep(10 * time.Millisecond) // Simulate work
			return "op-adv", nil
		},
		undoSpec: &spine.UndoSpec{Irreversible: true},
	}

	const numConcurrent = 10
	actionID := "adversarial-dup-123"
	results := make(chan *spine.Result, numConcurrent)
	errors := make(chan error, numConcurrent)

	var wg sync.WaitGroup
	for i := 0; i < numConcurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			action := &spine.Action{
				ID:           actionID,
				Kind:         mutation.kind,
				Target:       mutation.target,
				Actor:        "adversary",
				Mutation:     mutation,
				ApprovalMode: spine.ApprovalAuto,
			}
			result, err := spine.New(ws).Run(context.Background(), action)
			if err != nil {
				errors <- err
			} else {
				results <- result
			}
		}()
	}
	wg.Wait()
	close(results)
	close(errors)

	// Only one should have executed
	if executed != 1 {
		t.Fatalf("expected exactly 1 execution under concurrency, got %d", executed)
	}

	// All results should be the same (cached)
	firstResult := <-results
	for result := range results {
		if result.ActionID != firstResult.ActionID || result.OperationID != firstResult.OperationID {
			t.Fatalf("concurrent results differ: %+v vs %+v", result, firstResult)
		}
	}

	// All errors should be "already in progress"
	for err := range errors {
		if err.Error() != `action "adversarial-dup-123" already in progress` {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// Verify final status is completed
	data, err := ws.IdempotencyGet("adversarial-dup-123")
	if err != nil {
		t.Fatalf("idempotency record not found: %v", err)
	}
	var record map[string]interface{}
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("invalid record: %v", err)
	}
	if record["status"] != "completed" {
		t.Fatalf("expected completed status, got %v", record["status"])
	}
}

// countingMutation is a test mutation that counts executions.
type countingMutation struct {
	kind     string
	target   string
	execFn   func(context.Context) (string, error)
	undoSpec *spine.UndoSpec
}

func (m *countingMutation) Kind() string   { return m.kind }
func (m *countingMutation) Target() string { return m.target }
func (m *countingMutation) BeforeState() ([]byte, error) {
	return nil, nil
}
func (m *countingMutation) Execute(ctx context.Context) (string, error) {
	return m.execFn(ctx)
}
func (m *countingMutation) AfterState() ([]byte, error) {
	return []byte(`{"result":"ok"}`), nil
}
func (m *countingMutation) UndoRecipe() *spine.UndoSpec { return m.undoSpec }