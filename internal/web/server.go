package web

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	engine "github.com/Debajyoti0-0/aether/internal/engine/graph"
	"github.com/Debajyoti0-0/aether/internal/store"
)

// listen is indirected so tests can bind an ephemeral port without racing on a
// fixed one.
var listen = net.Listen

// CapDashboardRead is the capability a remote caller must hold to read the
// dashboard. It is registered in internal/api/capabilities.go; the constant is
// repeated here as documentation of the dependency, and a mismatch is caught by
// TestCapabilityConstantMatchesAPI.
const CapDashboardRead = "dashboard.read"

// Server serves the read-only dashboard.
type Server struct {
	cfg    Config
	src    *Source
	tmpl   *template.Template
	mux    *http.ServeMux
	http   *http.Server
	ln     net.Listener
	lnErr  chan error
	closed sync.Once

	// audit records dashboard lifecycle events. A nil log disables lifecycle
	// auditing, which is only appropriate for a test server.
	audit *store.Log
}

// New builds a dashboard server. It fails if the embedded templates cannot be
// parsed, so a template error is a startup error rather than a blank page at
// 3am during an engagement.
//
// It also refuses to start while any embedded asset is still a build
// placeholder. A placeholder serves successfully and does nothing, which would
// leave the operator looking at a page whose graph and verifier simply never
// run — a silent failure in a tool whose whole purpose is to be trustworthy.
// `make wasm` regenerates them; TestNoAssetIsAPlaceholder keeps the set honest.
func New(cfg Config, src *Source, log *store.Log) (*Server, error) {
	cfg = cfg.withDefaults()
	if _, err := cfg.Addr(); err != nil {
		return nil, err
	}
	if bad := placeholders(); len(bad) > 0 {
		return nil, fmt.Errorf(
			"embedded assets are still build placeholders: %s. Run 'make wasm' and rebuild; "+
				"a placeholder serves successfully and silently does nothing",
			strings.Join(bad, ", "))
	}
	tmpl, err := template.New("aether").Funcs(templateFuncs()).ParseFS(assets, "templates/*.html", "templates/partials/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse dashboard templates: %w", err)
	}
	s := &Server{cfg: cfg, src: src, tmpl: tmpl, audit: log}
	s.routes()
	return s, nil
}

// Handler exposes the router for tests without binding a socket.
func (s *Server) Handler() http.Handler { return s.gate(s.mux) }

// routes registers every endpoint.
//
// Every route is GET-only. Go's method-aware pattern matching means a POST,
// PUT, PATCH or DELETE to any of these paths returns 405 without the request
// ever reaching a handler body. That is the mechanism behind the read-only
// claim, and TestAllRoutesAreGetOnly enforces it against this table rather than
// against a comment.
func (s *Server) routes() {
	mux := http.NewServeMux()

	// HTML pages.
	mux.HandleFunc("GET /{$}", s.handleDashboard)
	mux.HandleFunc("GET /audit", s.handleAudit)
	mux.HandleFunc("GET /graph", s.handleGraphPage)

	// HTMX partials.
	mux.HandleFunc("GET /partials/chain-status", s.handleChainStatus)
	mux.HandleFunc("GET /partials/workspace-summary", s.handleWorkspaceSummary)
	mux.HandleFunc("GET /partials/graph-node", s.handleGraphNode)
	mux.HandleFunc("GET /partials/time-travel", s.handleTimeTravel)
	mux.HandleFunc("GET /partials/diff-mode", s.handleDiffMode)
	mux.HandleFunc("GET /partials/choke-points", s.handleChokePoints)

	// JSON APIs.
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/audit/chain", s.handleChainJSON)
	mux.HandleFunc("GET /api/audit/pubkey", s.handlePubkeyJSON)
	mux.HandleFunc("GET /api/audit/entry/{seq}", s.handleEntryJSON)
	mux.HandleFunc("GET /api/graph", s.handleGraphJSON)
	mux.HandleFunc("GET /api/graph/at/{seq}", s.handleGraphAtSeqJSON)
	mux.HandleFunc("GET /api/graph/diff", s.handleGraphDiffJSON)
	mux.HandleFunc("GET /api/graph/choke-points", s.handleChokePointsJSON)
	mux.HandleFunc("GET /api/graph/node/{id}/audit-entry", s.handleNodeAuditEntry)
	mux.HandleFunc("GET /api/proof/{seq}", s.handleProofJSON)

	// Push channel.
	mux.HandleFunc("GET /ws", s.handleWebSocket)

	// Embedded assets.
	sub, err := staticFS()
	if err != nil {
		// staticFS can only fail on a malformed embed, which is a build-time
		// condition. Registering a handler that reports it beats panicking at
		// request time.
		mux.HandleFunc("GET /static/", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "static assets unavailable", http.StatusInternalServerError)
		})
	} else {
		mux.Handle("GET /static/", noDirListing(http.StripPrefix("/static/", http.FileServer(http.FS(sub)))))
	}

	// WebAssembly modules. They are served from their own prefix rather than
	// from /static/ so that the module inventory a page may execute is a
	// distinct, auditable list from its scripts and stylesheets.
	wasmSub, err := wasmFS()
	if err != nil {
		mux.HandleFunc("GET /wasm/", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "wasm assets unavailable", http.StatusInternalServerError)
		})
	} else {
		mux.Handle("GET /wasm/", noDirListing(http.StripPrefix("/wasm/", wasmContentType(http.FileServer(http.FS(wasmSub))))))
	}

	s.mux = mux
	s.http = &http.Server{
		Addr:              mustAddr(s.cfg),
		Handler:           s.gate(mux),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

// Listen binds the socket without serving, so a caller can learn the port (for
// port 0) before traffic starts. Splitting this from Serve lets integration
// tests bind an ephemeral port and never collide.
func (s *Server) Listen() (string, error) {
	addr, err := s.cfg.Addr()
	if err != nil {
		return "", err
	}
	ln, err := listen("tcp", addr)
	if err != nil {
		return "", err
	}
	s.ln = ln
	s.http.Addr = ln.Addr().String()
	return ln.Addr().String(), nil
}

// Serve binds (if not already bound) and serves until Close.
func (s *Server) Serve() error {
	if s.ln == nil {
		if _, err := s.Listen(); err != nil {
			return err
		}
	}
	return s.serveOn(s.ln)
}

// ServeTLS serves over HTTPS using the configured certificate and key.
func (s *Server) ServeTLS() error {
	if s.cfg.TLSCert == "" || s.cfg.TLSKey == "" {
		return fmt.Errorf("both --tls-cert and --tls-key are required for TLS")
	}
	if s.ln == nil {
		if _, err := s.Listen(); err != nil {
			return err
		}
	}
	return s.serveOn(tls.NewListener(s.ln, &tls.Config{
		MinVersion: tls.VersionTLS12,
	}))
}

// ServeListener serves on a listener the caller already owns. Integration tests
// use it to bind an ephemeral port and avoid collisions.
func (s *Server) ServeListener(ln net.Listener) error {
	s.ln = ln
	s.http.Addr = ln.Addr().String()
	return s.serveOn(ln)
}

func (s *Server) serveOn(ln net.Listener) error {
	s.recordLifecycle("dashboard.start", fmt.Sprintf("dashboard listening on %s (loopback-only=%v, capability=%s)",
		ln.Addr().String(), !s.cfg.BindAll, s.cfg.Capability))
	err := s.http.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Close shuts the server down.
func (s *Server) Close() error {
	s.recordLifecycle("dashboard.stop", "dashboard stopped")
	var err error
	s.closed.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = s.http.Shutdown(ctx)
	})
	return err
}

// recordLifecycle appends a signed audit entry for a dashboard lifecycle event.
// A failure to audit is logged and swallowed: refusing to start a read-only
// viewer because the audit append failed would be a worse outcome than serving
// a dashboard whose start is unrecorded, and the operator is told which it was.
func (s *Server) recordLifecycle(command, result string) {
	if s.audit == nil {
		return
	}
	if _, err := s.audit.Append(command, result); err != nil {
		log.Printf("aether dashboard: audit append failed for %q: %v", command, err)
	}
}

// gate applies the capability check to every request.
//
// On loopback with no capability configured the dashboard is open, which is the
// intended local-operator default. Config.Addr refuses a non-loopback bind
// without a capability, so the open case is only ever reachable locally.
func (s *Server) gate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		// The dashboard loads only same-origin assets. A restrictive CSP keeps a
		// hypothetical injected <script> from reaching the network, which is the
		// one thing a read-only view must not do with engagement data.
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self'; "+
				"img-src 'self' data:; connect-src 'self' ws: wss:; font-src 'self'; object-src 'none'; base-uri 'none'; form-action 'none'")
		if s.cfg.Capability != "" {
			if got := r.Header.Get("X-Aether-Capability"); got != s.cfg.Capability {
				w.Header().Set("WWW-Authenticate", `Bearer realm="aether-dashboard"`)
				http.Error(w, "forbidden: this dashboard requires the "+s.cfg.Capability+" capability", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// writeJSON emits a JSON response. Logging never touches this writer, so a
// diagnostic can never corrupt a JSON body.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// writeErr emits a classified JSON error. The Layer field exists so a failure
// can be attributed to the right subsystem; Stage 47B §41 requires that a
// failure never be reported at the wrong layer.
func writeErr(w http.ResponseWriter, status int, layer, msg string) {
	writeJSON(w, status, map[string]any{
		"error": msg,
		"layer": layer,
	})
}

// pubOrNil is a small helper for handlers that need the key.
func (s *Server) pubOrNil() ed25519.PublicKey { return s.src.PublicKey() }

// mustAddr resolves the configured address, falling back to loopback if the
// config is somehow zero-valued. Listen and Serve call cfg.Addr() first, so this
// only matters for a zero-value Server built directly in a test.
func mustAddr(c Config) string {
	addr, err := c.withDefaults().Addr()
	if err != nil {
		return "127.0.0.1:0"
	}
	return addr
}

// noDirListing prevents directory enumeration of the embedded static tree.
// http.FileServer would otherwise list every asset name, which leaks the
// module inventory to anyone who can reach the port.
//
// It must wrap StripPrefix rather than sit inside it. StripPrefix rewrites
// "/static/" to "", so a check made after it sees a path with no trailing
// slash, passes, and hands an empty path to FileServer, which renders the
// listing anyway.
func noDirListing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// wasmContentType pins the media type of a .wasm response.
//
// WebAssembly.instantiateStreaming requires application/wasm and refuses
// anything else, so relying on the host's mime table would make the dashboard
// depend on an OS registry lookup. Setting it here means the module loads
// identically on a machine with no mime.types file.
func wasmContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".wasm") {
			w.Header().Set("Content-Type", "application/wasm")
		}
		next.ServeHTTP(w, r)
	})
}

// graphOrEmpty returns the graph or an empty one, so a dashboard started
// without a graph file still renders and says so.
func graphOrEmpty(g *engine.IdentityGraph) *engine.IdentityGraph {
	if g == nil {
		return &engine.IdentityGraph{Nodes: []engine.GraphNode{}, Edges: []engine.GraphEdge{}}
	}
	return g
}
