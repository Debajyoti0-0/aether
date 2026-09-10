package mutation

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Debajyoti0-0/aether/internal/engine/rollback"
	"github.com/Debajyoti0-0/aether/internal/engine/spine"
	"github.com/Debajyoti0-0/aether/internal/store"
	"github.com/Debajyoti0-0/aether/internal/workspace"
)

func testWorkspace(t *testing.T, name string) *workspace.Workspace {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AETHER_CONFIG_DIR", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("AppData", dir)
	t.Setenv("HOME", dir)
	w, err := workspace.Create(name, "test-pass")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	return w
}

type fakeMutation struct {
	kind      string
	target    string
	before    []byte
	beforeErr error
	execute   func(ctx context.Context) (string, error)
	after     []byte
	afterErr  error
	undo      *UndoSpec
}

func (f *fakeMutation) Kind() string   { return f.kind }
func (f *fakeMutation) Target() string { return f.target }
func (f *fakeMutation) BeforeState() ([]byte, error) {
	return f.before, f.beforeErr
}
func (f *fakeMutation) Execute(ctx context.Context) (string, error) {
	if f.execute == nil {
		return "op-1", nil
	}
	return f.execute(ctx)
}
func (f *fakeMutation) AfterState() ([]byte, error) { return f.after, f.afterErr }
func (f *fakeMutation) UndoRecipe() *UndoSpec       { return f.undo }

func mutationEntries(entries []store.Entry, kind string) []string {
	var out []string
	for _, e := range entries {
		if e.Command == "mutation:"+kind {
			out = append(out, e.Result)
		}
	}
	return out
}

func TestPipelineSuccess(t *testing.T) {
	ws := testWorkspace(t, "PipeOK")

	m := &fakeMutation{
		kind:   "exec.test",
		target: "vm-1",
		before: []byte(`{"state":"before"}`),
		after:  []byte(`{"state":"after"}`),
		undo: &UndoSpec{
			Provider: "test", Op: "undo_test",
			Detail: "test reversal",
		},
	}

	res, err := Run(context.Background(), ws, m)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Status != "completed" {
		t.Errorf("status = %q, want completed", res.Status)
	}
	if res.OperationID != "op-1" {
		t.Errorf("operation id = %q", res.OperationID)
	}

	// 2 audit entries: before + after, both with the action id.
	log, err := ws.AuditLog()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := log.Entries()
	if err != nil {
		t.Fatal(err)
	}
	mut := mutationEntries(entries, "exec.test")
	if len(mut) != 2 {
		t.Fatalf("audit entries for mutation = %d, want 2", len(mut))
	}
	if !strings.Contains(mut[0], `"phase":"before"`) || !strings.Contains(mut[1], `"phase":"after"`) {
		t.Errorf("phases wrong: %q", mut)
	}
	for _, r := range mut {
		if !strings.Contains(r, res.ActionID) {
			t.Errorf("audit entry missing action id: %s", r)
		}
		if !strings.Contains(r, `"approval_mode":"auto"`) {
			t.Errorf("audit entry missing approval mode: %s", r)
		}
	}

	// 1 rollback entry with the action id and the declared recipe.
	stack := ws.RollbackStack()
	actions, err := stack.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 {
		t.Fatalf("rollback entries = %d, want 1", len(actions))
	}
	if actions[0].ID != res.ActionID {
		t.Errorf("rollback id = %q, want %q", actions[0].ID, res.ActionID)
	}
	if actions[0].Undo.Op != "undo_test" {
		t.Errorf("undo op = %q", actions[0].Undo.Op)
	}

	// The signed audit chain must verify end-to-end.
	vr, err := log.Verify()
	if err != nil {
		t.Fatalf("audit chain verify error: %v", err)
	}
	if !vr.ValidAll {
		t.Fatalf("audit chain verification failed: %+v", vr)
	}

	// One evidence record exists for the action (Stage 2 T3 seed).
	var ev map[string]any
	if err := ws.LoadRecord(workspace.BucketEvidence, res.ActionID+"-ev1", &ev); err != nil {
		t.Fatalf("evidence record missing: %v", err)
	}
	if ev["epistemic_class"] != "observed" || ev["confidence"] != 1.0 {
		t.Errorf("evidence = %+v", ev)
	}
}

func TestPipelineExecuteFailure(t *testing.T) {
	ws := testWorkspace(t, "PipeFail")

	m := &fakeMutation{
		kind:   "exec.test",
		target: "vm-2",
		undo:   &UndoSpec{Provider: "test", Op: "undo_test"},
		execute: func(ctx context.Context) (string, error) {
			return "", errors.New("provider exploded")
		},
	}

	res, err := Run(context.Background(), ws, m)
	if err == nil || err.Error() != "provider exploded" {
		t.Fatalf("err = %v, want provider exploded", err)
	}
	if res.Status != "failed" {
		t.Errorf("status = %q, want failed", res.Status)
	}

	log, _ := ws.AuditLog()
	entries, _ := log.Entries()
	mut := mutationEntries(entries, "exec.test")
	if len(mut) != 2 {
		t.Fatalf("audit entries = %d, want 2 (before+after even on failure)", len(mut))
	}
	failed := false
	for _, r := range mut {
		if strings.Contains(r, `"status":"failed"`) {
			failed = true
		}
	}
	if !failed {
		t.Error("audit is missing the failed status")
	}

	// The rollback entry survives the failure (registered pre-execution).
	actions, _ := ws.RollbackStack().List()
	if len(actions) != 1 {
		t.Errorf("rollback entries after failure = %d, want 1", len(actions))
	}
}

func TestPipelineAuditFailureRefusesMutation(t *testing.T) {
	ws := testWorkspace(t, "PipeNoAudit")

	executed := false
	m := &fakeMutation{
		kind:   "exec.test",
		target: "vm-3",
		undo:   &UndoSpec{Provider: "test", Op: "undo_test"},
		execute: func(ctx context.Context) (string, error) {
			executed = true
			return "op-x", nil
		},
	}

	// Inject a broken audit chain via the spine's failure-injection seam
	// (the production default uses the workspace vault).
	s := spine.New(ws)
	s.Hooks.AuditLog = func() (*store.Log, error) {
		return nil, errors.New("audit chain unavailable")
	}
	if _, err := s.Run(context.Background(), &spine.Action{
		Kind: m.Kind(), Target: m.Target(), Actor: "cli", Mutation: m,
		ApprovalMode: spine.ApprovalAuto,
	}); err == nil {
		t.Fatal("Run with broken audit chain = nil error, want refusal")
	}
	if executed {
		t.Fatal("mutation EXECUTED despite broken audit chain — governance bypass")
	}
}

func TestPipelineRollbackRegistrationFailureRefusesMutation(t *testing.T) {
	ws := testWorkspace(t, "PipeNoRollback")

	executed := false
	m := &fakeMutation{
		kind:   "exec.test",
		target: "vm-4",
		undo:   &UndoSpec{Provider: "test", Op: "undo_test"},
		execute: func(ctx context.Context) (string, error) {
			executed = true
			return "op-x", nil
		},
	}

	s := spine.New(ws)
	s.Hooks.RollbackStack = func() *rollback.Stack {
		// A stack bound to a CLOSED vault fails every write (bbolt:
		// "database not open") — the production default uses the live
		// workspace vault; this seam only simulates the failure.
		closed, err := store.OpenVault(ws.Root + "/closed-vault.db")
		if err != nil {
			panic(err)
		}
		_ = closed.Close()
		return rollback.New(closed)
	}
	res, err := s.Run(context.Background(), &spine.Action{
		Kind: m.Kind(), Target: m.Target(), Actor: "cli", Mutation: m,
		ApprovalMode: spine.ApprovalAuto,
	})
	if err == nil {
		t.Fatal("Run with broken rollback stack = nil error, want refusal")
	}
	if executed {
		t.Fatal("mutation EXECUTED despite rollback registration failure — governance bypass")
	}
	if res.Status != spine.StatusAbortedRBReg {
		t.Errorf("status = %q, want aborted_rollback_registration", res.Status)
	}
}

func TestPipelineStateUnknownIsNotSuccess(t *testing.T) {
	ws := testWorkspace(t, "PipeUnknown")

	m := &fakeMutation{
		kind:     "exec.test",
		target:   "vm-5",
		undo:     &UndoSpec{Irreversible: true},
		afterErr: ErrStateUnavailable,
	}
	res, err := Run(context.Background(), ws, m)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Status != "completed_state_unknown" {
		t.Errorf("status = %q, want completed_state_unknown", res.Status)
	}
	log, _ := ws.AuditLog()
	entries, _ := log.Entries()
	found := false
	for _, r := range mutationEntries(entries, "exec.test") {
		if strings.Contains(r, "AFTER_STATE_UNAVAILABLE") {
			found = true
		}
	}
	if !found {
		t.Error("audit is missing AFTER_STATE_UNAVAILABLE")
	}
	// Unknown after-state ⇒ evidence must be ClassUnknown, confidence 0.
	var ev map[string]any
	if err := ws.LoadRecord(workspace.BucketEvidence, res.ActionID+"-ev1", &ev); err != nil {
		t.Fatal(err)
	}
	if ev["epistemic_class"] != "unknown" || ev["confidence"] != 0.0 {
		t.Errorf("evidence = %+v, want unknown/0.0", ev)
	}
}

func TestPipelineCrashMidExecuteLeavesRollbackTrace(t *testing.T) {
	ws := testWorkspace(t, "PipeCrash")

	m := &fakeMutation{
		kind:   "exec.test",
		target: "vm-6",
		undo:   &UndoSpec{Provider: "test", Op: "undo_test"},
		execute: func(ctx context.Context) (string, error) {
			panic("simulated crash mid-execute")
		},
	}
	func() {
		defer func() { _ = recover() }()
		_, _ = Run(context.Background(), ws, m)
	}()

	// The rollback entry was registered BEFORE execution, so the crash
	// leaves a recoverable trace.
	actions, err := ws.RollbackStack().List()
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 {
		t.Fatalf("rollback entries after crash = %d, want 1 (pre-execution registration)", len(actions))
	}
	if actions[0].Target != "vm-6" {
		t.Errorf("rollback target = %q", actions[0].Target)
	}
}

func TestPipelineIrreversibleRecordedHonestly(t *testing.T) {
	ws := testWorkspace(t, "PipeIrrev")

	m := &fakeMutation{
		kind:   "simulate.stream",
		target: "https://hec.example.com",
		undo:   &UndoSpec{Irreversible: true},
	}
	res, err := Run(context.Background(), ws, m)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Status != "completed" {
		t.Errorf("status = %q", res.Status)
	}
	log, _ := ws.AuditLog()
	entries, _ := log.Entries()
	found := false
	for _, r := range mutationEntries(entries, "simulate.stream") {
		if strings.Contains(r, `"rollback":"irreversible"`) {
			found = true
		}
	}
	if !found {
		t.Error("audit must record irreversibility explicitly")
	}
	actions, _ := ws.RollbackStack().List()
	if len(actions) != 0 {
		t.Errorf("irreversible mutation pushed %d rollback entries, want 0", len(actions))
	}
}

func TestNewActionIDUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id, err := spine.NewActionID()
		if err != nil {
			t.Fatal(err)
		}
		if seen[id] {
			t.Fatalf("duplicate action id %q", id)
		}
		seen[id] = true
	}
}
