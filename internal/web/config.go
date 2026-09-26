package web

import (
	"fmt"
	"net"
	"time"
)

// Config controls the dashboard server.
type Config struct {
	// Port is the TCP port to listen on.
	Port int
	// BindAll opts out of loopback-only binding. It is deliberately explicit and
	// requires AllowRemoteBinding so that a default config can never expose the
	// engagement graph to the network by accident.
	BindAll bool
	// AllowRemoteBinding is the second, independent switch required for a
	// non-loopback bind. Two switches exist so that neither a config file nor a
	// single stray flag can do it.
	AllowRemoteBinding bool
	// TLSCert and TLSKey enable HTTPS when both are set.
	TLSCert string
	TLSKey  string
	// Workspace is the engagement/workspace name shown in the UI.
	Workspace string
	// Operator is the operator identity recorded in audit entries.
	Operator string
	// Capability, when set, is required for every request. An empty value means
	// no capability gate, which is only appropriate for a loopback-only local
	// dashboard; BindAll without a capability is refused at Start.
	Capability string
	// MaxGraphBytes bounds a single graph JSON response.
	MaxGraphBytes int64
	// ReadTimeout / WriteTimeout bound a single request. They are set because a
	// dashboard holds long-poll style WebSocket connections, so they are
	// applied per-response rather than to the whole server.
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

const (
	defaultMaxGraphBytes = 256 << 20 // 256 MiB
	defaultReadTimeout   = 15 * time.Second
	defaultWriteTimeout  = 60 * time.Second
)

func (c Config) withDefaults() Config {
	if c.Port <= 0 {
		c.Port = 8443
	}
	if c.MaxGraphBytes <= 0 {
		c.MaxGraphBytes = defaultMaxGraphBytes
	}
	if c.ReadTimeout <= 0 {
		c.ReadTimeout = defaultReadTimeout
	}
	if c.WriteTimeout <= 0 {
		c.WriteTimeout = defaultWriteTimeout
	}
	return c
}

// Addr returns the listen address.
//
// Loopback is the default and the only mode that requires no further
// justification. Any other bind is refused unless both opt-in switches are set,
// because this process serves the full engagement graph and the audit chain.
func (c Config) Addr() (string, error) {
	c = c.withDefaults()
	if !c.BindAll {
		return net.JoinHostPort("127.0.0.1", fmt.Sprint(c.Port)), nil
	}
	if !c.AllowRemoteBinding {
		return "", fmt.Errorf(
			"refusing to bind a non-loopback address: --bind-all also requires --allow-remote-binding. " +
				"This server exposes the engagement graph and the signed audit chain without authentication " +
				"unless a capability is configured")
	}
	if c.Capability == "" {
		return "", fmt.Errorf(
			"refusing to bind a non-loopback address with no capability configured: " +
				"set --require-capability so remote reads are at least capability-gated")
	}
	return net.JoinHostPort("", fmt.Sprint(c.Port)), nil
}

// RemoteBindWarning is shown in the UI when the dashboard is reachable off
// loopback, so the operator is told on the page rather than only in a log line
// they may never read.
func (c Config) RemoteBindWarning() string {
	if !c.BindAll {
		return ""
	}
	return "WARNING: this dashboard is bound to all interfaces and is reachable from the network. " +
		"It is read-only, but the engagement graph and the full audit chain are exposed to anyone who can reach it."
}
