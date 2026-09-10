package mutation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Debajyoti0-0/aether/internal/engine/rollback"
	"github.com/Debajyoti0-0/aether/internal/store"
	"github.com/Debajyoti0-0/aether/internal/workspace"
)

func writeFile(path string, data []byte) error { return os.WriteFile(path, data, 0o600) }
func mkDirAll(path string) error               { return os.MkdirAll(path, 0o700) }

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

func auditPath(ws *workspace.Workspace) string { return filepath.Join(ws.Root, "db", "audit.jsonl") }
func rollbackPath(ws *workspace.Workspace) string {
	return filepath.Join(ws.Root, "db", "rollback.jsonl")
}

func readAudit(t *testing.T, ws *workspace.Workspace) []store.Entry {
	t.Helper()
	log, err := store.New(auditPath(ws), filepath.Join(ws.Root, "db", "audit.key"))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := log.Entries()
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func readRollbackStack(t *testing.T, ws *workspace.Workspace) []rollback.Action {
	t.Helper()
	stack := rollback.New(rollbackPath(ws))
	var out []rollback.Action
	for {
		a, err := stack.Pop()
		if err != nil {
			break
		}
		out = append(out, *a)
	}
	return out
}

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
	if res.Status != statusCompleted {
		t.Errorf("status = %q, want completed", res.Status)
	}
	if res.OperationID != "op-1" {
		t.Errorf("operation id = %q", res.OperationID)
	}

	// 2 audit entries: before + after, both with the action id.
	entries := readAudit(t, ws)
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
	}

	// 1 rollback entry with the action id and the declared recipe.
	stack := readRollbackStack(t, ws)
	if len(stack) != 1 {
		t.Fatalf("rollback entries = %d, want 1", len(stack))
	}
	if stack[0].ID != res.ActionID {
		t.Errorf("rollback id = %q, want %q", stack[0].ID, res.ActionID)
	}
	if stack[0].Undo.Op != "undo_test" {
		t.Errorf("undo op = %q", stack[0].Undo.Op)
	}

	// The signed audit chain must verify end-to-end.
	log, err := store.New(auditPath(ws), filepath.Join(ws.Root, "db", "audit.key"))
	if err != nil {
		t.Fatal(err)
	}
	vr, err := log.Verify()
	if err != nil {
		t.Fatalf("audit chain verify error: %v", err)
	}
	if !vr.ValidAll {
		t.Fatalf("audit chain verification failed: %+v", vr)
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
	if res.Status != statusFailed {
		t.Errorf("status = %q, want failed", res.Status)
	}

	mut := mutationEntries(readAudit(t, ws), "exec.test")
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
	stack := readRollbackStack(t, ws)
	if len(stack) != 1 {
		t.Errorf("rollback entries after failure = %d, want 1", len(stack))
	}
}

func TestPipelineAuditFailureRefusesMutation(t *testing.T) {
	ws := testWorkspace(t, "PipeNoAudit")

	// A corrupt JSONL line breaks log appends.
	if err := writeFile(auditPath(ws), []byte("not-json\n")); err != nil {
		t.Fatal(err)
	}

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
	if _, err := Run(context.Background(), ws, m); err == nil {
		t.Fatal("Run with broken audit chain = nil error, want refusal")
	}
	if executed {
		t.Fatal("mutation EXECUTED despite broken audit chain — governance bypass")
	}
}

func TestPipelineRollbackRegistrationFailureRefusesMutation(t *testing.T) {
	ws := testWorkspace(t, "PipeNoRollback")

	// Make rollback.jsonl a directory so the stack's OpenFile fails.
	if err := mkDirAll(rollbackPath(ws)); err != nil {
		t.Fatal(err)
	}

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
	res, err := Run(context.Background(), ws, m)
	if err == nil {
		t.Fatal("Run with broken rollback stack = nil error, want refusal")
	}
	if executed {
		t.Fatal("mutation EXECUTED despite rollback registration failure — governance bypass")
	}
	if res.Status != statusAbortedRollbackReg {
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
	if res.Status != statusStateUnknown {
		t.Errorf("status = %q, want completed_state_unknown", res.Status)
	}
	found := false
	for _, r := range mutationEntries(readAudit(t, ws), "exec.test") {
		if strings.Contains(r, "AFTER_STATE_UNAVAILABLE") {
			found = true
		}
	}
	if !found {
		t.Error("audit is missing AFTER_STATE_UNAVAILABLE")
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
	stack := readRollbackStack(t, ws)
	if len(stack) != 1 {
		t.Fatalf("rollback entries after crash = %d, want 1 (pre-execution registration)", len(stack))
	}
	if stack[0].Target != "vm-6" {
		t.Errorf("rollback target = %q", stack[0].Target)
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
	if res.Status != statusCompleted {
		t.Errorf("status = %q", res.Status)
	}
	found := false
	for _, r := range mutationEntries(readAudit(t, ws), "simulate.stream") {
		if strings.Contains(r, `"rollback":"irreversible"`) {
			found = true
		}
	}
	if !found {
		t.Error("audit must record irreversibility explicitly")
	}
	stack := readRollbackStack(t, ws)
	if len(stack) != 0 {
		t.Errorf("irreversible mutation pushed %d rollback entries, want 0", len(stack))
	}
}

func TestNewActionIDUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id, err := newActionID()
		if err != nil {
			t.Fatal(err)
		}
		if seen[id] {
			t.Fatalf("duplicate action id %q", id)
		}
		seen[id] = true
	}
}
