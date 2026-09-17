package observability

import (
	"context"
	"net/http"
	"time"

	"github.com/Debajyoti0-0/aether/internal/workspace"
)

// Config holds configuration for the observability server.
type Config struct {
	Enabled        bool
	ListenAddress  string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	ShutdownGrace  time.Duration
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:       true,
		ListenAddress: "127.0.0.1:9090",
		ReadTimeout:   10 * time.Second,
		WriteTimeout:  10 * time.Second,
		IdleTimeout:   60 * time.Second,
		ShutdownGrace: 5 * time.Second,
	}
}

// ObservabilityServer bundles metrics and health endpoints.
type ObservabilityServer struct {
	metrics *Metrics
	health  *HealthServer
	server  *http.Server
	config  Config
}

// NewObservabilityServer creates a new observability server.
func NewObservabilityServer(
	vault *workspace.Workspace,
	auditKey string,
	config Config,
) *ObservabilityServer {
	metrics := NewMetrics()
	health := NewHealthServer(vault, "")

	obs := &ObservabilityServer{
		metrics: metrics,
		health:  health,
		config:  config,
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.MetricsHandler())
	mux.HandleFunc("/healthz", health.HealthzHandler)
	mux.HandleFunc("/readyz", health.ReadyzHandler)

	obs.server = &http.Server{
		Addr:         config.ListenAddress,
		Handler:      mux,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
	}

	return obs
}

// Start starts the observability server.
func (s *ObservabilityServer) Start() error {
	if !s.config.Enabled {
		return nil
	}
	return s.server.ListenAndServe()
}

// Stop gracefully stops the observability server.
func (s *ObservabilityServer) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

// Metrics returns the metrics instance.
func (s *ObservabilityServer) Metrics() *Metrics {
	return s.metrics
}

// Health returns the health server.
func (s *ObservabilityServer) Health() *HealthServer {
	return s.health
}

// StartSystemMetricsUpdater starts a background goroutine to update system metrics.
func (s *ObservabilityServer) StartSystemMetricsUpdater(ctx context.Context, interval time.Duration) {
	if !s.config.Enabled {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.metrics.UpdateSystemMetrics()
			}
		}
	}()
}