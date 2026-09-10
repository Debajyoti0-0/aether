package providers

import (
	"context"

	"github.com/Debajyoti0-0/aether/internal/types"
)

// CloudProvider abstracts a cloud execution target so new providers
// (GCP, OCI, ...) can be plugged in.
type CloudProvider interface {
	// Name returns the provider identifier (e.g. "azure").
	Name() string
	// ValidateToken checks whether the credentials are usable.
	ValidateToken(ctx context.Context) error
	// Execute runs a command on a target (VM id, instance id, runner).
	Execute(ctx context.Context, target, command string) (*types.CommandResult, error)
}

// Registry holds registered providers.
type Registry struct {
	providers map[string]CloudProvider
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{providers: map[string]CloudProvider{}}
}

// Register adds a provider; it overwrites existing names.
func (r *Registry) Register(p CloudProvider) {
	r.providers[p.Name()] = p
}

// Get returns a provider by name.
func (r *Registry) Get(name string) (CloudProvider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

// Names lists registered provider names.
func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.providers))
	for name := range r.providers {
		out = append(out, name)
	}
	return out
}
