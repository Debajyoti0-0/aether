package exec

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/Debajyoti0-0/aether/internal/types"
)

// ParallelExecutor fans a command out over many targets with a bounded
// worker pool, collecting per-target results.
type ParallelExecutor struct {
	// Run executes one target. Injected by the CLI (azure/aws/github runner).
	Run func(ctx context.Context, target, command string) (*types.CommandResult, error)
	// Concurrency caps the worker pool (default 5).
	Concurrency int
}

// TargetResult is one fan-out outcome.
type TargetResult struct {
	Target string               `json:"target"`
	OK     bool                 `json:"ok"`
	Result *types.CommandResult `json:"result,omitempty"`
	Error  string               `json:"error,omitempty"`
}

// LoadTargets reads a target list from a file (one per line, '#'
// comments allowed) or falls back to the comma-separated inline list.
func LoadTargets(file, inline string) ([]string, error) {
	var targets []string
	if file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read targets: %w", err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			targets = append(targets, line)
		}
	} else if inline != "" {
		for _, t := range strings.Split(inline, ",") {
			if t = strings.TrimSpace(t); t != "" {
				targets = append(targets, t)
			}
		}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("no targets (use --targets-file or --targets)")
	}
	return targets, nil
}

// Run executes the command against every target in parallel with a
// bounded worker pool. It always returns the full result set; failures
// are per-target, and results keep input order.
func (p *ParallelExecutor) RunAll(ctx context.Context, targets []string, command string) []TargetResult {
	concurrency := p.Concurrency
	if concurrency < 1 {
		concurrency = 5
	}
	if concurrency > len(targets) {
		concurrency = len(targets)
	}

	type indexed struct {
		idx int
		res TargetResult
	}

	jobs := make(chan int)
	collected := make(chan indexed, len(targets))

	var wg sync.WaitGroup
	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				target := targets[idx]
				res, err := p.Run(ctx, target, command)
				tr := TargetResult{Target: target}
				if err != nil {
					tr.Error = err.Error()
				} else {
					tr.OK = true
					tr.Result = res
				}
				collected <- indexed{idx: idx, res: tr}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for i := range targets {
			select {
			case jobs <- i:
			case <-ctx.Done():
				return
			}
		}
	}()
	wg.Wait()
	close(collected)

	results := make([]TargetResult, len(targets))
	for kv := range collected {
		results[kv.idx] = kv.res
	}
	return results
}

// RenderResults formats fan-out results for the terminal.
func RenderResults(rs []TargetResult) string {
	var b strings.Builder
	okCount, failCount := 0, 0
	for _, r := range rs {
		mark := "OK  "
		if !r.OK {
			mark = "FAIL"
			failCount++
		} else {
			okCount++
		}
		fmt.Fprintf(&b, "[%s] %s", mark, r.Target)
		if r.Error != "" {
			fmt.Fprintf(&b, " — %s", r.Error)
		} else if r.Result != nil {
			fmt.Fprintf(&b, " — %s (exit %d)", strings.ReplaceAll(r.Result.Output, "\n", "; "), r.Result.ExitCode)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "\n%d succeeded, %d failed of %d\n", okCount, failCount, len(rs))
	return b.String()
}
