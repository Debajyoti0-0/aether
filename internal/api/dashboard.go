package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Dashboard is a lightweight web UI: it serves the identity graph
// visualization and workspace activity via JSON endpoints.
//
// Security model (Stage 1, forensic F3): the dashboard exposes
// engagement data (graph topology, event history), so every route
// requires a bearer token. The token is generated per start and must be
// passed as `Authorization: Bearer <t>`, `X-Aether-Token: <t>`, or a
// `?token=` query parameter. The default bind address is loopback.
type Dashboard struct {
	mu      sync.RWMutex
	events  []DashboardEvent
	started time.Time
	token   string
}

// DashboardEvent is one activity record shown on the dashboard.
type DashboardEvent struct {
	Time    time.Time `json:"time"`
	Kind    string    `json:"kind"`
	Detail  string    `json:"detail"`
	Risk    int       `json:"risk,omitempty"`
}

// NewDashboard creates a dashboard server with a fresh 32-byte random
// access token (returned via Token()).
func NewDashboard() (*Dashboard, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generate dashboard token: %w", err)
	}
	return &Dashboard{started: time.Now().UTC(), token: hex.EncodeToString(raw)}, nil
}

// Token returns the per-start access token. Print it once to the
// operator; never log it elsewhere.
func (d *Dashboard) Token() string { return d.token }

// Publish records an activity event (thread-safe; retains last 200).
func (d *Dashboard) Publish(e DashboardEvent) {
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}
	d.mu.Lock()
	d.events = append(d.events, e)
	if len(d.events) > 200 {
		d.events = d.events[len(d.events)-200:]
	}
	d.mu.Unlock()
}

// authorize checks the request token against the dashboard token using
// a constant-time comparison. Token may arrive via Authorization
// header, X-Aether-Token header, or ?token= query parameter.
func (d *Dashboard) authorize(r *http.Request) bool {
	presented := r.Header.Get("Authorization")
	presented = strings.TrimSpace(strings.TrimPrefix(presented, "Bearer"))
	if presented == "" {
		presented = r.Header.Get("X-Aether-Token")
	}
	if presented == "" {
		presented = r.URL.Query().Get("token")
	}
	if presented == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(presented), []byte(d.token)) == 1
}

func (d *Dashboard) deny(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="aether-dashboard"`)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	fmt.Fprintln(w, "401 unauthorized: dashboard token required")
}

// Handler returns the authenticated HTTP mux: /  (HTML), /api/graph
// (graph JSON), /api/events (activity JSON), /api/health. All routes
// require the access token.
func (d *Dashboard) Handler(graphHTML string, graphJSON any) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !d.authorize(r) {
			d.deny(w)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, graphHTML)
	})

	mux.HandleFunc("/api/graph", func(w http.ResponseWriter, r *http.Request) {
		if !d.authorize(r) {
			d.deny(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(graphJSON)
	})

	mux.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) {
		if !d.authorize(r) {
			d.deny(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		d.mu.RLock()
		events := d.events
		d.mu.RUnlock()
		// Newest first.
		out := make([]DashboardEvent, len(events))
		for i, e := range events {
			out[len(events)-1-i] = e
		}
		json.NewEncoder(w).Encode(out)
	})

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		if !d.authorize(r) {
			d.deny(w)
			return
		}
		d.mu.RLock()
		defer d.mu.RUnlock()
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"started": d.started,
			"events":  len(d.events),
		})
	})

	return mux
}

// IsLoopback reports whether the host part of addr is a loopback
// address (default-safe dashboard binding). An empty host means a
// wildcard bind (all interfaces) and is NOT considered loopback.
func IsLoopback(addr string) bool {
	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Serve starts the dashboard HTTP server (blocking). Use ServeTLS for
// non-loopback bindings.
func (d *Dashboard) Serve(addr string, handler http.Handler) error {
	srv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	return srv.ListenAndServe()
}

// ServeTLS starts the dashboard HTTPS server (blocking).
func (d *Dashboard) ServeTLS(addr, certFile, keyFile string, handler http.Handler) error {
	srv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	return srv.ListenAndServeTLS(certFile, keyFile)
}
