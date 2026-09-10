// Package spine is the canonical Action control plane (Stage 2,
// forensic F7). Every external mutation flows through Spine.Run:
//
//	AUTHZ â†’ RISK â†’ POLICY â†’ APPROVAL â†’ before-state â†’ AUDIT(before) â†’
//	ROLLBACK REGISTRATION â†’ EXECUTE â†’ after-state â†’ EVIDENCE â†’ AUDIT(after)
//
// Fail-closed rules (inherited from Stage 1, unchanged):
//   - authorization, risk, or policy rejection â‡’ no execution;
//   - audit chain unavailable â‡’ no execution;
//   - reversible-mutation rollback registration failure â‡’ no execution;
//   - after-state capture failure â‡’ completed_state_unknown, never bare
//     success.
//
// Approval semantics (the Stage-1 `--auto` rule): ApprovalModeAuto
// removes the human confirmation prompt but NEVER bypasses
// authorization, risk, policy, audit, rollback, or evidence. Every
// stage's decision is written into the signed audit trail.
package spine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Debajyoti0-0/aether/internal/engine/rollback"
	"github.com/Debajyoti0-0/aether/internal/store"
	"github.com/Debajyoti0-0/aether/internal/types"
	"github.com/Debajyoti0-0/aether/internal/workspace"
)

// ErrStateUnavailable is returned by BeforeState/AfterState when the
// provider exposes no safe way to snapshot the target. It is recorded
// as BEFORE_STATE_UNAVAILABLE / AFTER_STATE_UNAVAILABLE â€” never
// fabricated.
var ErrStateUnavailable = errors.New("state unavailable for this mutation type")

// Mutation is the execution contract of one externally-visible state
// change (the Stage 1 Mutation contract, canonicalized here).
type Mutation interface {
	// Kind classifies the mutation: "exec.azure", "exec.aws",
	// "exec.github", "simulate.stream", "plugins.install", ...
	Kind() string
	// Target identifies what is mutated (VM id, instance id, repo,
	// plugin name@version, HEC endpoint, ...).
	Target() string
	// BeforeState returns a JSON-serializable snapshot taken before the
	// mutation. Return (nil, ErrStateUnavailable) when not collectible.
	BeforeState() ([]byte, error)
	// Execute performs the mutation. It returns the provider operation
	// identifier when the provider issues one (Azure operation id, SSM
	// CommandId, GitHub run id, ...), otherwise "".
	Execute(ctx context.Context) (OperationID string, err error)
	// AfterState returns a JSON-serializable snapshot taken after the
	// mutation. Return (nil, ErrStateUnavailable) when not collectible.
	AfterState() ([]byte, error)
	// UndoRecipe returns the rollback entry describing how to reverse
	// this mutation, or nil when the mutation is irreversible. The
	// spine stamps the returned entry with the action ID.
	UndoRecipe() *UndoSpec
}

// UndoSpec is the rollback recipe declared by a Mutation. It mirrors
// rollback.Action minus the fields the spine owns (ID, Timestamp,
// Kind, Target, Metadata) so the spine can correlate the rollback
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

// Stage names a control-plane stage.
type Stage string

const (
	StageAuthZ    Stage = "authz"
	StageRisk     Stage = "risk"
	StagePolicy   Stage = "policy"
	StageApproval Stage = "approval"
	StageExecute  Stage = "execute"
	StageEvidence Stage = "evidence"
	StageAudit    Stage = "audit"
	StageRollback Stage = "rollback"
)

// Status is the terminal classification of an Action.
type Status string

const (
	StatusCompleted     Status = "completed"
	StatusFailed        Status = "failed"
	StatusAborted       Status = "aborted"
	StatusStateUnknown  Status = "completed_state_unknown"
	StatusAbortedRBReg  Status = "aborted_rollback_registration"
	StatusRollbackPend  Status = "rollback_pending"
	StatusRolledBack    Status = "rolled_back"
	StatusRollbackFail  Status = "rollback_failed"
)

// ApprovalMode selects how the approval stage resolves.
type ApprovalMode string

const (
	// ApprovalAuto removes the human confirmation ONLY; all other
	// stages still run and are audited.
	ApprovalAuto ApprovalMode = "auto"
	// ApprovalInteractive requires an Approver callback to accept.
	ApprovalInteractive ApprovalMode = "interactive"
	// ApprovalPreApproved requires a prior approval reference.
	ApprovalPreApproved ApprovalMode = "pre-approved"
)

// Action is one governed operation.
type Action struct {
	ID           string            // minted by the spine
	Kind         string            // "exec.azure", "prt.import", "dag.node.exec.azure", ...
	Target       string
	Actor        string            // "cli" for local operators; operator ID for the teamserver
	Payload      map[string]any    // structured intent metadata (no secrets)
	Mutation     Mutation // the execution contract (BeforeState/Execute/AfterState/UndoRecipe)
	RequiredCaps []string          // capability identifiers checked by StageAuthZ
	RiskScore    int               // 0-100; set before the risk stage or by the risk evaluator
	PolicyRefs   []string          // policy IDs consulted (filled by the policy stage)
	ApprovalMode ApprovalMode
	// ApprovalRef carries a prior-approval reference for
	// ApprovalPreApproved mode.
	ApprovalRef string
}

// Result reports the spine outcome.
type Result struct {
	ActionID    string
	Kind        string
	Target      string
	Stage       Stage  // last successful stage
	Status      Status
	OperationID string
	Error       string
	EvidenceIDs []string
}

// Capabilities answers capability questions for StageAuthZ. The local
// CLI uses AllowAll; the teamserver maps operator certificates to
// capability sets (Stage 3).
type Capabilities interface {
	Has(capability string) bool
}

// AllowAll grants every capability â€” the local CLI operator model.
type AllowAll struct{}

func (AllowAll) Has(string) bool { return true }

// RiskEvaluator scores an action (0-100). Nil default: the action's
// pre-set RiskScore passes through unchanged.
type RiskEvaluator func(a *Action) int

// PolicyEvaluator rejects an action with an error when a deny rule
// matches. Nil default: no policy set loaded â†’ allow (recorded as
// "no policy set").
type PolicyEvaluator func(a *Action) error

// Approver resolves interactive approval. Nil default with
// ApprovalInteractive: refuse (fail closed).
type Approver func(a *Action) bool

// Hooks are test seams for failure injection. In production all are
// nil and the spine uses the workspace's real stores.
type Hooks struct {
	AuditLog      func() (*store.Log, error)
	RollbackStack func() *rollback.Stack
}

// Spine is bound to one workspace.
type Spine struct {
	ws      *workspace.Workspace
	Caps    Capabilities
	MaxRisk int
	Risk    RiskEvaluator
	Policy  PolicyEvaluator
	Approve Approver
	Hooks   Hooks
}

// New binds a spine to a workspace with the local-CLI defaults:
// AllowAll capabilities, MaxRisk from the aether config (default 50),
// no external risk/policy evaluator (the action's pre-set RiskScore is
// checked against MaxRisk), no interactive approver.
func New(ws *workspace.Workspace) *Spine {
	maxRisk := 50
	if cfg, err := store.LoadConfig(""); err == nil && cfg.MaxRiskThreshold > 0 {
		maxRisk = cfg.MaxRiskThreshold
	}
	return &Spine{ws: ws, Caps: AllowAll{}, MaxRisk: maxRisk}
}

// Run executes one Action through the full stage pipeline.
func (s *Spine) Run(ctx context.Context, a *Action) (*Result, error) {
	if a.Mutation == nil {
		return nil, fmt.Errorf("action %q has no mutation contract", a.Kind)
	}
	if a.ID == "" {
		id, err := newActionID()
		if err != nil {
			return nil, fmt.Errorf("generate action id: %w", err)
		}
		a.ID = id
	}
	res := &Result{ActionID: a.ID, Kind: a.Kind, Target: a.Target}

	// ---- StageAuthZ ----
	if err := s.stageAuthz(a); err != nil {
		return s.abort(res, StageAuthZ, StatusAborted, err)
	}
	res.Stage = StageAuthZ

	// ---- StageRisk ----
	if err := s.stageRisk(a); err != nil {
		return s.abort(res, StageRisk, StatusAborted, err)
	}
	res.Stage = StageRisk

	// ---- StagePolicy ----
	if err := s.stagePolicy(a); err != nil {
		return s.abort(res, StagePolicy, StatusAborted, err)
	}
	res.Stage = StagePolicy

	// ---- StageApproval ----
	if err := s.stageApproval(a); err != nil {
		return s.abort(res, StageApproval, StatusAborted, err)
	}
	res.Stage = StageApproval

	// ---- EXECUTION PIPELINE (Stage-1 semantics preserved) ----
	auditLog, err := s.auditLog()
	if err != nil {
		return s.abort(res, StageAudit, StatusAborted,
			fmt.Errorf("open audit chain (action refused â€” governance unavailable): %w", err))
	}
	stack := s.rollbackStack()

	// before-state
	before, beforeErr := a.Mutation.BeforeState()
	beforeNote := ""
	if beforeErr != nil {
		if beforeErr != ErrStateUnavailable {
			return s.abort(res, StageExecute, StatusAborted, fmt.Errorf("capture before state: %w", beforeErr))
		}
		beforeNote = "BEFORE_STATE_UNAVAILABLE"
	}

	// rollback registration BEFORE execution
	rollbackState := "irreversible"
	recipe := a.Mutation.UndoRecipe()
	if recipe != nil && !recipe.Irreversible {
		entry := rollback.Action{
			ID:     a.ID,
			Kind:   a.Kind,
			Target: a.Target,
			Detail: recipe.Detail,
			Undo: rollback.UndoAction{
				Provider: recipe.Provider,
				Op:       recipe.Op,
				Args:     recipe.Args,
				Command:  recipe.Command,
			},
			Metadata: map[string]string{
				"action_id":     a.ID,
				"actor":         a.Actor,
				"approval_mode": string(a.ApprovalMode),
			},
		}
		if err := stack.Push(entry); err != nil {
			_ = appendAudit(auditLog, a, auditPayload{
				ActionID: a.ID, Phase: "before", Target: a.Target,
				Status: string(StatusAbortedRBReg), StateNote: beforeNote,
				Error:        "rollback registration failed: " + err.Error(),
				ApprovalMode: string(a.ApprovalMode),
			})
			res.Stage, res.Status = StageRollback, StatusAbortedRBReg
			res.Error = err.Error()
			return res, fmt.Errorf("rollback registration failed; action aborted: %w", err)
		}
		rollbackState = "registered"
	}

	// AUDIT(before)
	if err := appendAudit(auditLog, a, auditPayload{
		ActionID: a.ID, Phase: "before", Target: a.Target,
		BeforeState: before, StateNote: beforeNote, Rollback: rollbackState,
		RiskScore: a.RiskScore, PolicyRefs: a.PolicyRefs,
		ApprovalMode: string(a.ApprovalMode), ApprovalRef: a.ApprovalRef,
		Actor: a.Actor,
	}); err != nil {
		res.Stage, res.Status = StageAudit, StatusAborted
		res.Error = err.Error()
		return res, fmt.Errorf("audit(before) write failed; action aborted: %w", err)
	}
	res.Stage = StageAudit

	// EXECUTE
	opID, execErr := a.Mutation.Execute(ctx)
	res.OperationID = opID

	// after-state
	after, afterErr := a.Mutation.AfterState()
	afterNote := ""
	if afterErr != nil {
		if afterErr == ErrStateUnavailable {
			afterNote = "AFTER_STATE_UNAVAILABLE"
		} else {
			afterNote = "after-state error: " + afterErr.Error()
		}
	}

	switch {
	case execErr != nil:
		res.Status = StatusFailed
	case afterErr != nil:
		res.Status = StatusStateUnknown
	default:
		res.Status = StatusCompleted
	}
	res.Stage = StageExecute

	// EVIDENCE: one record per action (T3 seed).
	evIDs := s.writeEvidence(a, before, after, afterErr, opID, execErr)
	res.EvidenceIDs = evIDs
	res.Stage = StageEvidence

	// AUDIT(after) â€” a missing after-entry must be loud.
	audErr := appendAudit(auditLog, a, auditPayload{
		ActionID: a.ID, Phase: "after", Target: a.Target,
		Status: string(res.Status), BeforeState: before, AfterState: after,
		StateNote: afterNote, OperationID: opID,
		Error: errString(execErr), Rollback: rollbackState,
		RiskScore: a.RiskScore, PolicyRefs: a.PolicyRefs,
		ApprovalMode: string(a.ApprovalMode), ApprovalRef: a.ApprovalRef,
		Actor:        a.Actor,
		EvidenceRefs: evIDs,
	})
	if audErr != nil {
		res.Error = audErr.Error()
		if execErr == nil {
			return res, fmt.Errorf("action succeeded but AUDIT(after) write failed (%w); status is not fully auditable", audErr)
		}
	}

	if execErr != nil {
		res.Error = execErr.Error()
		return res, execErr
	}

	// Journal entry.
	_ = s.ws.LogEvent("action", fmt.Sprintf("%s %s %s (action %s, actor %s, approval %s)",
		a.Kind, a.Target, res.Status, a.ID, a.Actor, a.ApprovalMode))

	return res, nil
}

func (s *Spine) abort(res *Result, stage Stage, status Status, err error) (*Result, error) {
	res.Stage, res.Status, res.Error = stage, status, err.Error()
	_ = s.ws.LogEvent("action_aborted", fmt.Sprintf("%s %s at stage %s: %v", res.Kind, res.Target, stage, err))
	return res, err
}

// ---- stage implementations ----

func (s *Spine) stageAuthz(a *Action) error {
	if s.Caps == nil {
		s.Caps = AllowAll{}
	}
	for _, cap := range a.RequiredCaps {
		if !s.Caps.Has(cap) {
			return fmt.Errorf("actor %q lacks required capability %q", a.Actor, cap)
		}
	}
	return nil
}

func (s *Spine) stageRisk(a *Action) error {
	if s.Risk != nil {
		a.RiskScore = s.Risk(a)
	}
	if a.RiskScore > s.MaxRisk {
		return fmt.Errorf("risk score %d exceeds the accepted maximum %d (action aborted)", a.RiskScore, s.MaxRisk)
	}
	return nil
}

func (s *Spine) stagePolicy(a *Action) error {
	if s.Policy == nil {
		a.PolicyRefs = []string{"none"}
		return nil
	}
	if err := s.Policy(a); err != nil {
		return fmt.Errorf("policy denial: %w", err)
	}
	a.PolicyRefs = []string{"evaluated"}
	return nil
}

func (s *Spine) stageApproval(a *Action) error {
	switch a.ApprovalMode {
	case "", ApprovalAuto:
		// Automation removes the human prompt; every other stage still
		// runs and the mode is recorded in the audit trail.
		a.ApprovalMode = ApprovalAuto
		return nil
	case ApprovalPreApproved:
		if a.ApprovalRef == "" {
			return fmt.Errorf("pre-approved action requires an ApprovalRef")
		}
		return nil
	case ApprovalInteractive:
		if s.Approve == nil {
			return fmt.Errorf("interactive approval required but no approver is available (refusing)")
		}
		if !s.Approve(a) {
			return fmt.Errorf("operator denied approval")
		}
		return nil
	default:
		return fmt.Errorf("unknown approval mode %q", a.ApprovalMode)
	}
}

func (s *Spine) auditLog() (*store.Log, error) {
	if s.Hooks.AuditLog != nil {
		return s.Hooks.AuditLog()
	}
	return s.ws.AuditLog()
}

func (s *Spine) rollbackStack() *rollback.Stack {
	if s.Hooks.RollbackStack != nil {
		return s.Hooks.RollbackStack()
	}
	return s.ws.RollbackStack()
}

// writeEvidence persists one EvidenceRecord per action into the
// evidence records bucket (T3 seed).
func (s *Spine) writeEvidence(a *Action, before, after []byte, afterErr error, opID string, execErr error) []string {
	class := types.ClassObserved
	confidence := 1.0
	payload := after
	if afterErr != nil || after == nil {
		class = types.ClassUnknown
		confidence = 0
		payload = before
	}
	if execErr != nil {
		class = types.ClassUnknown
		confidence = 0
	}
	ev := types.EvidenceRecord{
		ID:             a.ID + "-ev1",
		ActionID:       a.ID,
		Kind:           a.Kind + ".result",
		Target:         a.Target,
		EpistemicClass: class,
		Confidence:     confidence,
		CollectedAt:    time.Now().UTC(),
		Method:         "spine." + string(StageExecute),
		Payload:        payload,
	}
	if err := ev.Validate(); err != nil {
		// A malformed evidence record must never silently vanish.
		_ = s.ws.LogEvent("evidence_invalid", fmt.Sprintf("action %s: %v", a.ID, err))
		return nil
	}
	if err := s.ws.SaveRecord(workspace.BucketEvidence, ev.ID, ev); err != nil {
		_ = s.ws.LogEvent("evidence_write_failed", fmt.Sprintf("action %s: %v", a.ID, err))
		return nil
	}
	return []string{ev.ID}
}

// NewActionID mints a fresh unique action identifier.
func NewActionID() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("mut-%d-%s", time.Now().UnixNano(), hex.EncodeToString(b[:])), nil
}

func newActionID() (string, error) { return NewActionID() }

// ---- audit payload (Stage 1 format; new optional fields) ----

type auditPayload struct {
	ActionID     string          `json:"action_id"`
	Phase        string          `json:"phase"`
	Target       string          `json:"target"`
	Status       string          `json:"status,omitempty"`
	BeforeState  json.RawMessage `json:"before_state,omitempty"`
	AfterState   json.RawMessage `json:"after_state,omitempty"`
	StateNote    string          `json:"state_note,omitempty"`
	OperationID  string          `json:"operation_id,omitempty"`
	Error        string          `json:"error,omitempty"`
	Rollback     string          `json:"rollback,omitempty"`
	Actor        string          `json:"actor,omitempty"`
	RiskScore    int             `json:"risk_score,omitempty"`
	PolicyRefs   []string        `json:"policy_refs,omitempty"`
	ApprovalMode string          `json:"approval_mode,omitempty"`
	ApprovalRef  string          `json:"approval_ref,omitempty"`
	EvidenceRefs []string        `json:"evidence_refs,omitempty"`
}

func appendAudit(log *store.Log, a *Action, p auditPayload) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = log.Append("mutation:"+a.Kind, string(raw))
	return err
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

