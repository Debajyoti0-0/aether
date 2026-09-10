package rollback

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Action is one reversible operation recorded during an engagement.
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
// undo. The stack is stored as a JSONL file (one action per line) inside
// the workspace so it survives process restarts.
type Stack struct {
	mu   sync.Mutex
	path string
}

// New builds a rollback stack backed by path.
func New(path string) *Stack {
	return &Stack{path: path}
}

// Push records a reversible action.
func (s *Stack) Push(a Action) error {
	if a.Kind == "" {
		return fmt.Errorf("action kind is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
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
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}

// Pop removes and returns the most recent action (LIFO).
func (s *Stack) Pop() (*Action, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	lines, err := s.readLines()
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("rollback stack is empty")
	}

	last := lines[len(lines)-1]
	if err := os.WriteFile(s.path, []byte(strings.Join(lines[:len(lines)-1], "\n")+"\n"), 0o600); err != nil {
		return nil, err
	}

	a := &Action{}
	if err := json.Unmarshal([]byte(last), a); err != nil {
		return nil, fmt.Errorf("corrupt rollback entry: %w", err)
	}
	return a, nil
}

// Peek returns the most recent action without removing it.
func (s *Stack) Peek() (*Action, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	lines, err := s.readLines()
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("rollback stack is empty")
	}
	a := &Action{}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), a); err != nil {
		return nil, err
	}
	return a, nil
}

// List returns all actions from oldest to newest.
func (s *Stack) List() ([]Action, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	lines, err := s.readLines()
	if err != nil {
		return nil, err
	}
	out := make([]Action, 0, len(lines))
	for _, l := range lines {
		a := Action{}
		if err := json.Unmarshal([]byte(l), &a); err != nil {
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
	lines, err := s.readLines()
	if err != nil {
		return 0, err
	}
	return len(lines), nil
}

// UndoRunner executes the reversal for one action. The CLI injects the
// provider dispatch; tests use fakes.
type UndoRunner func(ctx context.Context, a *Action) error

// UndoAll pops and reverses every action newest-first, recording
// per-action outcomes. Failed reversals do not abort the run.
func (s *Stack) UndoAll(ctx context.Context, run UndoRunner) ([]UndoOutcome, error) {
	var outcomes []UndoOutcome
	for {
		if ctx.Err() != nil {
			return outcomes, ctx.Err()
		}
		a, err := s.Pop()
		if err != nil {
			break // empty
		}
		o := UndoOutcome{ActionID: a.ID, Kind: a.Kind, Target: a.Target}
		if run == nil {
			o.Error = "no undo runner configured"
		} else if err := run(ctx, a); err != nil {
			o.Error = err.Error()
		} else {
			o.Reverted = true
		}
		outcomes = append(outcomes, o)
	}
	return outcomes, nil
}

// UndoOutcome is the result of one reversal.
type UndoOutcome struct {
	ActionID string `json:"action_id"`
	Kind     string `json:"kind"`
	Target   string `json:"target"`
	Reverted bool   `json:"reverted"`
	Error    string `json:"error,omitempty"`
}

func (s *Stack) readLines() ([]string, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, l := range strings.Split(string(data), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	return lines, nil
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
		fmt.Fprintf(&b, "[%s] %-18s %-20s %s\n", mark, o.Kind, o.Target, o.Error)
	}
	fmt.Fprintf(&b, "\n%d reverted, %d failed of %d\n", ok, fail, len(outcomes))
	return b.String()
}
