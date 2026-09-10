package spine

import (
	"fmt"
)

// State names the lifecycle position of one Action (Stage 3, T4:
// the explicit transition table that Stage 2 deferred).
type State string

const (
	StateCreated               State = "created"
	StateAuthzPassed           State = "authz_passed"
	StateRiskPassed            State = "risk_passed"
	StatePolicyPassed          State = "policy_passed"
	StateApprovalPassed        State = "approval_passed"
	StateBeforeCaptured        State = "before_captured"
	StateRollbackRegistered    State = "rollback_registered"
	StateAuditedBefore         State = "audited_before"
	StateExecuting             State = "executing"
	StateExecuted              State = "executed"
	StateAfterCaptured         State = "after_captured"
	StateEvidenceWritten       State = "evidence_written"
	StateAuditedAfter          State = "audited_after"
	StateCompleted             State = "completed"
	StateCompletedStateUnknown State = "completed_state_unknown"
	StateFailed                State = "failed"
	StateAborted               State = "aborted"
)

// Terminal states accept no outgoing transitions.
func (s State) Terminal() bool {
	switch s {
	case StateCompleted, StateCompletedStateUnknown, StateFailed, StateAborted:
		return true
	}
	return false
}

// ActionState is the observable state machine: the current state plus
// the ordered transition history (recorded in the journal as state_seq).
type ActionState struct {
	Current State
	Seq     []State
}

// NewActionState starts a machine in the created state.
func NewActionState() *ActionState {
	return &ActionState{Current: StateCreated, Seq: []State{StateCreated}}
}

// Transition moves the machine from its current state to `to`. It
// returns an error for any transition not in the legal table — the
// spine can never drift into an unrepresentable state.
func (a *ActionState) Transition(to State) error {
	if !legal(a.Current, to) {
		return fmt.Errorf("illegal action state transition %s -> %s", a.Current, to)
	}
	a.Current = to
	a.Seq = append(a.Seq, to)
	return nil
}

// transitions encodes the legal graph. The spine's happy path is the
// chain created → authz → risk → policy → approval → (before →
// rollback → audit-before) → executing → executed → after → evidence →
// audit-after → completed/unknown; failures branch from executing;
// aborts branch from every pre-execution stage.
var transitions = map[State][]State{
	StateCreated: {StateAuthzPassed, StateAborted},
	StateAuthzPassed: {StateRiskPassed, StateAborted},
	StateRiskPassed: {StatePolicyPassed, StateAborted},
	StatePolicyPassed: {StateApprovalPassed, StateAborted},
	StateApprovalPassed: {StateBeforeCaptured, StateAborted},
	StateBeforeCaptured: {StateRollbackRegistered, StateAuditedBefore, StateAborted},
	StateRollbackRegistered: {StateAuditedBefore, StateAborted},
	StateAuditedBefore: {StateExecuting, StateAborted},
	StateExecuting: {StateExecuted, StateFailed, StateAborted},
	StateExecuted: {StateAfterCaptured, StateFailed, StateCompletedStateUnknown},
	StateAfterCaptured: {StateEvidenceWritten, StateCompletedStateUnknown},
	StateEvidenceWritten: {StateAuditedAfter},
	StateAuditedAfter: {StateCompleted, StateCompletedStateUnknown, StateFailed},
}

func legal(from, to State) bool {
	for _, next := range transitions[from] {
		if next == to {
			return true
		}
	}
	return false
}
