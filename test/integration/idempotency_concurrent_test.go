//go:build integration
// +build integration

package integration

import (
	"context"
	"sync"
	"testing"

	"github.com/Debajyoti0-0/aether/internal/engine/spine"
)

func TestIdempotencyConcurrentSimple(t *testing.T) {
	// Disable parallel to isolate this test
	// t.Parallel()
	ws := newWorkspace(t, "idempotency-concurrent-simple")

	const numConcurrent = 10
	actionID := "concurrent-test-123"
	executed := 0
	var execMu sync.Mutex

	mutation := &countingMutation{
		kind:   "test.concurrent",
		target: "target-concurrent",
		execFn: func(ctx context.Context) (string, error) {
			execMu.Lock()
			executed++
			execMu.Unlock()
			return "op-ok", nil
		},
		undoSpec: &spine.UndoSpec{Irreversible: true},
	}

	results := make(chan *spine.Result, numConcurrent)
	errors := make(chan error, numConcurrent)

	var wg sync.WaitGroup
	for i := 0; i < numConcurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			action := &spine.Action{
				ID:           actionID,
				Kind:         "test.concurrent",
				Target:       "target-concurrent",
				Actor:        "test",
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

	successCount := 0
	errorCount := 0
	for err := range errors {
		if err != nil {
			errorCount++
		}
	}
	for range results {
		successCount++
	}

	t.Logf("Successful executions: %d, Errors: %d, Total executed: %d", successCount, errorCount, executed)

	if successCount != 1 {
		t.Fatalf("expected 1 success, got %d", successCount)
	}
	if errorCount != 9 {
		t.Fatalf("expected 9 errors, got %d", errorCount)
	}
	if executed != 1 {
		t.Fatalf("expected 1 execution, got %d", executed)
	}
}