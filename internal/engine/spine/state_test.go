package spine

import (
	"strings"
	"testing"
)

func TestTransitionTableHappyPath(t *testing.T) {
	st := NewActionState()
	path := []State{
		StateAuthzPassed, StateRiskPassed, StatePolicyPassed, StateApprovalPassed,
		StateBeforeCaptured, StateRollbackRegistered, StateAuditedBefore, StateExecuting,
		StateExecuted, StateAfterCaptured, StateEvidenceWritten, StateAuditedAfter, StateCompleted,
	}
	for _, to := range path {
		if err := st.Transition(to); err != nil {
			t.Fatalf("legal transition %s: %v", to, err)
		}
	}
	if !st.Current.Terminal() {
		t.Errorf("completed state not terminal: %s", st.Current)
	}
	if len(st.Seq) != len(path)+1 {
		t.Errorf("seq len = %d, want %d", len(st.Seq), len(path)+1)
	}
	if st.Seq[0] != StateCreated || st.Seq[len(st.Seq)-1] != StateCompleted {
		t.Errorf("seq endpoints wrong: %v", st.Seq)
	}
}

func TestIllegalTransitionsRejected(t *testing.T) {
	// The directive's explicit example: CREATED -> EXECUTING is impossible.
	st := NewActionState()
	if err := st.Transition(StateExecuting); err == nil {
		t.Fatal("CREATED -> EXECUTING accepted — governance bypass possible")
	}
	// FAILED -> COMPLETED is not a legal edge.
	st2 := NewActionState()
	_ = st2.Transition(StateAuthzPassed)
	_ = st2.Transition(StateRiskPassed)
	_ = st2.Transition(StatePolicyPassed)
	_ = st2.Transition(StateApprovalPassed)
	_ = st2.Transition(StateBeforeCaptured)
	_ = st2.Transition(StateAuditedBefore)
	_ = st2.Transition(StateExecuting)
	_ = st2.Transition(StateFailed)
	if err := st2.Transition(StateCompleted); err == nil {
		t.Fatal("FAILED -> COMPLETED accepted — false-success conversion possible")
	}
	// Terminal states are frozen.
	st3 := NewActionState()
	_ = st3.Transition(StateAborted)
	if !st3.Current.Terminal() {
		t.Fatal("aborted should be terminal")
	}
	if err := st3.Transition(StateExecuting); err == nil {
		t.Fatal("ABORTED -> EXECUTING accepted")
	}
}

func TestUnknownNeverBecomesSuccess(t *testing.T) {
	// completed_state_unknown is terminal: uncertainty can never be
	// silently reconciled into success by the machine itself.
	st := NewActionState()
	for _, to := range []State{
		StateAuthzPassed, StateRiskPassed, StatePolicyPassed, StateApprovalPassed,
		StateBeforeCaptured, StateAuditedBefore, StateExecuting, StateExecuted,
		StateCompletedStateUnknown,
	} {
		if err := st.Transition(to); err != nil {
			t.Fatalf("transition %s: %v", to, err)
		}
	}
	if err := st.Transition(StateCompleted); err == nil {
		t.Fatal("COMPLETED_STATE_UNKNOWN -> COMPLETED accepted silently")
	}
}

func TestAbortBranchesFromEveryPreExecutionStage(t *testing.T) {
	prefix := []State{StateAuthzPassed, StateRiskPassed, StatePolicyPassed, StateApprovalPassed, StateBeforeCaptured}
	for i := 0; i <= len(prefix); i++ {
		st := NewActionState()
		for j := 0; j < i; j++ {
			if err := st.Transition(prefix[j]); err != nil {
				t.Fatal(err)
			}
		}
		if err := st.Transition(StateAborted); err != nil {
			t.Errorf("abort from stage %d: %v", i, err)
		}
	}
}

func TestStateSeqRendering(t *testing.T) {
	st := NewActionState()
	_ = st.Transition(StateAuthzPassed)
	_ = st.Transition(StateAborted)
	out := stateSeqString(st.Seq)
	if !strings.Contains(out, "created>") || !strings.Contains(out, "aborted") {
		t.Errorf("render = %q", out)
	}
}

// TestSpineRunsRecordStateSeq: the public Run path produces the exact
// expected transition sequence for a happy-path action.
func TestSpineRunsRecordStateSeq(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AETHER_CONFIG_DIR", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("AppData", dir)
	t.Setenv("HOME", dir)

	w, err := newWorkspaceForStateTest(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	s := New(w)
	res, err := s.Run(testContext(), &Action{
		Kind: "exec.test", Target: "t-1", Actor: "test",
		Mutation:     &stateTestMutation{},
		ApprovalMode: ApprovalAuto,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "created>authz_passed>risk_passed>policy_passed>approval_passed>before_captured>audited_before>executing>executed>after_captured>evidence_written>audited_after>completed"
	if stateSeqString(res.StateSeq) != want {
		t.Errorf("state seq = %s\nwant           %s", stateSeqString(res.StateSeq), want)
	}
}
