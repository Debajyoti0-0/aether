package rollback

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestPushPopLIFO(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "rollback.jsonl"))

	a1 := Action{Kind: "sp_secret", Target: "sp-1", Undo: UndoAction{Provider: "entra", Op: "remove_password", Args: map[string]string{"keyId": "k1"}}}
	a2 := Action{Kind: "group_member", Target: "g1", Undo: UndoAction{Provider: "entra", Op: "remove_member"}}

	if err := s.Push(a1); err != nil {
		t.Fatalf("push: %v", err)
	}
	if err := s.Push(a2); err != nil {
		t.Fatalf("push: %v", err)
	}

	depth, _ := s.Len()
	if depth != 2 {
		t.Fatalf("depth = %d", depth)
	}

	// Peek doesn't remove.
	peeked, err := s.Peek()
	if err != nil || peeked.Target != "g1" {
		t.Errorf("peek = %+v err=%v", peeked, err)
	}
	depth, _ = s.Len()
	if depth != 2 {
		t.Errorf("depth after peek = %d", depth)
	}

	// Pop returns newest first.
	popped, err := s.Pop()
	if err != nil || popped.Target != "g1" {
		t.Errorf("pop = %+v err=%v", popped, err)
	}
	popped, err = s.Pop()
	if err != nil || popped.Target != "sp-1" {
		t.Errorf("pop = %+v err=%v", popped, err)
	}
	if _, err := s.Pop(); err == nil {
		t.Error("empty stack should fail to pop")
	}
}

func TestPushRequiresKind(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "rb.jsonl"))
	if err := s.Push(Action{}); err == nil {
		t.Error("empty action should fail")
	}
}

func TestUndoAllNewestFirst(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "rb.jsonl"))
	s.Push(Action{Kind: "sp_secret", Target: "sp-1"})
	s.Push(Action{Kind: "group_member", Target: "g1"})
	s.Push(Action{Kind: "pipeline_dispatch", Target: "repo-1"})

	var order []string
	outcomes, err := s.UndoAll(context.Background(), func(ctx context.Context, a *Action) error {
		order = append(order, a.Target)
		return nil
	})
	if err != nil {
		t.Fatalf("undo all: %v", err)
	}

	if len(order) != 3 || order[0] != "repo-1" || order[2] != "sp-1" {
		t.Errorf("order = %v (want newest-first)", order)
	}
	for _, o := range outcomes {
		if !o.Reverted {
			t.Errorf("outcome = %+v", o)
		}
	}

	depth, _ := s.Len()
	if depth != 0 {
		t.Errorf("depth after undo = %d", depth)
	}
}

func TestUndoAllSurvivesFailures(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "rb.jsonl"))
	s.Push(Action{Kind: "sp_secret", Target: "a"})
	s.Push(Action{Kind: "group_member", Target: "b"})

	outcomes, err := s.UndoAll(context.Background(), func(ctx context.Context, a *Action) error {
		if a.Target == "b" {
			return errors.New("insufficient privileges")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("undo all: %v", err)
	}
	if len(outcomes) != 2 {
		t.Fatalf("outcomes = %d", len(outcomes))
	}
	// b failed but was popped; a (pushed first) still reversed.
	if outcomes[0].Reverted || outcomes[0].Target != "b" {
		t.Errorf("first outcome = %+v", outcomes[0])
	}
	if !outcomes[1].Reverted {
		t.Errorf("second outcome = %+v", outcomes[1])
	}
}

func TestUndoAllNoRunner(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "rb.jsonl"))
	s.Push(Action{Kind: "k", Target: "t"})
	outcomes, _ := s.UndoAll(context.Background(), nil)
	if len(outcomes) != 1 || outcomes[0].Error == "" {
		t.Errorf("outcomes = %+v", outcomes)
	}
}

func TestRenderOutcomes(t *testing.T) {
	out := RenderOutcomes([]UndoOutcome{
		{ActionID: "1", Kind: "sp_secret", Target: "sp-1", Reverted: true},
		{ActionID: "2", Kind: "group_member", Target: "g1", Error: "denied"},
	})
	if !strings.Contains(out, "1 reverted, 1 failed of 2") {
		t.Errorf("render = %q", out)
	}
}

func TestStackPersistsAcrossInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rb.jsonl")
	s1 := New(path)
	s1.Push(Action{Kind: "k", Target: "t"})

	s2 := New(path)
	peeked, err := s2.Peek()
	if err != nil || peeked.Target != "t" {
		t.Errorf("persisted peek = %+v err=%v", peeked, err)
	}
}
