package plugins

import "fmt"

// Plugin is the extension point for adding new capabilities.
type Plugin interface {
	Name() string
	Version() string
	Init(host Host) error
}

// Host provides services to plugins.
type Host interface {
	Logf(format string, args ...any)
	Store() interface {
		Put(key string, value []byte) error
		Get(key string) ([]byte, error)
	}
}

// Registry tracks loaded plugins.
type Registry struct {
	plugins map[string]Plugin
}

// NewRegistry creates an empty plugin registry.
func NewRegistry() *Registry {
	return &Registry{plugins: map[string]Plugin{}}
}

// Register validates and registers a plugin.
func (r *Registry) Register(p Plugin) error {
	if p.Name() == "" {
		return fmt.Errorf("plugin name is required")
	}
	if _, exists := r.plugins[p.Name()]; exists {
		return fmt.Errorf("plugin %q already registered", p.Name())
	}
	r.plugins[p.Name()] = p
	return nil
}

// Get returns a plugin by name.
func (r *Registry) Get(name string) (Plugin, bool) {
	p, ok := r.plugins[name]
	return p, ok
}

// InitAll initializes all registered plugins.
func (r *Registry) InitAll(host Host) error {
	for name, p := range r.plugins {
		if err := p.Init(host); err != nil {
			return fmt.Errorf("init plugin %s: %w", name, err)
		}
	}
	return nil
}
