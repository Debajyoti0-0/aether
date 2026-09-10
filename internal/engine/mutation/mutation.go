// Package mutation defines the single controlled execution contract
// for every external mutation Aether performs (Stage 1, forensic F5).
//
// Stage 2: the governance stages live in internal/engine/spine — the
// canonical Action spine. The Mutation interface and UndoSpec are
// aliases of the spine's canonical types; mutation.Run is a thin
// compatibility entry point that executes a Mutation through the spine
// with local-CLI defaults, so every Stage 1 caller is spine-native.
package mutation

import (
	"context"

	"github.com/Debajyoti0-0/aether/internal/engine/spine"
	"github.com/Debajyoti0-0/aether/internal/workspace"
)

// ErrStateUnavailable is returned by BeforeState/AfterState when the
// provider exposes no safe way to snapshot the target. It is recorded
// as BEFORE_STATE_UNAVAILABLE / AFTER_STATE_UNAVAILABLE — never
// fabricated.
var ErrStateUnavailable = spine.ErrStateUnavailable

// Mutation is one externally-visible state change (canonical type:
// spine.Mutation).
type Mutation = spine.Mutation

// UndoSpec is the rollback recipe declared by a Mutation (canonical
// type: spine.UndoSpec).
type UndoSpec = spine.UndoSpec

// Result is the outcome of a governed mutation.
type Result struct {
	ActionID    string `json:"action_id"`
	Kind        string `json:"kind"`
	Target      string `json:"target"`
	OperationID string `json:"operation_id,omitempty"`
	Status      string `json:"status"` // completed | failed | completed_state_unknown | aborted
}

// Run executes one mutation through the canonical Action spine with
// the local-CLI defaults (actor "cli", ApprovalAuto, AllowAll
// capabilities, config max-risk threshold).
//
// Fail-closed rules (unchanged from Stage 1):
//   - if the audit chain cannot be written, the mutation does NOT run;
//   - if a rollback recipe exists but registration fails, the mutation
//     does NOT run;
//   - if execution succeeds but the after-state cannot be captured, the
//     status is completed_state_unknown (never a bare success).
func Run(ctx context.Context, ws *workspace.Workspace, m Mutation) (Result, error) {
	s := spine.New(ws)
	res, err := s.Run(ctx, &spine.Action{
		Kind:         m.Kind(),
		Target:       m.Target(),
		Actor:        "cli",
		Mutation:     m,
		ApprovalMode: spine.ApprovalAuto,
	})
	if res != nil {
		out := Result{
			ActionID:    res.ActionID,
			Kind:        res.Kind,
			Target:      res.Target,
			OperationID: res.OperationID,
			Status:      string(res.Status),
		}
		return out, err
	}
	return Result{}, err
}
