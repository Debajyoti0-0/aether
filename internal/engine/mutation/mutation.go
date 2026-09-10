// Package mutation defines the single controlled execution boundary for
// every external mutation Aether performs (Stage 1, forensic F5).
//
// Invariant: an external mutation cannot occur without passing through
// Pipeline.Run, which captures before/after state, writes signed audit
// entries, and registers a rollback entry BEFORE the mutation executes.
package mutation

import (
	"context"
	"errors"
)

// ErrStateUnavailable is returned by BeforeState/AfterState when the
// provider exposes no safe way to snapshot the target. It is recorded
// as BEFORE_STATE_UNAVAILABLE / AFTER_STATE_UNAVAILABLE — never
// fabricated.
var ErrStateUnavailable = errors.New("state unavailable for this mutation type")

// Mutation is one externally-visible state change.
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
	// pipeline stamps the returned entry with the action ID.
	UndoRecipe() *UndoSpec
}
