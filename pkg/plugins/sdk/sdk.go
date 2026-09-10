// Package sdk is the Aether plugin SDK: official providers implement
// this contract and register through plugins.Registry.
package sdk

import (
	"context"
	"fmt"
)

// Host provides services to plugins.
type Host interface {
	Logf(format string, args ...any)
}

// Plugin is the extension point for new providers.
type Plugin interface {
	Name() string
	Version() string
	Init(host Host) error
}

// Provider is the capability surface a provider plugin implements.
type Provider interface {
	ValidateToken(ctx context.Context) error
	Execute(ctx context.Context, target, command string) (*Result, error)
}

// Result is the outcome of a plugin Execute call.
type Result struct {
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
	Status   string `json:"status,omitempty"`
}

// Adapter bridges a functional plugin to the Plugin + Provider
// interfaces.
type Adapter struct {
	NameV     string
	VersionV  string
	ProviderV Provider
}

// NewAdapter builds an adapter.
func NewAdapter(name, version string, prov Provider) *Adapter {
	return &Adapter{NameV: name, VersionV: version, ProviderV: prov}
}

func (a *Adapter) Name() string    { return a.NameV }
func (a *Adapter) Version() string { return a.VersionV }
func (a *Adapter) Init(_ Host) error {
	if a.ProviderV == nil {
		return fmt.Errorf("plugin %s has no provider", a.NameV)
	}
	return nil
}

// Provider returns the wrapped provider.
func (a *Adapter) Provider() Provider { return a.ProviderV }
