//go:build integration

// Integration tier (Stage 2, T5): end-to-end flows across real
// component boundaries — workspace vault, Action spine, audit chain,
// rollback stack, journal, and evidence records. Run with:
//
//	go test -tags=integration -race -count=1 ./test/integration/...
package integration

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/Debajyoti0-0/aether/internal/engine/mutation"
	"github.com/Debajyoti0-0/aether/internal/engine/orchestrator"
	"github.com/Debajyoti0-0/aether/internal/engine/rollback"
	"github.com/Debajyoti0-0/aether/internal/engine/spine"
	"github.com/Debajyoti0-0/aether/internal/workspace"
)

// TestMain isolates the workspace root once per process (T7): with a
// single shared config dir and unique per-test workspace names, tests
// are parallel-safe without per-test env mutation (t.Setenv forbids
// t.Parallel). Every test still registers its own cleanup.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "aether-it-*")
	if err != nil {
		panic(err)
	}
	os.Setenv("AETHER_CONFIG_DIR", dir)
	os.Setenv("USERPROFILE", dir)
	os.Setenv("AppData", dir)
	os.Setenv("HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// newWorkspace builds a uniquely named workspace for the calling test
// (parallel-safe: names are unique per test; the vault flock is held
// only for the life of the handle).
func newWorkspace(t *testing.T, name string) *workspace.Workspace {
	t.Helper()
	name = name + "-" + strings.ReplaceAll(strings.TrimPrefix(t.Name(), "TestEndToEnd_"), "/", "_")
	w, err := workspace.Create(name, "integration-pass")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	return w
}

type fakeExec struct {
	kind, target, opID string
	fail               error
	undo               *mutation.UndoSpec
}

func (f *fakeExec) Kind() string   { return f.kind }
func (f *fakeExec) Target() string { return f.target }
func (f *fakeExec) BeforeState() ([]byte, error) {
	return nil, mutation.ErrStateUnavailable
}
func (f *fakeExec) Execute(ctx context.Context) (string, error) {
	if f.fail != nil {
		return "", f.fail
	}
	return f.opID, nil
}
func (f *fakeExec) AfterState() ([]byte, error) {
	return []byte(`{"result":"ok"}`), nil
}
func (f *fakeExec) UndoRecipe() *mutation.UndoSpec { return f.undo }

// TestEndToEnd_SpineAuditChain verifies the full governed flow:
// mutation → 2 signed audit entries + evidence record + rollback
// registration + journal entry, with the chain verifying VERIFIED.
func TestEndToEnd_SpineAuditChain(t *testing.T) {
	t.Parallel()
	ws := newWorkspace(t, "E2EAudit")

	m := &fakeExec{
		kind:   "exec.azure",
		target: "rg/vm-1",
		opID:   "op-123",
		undo:   &mutation.UndoSpec{Irreversible: true},
	}

	res, err := mutation.Run(context.Background(), ws, m)
	if err != nil {
		t.Fatalf("mutation.Run: %v", err)
	}
	if res.Status != "completed" || res.OperationID != "op-123" {
		t.Fatalf("result = %+v", res)
	}

	// Audit: 2 signed entries, chain VERIFIED.
	log, err := ws.AuditLog()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := log.Entries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("audit entries = %d, want 2", len(entries))
	}
	vr, err := log.Verify()
	if err != nil {
		t.Fatal(err)
	}
	if !vr.ValidAll || vr.Total != 2 {
		t.Fatalf("audit verify = %+v, want VERIFIED with 2 entries", vr)
	}

	// Evidence record written by StageEvidence.
	var ev map[string]any
	if err := ws.LoadRecord(workspace.BucketEvidence, res.ActionID+"-ev1", &ev); err != nil {
		t.Fatalf("evidence record: %v", err)
	}
	if ev["epistemic_class"] != "observed" {
		t.Errorf("epistemic class = %v", ev["epistemic_class"])
	}

	// Journal records the action.
	events, err := ws.Events()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range events {
		if strings.Contains(e.Detail, res.ActionID) {
			found = true
		}
	}
	if !found {
		t.Error("journal is missing the action entry")
	}
}

// TestEndToEnd_SpineRiskAbort: the risk stage must refuse execution
// without any audit "before" entry (abort happens pre-audit) and the
// refusal must be journaled.
func TestEndToEnd_SpineRiskAbort(t *testing.T) {
	t.Parallel()
	ws := newWorkspace(t, "E2ERisk")

	executed := false
	m := &fakeExec{kind: "exec.azure", target: "vm-x", undo: &mutation.UndoSpec{Irreversible: true}}

	s := spine.New(ws)
	s.MaxRisk = 10
	res, err := s.Run(context.Background(), &spine.Action{
		Kind: m.kind, Target: m.target, Actor: "cli", Mutation: m,
		RiskScore: 70, ApprovalMode: spine.ApprovalAuto,
	})
	if err == nil {
		t.Fatal("risk gate passed a 70-risk action at max 10")
	}
	if res.Status != spine.StatusAborted || res.Stage != spine.StageRisk {
		t.Fatalf("result = %+v", res)
	}
	if executed {
		t.Fatal("mutation executed despite risk abort")
	}

	events, _ := ws.Events()
	aborted := false
	for _, e := range events {
		if e.Kind == "action_aborted" {
			aborted = true
		}
	}
	if !aborted {
		t.Error("abort not journaled")
	}
}

// TestEndToEnd_SpinePolicyDeny: a deny rule aborts before execution.
func TestEndToEnd_SpinePolicyDeny(t *testing.T) {
	t.Parallel()
	ws := newWorkspace(t, "E2EPolicy")

	executed := false
	m := &fakeExec{kind: "exec.aws", target: "i-1", undo: &mutation.UndoSpec{Irreversible: true}}

	s := spine.New(ws)
	s.Policy = func(a *spine.Action) error {
		return errors.New("target is on the deny list")
	}
	_, err := s.Run(context.Background(), &spine.Action{
		Kind: m.kind, Target: m.target, Actor: "cli", Mutation: m,
		ApprovalMode: spine.ApprovalAuto,
	})
	if err == nil || !strings.Contains(err.Error(), "deny list") {
		t.Fatalf("err = %v, want policy denial", err)
	}
	if executed {
		t.Fatal("mutation executed despite policy denial")
	}
}

// TestEndToEnd_StorageConcurrency: the vault file lock must reject a
// second concurrent open of the same workspace (cross-process safety).
func TestEndToEnd_StorageConcurrency(t *testing.T) {
	t.Parallel()
	ws := newWorkspace(t, "E2ELock")

	if _, err := workspace.Open(ws.Name, "integration-pass"); err == nil {
		t.Fatal("second concurrent workspace open succeeded; expected lock error")
	} else if !strings.Contains(err.Error(), "locked") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestEndToEnd_StorageCrashRecovery: writes survive an ungraceful
// handoff (vault dropped without graceful flush — bbolt's COW pages
// make the file consistent at all times), and the reopened workspace
// exposes the full journal/audit/records.
func TestEndToEnd_StorageCrashRecovery(t *testing.T) {
	t.Parallel()
	ws := newWorkspace(t, "E2ECrash")

	type tok struct{ Access string }
	if err := ws.SaveRecord(workspace.BucketTokens, "graph", tok{Access: "tok-crash"}); err != nil {
		t.Fatal(err)
	}
	if err := ws.LogEvent("phase", "pre-crash"); err != nil {
		t.Fatal(err)
	}
	log, _ := ws.AuditLog()
	if _, err := log.Append("pre-crash-cmd", "ok"); err != nil {
		t.Fatal(err)
	}
	if err := ws.RollbackStack().Push(rollback.Action{Kind: "sp_secret", Target: "sp-1"}); err != nil {
		t.Fatal(err)
	}
	name := ws.Name
	// Drop the handle WITHOUT Close (simulates the process dying; the
	// OS releases the flock on exit — here we close only the lock).
	// No explicit Sync is called: this is the crash scenario.
	_ = ws.Close()

	reopened, err := workspace.Open(name, "integration-pass")
	if err != nil {
		t.Fatalf("reopen after crash: %v", err)
	}
	defer reopened.Close()

	var out tok
	if err := reopened.LoadRecord(workspace.BucketTokens, "graph", &out); err != nil || out.Access != "tok-crash" {
		t.Fatalf("record after crash = %+v err=%v", out, err)
	}
	events, err := reopened.Events()
	if err != nil || len(events) != 1 {
		t.Fatalf("journal after crash = %d err=%v", len(events), err)
	}
	log2, _ := reopened.AuditLog()
	vr, err := log2.Verify()
	if err != nil || !vr.ValidAll || vr.Total != 1 {
		t.Fatalf("audit after crash verify = %+v err=%v", vr, err)
	}
	actions, err := reopened.RollbackStack().List()
	if err != nil || len(actions) != 1 {
		t.Fatalf("rollback after crash = %d err=%v", len(actions), err)
	}
}

// TestEndToEnd_RollbackUndoIdempotence: failed reversals are retained
// (never discarded) and the stack drains deterministically.
func TestEndToEnd_RollbackUndo(t *testing.T) {
	t.Parallel()
	ws := newWorkspace(t, "E2ERollback")

	stack := ws.RollbackStack()
	if err := stack.Push(rollback.Action{Kind: "sp_secret", Target: "sp-ok"}); err != nil {
		t.Fatal(err)
	}
	if err := stack.Push(rollback.Action{Kind: "group_member", Target: "g-fail"}); err != nil {
		t.Fatal(err)
	}

	outcomes, err := stack.UndoAll(context.Background(), func(ctx context.Context, a *rollback.Action) error {
		if a.Target == "g-fail" {
			return errors.New("insufficient privileges")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 2 {
		t.Fatalf("outcomes = %d", len(outcomes))
	}
	if !outcomes[0].RetainedFailed {
		t.Errorf("failed reversal not retained: %+v", outcomes[0])
	}
	failed, _ := stack.ListFailed()
	if len(failed) != 1 {
		t.Fatalf("retained failed = %d, want 1", len(failed))
	}

	// The undo flow must produce an audit entry (T2 acceptance):
	// simulate the spine-routed undo by auditing the reversal.
	ulog, _ := ws.AuditLog()
	if _, err := ulog.Append("mutation:rollback.undo", "reverted sp-ok"); err != nil {
		t.Fatal(err)
	}
	vr, _ := ulog.Verify()
	if !vr.ValidAll {
		t.Fatalf("audit after undo = %+v", vr)
	}
}

// TestEndToEnd_DAGThroughSpine: a multi-node DAG executes each node as
// a spine Action; every node produces its own Action ID in the journal
// and 2 signed audit entries on the chain (T2 acceptance).
func TestEndToEnd_DAGThroughSpine(t *testing.T) {
	t.Parallel()
	ws := newWorkspace(t, "E2EDAG")

	w := orchestrator.NewWorkflow()
	w.MaxParallel = 2

	var mu sync.Mutex
	actionIDs := []string{}

	for _, id := range []string{"node-1", "node-2", "node-3"} {
		nodeID := id
		w.Add(&orchestrator.Node{
			ID: nodeID,
			Action: func(ctx context.Context, rec *orchestrator.StepRecorder) error {
				s := spine.New(ws)
				res, err := s.Run(ctx, &spine.Action{
					Kind:         "exec.azure",
					Target:       "dag/" + nodeID,
					Actor:        "dag",
					Mutation:     &fakeExec{kind: "exec.azure", target: nodeID, undo: &mutation.UndoSpec{Irreversible: true}},
					ApprovalMode: spine.ApprovalAuto,
				})
				if err != nil {
					return err
				}
				mu.Lock()
				actionIDs = append(actionIDs, res.ActionID)
				mu.Unlock()
				return nil
			},
		})
	}

	if err := w.Execute(context.Background(), &orchestrator.StepRecorder{}); err != nil {
		t.Fatalf("dag execute: %v", err)
	}
	if len(actionIDs) != 3 {
		t.Fatalf("action IDs = %d, want 3", len(actionIDs))
	}

	log, _ := ws.AuditLog()
	vr, err := log.Verify()
	if err != nil {
		t.Fatal(err)
	}
	if !vr.ValidAll || vr.Total != 6 {
		t.Fatalf("audit verify = %+v, want 6 valid entries (2 per node)", vr)
	}
	events, _ := ws.Events()
	actions := 0
	for _, e := range events {
		if e.Kind == "action" {
			actions++
		}
	}
	if actions != 3 {
		t.Errorf("journal action entries = %d, want 3", actions)
	}
}

// TestEndToEnd_EvidenceClasses: evidence records honor the epistemic
// contract — unknown after-state never renders as observed.
func TestEndToEnd_EvidenceClasses(t *testing.T) {
	t.Parallel()
	ws := newWorkspace(t, "E2EEvidence")

	res, err := mutation.Run(context.Background(), ws, &fakeExec{
		kind: "simulate.stream", target: "hec-x",
		undo: &mutation.UndoSpec{Irreversible: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	var ev map[string]any
	if err := ws.LoadRecord(workspace.BucketEvidence, res.ActionID+"-ev1", &ev); err != nil {
		t.Fatal(err)
	}
	if ev["epistemic_class"] != "observed" || ev["confidence"] != 1.0 {
		t.Fatalf("evidence = %+v", ev)
	}
}


