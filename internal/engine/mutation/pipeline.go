package mutation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/Debajyoti0-0/aether/internal/engine/rollback"
	"github.com/Debajyoti0-0/aether/internal/store"
	"github.com/Debajyoti0-0/aether/internal/workspace"
)

// UndoSpec is the rollback recipe declared by a Mutation. It mirrors
// rollback.Action minus the fields the pipeline owns (ID, Timestamp,
// Kind, Target, Metadata) so the pipeline can correlate the rollback
// entry with the audit chain.
type UndoSpec struct {
	Provider string            `json:"provider"`
	Op       string            `json:"op"`
	Args     map[string]string `json:"args,omitempty"`
	Command  string            `json:"command,omitempty"`
	Detail   string            `json:"detail,omitempty"`
	// Irreversible marks mutations with no reversal path; the audit
	// entry records this explicitly rather than implying recoverability.
	Irreversible bool `json:"irreversible,omitempty"`
}

// Result is the outcome of a governed mutation.
type Result struct {
	ActionID   string `json:"action_id"`
	Kind       string `json:"kind"`
	Target     string `json:"target"`
	OperationID string `json:"operation_id,omitempty"`
	Status     string `json:"status"` // completed | failed | completed_state_unknown | aborted
}

// pipelineState names for audit entries.
const (
	statusCompleted          = "completed"
	statusFailed             = "failed"
	statusStateUnknown       = "completed_state_unknown"
	statusAbortedRollbackReg = "aborted_rollback_registration"
)

// auditPayload is the structured body written into the Result field of
// each signed audit entry. The Entry.Command field carries
// "mutation:<kind>" and Entry.Timestamp is supplied by the log.
type auditPayload struct {
	ActionID    string          `json:"action_id"`
	Phase       string          `json:"phase"` // before | after
	Target      string          `json:"target"`
	Status      string          `json:"status,omitempty"`
	BeforeState json.RawMessage `json:"before_state,omitempty"`
	AfterState  json.RawMessage `json:"after_state,omitempty"`
	StateNote   string          `json:"state_note,omitempty"`
	OperationID string          `json:"operation_id,omitempty"`
	Error       string          `json:"error,omitempty"`
	Rollback    string          `json:"rollback,omitempty"` // registered | irreversible | registration_failed
}

// Run executes one mutation through the governance pipeline:
//
//	before-state → AUDIT(before) → ROLLBACK REGISTRATION → EXECUTE →
//	after-state → AUDIT(after)
//
// Fail-closed rules:
//   - if the audit chain cannot be written, the mutation does NOT run;
//   - if a rollback recipe exists but registration fails, the mutation
//     does NOT run (an ungoverned mutation is worse than no mutation);
//   - if execution succeeds but the after-state cannot be captured, the
//     audit status is completed_state_unknown (never a bare success).
func Run(ctx context.Context, ws *workspace.Workspace, m Mutation) (Result, error) {
	actionID, err := newActionID()
	if err != nil {
		return Result{}, fmt.Errorf("generate action id: %w", err)
	}
	res := Result{ActionID: actionID, Kind: m.Kind(), Target: m.Target()}

	auditLog, err := openAudit(ws)
	if err != nil {
		return res, fmt.Errorf("open audit chain (mutation refused — governance unavailable): %w", err)
	}
	stack := rollback.New(filepath.Join(ws.Root, "db", "rollback.jsonl"))

	// 1. Before state.
	before, beforeErr := m.BeforeState()
	beforeNote := ""
	if beforeErr != nil {
		if !isStateUnavailable(beforeErr) {
			return res, fmt.Errorf("capture before state: %w", beforeErr)
		}
		beforeNote = "BEFORE_STATE_UNAVAILABLE"
	}

	// 2. Rollback registration BEFORE execution (crash mid-execute must
	// leave a recoverable trace).
	rollbackState := "irreversible"
	recipe := m.UndoRecipe()
	if recipe != nil && !recipe.Irreversible {
		entry := rollback.Action{
			ID:        actionID,
			Kind:      m.Kind(),
			Target:    m.Target(),
			Detail:    recipe.Detail,
			Undo: rollback.UndoAction{
				Provider: recipe.Provider,
				Op:       recipe.Op,
				Args:     recipe.Args,
				Command:  recipe.Command,
			},
			Metadata: map[string]string{"action_id": actionID},
		}
		if err := stack.Push(entry); err != nil {
			// Fail closed: do not execute an ungovernable mutation.
			audErr := appendAudit(auditLog, m, auditPayload{
				ActionID: actionID, Phase: "before", Target: m.Target(),
				Status: statusAbortedRollbackReg, StateNote: beforeNote,
				Error: "rollback registration failed: " + err.Error(),
			})
			res.Status = statusAbortedRollbackReg
			if audErr != nil {
				return res, fmt.Errorf("rollback registration failed (%v) AND audit write failed (%v); mutation aborted", err, audErr)
			}
			return res, fmt.Errorf("rollback registration failed; mutation aborted: %w", err)
		}
		rollbackState = "registered"
	}

	// 3. AUDIT(before).
	if err := appendAudit(auditLog, m, auditPayload{
		ActionID: actionID, Phase: "before", Target: m.Target(),
		BeforeState: before, StateNote: beforeNote, Rollback: rollbackState,
	}); err != nil {
		return res, fmt.Errorf("audit(before) write failed; mutation aborted: %w", err)
	}

	// 4. Execute.
	opID, execErr := m.Execute(ctx)
	res.OperationID = opID

	// 5. After state.
	after, afterErr := m.AfterState()
	afterNote := ""
	if afterErr != nil && !isStateUnavailable(afterErr) {
		afterNote = "after-state error: " + afterErr.Error()
	} else if afterErr != nil {
		afterNote = "AFTER_STATE_UNAVAILABLE"
	}

	// 6. Status classification (never convert uncertainty into success).
	switch {
	case execErr != nil:
		res.Status = statusFailed
	case afterErr != nil:
		res.Status = statusStateUnknown
	default:
		res.Status = statusCompleted
	}

	// 7. AUDIT(after). A missing after-entry must be loud.
	audErr := appendAudit(auditLog, m, auditPayload{
		ActionID: actionID, Phase: "after", Target: m.Target(),
		Status: res.Status, BeforeState: before, AfterState: after,
		StateNote: afterNote, OperationID: opID,
		Error: errString(execErr), Rollback: rollbackState,
	})

	switch {
	case execErr != nil:
		return res, execErr
	case audErr != nil:
		return res, fmt.Errorf("mutation succeeded but AUDIT(after) write failed (%v); status is not fully auditable", audErr)
	}

	// Journal entry (best effort; surfaced, never silently dropped when
	// the workspace supports it).
	_ = ws.LogEvent("mutation", fmt.Sprintf("%s %s %s (action %s)", m.Kind(), m.Target(), res.Status, actionID))

	return res, nil
}

func openAudit(ws *workspace.Workspace) (*store.Log, error) {
	return store.New(
		filepath.Join(ws.Root, "db", "audit.jsonl"),
		filepath.Join(ws.Root, "db", "audit.key"),
	)
}

func appendAudit(log *store.Log, m Mutation, p auditPayload) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = log.Append("mutation:"+m.Kind(), string(raw))
	return err
}

func isStateUnavailable(err error) bool { return err == ErrStateUnavailable }

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func newActionID() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("mut-%d-%s", time.Now().UnixNano(), hex.EncodeToString(b[:])), nil
}
