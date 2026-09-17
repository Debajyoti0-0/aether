package observability

import (
	"net/http"
	"runtime"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics for Aether.
type Metrics struct {
	// HTTP metrics
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec

	// System metrics
	Goroutines       prometheus.Gauge
	MemoryAllocBytes prometheus.Gauge
	MemorySysBytes   prometheus.Gauge

	// Aether-specific metrics
	CommandsExecuted   *prometheus.CounterVec
	CommandDuration    *prometheus.HistogramVec
	CommandErrors      *prometheus.CounterVec
	SpineOperations    *prometheus.CounterVec
	SpineDuration      *prometheus.HistogramVec
	VaultOperations    *prometheus.CounterVec
	VaultDuration      *prometheus.HistogramVec
	ProviderOperations *prometheus.CounterVec
	ProviderDuration   *prometheus.HistogramVec
	ProviderErrors     *prometheus.CounterVec
}

// NewMetrics creates and registers all metrics.
func NewMetrics() *Metrics {
	m := &Metrics{
		HTTPRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "aether_http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "endpoint", "status"},
		),
		HTTPRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "aether_http_request_duration_seconds",
				Help:    "HTTP request latency in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "endpoint"},
		),
		Goroutines: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "aether_goroutines",
				Help: "Number of goroutines",
			},
		),
		MemoryAllocBytes: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "aether_memory_alloc_bytes",
				Help: "Memory allocated in bytes",
			},
		),
		MemorySysBytes: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "aether_memory_sys_bytes",
				Help: "Memory system bytes",
			},
		),
		CommandsExecuted: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "aether_commands_executed_total",
				Help: "Total number of commands executed",
			},
			[]string{"command", "status"},
		),
		CommandDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "aether_command_duration_seconds",
				Help:    "Command execution duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"command"},
		),
		CommandErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "aether_command_errors_total",
				Help: "Total number of command errors",
			},
			[]string{"command", "error_type"},
		),
		SpineOperations: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "aether_spine_operations_total",
				Help: "Total number of spine operations",
			},
			[]string{"operation", "status"},
		),
		SpineDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "aether_spine_duration_seconds",
				Help:    "Spine operation duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation"},
		),
		VaultOperations: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "aether_vault_operations_total",
				Help: "Total number of vault operations",
			},
			[]string{"operation", "status"},
		),
		VaultDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "aether_vault_duration_seconds",
				Help:    "Vault operation duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation"},
		),
		ProviderOperations: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "aether_provider_operations_total",
				Help: "Total number of provider operations",
			},
			[]string{"provider", "operation", "status"},
		),
		ProviderDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "aether_provider_duration_seconds",
				Help:    "Provider operation duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"provider", "operation"},
		),
		ProviderErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "aether_provider_errors_total",
				Help: "Total number of provider errors",
			},
			[]string{"provider", "error_type"},
		),
	}
	return m
}

// UpdateSystemMetrics updates system-level metrics.
func (m *Metrics) UpdateSystemMetrics() {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	m.Goroutines.Set(float64(runtime.NumGoroutine()))
	m.MemoryAllocBytes.Set(float64(ms.Alloc))
	m.MemorySysBytes.Set(float64(ms.Sys))
}

// MetricsHandler returns the Prometheus metrics HTTP handler.
func (m *Metrics) MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// RecordHTTPRequest records an HTTP request metric.
func (m *Metrics) RecordHTTPRequest(method, endpoint string, status int) {
	m.HTTPRequestsTotal.WithLabelValues(method, endpoint, httpStatusString(status)).Inc()
}

// RecordHTTPDuration records an HTTP request duration.
func (m *Metrics) RecordHTTPDuration(method, endpoint string, duration float64) {
	m.HTTPRequestDuration.WithLabelValues(method, endpoint).Observe(duration)
}

// RecordCommand records a command execution.
func (m *Metrics) RecordCommand(command, status string) {
	m.CommandsExecuted.WithLabelValues(command, status).Inc()
}

// RecordCommandDuration records a command duration.
func (m *Metrics) RecordCommandDuration(command string, duration float64) {
	m.CommandDuration.WithLabelValues(command).Observe(duration)
}

// RecordCommandError records a command error.
func (m *Metrics) RecordCommandError(command, errorType string) {
	m.CommandErrors.WithLabelValues(command, errorType).Inc()
}

// RecordSpineOperation records a spine operation.
func (m *Metrics) RecordSpineOperation(operation, status string) {
	m.SpineOperations.WithLabelValues(operation, status).Inc()
}

// RecordSpineDuration records a spine operation duration.
func (m *Metrics) RecordSpineDuration(operation string, duration float64) {
	m.SpineDuration.WithLabelValues(operation).Observe(duration)
}

// RecordVaultOperation records a vault operation.
func (m *Metrics) RecordVaultOperation(operation, status string) {
	m.VaultOperations.WithLabelValues(operation, status).Inc()
}

// RecordVaultDuration records a vault operation duration.
func (m *Metrics) RecordVaultDuration(operation string, duration float64) {
	m.VaultDuration.WithLabelValues(operation).Observe(duration)
}

// RecordProviderOperation records a provider operation.
func (m *Metrics) RecordProviderOperation(provider, operation, status string) {
	m.ProviderOperations.WithLabelValues(provider, operation, status).Inc()
}

// RecordProviderDuration records a provider operation duration.
func (m *Metrics) RecordProviderDuration(provider, operation string, duration float64) {
	m.ProviderDuration.WithLabelValues(provider, operation).Observe(duration)
}

// RecordProviderError records a provider error.
func (m *Metrics) RecordProviderError(provider, errorType string) {
	m.ProviderErrors.WithLabelValues(provider, errorType).Inc()
}

func httpStatusString(status int) string {
	if status >= 200 && status < 300 {
		return "2xx"
	}
	if status >= 300 && status < 400 {
		return "3xx"
	}
	if status >= 400 && status < 500 {
		return "4xx"
	}
	return "5xx"
}