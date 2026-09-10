package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Debajyoti0-0/aether/internal/api"
	"github.com/Debajyoti0-0/aether/internal/engine/spine"
)

// serveCommandRunner dispatches one authenticated remote command
// through the Action spine (Stage 3, T3). The teamserver is a
// transport/authz layer — it never executes provider operations
// directly. Pipeline:
//
//	cert-derived operator (server) → capability check → intent whitelist
//	  → spine.Run (AuthZ → Risk → Policy → Approval → Execute →
//	    Evidence → Audit → Rollback) → correlated response
func serveCommandRunner(op *api.Operator, req *api.CommandRequest) (*api.CommandResponse, error) {
	if tsWorkspaceOp == nil {
		return nil, fmt.Errorf("no workspace attached to the teamserver (pass --workspace)")
	}
	fields := strings.Fields(strings.TrimPrefix(strings.TrimSpace(req.CommandLine), "aether "))
	if len(fields) < 2 || !isMutatingIntent(fields) {
		return &api.CommandResponse{
			Status: string(spine.StatusAborted),
			Error:  "command not permitted on teamserver: only whitelisted mutating intents (exec azure|aws|github|gcp, simulate stream) are accepted",
		}, nil
	}

	mut, kind, target, err := buildIntentMutation(fields)
	if err != nil {
		return &api.CommandResponse{Status: string(spine.StatusAborted), Error: err.Error()}, nil
	}

	// Capability authorization (cert-derived operator, server-enforced).
	requiredCap := capabilityForKind(kind)
	if !op.CanExecute(requiredCap) {
		return &api.CommandResponse{
			ActionID: kind,
			Status:   string(spine.StatusAborted),
			Error:    fmt.Sprintf("operator %q lacks capability %q", op.Name, requiredCap),
		}, nil
	}

	s := spine.New(tsWorkspaceOp)
	s.Caps = op.Caps // workspace+capability authorization from the cert
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	res, err := s.Run(ctx, &spine.Action{
		Kind:         kind,
		Target:       target,
		Actor:        "operator:" + op.Name, // cert-derived, recorded in the audit chain
		Mutation:     mut,
		RequiredCaps: []string{requiredCap},
		ApprovalMode: spine.ApprovalAuto, // automation only; every other stage still runs and is audited
	})
	if res == nil {
		return nil, err
	}
	resp := &api.CommandResponse{
		ActionID:    res.ActionID,
		Status:      string(res.Status),
		Stage:       string(res.Stage),
		OperationID: res.OperationID,
	}
	if err != nil {
		resp.Error = err.Error()
	}
	return resp, err
}

// capabilityForKind maps a whitelisted intent kind to its canonical
// capability identifier.
func capabilityForKind(kind string) string {
	switch kind {
	case "exec.azure":
		return api.CapExecAzure
	case "exec.aws":
		return api.CapExecAWS
	case "exec.github":
		return api.CapExecGitHub
	case "exec.gcp":
		return api.CapExecGCP
	case "simulate.stream":
		return api.CapSimulateStream
	default:
		return "unknown"
	}
}
