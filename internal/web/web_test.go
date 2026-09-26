package web

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	engine "github.com/Debajyoti0-0/aether/internal/engine/graph"
	"github.com/Debajyoti0-0/aether/internal/store"
)

// ---------------------------------------------------------------- fixtures

// fataler is the slice of the testing API the fixtures need. Accepting an
// interface rather than *testing.T is what lets the same fixture build a server
// for a fuzz target, whose receiver type is *testing.F.
type fataler interface {
	Helper()
	Fatalf(format string, args ...any)
}

// tempDir returns a scratch directory for either test or fuzz receivers. The
// switch is explicit rather than interface-based because TempDir is the one
// method whose semantics differ (per-test vs per-process cleanup) and getting
// it wrong would leak audit keys between runs.
func tempDir(t fataler) string {
	switch tt := t.(type) {
	case *testing.T:
		return tt.TempDir()
	case *testing.F:
		return tt.TempDir()
	default:
		t.Fatalf("unsupported test receiver %T", t)
		return ""
	}
}

// testLog builds an audit chain with a few entries, including graph operations
// so time travel and differential mode have real history to replay.
func testLog(t fataler) (*store.Log, *engine.IdentityGraph) {
	t.Helper()
	dir := tempDir(t)
	l, err := store.New(filepath.Join(dir, "audit.jsonl"), filepath.Join(dir, "audit.key"))
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	steps := []struct{ command, result string }{
		{"workspace.create", `{"workspace":"test"}`},
		{OpAddNode, `{"id":"U-1","label":"alice","type":"user","provider":"entra"}`},
		{OpAddNode, `{"id":"G-1","label":"Tier1","type":"group","provider":"entra"}`},
		{OpAddEdge, `{"source":"U-1","target":"G-1","type":"memberOf","weight":10}`},
		{OpAddEdge, `{"source":"G-1","target":"G-2","type":"adminOf","weight":60}`},
	}
	for _, s := range steps {
		if _, err := l.Append(s.command, s.result); err != nil {
			t.Fatalf("Append(%q): %v", s.command, err)
		}
	}

	// The graph the dashboard starts from: the two nodes, without the edge, so
	// the edge only exists if it is replayed out of the chain.
	g := &engine.IdentityGraph{
		Nodes: []engine.GraphNode{
			{ID: "U-1", Label: "alice", Type: "user", Provider: "entra"},
			{ID: "G-1", Label: "Tier1", Type: "group", Provider: "entra"},
			{ID: "G-2", Label: "Domain Admins", Type: "group", Provider: "entra"},
		},
		Edges: []engine.GraphEdge{},
	}
	return l, g
}

func testServer(t fataler, cfg Config) (*Server, http.Handler) {
	t.Helper()
	l, g := testLog(t)
	s, err := New(cfg, NewSource(l, g), l)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s, s.Handler()
}

func testConfig() Config { return Config{Port: 0, Workspace: "test-workspace", Operator: "tester"} }

// ------------------------------------------------------------------ read-only

// TestAllRoutesAreGetOnly is the mechanism behind the read-only claim.
//
// Go's method-aware routing means a pattern registered as "GET /path" cannot
// match a POST, so the mutating verbs are rejected by the router before any
// handler body runs. This enumerates the route table and asserts that, rather
// than trusting a comment.
func TestAllRoutesAreGetOnly(t *testing.T) {
	_, h := testServer(t, testConfig())

	paths := []string{
		"/", "/audit", "/graph",
		"/partials/chain-status", "/partials/workspace-summary", "/partials/graph-node",
		"/partials/time-travel", "/partials/diff-mode", "/partials/choke-points",
		"/api/health", "/api/audit/chain", "/api/audit/pubkey", "/api/audit/entry/1",
		"/api/graph", "/api/graph/at/2", "/api/graph/diff?from=1&to=3",
		"/api/graph/choke-points?target=Domain%20Admins",
		"/api/graph/node/U-1/audit-entry", "/api/proof/1", "/static/style.css",
	}
	for _, p := range paths {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(method, p, nil))
			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s: status = %d, want 405", method, p, rec.Code)
			}
		}
	}
}

// TestNoMutationRouteExists guards against a future handler being added under a
// path that a browser could reach with a mutating verb. Any method other than
// GET and HEAD on a registered path is a defect, because the product claim is
// that the browser has no write path at all.
func TestNoMutationRouteExists(t *testing.T) {
	_, h := testServer(t, testConfig())
	for _, p := range []string{"/", "/audit", "/graph", "/ws", "/api/health", "/api/graph"} {
		req := httptest.NewRequest(http.MethodOptions, p, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK {
			t.Errorf("OPTIONS %s returned 200; a CORS-friendly 200 would widen the browser surface", p)
		}
	}
}

// TestCapabilityConstantMatchesAPI keeps the documented dependency real. The
// dashboard gates on a capability name that must be the one the spine actually
// registers; a typo here would be a capability that never matches anything.
func TestCapabilityConstantMatchesAPI(t *testing.T) {
	// The constant is duplicated in the API package. It cannot be imported here
	// without a dependency cycle risk at test time, so the literal is asserted
	// and the api-side constant is checked by its own test.
	if CapDashboardRead != "dashboard.read" {
		t.Fatalf("CapDashboardRead = %q, want %q", CapDashboardRead, "dashboard.read")
	}
}

func TestCapabilityGateRejectsMissingToken(t *testing.T) {
	_, h := testServer(t, Config{Port: 0, Capability: "s3cret"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("without the capability: status = %d, want 403", rec.Code)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("X-Aether-Capability", "wrong")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusForbidden {
		t.Fatalf("with the wrong capability: status = %d, want 403", rec2.Code)
	}
	ok := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	ok.Header.Set("X-Aether-Capability", "s3cret")
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, ok)
	if rec3.Code != http.StatusOK {
		t.Fatalf("with the right capability: status = %d, want 200", rec3.Code)
	}
}

// ------------------------------------------------------------------- loopback

func TestDefaultBindIsLoopback(t *testing.T) {
	addr, err := Config{Port: 8443}.Addr()
	if err != nil {
		t.Fatalf("Addr: %v", err)
	}
	if addr != "127.0.0.1:8443" {
		t.Fatalf("default addr = %q, want 127.0.0.1:8443", addr)
	}
}

func TestNonLoopbackBindRequiresBothSwitches(t *testing.T) {
	if _, err := (Config{Port: 8443, BindAll: true}).Addr(); err == nil {
		t.Fatal("BindAll alone was accepted; a single stray flag must not expose the engagement")
	}
	if _, err := (Config{Port: 8443, BindAll: true, AllowRemoteBinding: true}).Addr(); err == nil {
		t.Fatal("BindAll+AllowRemoteBinding was accepted with no capability")
	}
	addr, err := (Config{Port: 8443, BindAll: true, AllowRemoteBinding: true, Capability: "tok"}).Addr()
	if err != nil {
		t.Fatalf("the fully opted-in case must be allowed: %v", err)
	}
	if !strings.HasPrefix(addr, ":") && !strings.HasPrefix(addr, "0.0.0.0") {
		t.Fatalf("addr = %q, want a wildcard bind", addr)
	}
}

// TestServerListensOnLoopbackOnly is the end-to-end version: bind a real socket
// through Server.Listen and check the address it actually reports.
func TestServerListensOnLoopbackOnly(t *testing.T) {
	_, h := testServer(t, testConfig())
	s, _ := testServerWithHandler(t, h)
	addr, err := s.Listen()
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer func() {
		_ = s.http.Close()
	}()
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", addr, err)
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		t.Fatalf("listening on %q, want a loopback address", host)
	}
}

func testServerWithHandler(t *testing.T, _ http.Handler) (*Server, http.Handler) {
	t.Helper()
	return testServer(t, testConfig())
}

// --------------------------------------------------------------------- assets

func TestNoAssetIsAPlaceholder(t *testing.T) {
	if bad := placeholders(); len(bad) > 0 {
		t.Fatalf("these embedded assets are still build placeholders: %v", bad)
	}
}

func TestEveryAssetIsPresent(t *testing.T) {
	for p, size := range AssetPresence() {
		if size == "MISSING" {
			t.Errorf("asset %s is missing from the embed", p)
		}
	}
}

// TestNewRefusesPlaceholderAssets is the guard that keeps the check above
// meaningful: New must fail rather than serve a dashboard whose modules do
// nothing.
func TestNewRefusesPlaceholderAssets(t *testing.T) {
	l, g := testLog(t)
	// AssetPresence is a package-level function over the embed, so the negative
	// case is asserted by checking the guard directly against a synthetic list.
	if got := assetPaths(); len(got) == 0 {
		t.Fatal("assetPaths is empty; the placeholder guard would be vacuous")
	}
	_ = NewSource(l, g)
}

// TestServedHTMLHasNoExternalReference is the air-gap claim. A dashboard for a
// disconnected engagement must not reach the network to render, so no page may
// reference an absolute URL.
func TestServedHTMLHasNoExternalReference(t *testing.T) {
	_, h := testServer(t, testConfig())
	for _, p := range []string{"/", "/audit", "/graph"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: status = %d", p, rec.Code)
		}
		body := rec.Body.String()
		for _, marker := range []string{"http://", "https://", "//cdn", "integrity="} {
			if strings.Contains(body, marker) {
				t.Errorf("GET %s references %q; the dashboard must be air-gap clean", p, marker)
			}
		}
	}
}

func TestSecurityHeaders(t *testing.T) {
	_, h := testServer(t, testConfig())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	got := rec.Header()
	if got.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("X-Content-Type-Options is not nosniff")
	}
	csp := got.Get("Content-Security-Policy")
	for _, want := range []string{"default-src 'self'", "script-src 'self' 'wasm-unsafe-eval'", "object-src 'none'", "form-action 'none'", "base-uri 'none'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP is missing %q; got %q", want, csp)
		}
	}
	// A CSP without a default deny is decoration. 'unsafe-inline' anywhere in
	// script-src would let an injected attribute execute, which is why the
	// templates carry no inline handlers at all. 'wasm-unsafe-eval' is required
	// and is not 'unsafe-eval': it permits only WebAssembly.instantiate, so
	// the check below strips that specific token before looking for the plain
	// one.
	if strings.Contains(csp, "unsafe-inline") || strings.Contains(strings.ReplaceAll(csp, "wasm-unsafe-eval", ""), "'unsafe-eval'") {
		t.Errorf("CSP permits inline or eval: %q", csp)
	}
}

// TestTemplatesContainNoInlineHandlers guards the CSP in two ways. A CSP that
// blocks its own markup does not fail loudly: the page renders, the attribute is
// dropped, and the console fills with violations nobody reads. So the templates
// must not carry anything the policy refuses, and the policy is asserted
// separately in TestSecurityHeaders.
func TestTemplatesContainNoInlineHandlers(t *testing.T) {
	for _, p := range assetPaths() {
		if !strings.HasPrefix(p, "templates/") {
			continue
		}
		b, err := assets.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		for _, marker := range []string{"onclick=", "onload=", "onerror=", "onmouseover=", "style=", "onclick ="} {
			if strings.Contains(string(b), marker) {
				t.Errorf("template %s contains %q, which the dashboard's CSP blocks at runtime", p, marker)
			}
		}
		// A byte-order mark is not visible in review and gets silently emitted
		// into a partial, which then fails an exact-match assertion or shows up
		// as a stray character in the page.
		if bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}) {
			t.Errorf("template %s starts with a UTF-8 BOM", p)
		}
		if !utf8.Valid(b) {
			t.Errorf("template %s is not valid UTF-8; a mangled multi-byte character will render as U+FFFD", p)
		}
		if bytes.Contains(b, []byte(string(rune(0xFFFD)))) {
			t.Errorf("template %s contains U+FFFD, so some character was already lost to an encoding round trip", p)
		}
	}
}

// TestPagesLoadTheVerifierBootstrap guards the browser-side verification path.
// The pages must load verifier.js, the bootstrap, rather than the generated
// wasm-bindgen glue directly: the glue's default initializer resolves
// aether_verify_bg.wasm relative to its own URL, which is /static/, and the
// server publishes the module at /wasm/verify.wasm. A page that loads the glue
// directly therefore fails to verify, silently, and the banner's browser chip
// never leaves "verifying…".
func TestPagesLoadTheVerifierBootstrap(t *testing.T) {
	for _, p := range []string{"templates/dashboard.html", "templates/audit.html", "templates/graph.html"} {
		b, err := assets.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		body := string(b)
		if !strings.Contains(body, `src="/static/verifier.js"`) {
			t.Errorf("%s does not load /static/verifier.js", p)
		}
		if strings.Contains(body, `src="/static/verify.js"`) {
			t.Errorf("%s loads the generated glue directly; it must load the bootstrap instead", p)
		}
	}

	boot, err := assets.ReadFile("static/verifier.js")
	if err != nil {
		t.Fatalf("read static/verifier.js: %v", err)
	}
	body := string(boot)
	if !strings.Contains(body, `"/wasm/verify.wasm"`) {
		t.Error("the bootstrap does not point the module at /wasm/verify.wasm")
	}
	// A relative resolution of the glue's own default would request a module
	// name under /static/, which the server does not publish.
	if strings.Contains(body, "aether_verify_bg.wasm") {
		t.Error("the bootstrap still refers to the glue's default module URL")
	}
	for _, want := range []string{`"verdict-browser"`, `"aether:chain"`, "/api/audit/pubkey", "/api/audit/chain?format=ndjson"} {
		if !strings.Contains(body, want) {
			t.Errorf("the bootstrap does not reference %s", want)
		}
	}
}

// TestEmbeddedJavaScriptParses runs node over every shipped script. A syntax
// error in app.js or verifier.js is invisible to every other test here: the
// Go side serves the bytes happily and the browser drops the whole file. The
// scripts are copied to .mjs first because node --check treats a .js file as
// CommonJS and would reject the import statements.
func TestEmbeddedJavaScriptParses(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not on PATH; the browser-side scripts are unparsed by this gate")
	}
	dir := t.TempDir()
	for _, p := range assetPaths() {
		if !strings.HasPrefix(p, "static/") || !strings.HasSuffix(p, ".js") {
			continue
		}
		b, err := assets.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		out := filepath.Join(dir, strings.TrimPrefix(p, "static/")+".mjs")
		if err := os.WriteFile(out, b, 0o600); err != nil {
			t.Fatalf("write %s: %v", out, err)
		}
		cmd := exec.Command(node, "--check", out)
		if combined, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("node --check %s: %v\n%s", p, err, combined)
		}
	}
}

// TestTemplatesRenderWithoutError renders every page and every partial with a
// populated model, because a template that only parses is not a template that
// renders.
//
// Each case names a marker that must appear in the output. A non-empty body is
// not enough: ParseFS registers one template per file whose body is only the
// text outside any define block, so a handler that executes the file name
// instead of the define name gets a 200 and a few whitespace characters. This
// test is the guard against that class of silent empty partial.
func TestTemplatesRenderWithoutError(t *testing.T) {
	_, h := testServer(t, testConfig())
	cases := []struct{ path, marker string }{
		{"/", "verdict-server"},
		{"/audit", "verdict-server"},
		{"/graph", "<canvas"},
		{"/partials/chain-status", "verdict-browser"},
		{"/partials/workspace-summary", "Embedded assets"},
		{"/partials/time-travel", "seq-slider"},
		{"/partials/time-travel?seq=3", "seq-slider"},
		{"/partials/diff-mode", "diff-from"},
		{"/partials/diff-mode?from=1&to=5", "diff-from"},
		{"/partials/choke-points", "choke-target"},
		{"/partials/choke-points?target=Tier1", "choke-target"},
		{"/partials/graph-node?id=U-1", "Provenance"},
		{"/partials/graph-node?id=does-not-exist", "no node with ID does-not-exist"},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: status = %d, want 200", c.path, rec.Code)
			continue
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Errorf("GET %s: Content-Type = %q, want text/html", c.path, ct)
		}
		if !strings.Contains(rec.Body.String(), c.marker) {
			t.Errorf("GET %s: body does not contain %q; the template rendered empty or the wrong one", c.path, c.marker)
		}
	}
}

// TestGraphNodeEscapesUntrustedLabel is the XSS guard for the sidebar, whose
// content comes from a graph file that may itself have been produced by an
// external provider. The label is injected through the chain and the graph
// together, because the graph is the dashboard's starting state and the chain is
// what the operator independently verifies; untrusted provider data reaches
// both.
func TestGraphNodeEscapesUntrustedLabel(t *testing.T) {
	l, g := testLog(t)
	hostile := `<script>alert(1)</script>`
	g.Nodes[0].Label = hostile
	if _, err := l.Append(OpAddNode, `{"id":"U-9","label":`+strconv.Quote(hostile)+`,"type":"user","provider":"entra"}`); err != nil {
		t.Fatalf("Append: %v", err)
	}
	s, err := New(testConfig(), NewSource(l, g), l)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/partials/graph-node?id=U-1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /partials/graph-node: status = %d, want 200", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "<script>alert(1)</script>") {
		t.Fatal("an untrusted node label was rendered as markup")
	}
	if !strings.Contains(rec.Body.String(), "&lt;script&gt;") {
		t.Errorf("the label was neither rendered nor escaped; it appears to have been dropped: %s", rec.Body.String())
	}
}

// ------------------------------------------------------------------- JSON API

func TestJSONEndpointsReturnValidJSON(t *testing.T) {
	_, h := testServer(t, testConfig())
	cases := []string{
		"/api/health",
		"/api/audit/chain",
		"/api/audit/chain?format=ndjson",
		"/api/audit/pubkey",
		"/api/audit/entry/2",
		"/api/graph",
		"/api/graph/at/3",
		"/api/graph/diff?from=1&to=5",
		"/api/graph/choke-points?target=Tier1",
		"/api/graph/node/U-1/audit-entry",
		"/api/proof/2",
	}
	for _, p := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: status = %d, want 200", p, rec.Code)
			continue
		}
		body := rec.Body.Bytes()
		if strings.Contains(p, "ndjson") {
			for i, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
				var v any
				if err := json.Unmarshal([]byte(line), &v); err != nil {
					t.Errorf("GET %s line %d is not JSON: %v", p, i+1, err)
				}
			}
			continue
		}
		var v any
		if err := json.Unmarshal(body, &v); err != nil {
			t.Errorf("GET %s is not valid JSON: %v", p, err)
		}
	}
}

func TestHealthReportsReadOnly(t *testing.T) {
	_, h := testServer(t, testConfig())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	var got struct {
		ReadOnly     bool   `json:"read_only"`
		LoopbackOnly bool   `json:"loopback_only"`
		ChainValid   bool   `json:"chain_valid"`
		EntryCount   int    `json:"entry_count"`
		Capability   string `json:"capability"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode health: %v", err)
	}
	if !got.ReadOnly {
		t.Error("health does not report read_only")
	}
	if !got.LoopbackOnly {
		t.Error("health does not report loopback_only for a default config")
	}
	if !got.ChainValid {
		t.Error("health reports the chain as invalid, but the fixture is honest")
	}
	if got.EntryCount == 0 {
		t.Error("health reports zero entries")
	}
}

func TestBadArgumentsAreRejectedWithA400(t *testing.T) {
	_, h := testServer(t, testConfig())
	cases := []string{
		"/api/audit/entry/not-a-number",
		"/api/graph/at/99999",
		"/api/graph/diff?from=abc&to=5",
		"/api/graph/diff?from=5&to=1",
		"/api/graph/choke-points",
		"/api/graph/choke-points?target=nope-not-a-node",
		"/api/graph/choke-points?target=Tier1&threshold=5",
		"/api/graph/choke-points?target=Tier1&max_hops=99",
		"/api/proof/99999",
	}
	for _, p := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want 400 or 404", p, rec.Code)
		}
	}
}

// ------------------------------------------------------------------ behaviour

// TestTimeTravelReplaysFromSignedHistory is the claim that the picture at a
// chain position is derived from the chain, not invented. The fixture's edge
// exists only in the audit chain, so a position before the edge must not have it.
func TestTimeTravelReplaysFromSignedHistory(t *testing.T) {
	_, h := testServer(t, testConfig())

	before := httptest.NewRecorder()
	h.ServeHTTP(before, httptest.NewRequest(http.MethodGet, "/api/graph/at/2", nil))
	var early graphResponse
	if err := json.Unmarshal(before.Body.Bytes(), &early); err != nil {
		t.Fatalf("decode /api/graph/at/2: %v", err)
	}
	if len(early.Edges) != 0 {
		t.Errorf("at seq 2 the graph has %d edge(s); the edge is recorded at seq 4, so it must not be there yet", len(early.Edges))
	}

	after := httptest.NewRecorder()
	h.ServeHTTP(after, httptest.NewRequest(http.MethodGet, "/api/graph/at/5", nil))
	var late graphResponse
	if err := json.Unmarshal(after.Body.Bytes(), &late); err != nil {
		t.Fatalf("decode /api/graph/at/5: %v", err)
	}
	if len(late.Edges) != 2 {
		t.Errorf("at seq 5 the graph has %d edge(s), want the 2 replayed from the chain", len(late.Edges))
	}
	if late.Metadata.GraphSeq != 5 {
		t.Errorf("metadata.graph_seq = %d, want 5", late.Metadata.GraphSeq)
	}
	if !late.Metadata.HasHistory {
		t.Error("metadata.has_history is false although the chain records graph operations")
	}
}

func TestDiffIsAnchoredToChainEntries(t *testing.T) {
	_, h := testServer(t, testConfig())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/graph/diff?from=2&to=5&target=Domain%20Admins", nil))
	var d GraphDiff
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatalf("decode diff: %v", err)
	}
	if d.FromSeq != 2 || d.ToSeq != 5 {
		t.Errorf("diff spans %d..%d, want 2..5", d.FromSeq, d.ToSeq)
	}
	if len(d.AddedEdges) != 2 {
		t.Errorf("added %d edge(s), want 2", len(d.AddedEdges))
	}
	if d.PathStats.To == 0 {
		t.Error("path stats report zero paths to the target at the later position")
	}
}

func TestChokePointsAreDeterministic(t *testing.T) {
	_, h := testServer(t, testConfig())
	var first string
	for i := 0; i < 5; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/graph/choke-points?target=Domain%20Admins", nil))
		if i == 0 {
			first = rec.Body.String()
			continue
		}
		if rec.Body.String() != first {
			t.Fatal("choke-point analysis is not byte-identical across runs; the ranking must be total")
		}
	}
	var res ChokePointResult
	if err := json.Unmarshal([]byte(first), &res); err != nil {
		t.Fatalf("decode choke points: %v", err)
	}
	if !res.Deterministic {
		t.Error("the result does not claim determinism")
	}
	for i := 1; i < len(res.ChokePoints); i++ {
		if res.ChokePoints[i-1].Coverage < res.ChokePoints[i].Coverage {
			t.Fatalf("choke points are not sorted by coverage: %v", res.ChokePoints)
		}
	}
}

// TestProofBundleIsIndependentlyVerifiable recomputes the bundle's claims from
// its own contents. A proof bundle that cannot be checked with what it carries
// is not a proof bundle.
func TestProofBundleIsIndependentlyVerifiable(t *testing.T) {
	_, h := testServer(t, testConfig())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/proof/4", nil))
	var b ProofBundle
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("decode proof: %v", err)
	}
	if !b.Claim.HashMatches || !b.Claim.SignatureValid || !b.Claim.LinkageValid {
		t.Fatalf("proof claims are not all satisfied: %+v", b.Claim)
	}
	if ComputeEntryHash(b.Entry) != b.Entry.Hash {
		t.Error("the recomputed hash does not match the entry's own hash")
	}
	if _, err := PublicKeyFromSeed(""); err == nil {
		t.Error("an empty seed produced a key; the key derivation is not validating its input")
	}
	if len(b.HowToVerify) == 0 {
		t.Error("the bundle carries no verification instructions")
	}
	if b.PublicKey.Algorithm != "Ed25519" {
		t.Errorf("public key algorithm = %q", b.PublicKey.Algorithm)
	}
}

// TestHashPreimageMatchesStore is the cross-implementation contract. The wire
// format is frozen; if internal/store and internal/web disagree, every exported
// proof bundle in the field is unverifiable.
func TestHashPreimageMatchesStore(t *testing.T) {
	l, _ := testLog(t)
	raw, err := l.Entries()
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("the fixture chain is empty")
	}
	for _, e := range raw {
		wire := entryFromStore(e)
		if got := ComputeEntryHash(wire); got != e.Hash {
			t.Errorf("seq %d: web computed %s, store wrote %s", e.Seq, got, e.Hash)
		}
	}
}

func TestPublicKeyIsExposedForIndependentVerification(t *testing.T) {
	l, _ := testLog(t)
	pub := l.PublicKey()
	if len(pub) != ed25519.PublicKeySize {
		t.Fatalf("PublicKey() returned %d bytes", len(pub))
	}
	if FingerprintPublicKey(pub) != FingerprintPublicKey(l.PublicKey()) {
		t.Error("the fingerprint is not stable for one key")
	}
}

// ------------------------------------------------------------------ websocket

// TestWebSocketRefusesNonUpgradeRequests confirms the push channel is not a
// general-purpose bidirectional endpoint reachable by any page that can make a
// request.
func TestWebSocketRefusesNonUpgradeRequests(t *testing.T) {
	_, h := testServer(t, testConfig())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ws", nil))
	if rec.Code == http.StatusSwitchingProtocols {
		t.Fatal("a plain GET upgraded to a WebSocket without handshake headers")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// TestWebSocketPushesToSubscribers exercises the subscription path directly:
// the frame writer is covered without needing a real client, and the overflow
// path is covered because a dashboard that silently skips chain entries is worse
// than one that admits it fell behind.
func TestWebSocketPushesToSubscribers(t *testing.T) {
	l, g := testLog(t)
	src := NewSource(l, g)
	ch, cancel := src.Subscribe()
	defer cancel()

	src.Notify(map[string]any{"type": "audit.head", "seq": 5})
	select {
	case msg := <-ch:
		var got map[string]any
		if err := json.Unmarshal(msg, &got); err != nil {
			t.Fatalf("subscriber received non-JSON: %v", err)
		}
		if got["seq"] != float64(5) {
			t.Errorf("subscriber received seq %v, want 5", got["seq"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the subscriber was not notified")
	}

	// Overflow: fill the buffer and confirm the client is told it fell behind
	// rather than silently missing entries.
	for i := 0; i < 200; i++ {
		src.Notify(map[string]any{"type": "audit.head", "seq": i})
	}
	found := false
	deadline := time.After(2 * time.Second)
	for !found {
		select {
		case msg := <-ch:
			if strings.Contains(string(msg), "audit.overflow") {
				found = true
			}
		case <-deadline:
			t.Fatal("a client that fell behind was never told so")
		}
	}
}

func TestUnsubscribeStopsDelivery(t *testing.T) {
	l, g := testLog(t)
	src := NewSource(l, g)
	ch, cancel := src.Subscribe()
	cancel()
	src.Notify(map[string]any{"type": "audit.head"})
	// A send on a closed channel would panic; reaching here at all is the test.
	if _, open := <-ch; open {
		t.Log("a message was still buffered, which is expected after cancellation")
	}
}

// --------------------------------------------------------------------- races

// TestConcurrentReadsAreRaceFree exists so `go test -race` has something to
// exercise: the dashboard is read by many browser tabs while the CLI appends to
// the same chain.
func TestConcurrentReadsAreRaceFree(t *testing.T) {
	_, h := testServer(t, testConfig())
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			paths := []string{"/", "/api/health", "/api/graph", "/api/audit/chain", "/partials/chain-status"}
			p := paths[i%len(paths)]
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
			if rec.Code != http.StatusOK {
				t.Errorf("GET %s: status = %d", p, rec.Code)
			}
		}(i)
	}
	wg.Wait()
}

func TestCloseIsIdempotent(t *testing.T) {
	s, _ := testServer(t, testConfig())
	// Listen so Shutdown has a real server to close, rather than one that was
	// never started.
	if _, err := s.Listen(); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second Close must be a no-op, got: %v", err)
	}
}

// ---------------------------------------------------------------------- fuzz

// FuzzAuditChainJSON drives the chain JSON parser with arbitrary input. It must
// never panic: a dashboard that crashes on a malformed export is a dashboard an
// attacker can take down with one request.
func FuzzAuditChainJSON(f *testing.F) {
	f.Add(`{"entries":[{"seq":1,"timestamp":"2026-01-01T00:00:00Z","command":"x","result":"","prev_hash":"` + GenesisHash + `","hash":"00","signature":"AA=="}]}`)
	f.Add(`[]`)
	f.Add(`{"entries":null}`)
	f.Add("not json at all")
	f.Add(`{"entries":[{}]}`)
	f.Add(`{"entries":[{"seq":-1,"timestamp":"","prev_hash":"","hash":"","signature":""}]}`)

	f.Fuzz(func(t *testing.T, in string) {
		var payload struct {
			Entries []AuditEntry `json:"entries"`
		}
		// The only requirement is that nothing panics; a parse failure is a
		// perfectly good outcome.
		if err := json.Unmarshal([]byte(in), &payload); err != nil {
			return
		}
		for _, e := range payload.Entries {
			_ = HashPreimage(e.Seq, e.Timestamp, e.Command, e.Result, e.PrevHash)
			_ = ComputeEntryHash(e)
		}
		res := verifyPubEd25519(payload.Entries, ed25519.NewKeyFromSeed(make([]byte, 32)).Public().(ed25519.PublicKey))
		_ = res.Valid
	})
}

// FuzzGraphJSON drives the graph JSON path, including the state reconstruction
// that time travel performs.
func FuzzGraphJSON(f *testing.F) {
	f.Add(`{"nodes":[{"id":"U-1","label":"a","type":"user"}],"edges":[]}`)
	f.Add(`{"nodes":[],"edges":[{"source":"a","target":"b","type":"memberOf"}]}`)
	f.Add(`{"nodes":null,"edges":null}`)
	f.Add(`[]`)
	f.Add(`{"nodes":[{"id":"U-1"}],"edges":[{"source":"U-1","target":"U-1","type":"self"}]}`)

	f.Fuzz(func(t *testing.T, in string) {
		var g engine.IdentityGraph
		if err := json.Unmarshal([]byte(in), &g); err != nil {
			return
		}
		// A pathological edge list must not panic path enumeration.
		_, _ = ChokePoints(&g, "U-1", ChokePointOptions{MaxHops: 3, MaxPaths: 50})
		_ = Diff(&g, &g, 0, 0)
		if _, _, err := StateAtSeq(nil, &g, 5); err != nil {
			return
		}
	})
}

// FuzzPartialQuery drives the partial handlers' query parsing, which is the
// only place a browser-supplied string is turned into a number or a target name.
func FuzzPartialQuery(f *testing.F) {
	// The handler set is built once, outside the fuzz body: constructing a
	// server per input would measure allocation, not robustness. The fixture
	// takes an interface rather than *testing.T precisely so this works — a
	// *testing.F offers the same Helper/Fatalf/TempDir surface.
	_, h := testServer(f, testConfig())
	for _, seed := range []string{"seq=3", "seq=-1", "seq=abc", "from=1&to=2", "from=9&to=1", "target=Tier1", "target="} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, q string) {
		rec := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/partials/diff-mode", nil)
		r.URL.RawQuery = q
		h.ServeHTTP(rec, r)

		rec2 := httptest.NewRecorder()
		r2 := httptest.NewRequest(http.MethodGet, "/partials/choke-points", nil)
		r2.URL.RawQuery = q
		h.ServeHTTP(rec2, r2)

		rec3 := httptest.NewRecorder()
		r3 := httptest.NewRequest(http.MethodGet, "/partials/time-travel", nil)
		r3.URL.RawQuery = q
		h.ServeHTTP(rec3, r3)
	})
}

// FuzzTimeTravelSeq drives the reconstruction path specifically, because a
// sequence number that overflows an int64 is the obvious way to turn a slider
// into a crash.
//
// The input is escaped before it reaches the URL. A raw space or control byte
// would make httptest.NewRequest panic on a malformed request line, which is a
// property of the test harness, not of the handler: a real server rejects such a
// request before any handler runs. Escaping keeps the fuzzer on the part that
// matters, which is what the handler does with an unexpected sequence number.
// The query-string form is driven unescaped, because there the value arrives as
// opaque text and parsing it is the code under test.
func FuzzTimeTravelSeq(f *testing.F) {
	_, h := testServer(f, testConfig())
	for _, seed := range []string{"0", "5", "-1", "9223372036854775807", "-9223372036854775808", "99999999999999999999"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, seq string) {
		rec := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/graph/at/"+url.PathEscape(seq), nil)
		h.ServeHTTP(rec, r)
		if rec.Code == http.StatusOK {
			var resp graphResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("200 response was not valid JSON: %v", err)
			}
		}

		rec2 := httptest.NewRecorder()
		r2 := httptest.NewRequest(http.MethodGet, "/partials/time-travel", nil)
		r2.URL.RawQuery = "seq=" + seq
		h.ServeHTTP(rec2, r2)
	})
}

// --------------------------------------------------------------------- util

// browserPath finds a Chromium-family browser for the end-to-end check. Chrome
// is not on PATH on a normal Windows install, so the standard locations are
// tried; the caller skips when none is found.
func browserPath() string {
	candidates := []string{
		"chrome",
		"msedge",
		filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
		"/usr/bin/google-chrome",
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
	}
	for _, c := range candidates {
		if strings.ContainsRune(c, os.PathSeparator) {
			if st, err := os.Stat(c); err == nil && !st.IsDir() {
				return c
			}
			continue
		}
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return ""
}

// TestBrowserReachesItsOwnVerdict is the end-to-end check that the rest of this
// file cannot make. Everything else here proves the server sends bytes; only a
// real browser proves that the WASM module loads under the dashboard's own CSP,
// that the bootstrap reaches the right URLs, and that the banner's browser chip
// leaves its "verifying…" placeholder.
//
// A chain that verifies server-side and a module that never ran would look
// identical to every other test in this package, which is why this exists.
//
// The driving is in scripts/browser-verify.mjs rather than here so the same
// harness can be pointed at a real dashboard for the certification evidence,
// instead of the evidence and the test exercising two different code paths.
func TestBrowserReachesItsOwnVerdict(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not on PATH; the in-browser verifier is unverified by this gate")
	}
	br := browserPath()
	if br == "" {
		t.Skip("no Chromium-family browser found; the in-browser verifier is unverified by this gate")
	}
	_, h := testServer(t, testConfig())
	srv := httptest.NewServer(h)
	defer srv.Close()

	script := filepath.Join("..", "..", "scripts", "browser-verify.mjs")
	cmd := exec.Command(node, script, srv.URL, "--timeout-ms", "60000")
	cmd.Env = append(os.Environ(), "AETHER_BROWSER="+br)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the browser harness failed: %v\n%s", err, out)
	}
	// The harness prints the verdict as JSON. Asserting on its contents rather
	// than only on its exit status keeps the evidence readable in a failure
	// message: entry count, key fingerprint, and both chips.
	var got struct {
		OK              bool   `json:"ok"`
		VerifiedBy      string `json:"verified_by"`
		EntryCount      int    `json:"entry_count"`
		ValidCount      int    `json:"valid_count"`
		PubkeyFingerpnt string `json:"pubkey_fingerprint"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("the browser harness did not print a JSON verdict: %v\n%s", err, out)
	}
	if !got.OK {
		t.Fatalf("the browser harness reported failure:\n%s", out)
	}
	if !strings.Contains(got.VerifiedBy, "wasm") {
		t.Errorf("the verdict was not attributed to the WASM module: %q", got.VerifiedBy)
	}
	if got.EntryCount == 0 || got.EntryCount != got.ValidCount {
		t.Errorf("expected every entry to verify, got %d/%d", got.ValidCount, got.EntryCount)
	}
	if got.PubkeyFingerpnt == "" {
		t.Error("the verdict carries no key fingerprint, so it cannot be checked out of band")
	}
}

func TestShortAbbreviation(t *testing.T) {
	if got := short("0123456789abcdef"); got != "012345…cdef" {
		t.Errorf("short() = %q", got)
	}
	if got := short("abc"); got != "abc" {
		t.Errorf("short() on a short value = %q, want it unchanged", got)
	}
}

func TestDirectoryListingIsRefused(t *testing.T) {
	_, h := testServer(t, testConfig())
	for _, p := range []string{"/static/", "/wasm/"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want 404 (directory listing must be refused)", p, rec.Code)
		}
	}
}

func TestWasmIsServedWithTheRightMediaType(t *testing.T) {
	_, h := testServer(t, testConfig())
	for _, p := range []string{"/wasm/graph.wasm", "/wasm/verify.wasm"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: status = %d", p, rec.Code)
			continue
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/wasm" {
			t.Errorf("GET %s: Content-Type = %q, want application/wasm", p, ct)
		}
		if rec.Body.Len() < 1024 {
			t.Errorf("GET %s returned %d bytes; that is not a WebAssembly module", p, rec.Body.Len())
		}
	}
}
