package rollback

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Debajyoti0-0/aether/internal/store"
)

// Action is one reversible operation recorded during an engagement.
// The JSON shape is Stage-1 frozen.
type Action struct {
	ID        string            `json:"id"`
	Timestamp time.Time         `json:"timestamp"`
	Kind      string            `json:"kind"` // sp_secret, group_member, pipeline_dispatch, runcommand, ...
	Target    string            `json:"target"`
	Detail    string            `json:"detail,omitempty"`
	Undo      UndoAction        `json:"undo"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// UndoAction describes how to revert the operation.
type UndoAction struct {
	// Provider that knows how to execute the reversal.
	Provider string `json:"provider"` // entra, aws, github, shell
	// Op is the reversal primitive (e.g. remove_password, remove_member).
	Op string `json:"op"`
	// Args are the provider-specific arguments.
	Args map[string]string `json:"args,omitempty"`
	// Command is a full aether/shell command when Op is empty.
	Command string `json:"command,omitempty"`
}

// Stack is a persistent LIFO of recorded actions supporting deterministic
// undo. Stage 2: the stack lives in the workspace vault (bbolt) — Pop is
// a single atomic transaction (the Stage 1 rewrite-before-validate
// corruption window is gone), failed reversals are retained in the
// rollback_failed bucket instead of being silently discarded, and the
// workspace file lock protects cross-process access.
type Stack struct {
	mu sync.Mutex
	v  *store.Vault
}

// New builds a stack over a workspace vault.
func New(v *store.Vault) *Stack {
	return &Stack{v: v}
}

// Push records a reversible action.
func (s *Stack) Push(a Action) error {
	if a.Kind == "" {
		return fmt.Errorf("action kind is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if a.ID == "" {
		a.ID = fmt.Sprintf("act-%d", time.Now().UnixNano())
	}
	if a.Timestamp.IsZero() {
		a.Timestamp = time.Now().UTC()
	}

	data, err := json.Marshal(a)
	if err != nil {
		return err
	}
	_, err = s.v.RollbackPush(data)
	return err
}

// Pop atomically removes and returns the most recent action (LIFO).
func (s *Stack) Pop() (*Action, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.popLocked()
}

func (s *Stack) popLocked() (*Action, error) {
	data, err := s.v.RollbackPop()
	if err != nil {
		return nil, err
	}
	a := &Action{}
	if err := json.Unmarshal(data, a); err != nil {
		// Corrupt entry: retain it for inspection instead of dropping it.
		_ = s.v.RollbackRetainFailed(data)
		return nil, fmt.Errorf("corrupt rollback entry (retained in rollback_failed): %w", err)
	}
	return a, nil
}

// Peek returns the most recent action without removing it.
func (s *Stack) Peek() (*Action, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.v.RollbackPeek()
	if err != nil {
		return nil, err
	}
	a := &Action{}
	if err := json.Unmarshal(data, a); err != nil {
		return nil, err
	}
	return a, nil
}

// List returns all actions from oldest to newest.
func (s *Stack) List() ([]Action, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	raws, err := s.v.RollbackList()
	if err != nil {
		return nil, err
	}
	out := make([]Action, 0, len(raws))
	for _, raw := range raws {
		a := Action{}
		if err := json.Unmarshal(raw, &a); err != nil {
			// Fail closed on corruption rather than silently skipping.
			return nil, fmt.Errorf("corrupt rollback entry: %w", err)
		}
		out = append(out, a)
	}
	return out, nil
}

// ListFailed returns retained failed-reversal actions (newest-first).
// Unparseable retained entries are returned as raw diagnostic markers
// rather than silently skipped.
func (s *Stack) ListFailed() ([]Action, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	raws, err := s.v.RollbackListFailed()
	if err != nil {
		return nil, err
	}
	out := make([]Action, 0, len(raws))
	for _, raw := range raws {
		a := Action{}
		if err := json.Unmarshal(raw, &a); err != nil {
			// A corrupt retained entry still counts as retained history.
			out = append(out, Action{Kind: "CORRUPT", Target: "retained-unparseable", Detail: string(raw)})
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

// Len returns the stack depth.
func (s *Stack) Len() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raws, err := s.v.RollbackList()
	if err != nil {
		return 0, err
	}
	return len(raws), nil
}

// UndoRunner executes the reversal for one action. The CLI injects the
// provider dispatch; tests use fakes.
type UndoRunner func(ctx context.Context, a *Action) error

// UndoAll pops and reverses every action newest-first, recording
// per-action outcomes. Failed reversals are RETAINED in the
// rollback_failed bucket and surfaced — never silently discarded.
func (s *Stack) UndoAll(ctx context.Context, run UndoRunner) ([]UndoOutcome, error) {
	var outcomes []UndoOutcome
	for {
		if ctx.Err() != nil {
			return outcomes, ctx.Err()
		}

		s.mu.Lock()
		a, err := s.popLocked()
		if err != nil {
			s.mu.Unlock()
			break // empty
		}
		var raw []byte
		raw, _ = json.Marshal(a)
		o := UndoOutcome{ActionID: a.ID, Kind: a.Kind, Target: a.Target}
		if run == nil {
			o.Error = "no undo runner configured"
		} else if err := run(ctx, a); err != nil {
			o.Error = err.Error()
		} else {
			o.Reverted = true
		}
		s.mu.Unlock()

		if !o.Reverted {
			// Stage 2 invariant: a failed reversal is never discarded.
			_ = s.v.RollbackRetainFailed(raw)
			o.RetainedFailed = true
		}
		outcomes = append(outcomes, o)
	}
	return outcomes, nil
}

// UndoOutcome is the result of one reversal.
type UndoOutcome struct {
	ActionID       string `json:"action_id"`
	Kind           string `json:"kind"`
	Target         string `json:"target"`
	Reverted       bool   `json:"reverted"`
	Error          string `json:"error,omitempty"`
	RetainedFailed bool   `json:"retained_failed,omitempty"`
}

// RenderOutcomes formats undo outcomes for the terminal.
func RenderOutcomes(outcomes []UndoOutcome) string {
	var b strings.Builder
	ok, fail := 0, 0
	for _, o := range outcomes {
		mark := "OK  "
		if !o.Reverted {
			mark = "FAIL"
			fail++
		} else {
			ok++
		}
		retained := ""
		if o.RetainedFailed {
			retained = " [retained]"
		}
		fmt.Fprintf(&b, "[%s] %-18s %-20s %s%s\n", mark, o.Kind, o.Target, o.Error, retained)
	}
	fmt.Fprintf(&b, "\n%d reverted, %d failed of %d\n", ok, fail, len(outcomes))
	return b.String()
}
