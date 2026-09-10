package plugins

import (
	"fmt"
	"sync"

	"github.com/Debajyoti0-0/aether/pkg/plugins/sdk"
)

// Registry tracks loaded plugins and provides typed provider access.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]sdk.Plugin
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{plugins: map[string]sdk.Plugin{}}
}

// Register validates and registers a plugin.
func (r *Registry) Register(p sdk.Plugin) error {
	if p.Name() == "" {
		return fmt.Errorf("plugin name is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.plugins[p.Name()]; exists {
		return fmt.Errorf("plugin %q already registered", p.Name())
	}
	r.plugins[p.Name()] = p
	return nil
}

// Get returns a plugin by name.
func (r *Registry) Get(name string) (sdk.Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	return p, ok
}

// Names lists registered plugin names (unordered).
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		out = append(out, name)
	}
	return out
}

// InitAll initializes all registered plugins.
func (r *Registry) InitAll(host sdk.Host) error {
	r.mu.RLock()
	snapshot := make([]sdk.Plugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		snapshot = append(snapshot, p)
	}
	r.mu.RUnlock()

	for _, p := range snapshot {
		if err := p.Init(host); err != nil {
			return fmt.Errorf("init plugin %s: %w", p.Name(), err)
		}
	}
	return nil
}

// Provider returns the provider surface of a registered plugin.
func (r *Registry) Provider(name string) (sdk.Provider, error) {
	p, ok := r.Get(name)
	if !ok {
		return nil, fmt.Errorf("plugin %q not registered", name)
	}
	if adapter, ok := p.(*sdk.Adapter); ok {
		return adapter.Provider(), nil
	}
	return nil, fmt.Errorf("plugin %q carries no provider", name)
}
