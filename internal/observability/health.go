package observability

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Debajyoti0-0/aether/internal/workspace"
)

// HealthChecker defines the interface for health checks.
type HealthChecker interface {
	// Check performs the health check and returns an error if unhealthy.
	Check(ctx context.Context) error
}

// HealthStatus represents the health status response.
type HealthStatus struct {
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Checks    map[string]string `json:"checks,omitempty"`
}

// HealthServer provides /healthz and /readyz endpoints.
type HealthServer struct {
	vault       *workspace.Workspace
	auditKey    string
	readyChecks []HealthChecker
}

// NewHealthServer creates a new health server.
func NewHealthServer(vault *workspace.Workspace, auditKey string) *HealthServer {
	return &HealthServer{
		vault:       vault,
		auditKey:    auditKey,
		readyChecks: []HealthChecker{},
	}
}

// AddReadyCheck adds a readiness check.
func (h *HealthServer) AddReadyCheck(check HealthChecker) {
	h.readyChecks = append(h.readyChecks, check)
}

// HealthzHandler handles the liveness probe endpoint.
func (h *HealthServer) HealthzHandler(w http.ResponseWriter, r *http.Request) {
	status := HealthStatus{
		Status:    "ok",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// ReadyzHandler handles the readiness probe endpoint.
func (h *HealthServer) ReadyzHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	status := HealthStatus{
		Status:    "ok",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    make(map[string]string),
	}

	allReady := true

	// Check vault accessibility
	if h.vault != nil {
		v := h.vault.Vault()
		if v != nil {
			// Try a simple operation to verify vault is accessible
			if err := v.Sync(); err != nil {
				status.Checks["vault"] = "unhealthy: " + err.Error()
				allReady = false
			} else {
				status.Checks["vault"] = "ok"
			}
		} else {
			status.Checks["vault"] = "not initialized"
			allReady = false
		}
	} else {
		status.Checks["vault"] = "not attached"
		allReady = false
	}

	// Check audit key
	if h.auditKey != "" {
		status.Checks["audit_key"] = "loaded"
	} else {
		status.Checks["audit_key"] = "not loaded"
		allReady = false
	}

	// Run additional ready checks
	for i, check := range h.readyChecks {
		checkName := "custom_check"
		if err := check.Check(ctx); err != nil {
			status.Checks[checkName+string(rune(i+'0'))] = "unhealthy: " + err.Error()
			allReady = false
		} else {
			status.Checks[checkName+string(rune(i+'0'))] = "ok"
		}
	}

	if !allReady {
		status.Status = "not ready"
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// PingHealthChecker implements a simple ping health check.
type PingHealthChecker struct {
	Name string
	Fn   func(ctx context.Context) error
}

func (p *PingHealthChecker) Check(ctx context.Context) error {
	return p.Fn(ctx)
}