package exec

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Debajyoti0-0/aether/internal/types"
)

func TestLoadTargetsFromFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "targets.txt")
	os.WriteFile(file, []byte("# comment\nvm-1\n\nvm-2\n  vm-3  \n"), 0o600)

	targets, err := LoadTargets(file, "")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(targets) != 3 || targets[0] != "vm-1" || targets[2] != "vm-3" {
		t.Errorf("targets = %v", targets)
	}
}

func TestLoadTargetsInline(t *testing.T) {
	targets, err := LoadTargets("", " a , b ,c ")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(targets) != 3 {
		t.Errorf("targets = %v", targets)
	}

	if _, err := LoadTargets("", ""); err == nil {
		t.Error("no targets should fail")
	}
}

func TestRunAllFanout(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]string{}

	p := &ParallelExecutor{
		Concurrency: 3,
		Run: func(ctx context.Context, target, command string) (*types.CommandResult, error) {
			mu.Lock()
			seen[target] = command
			mu.Unlock()
			return &types.CommandResult{Output: "ran on " + target, ExitCode: 0, Status: "succeeded"}, nil
		},
	}

	targets := []string{"a", "b", "c", "d", "e"}
	results := p.RunAll(context.Background(), targets, "id")

	if len(results) != 5 {
		t.Fatalf("results = %d", len(results))
	}
	// Order preserved.
	for i, r := range results {
		if r.Target != targets[i] {
			t.Errorf("results[%d].target = %q, want %q", i, r.Target, targets[i])
		}
		if !r.OK {
			t.Errorf("results[%d] failed: %s", i, r.Error)
		}
	}
	if len(seen) != 5 {
		t.Errorf("executed %d unique targets", len(seen))
	}
	if seen["c"] != "id" {
		t.Errorf("command = %q", seen["c"])
	}
}

func TestRunAllPerTargetFailure(t *testing.T) {
	p := &ParallelExecutor{
		Concurrency: 2,
		Run: func(ctx context.Context, target, command string) (*types.CommandResult, error) {
			if target == "bad" {
				return nil, errors.New("vm not found")
			}
			return &types.CommandResult{Output: "ok"}, nil
		},
	}

	results := p.RunAll(context.Background(), []string{"good1", "bad", "good2"}, "id")
	if len(results) != 3 {
		t.Fatalf("results = %d", len(results))
	}
	if results[1].OK || results[1].Error == "" {
		t.Errorf("bad target should fail: %+v", results[1])
	}
	if !results[0].OK || !results[2].OK {
		t.Error("good targets should succeed")
	}
}

func TestRunAllConcurrencyBound(t *testing.T) {
	var concurrent, peak int32
	p := &ParallelExecutor{
		Concurrency: 2,
		Run: func(ctx context.Context, target, command string) (*types.CommandResult, error) {
			cur := atomic.AddInt32(&concurrent, 1)
			for {
				old := atomic.LoadInt32(&peak)
				if cur <= old || atomic.CompareAndSwapInt32(&peak, old, cur) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			atomic.AddInt32(&concurrent, -1)
			return &types.CommandResult{}, nil
		},
	}

	targets := []string{"t1", "t2", "t3", "t4", "t5", "t6"}
	p.RunAll(context.Background(), targets, "x")
	if got := atomic.LoadInt32(&peak); got > 2 {
		t.Errorf("peak concurrency = %d, want <= 2", got)
	}
}

func TestRunAllContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p := &ParallelExecutor{
		Concurrency: 1,
		Run: func(ctx context.Context, target, command string) (*types.CommandResult, error) {
			cancel()
			return &types.CommandResult{}, nil
		},
	}
	results := p.RunAll(ctx, []string{"t1", "t2", "t3"}, "x")
	// All targets must have a result entry (skipped ones marked failed/empty).
	if len(results) != 3 {
		t.Fatalf("results = %d", len(results))
	}
}

func TestRenderResults(t *testing.T) {
	rs := []TargetResult{
		{Target: "a", OK: true, Result: &types.CommandResult{Output: "uid=0\n", ExitCode: 0}},
		{Target: "b", Error: "timeout"},
	}
	out := RenderResults(rs)
	if !strings.Contains(out, "[OK  ] a") || !strings.Contains(out, "[FAIL] b") {
		t.Errorf("render = %q", out)
	}
	if !strings.Contains(out, "1 succeeded, 1 failed of 2") {
		t.Errorf("summary missing: %q", out)
	}
	_ = fmt.Sprint
}
