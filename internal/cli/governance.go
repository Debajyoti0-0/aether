package cli

import (
	"context"
	"fmt"

	"github.com/Debajyoti0-0/aether/internal/engine/mutation"
	"github.com/Debajyoti0-0/aether/internal/workspace"
)

// Governance helpers for Stage 1 (forensic F5): every mutating command
// opens a workspace and executes through the mutation pipeline, so each
// external effect produces signed audit entries and a rollback record.

// govWorkspace is the shared --workspace flag for mutating commands.
var govWorkspace string

// openGovernedWorkspace resolves the workspace a mutating command must
// record into. Fail-closed: no workspace → no mutation.
func openGovernedWorkspace(name string) (*workspace.Workspace, error) {
	if name == "" {
		return nil, fmt.Errorf(
			"this command mutates external state and requires --workspace: " +
				"audit and rollback records are mandatory (pass --workspace <name>; " +
				"passphrase via AETHER_PASSPHRASE)")
	}
	w, err := workspaceOpen(name)
	if err != nil {
		return nil, fmt.Errorf("open workspace %q (governance requires a passphrase-protected workspace): %w", name, err)
	}
	return w, nil
}

// cliMutation adapts CLI handlers to the mutation.Mutation contract.
type cliMutation struct {
	kindV    string
	targetV  string
	beforeFn func() ([]byte, error)
	execFn   func(ctx context.Context) (string, error)
	afterFn  func() ([]byte, error)
	undoV    *mutation.UndoSpec
}

func (m *cliMutation) Kind() string   { return m.kindV }
func (m *cliMutation) Target() string { return m.targetV }
func (m *cliMutation) BeforeState() ([]byte, error) {
	if m.beforeFn == nil {
		return nil, mutation.ErrStateUnavailable
	}
	return m.beforeFn()
}
func (m *cliMutation) Execute(ctx context.Context) (string, error) {
	return m.execFn(ctx)
}
func (m *cliMutation) AfterState() ([]byte, error) {
	if m.afterFn == nil {
		return nil, mutation.ErrStateUnavailable
	}
	return m.afterFn()
}
func (m *cliMutation) UndoRecipe() *mutation.UndoSpec { return m.undoV }

// irreversibleShell marks shell-style mutations with no safe reversal
// path (remote command execution, telemetry injection, CI dispatch).
func irreversibleShell() *mutation.UndoSpec {
	return &mutation.UndoSpec{Irreversible: true, Detail: "remote command execution has no safe automatic reversal"}
}
