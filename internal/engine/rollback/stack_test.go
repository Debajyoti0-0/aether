package rollback

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Debajyoti0-0/aether/internal/store"
)

func testVault(t *testing.T) *store.Vault {
	t.Helper()
	v, err := store.OpenVault(t.TempDir() + "/vault.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	return v
}

func TestPushPopLIFO(t *testing.T) {
	s := New(testVault(t))

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
	s := New(testVault(t))
	if err := s.Push(Action{}); err == nil {
		t.Error("empty action should fail")
	}
}

func TestUndoAllNewestFirst(t *testing.T) {
	s := New(testVault(t))
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
	s := New(testVault(t))
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

	// Stage 2 invariant: the failed reversal is retained, never dropped.
	failed, err := s.ListFailed()
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(failed) != 1 || failed[0].Target != "b" {
		t.Fatalf("retained failed = %+v, want [b]", failed)
	}
	if !outcomes[0].RetainedFailed {
		t.Errorf("outcome must flag retention: %+v", outcomes[0])
	}
}

func TestUndoAllNoRunner(t *testing.T) {
	s := New(testVault(t))
	s.Push(Action{Kind: "k", Target: "t"})
	outcomes, _ := s.UndoAll(context.Background(), nil)
	if len(outcomes) != 1 || outcomes[0].Error == "" {
		t.Errorf("outcomes = %+v", outcomes)
	}
	if failed, _ := s.ListFailed(); len(failed) != 1 {
		t.Errorf("failed retention = %d, want 1", len(failed))
	}
}

func TestRenderOutcomes(t *testing.T) {
	out := RenderOutcomes([]UndoOutcome{
		{ActionID: "1", Kind: "sp_secret", Target: "sp-1", Reverted: true},
		{ActionID: "2", Kind: "group_member", Target: "g1", Error: "denied", RetainedFailed: true},
	})
	if !strings.Contains(out, "1 reverted, 1 failed of 2") {
		t.Errorf("render = %q", out)
	}
	if !strings.Contains(out, "[retained]") {
		t.Errorf("render must surface retention: %q", out)
	}
}

// Stage 2: the stack persists in the vault; a second stack instance on
// the SAME vault handle sees the same entries. (A second process would
// be blocked by the vault file lock.)
func TestStackPersistsAcrossInstances(t *testing.T) {
	v := testVault(t)
	s1 := New(v)
	s1.Push(Action{Kind: "k", Target: "t"})

	s2 := New(v)
	peeked, err := s2.Peek()
	if err != nil || peeked.Target != "t" {
		t.Errorf("persisted peek = %+v err=%v", peeked, err)
	}
}

// Stage 2: Pop is a single vault transaction — a corrupt entry is
// retained in rollback_failed, not destroyed.
func TestPopCorruptEntryRetained(t *testing.T) {
	v := testVault(t)
	if _, err := v.RollbackPush([]byte("not-json")); err != nil {
		t.Fatal(err)
	}
	s := New(v)
	if _, err := s.Pop(); err == nil {
		t.Fatal("corrupt entry popped without error")
	}
	failed, err := s.ListFailed()
	if err != nil || len(failed) == 0 {
		t.Fatalf("corrupt entry not retained: %v %v", failed, err)
	}
}
