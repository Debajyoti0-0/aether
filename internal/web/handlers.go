package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	engine "github.com/Debajyoti0-0/aether/internal/engine/graph"
)

// templateFuncs are the only functions available to the dashboard templates.
//
// The list is deliberately tiny. A template function that can reach arbitrary
// state is a way to defeat the escaping guarantees that make server-rendered
// HTML safe, so everything the UI needs is passed in the model instead.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"pct":   func(f float64) string { return strconv.FormatFloat(math.Round(f*1000)/10, 'f', 1, 64) + "%" },
		"short": func(s string) string { return short(s) },
		"since": func(t time.Time) string {
			if t.IsZero() {
				return "never"
			}
			return time.Since(t).Round(time.Second).String()
		},
		"join": strings.Join,
		"add":  func(a, b int64) int64 { return a + b },
	}
}

// viewModel is the data every page and partial receives.
type viewModel struct {
	Config   Config
	Status   Status
	Graph    *engine.IdentityGraph
	Timeline Timeline
	Seqs     []int64
	MinSeq   int64
	MaxSeq   int64
	FromSeq  int64
	ToSeq    int64
	Target   string
	Choke    *ChokePointResult
	Diff     *GraphDiff
	Node     *nodeView
	Entry    *AuditEntry
	Assets   map[string]string
	Warnings []string
}

// nodeView is the cryptographic proof panel model for one node or edge.
type nodeView struct {
	ID        string
	Label     string
	Type      string
	Provider  string
	Props     map[string]string
	InDegree  int
	OutDegree int
	Entry     *AuditEntry
	Found     bool
	Reason    string
}

func (s *Server) baseModel() viewModel {
	st := s.src.Status()
	vm := viewModel{
		Config:   s.cfg,
		Status:   st,
		Graph:    graphOrEmpty(s.src.Graph()),
		Timeline: st.Timeline,
		MinSeq:   0,
		MaxSeq:   st.HeadSeq,
		FromSeq:  0,
		ToSeq:    st.HeadSeq,
		Assets:   AssetPresence(),
	}
	vm.Seqs = seqRange(vm.MinSeq, vm.MaxSeq)
	if w := s.cfg.RemoteBindWarning(); w != "" {
		vm.Warnings = append(vm.Warnings, w)
	}
	if st.LoadError != "" {
		vm.Warnings = append(vm.Warnings, st.LoadError)
	}
	if st.Timeline.OpCount == 0 {
		vm.Warnings = append(vm.Warnings,
			"this chain records no graph operations, so time-travel has no history to replay and every slider position shows the current graph unchanged")
	}
	if len(st.Timeline.Errors) > 0 {
		vm.Warnings = append(vm.Warnings, fmt.Sprintf(
			"%d graph operation(s) in the chain could not be interpreted; time-travel skips them, so the reconstructed state may be incomplete",
			len(st.Timeline.Errors)))
	}
	if !st.HasPubKey {
		vm.Warnings = append(vm.Warnings,
			"no Ed25519 public key is available, so neither the server nor the browser can verify this chain")
	}
	return vm
}

// seqRange returns the sequence numbers a slider can address.
//
// A long chain produces a huge range, so it is clamped and reported. Clamping
// is stated in the UI rather than hidden, because a slider whose range silently
// stops short of the head would misrepresent the chain's extent.
func seqRange(min, max int64) []int64 {
	const maxPoints = 512
	if max < min {
		return nil
	}
	span := max - min + 1
	if span <= maxPoints {
		out := make([]int64, 0, span)
		for i := min; i <= max; i++ {
			out = append(out, i)
		}
		return out
	}
	step := (span + maxPoints - 1) / maxPoints
	out := make([]int64, 0, maxPoints+1)
	for i := min; i <= max; i += step {
		out = append(out, i)
	}
	if out[len(out)-1] != max {
		out = append(out, max)
	}
	return out
}

// render writes one template by name.
//
// Names are the {{define}} names, never the file names. ParseFS also registers
// one template per file under its base name whose body is only the text outside
// any define block, so "choke-points.html" exists and executes successfully
// while rendering nothing at all. A handler that passes the file name therefore
// looks like it works and returns an empty page. The pages are page-* for the
// mirror-image reason: a define named after its own file is a duplicate
// definition and fails the whole parse.
func (s *Server) render(w http.ResponseWriter, status int, name string, vm viewModel) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := s.tmpl.ExecuteTemplate(w, name, vm); err != nil {
		// Headers are already sent, so the only honest signal left is a log line
		// plus whatever partial output escaped. Rendering into a buffer first
		// would be cleaner, and is done in TestTemplatesRenderWithoutError.
		log.Printf("aether dashboard: template %s: %v", name, err)
	}
}

// The page templates are named page-* rather than after their file. html/template
// already registers a template per file under its base name, so a
// {{define "audit.html"}} inside templates/audit.html is a duplicate
// definition and ParseFS fails for the whole set.
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	s.src.Reload()
	s.render(w, http.StatusOK, "page-dashboard", s.baseModel())
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	s.src.Reload()
	s.render(w, http.StatusOK, "page-audit", s.baseModel())
}

func (s *Server) handleGraphPage(w http.ResponseWriter, r *http.Request) {
	s.src.Reload()
	s.render(w, http.StatusOK, "page-graph", s.baseModel())
}

func (s *Server) handleChainStatus(w http.ResponseWriter, r *http.Request) {
	s.src.Reload()
	vm := s.baseModel()
	// The independent verdict is never carried server-side. The banner's
	// authoritative state is whatever the browser's WASM verifier last
	// reported, injected client-side, so a compromised or buggy server cannot
	// assert "verified" on the operator's behalf.
	s.render(w, http.StatusOK, "chain-status", vm)
}

func (s *Server) handleWorkspaceSummary(w http.ResponseWriter, r *http.Request) {
	s.render(w, http.StatusOK, "workspace-summary", s.baseModel())
}

func (s *Server) handleTimeTravel(w http.ResponseWriter, r *http.Request) {
	vm := s.baseModel()
	if v := r.URL.Query().Get("seq"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			writeErr(w, http.StatusBadRequest, "ARGS", "seq must be a non-negative integer")
			return
		}
		vm.FromSeq, vm.ToSeq = n, n
	}
	if ck := NearestCheckpoint(s.src.Entries(), vm.ToSeq); ck > 0 {
		vm.Warnings = append(vm.Warnings, fmt.Sprintf(
			"replaying from checkpoint at seq %d (%d operation(s) to apply)", ck, nearestCheckpointOps(s.src.Entries(), vm.ToSeq)))
	}
	s.render(w, http.StatusOK, "time-travel", vm)
}

func nearestCheckpointOps(entries []AuditEntry, seq int64) int {
	n := 0
	for _, e := range entries {
		if e.Seq > seq {
			break
		}
		if IsGraphOp(e.Command) {
			n++
		}
	}
	return n
}

func (s *Server) handleChokePoints(w http.ResponseWriter, r *http.Request) {
	vm := s.baseModel()
	target := r.URL.Query().Get("target")
	if target == "" {
		s.render(w, http.StatusOK, "choke-points", vm)
		return
	}
	res, err := chokeFromRequest(s.src.Graph(), target, r)
	if err != nil {
		vm.Warnings = append(vm.Warnings, "choke-point analysis unavailable: "+err.Error())
		vm.Target = target
		s.render(w, http.StatusOK, "choke-points", vm)
		return
	}
	vm.Target = target
	vm.Choke = &res
	s.render(w, http.StatusOK, "choke-points", vm)
}

func (s *Server) handleDiffMode(w http.ResponseWriter, r *http.Request) {
	vm := s.baseModel()
	q := r.URL.Query()
	from, err1 := strconv.ParseInt(orZero(q.Get("from")), 10, 64)
	to, err2 := strconv.ParseInt(orZero(q.Get("to")), 10, 64)
	if err1 != nil || err2 != nil {
		vm.Warnings = append(vm.Warnings, "diff requires integer from and to sequence numbers")
		s.render(w, http.StatusOK, "diff-mode", vm)
		return
	}
	if to < from {
		vm.Warnings = append(vm.Warnings, "diff 'to' must not be earlier than 'from'")
		s.render(w, http.StatusOK, "diff-mode", vm)
		return
	}
	vm.FromSeq, vm.ToSeq = from, to
	vm.Diff = diffBetween(s.src.Entries(), s.src.Graph(), from, to, q.Get("target"))
	s.render(w, http.StatusOK, "diff-mode", vm)
}

func orZero(s string) string {
	if strings.TrimSpace(s) == "" {
		return "0"
	}
	return s
}

func (s *Server) handleGraphNode(w http.ResponseWriter, r *http.Request) {
	vm := s.baseModel()
	id := r.URL.Query().Get("id")
	nv := s.nodeViewFor(id)
	vm.Node = nv
	if nv != nil && !nv.Found && nv.Reason != "" {
		vm.Warnings = append(vm.Warnings, nv.Reason)
	}
	s.render(w, http.StatusOK, "graph-node-panel", vm)
}

// nodeViewFor resolves a node ID (or an edge key, prefixed "edge:") and finds
// the audit entry that first introduced it.
func (s *Server) nodeViewFor(id string) *nodeView {
	if id == "" {
		return nil
	}
	nv := &nodeView{ID: id}
	if key, ok := strings.CutPrefix(id, "edge:"); ok {
		parts := strings.Split(key, "\x1f")
		if len(parts) == 3 {
			nv.Type = "edge"
			nv.Label = parts[0] + " -[" + parts[2] + "]-> " + parts[1]
		} else {
			nv.Type = "edge"
			nv.Label = key
		}
		nv.Entry = s.firstEntryFor(func(cmd, payload string) bool {
			return cmd == OpAddEdge && strings.Contains(payload, parts[0]) && strings.Contains(payload, parts[1])
		})
		nv.Found = nv.Entry != nil
		if !nv.Found {
			nv.Reason = "no audit entry in this chain records the creation of this edge; it came from the graph file the dashboard was started with"
		}
		return nv
	}

	g := s.src.Graph()
	if g != nil {
		for _, n := range g.Nodes {
			if n.ID != id {
				continue
			}
			nv.Label, nv.Type, nv.Provider, nv.Props = n.Label, n.Type, n.Provider, n.Props
			nv.InDegree, nv.OutDegree = g.Degree(n.ID)
			nv.Found = true
			break
		}
	}
	if !nv.Found {
		nv.Reason = "no node with ID " + id + " exists in the current graph"
		return nv
	}
	nv.Entry = s.firstEntryFor(func(cmd, payload string) bool {
		return cmd == OpAddNode && strings.Contains(payload, `"`+id+`"`)
	})
	if nv.Entry == nil {
		nv.Reason = "this node exists in the graph file but no audit entry in this chain records when it was discovered, so no signed provenance can be shown for it"
	}
	return nv
}

// firstEntryFor returns the earliest chain entry whose command and payload
// satisfy pred.
func (s *Server) firstEntryFor(pred func(command, payload string) bool) *AuditEntry {
	entries := s.src.Entries()
	for i := range entries {
		if pred(entries[i].Command, entries[i].Result) {
			e := entries[i]
			return &e
		}
	}
	return nil
}

// ---------------------------------------------------------------- JSON APIs

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	st := s.src.Status()
	ok := st.LoadError == "" && st.HasPubKey && st.Verify.Valid
	writeJSON(w, http.StatusOK, map[string]any{
		"status":        map[bool]string{true: "ok", false: "degraded"}[ok],
		"entry_count":   st.EntryCount,
		"head_seq":      st.HeadSeq,
		"head_hash":     st.HeadHash,
		"chain_valid":   st.Verify.Valid,
		"node_count":    st.NodeCount,
		"edge_count":    st.EdgeCount,
		"load_error":    st.LoadError,
		"loopback_only": !s.cfg.BindAll,
		"capability":    s.cfg.Capability,
		"read_only":     true,
	})
}

func (s *Server) handleChainJSON(w http.ResponseWriter, r *http.Request) {
	s.src.Reload()
	entries := s.src.Entries()
	if r.URL.Query().Get("format") == "ndjson" {
		// NDJSON is what a very large chain should use: one entry per line lets
		// a streaming client start verifying before the last entry arrives,
		// instead of buffering a multi-megabyte array.
		w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		for _, e := range entries {
			if err := enc.Encode(e); err != nil {
				return
			}
		}
		return
	}
	pub := s.src.PublicKey()
	if pub == nil {
		writeErr(w, http.StatusServiceUnavailable, "AUDIT",
			"no audit chain or signing key is attached, so there is nothing to serve")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"entries":             entries,
		"pubkey":              base64Std(pub),
		"pubkey_fingerprint":  FingerprintPublicKey(pub),
		"genesis":             GenesisHash,
		"hash_format":         "sha256(lowercase hex) over \"<seq>|<timestamp>|<command>|<result>|<prev_hash>\"",
		"signature_format":    "Ed25519 over the ASCII bytes of the entry's hex hash, base64 std encoded",
		"server_verification": s.src.Status().Verify,
	})
}

func (s *Server) handlePubkeyJSON(w http.ResponseWriter, r *http.Request) {
	pub := s.src.PublicKey()
	if pub == nil {
		writeErr(w, http.StatusServiceUnavailable, "AUDIT", "no signing public key is available")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"algorithm":     "Ed25519",
		"pubkey_base64": base64Std(pub),
		"pubkey_hex":    hexOf(pub),
		"fingerprint":   FingerprintPublicKey(pub),
		"genesis":       GenesisHash,
		"note":          "The browser verifies the chain against this key. The key is served over the same channel as the chain, so a server that lies about the chain is caught by the signatures, and a server that lies about the key is caught by the operator comparing the fingerprint out of band.",
	})
}

func (s *Server) handleEntryJSON(w http.ResponseWriter, r *http.Request) {
	seq, err := strconv.ParseInt(r.PathValue("seq"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "ARGS", "seq must be an integer")
		return
	}
	e, ok := s.entryAt(seq)
	if !ok {
		writeErr(w, http.StatusNotFound, "AUDIT", fmt.Sprintf("no audit entry with seq %d", seq))
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func (s *Server) entryAt(seq int64) (AuditEntry, bool) {
	for _, e := range s.src.Entries() {
		if e.Seq == seq {
			return e, true
		}
	}
	return AuditEntry{}, false
}

// graphResponse is the wire shape of a graph, with provenance attached.
type graphResponse struct {
	Nodes     []engine.GraphNode `json:"nodes"`
	Edges     []engine.GraphEdge `json:"edges"`
	Metadata  graphMetadata      `json:"metadata"`
	ChokeKeys []string           `json:"choke_edge_keys,omitempty"`
}

// graphMetadata states where this graph came from, so a rendered picture always
// says which point in history it depicts.
type graphMetadata struct {
	Workspace      string `json:"workspace,omitempty"`
	ChainHead      string `json:"chain_head"`
	GraphSeq       int64  `json:"graph_seq"`
	HeadSeq        int64  `json:"head_seq"`
	AppliedOps     int    `json:"applied_ops"`
	FromCheckpoint int64  `json:"from_checkpoint,omitempty"`
	HasHistory     bool   `json:"has_history"`
	ChainValid     bool   `json:"chain_valid"`
	Source         string `json:"source"`
	Note           string `json:"note,omitempty"`
}

func (s *Server) handleGraphJSON(w http.ResponseWriter, r *http.Request) {
	g := graphOrEmpty(s.src.Graph())
	st := s.src.Status()
	meta := graphMetadata{
		Workspace:  s.cfg.Workspace,
		ChainHead:  st.HeadHash,
		GraphSeq:   st.HeadSeq,
		HeadSeq:    st.HeadSeq,
		ChainValid: st.Verify.Valid,
		Source:     "current graph (graph file as loaded at dashboard start)",
	}
	if st.Timeline.OpCount == 0 {
		meta.Note = "this chain records no graph operations, so the current graph is the only state that can be shown"
	}
	writeJSON(w, http.StatusOK, graphResponse{Nodes: g.Nodes, Edges: g.Edges, Metadata: meta})
}

func (s *Server) handleGraphAtSeqJSON(w http.ResponseWriter, r *http.Request) {
	seq, err := strconv.ParseInt(r.PathValue("seq"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "ARGS", "seq must be an integer")
		return
	}
	entries := s.src.Entries()
	st := s.src.Status()
	if seq > st.HeadSeq {
		writeErr(w, http.StatusBadRequest, "ARGS",
			fmt.Sprintf("seq %d is beyond the chain head at %d", seq, st.HeadSeq))
		return
	}
	base := s.src.Graph()
	g, applied, err := StateAtSeq(entries, base, seq)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	ck := NearestCheckpoint(entries, seq)
	meta := graphMetadata{
		Workspace:      s.cfg.Workspace,
		ChainHead:      st.HeadHash,
		GraphSeq:       seq,
		HeadSeq:        st.HeadSeq,
		AppliedOps:     applied,
		FromCheckpoint: ck,
		HasHistory:     st.Timeline.OpCount > 0,
		ChainValid:     st.Verify.Valid,
		Source:         fmt.Sprintf("chain entry %d", seq),
	}
	if st.Timeline.OpCount == 0 {
		meta.Note = "no graph operations exist in this chain, so this position reconstructs to the current graph unchanged; time-travel has no history to show"
	}
	resp := graphResponse{Nodes: g.Nodes, Edges: g.Edges, Metadata: meta}
	if target := r.URL.Query().Get("choke_target"); target != "" {
		if res, err := chokeFromRequest(g, target, r); err == nil {
			keys := make([]string, 0, len(res.ChokePoints))
			for _, c := range res.ChokePoints {
				keys = append(keys, c.Key())
			}
			resp.ChokeKeys = keys
			meta.Note = strings.TrimSpace(meta.Note + " choke-point analysis is exact over the enumerated path set; see /api/graph/choke-points for the truncation flag.")
		} else {
			meta.Note = strings.TrimSpace(meta.Note + " choke-point analysis failed: " + err.Error())
		}
		resp.Metadata = meta
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGraphDiffJSON(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from, err1 := strconv.ParseInt(orZero(q.Get("from")), 10, 64)
	to, err2 := strconv.ParseInt(orZero(q.Get("to")), 10, 64)
	if err1 != nil || err2 != nil {
		writeErr(w, http.StatusBadRequest, "ARGS", "from and to must be integers")
		return
	}
	if to < from {
		writeErr(w, http.StatusBadRequest, "ARGS", "to must not be earlier than from")
		return
	}
	st := s.src.Status()
	if to > st.HeadSeq {
		writeErr(w, http.StatusBadRequest, "ARGS", fmt.Sprintf("to=%d is beyond the chain head at %d", to, st.HeadSeq))
		return
	}
	d := diffBetween(s.src.Entries(), s.src.Graph(), from, to, q.Get("target"))
	if d == nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not reconstruct one of the requested graph states")
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// diffBetween reconstructs both endpoints and diffs them. Both endpoints are
// anchored to chain sequence numbers, so the comparison is against signed
// history rather than against two arbitrary graphs.
func diffBetween(entries []AuditEntry, base *engine.IdentityGraph, from, to int64, target string) *GraphDiff {
	fromG, _, err := StateAtSeq(entries, base, from)
	if err != nil {
		return nil
	}
	toG, _, err := StateAtSeq(entries, base, to)
	if err != nil {
		return nil
	}
	d := Diff(fromG, toG, from, to)
	if target != "" {
		DiffPaths(d, fromG, toG, target, ChokePointOptions{})
	}
	return d
}

func (s *Server) handleChokePointsJSON(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("target")
	if strings.TrimSpace(target) == "" {
		writeErr(w, http.StatusBadRequest, "ARGS", "target is required")
		return
	}
	res, err := chokeFromRequest(s.src.Graph(), target, r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "ARGS", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// chokeFromRequest parses the analysis parameters and runs the analysis.
func chokeFromRequest(g *engine.IdentityGraph, target string, r *http.Request) (ChokePointResult, error) {
	opts := ChokePointOptions{}
	q := r.URL.Query()
	if v := q.Get("threshold"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f <= 0 || f > 1 {
			return ChokePointResult{}, fmt.Errorf("threshold must be a number in (0,1]")
		}
		opts.Threshold = f
	}
	if v := q.Get("max_hops"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 12 {
			return ChokePointResult{}, fmt.Errorf("max_hops must be an integer in [1,12]")
		}
		opts.MaxHops = n
	}
	if v := q.Get("max_paths"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 5000000 {
			return ChokePointResult{}, fmt.Errorf("max_paths must be an integer in [1,5000000]")
		}
		opts.MaxPaths = n
	}
	return ChokePoints(graphOrEmpty(g), target, opts)
}

func (s *Server) handleNodeAuditEntry(w http.ResponseWriter, r *http.Request) {
	nv := s.nodeViewFor(r.PathValue("id"))
	if nv == nil {
		writeErr(w, http.StatusBadRequest, "ARGS", "id is required")
		return
	}
	if nv.Entry == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"id":     nv.ID,
			"label":  nv.Label,
			"found":  false,
			"reason": nv.Reason,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":    nv.ID,
		"label": nv.Label,
		"type":  nv.Type,
		"found": true,
		"entry": nv.Entry,
		"proof": "/api/proof/" + strconv.FormatInt(nv.Entry.Seq, 10),
	})
}

// handleProofJSON emits an exportable, independently verifiable proof bundle
// for one chain entry.
func (s *Server) handleProofJSON(w http.ResponseWriter, r *http.Request) {
	seq, err := strconv.ParseInt(r.PathValue("seq"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "ARGS", "seq must be an integer")
		return
	}
	b, err := s.ProofBundle(seq)
	if err != nil {
		status := http.StatusInternalServerError
		layer := "INTERNAL"
		if !IsGraphOp("") && (seq < 0) {
			status, layer = http.StatusBadRequest, "ARGS"
		}
		if _, ok := s.entryAt(seq); !ok {
			status, layer = http.StatusNotFound, "AUDIT"
		}
		writeErr(w, status, layer, err.Error())
		return
	}
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="aether-proof-%d.json"`, seq))
	writeJSON(w, http.StatusOK, b)
}

// sortedStrings returns a sorted copy, used where output order must not depend
// on Go map iteration.
func sortedStrings(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
