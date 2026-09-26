package web

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"sync"
	"time"

	engine "github.com/Debajyoti0-0/aether/internal/engine/graph"
	"github.com/Debajyoti0-0/aether/internal/store"
)

// b64 is the standard base64 alphabet, matching how the audit chain encodes
// Ed25519 signatures, so a single decoder serves both.
var b64 = base64.StdEncoding

// Source is the dashboard's view of the engagement: the signed audit chain and
// the current identity graph.
//
// The two are read together on purpose. The graph is the state; the chain is the
// only thing that can say *when* that state was true and *who* attested to it.
// Serving a graph with no chain would present a snapshot whose provenance the
// operator cannot check, which is the failure mode this dashboard exists to
// prevent.
type Source struct {
	mu      sync.RWMutex
	log     *store.Log
	graph   *engine.IdentityGraph
	entries []AuditEntry
	pub     ed25519.PublicKey
	loaded  time.Time
	loadErr string

	subMu sync.Mutex
	subs  map[chan []byte]struct{}
}

// NewSource builds a Source over an existing audit log and graph. Either may be
// nil: a missing graph renders an empty graph with an explicit notice, and a
// missing log renders an empty chain. Neither case is allowed to panic or to
// present empty as if it were complete.
func NewSource(log *store.Log, g *engine.IdentityGraph) *Source {
	s := &Source{subs: make(map[chan []byte]struct{})}
	s.log = log
	s.graph = g
	if log != nil {
		s.pub = log.PublicKey()
	}
	s.Reload()
	return s
}

// Reload re-reads the chain from its backing store.
//
// The chain is append-only, so a reload is how a long-lived dashboard picks up
// entries appended by another process (the CLI, or a second operator). Entries
// are sorted by sequence before use so a reordered backing store is caught as a
// linkage failure rather than silently accepted.
func (s *Source) Reload() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loaded = time.Now().UTC()
	if s.log == nil {
		s.entries = nil
		s.loadErr = "no audit chain is attached to this dashboard"
		return
	}
	raw, err := s.log.Entries()
	if err != nil {
		s.loadErr = "read audit chain: " + err.Error()
		return
	}
	s.loadErr = ""
	entries := make([]AuditEntry, 0, len(raw))
	for _, e := range raw {
		entries = append(entries, entryFromStore(e))
	}
	sortEntries(entries)
	s.entries = entries
	if s.pub == nil {
		s.pub = s.log.PublicKey()
	}
}

// Entries returns a copy of the current chain, safe to hold and iterate.
func (s *Source) Entries() []AuditEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AuditEntry, len(s.entries))
	copy(out, s.entries)
	return out
}

// Graph returns the current graph. The caller must not mutate it; the dashboard
// treats it as immutable and derives every other state from a copy.
func (s *Source) Graph() *engine.IdentityGraph {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.graph
}

// PublicKey returns the chain's signing public key, or nil when no chain is
// attached.
func (s *Source) PublicKey() ed25519.PublicKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pub
}

// Status is the server's own view of chain health, for the initial render. The
// browser's WASM verifier reaches its own verdict independently; the UI shows
// both so a disagreement is visible rather than resolved in the server's favour.
type Status struct {
	Loaded      time.Time `json:"loaded"`
	LoadError   string    `json:"load_error,omitempty"`
	EntryCount  int       `json:"entry_count"`
	HeadSeq     int64     `json:"head_seq"`
	HeadHash    string    `json:"head_hash"`
	Genesis     string    `json:"genesis"`
	PubKey      string    `json:"pubkey_base64,omitempty"`
	PubKeyPrint string    `json:"pubkey_fingerprint,omitempty"`
	HasPubKey   bool      `json:"has_pubkey"`
	Verify      VerifyChainResult
	Timeline    Timeline `json:"timeline"`
	NodeCount   int      `json:"node_count"`
	EdgeCount   int      `json:"edge_count"`
	// Independent reports whether an independent verifier (the in-browser WASM
	// module) has reported a verdict yet. Until it does, the banner must not
	// claim the chain is verified.
	Independent string `json:"independent_state"`
	// Entries is the chain itself, newest-last, for server-rendered pages. The
	// JSON API serves the same data from /api/audit/chain; carrying it here too
	// keeps one reload path for both renderers.
	Entries []AuditEntry `json:"-"`
}

// Status builds the render model for the dashboard and chain-status partial.
func (s *Source) Status() Status {
	entries := s.Entries()
	s.mu.RLock()
	st := Status{
		Loaded:    s.loaded,
		LoadError: s.loadErr,
		HasPubKey: s.pub != nil,
	}
	g := s.graph
	s.mu.RUnlock()

	st.EntryCount = len(entries)
	st.Genesis = GenesisHash
	if len(entries) > 0 {
		st.HeadSeq = entries[len(entries)-1].Seq
		st.HeadHash = entries[len(entries)-1].Hash
	}
	if pub := s.PublicKey(); pub != nil {
		st.PubKey = base64Std(pub)
		st.PubKeyPrint = FingerprintPublicKey(pub)
		st.Verify = verifyPubEd25519(entries, pub)
	} else {
		st.Verify = VerifyChainResult{
			Valid:      false,
			EntryCount: len(entries),
			Error:      "no signing public key is available, so the chain cannot be verified",
			VerifiedBy: "none",
		}
	}
	if g != nil {
		st.NodeCount = len(g.Nodes)
		st.EdgeCount = len(g.Edges)
	}
	st.Timeline = BuildTimeline(entries, g)
	st.Independent = "pending"
	// Newest first for display. The copy is reversed in place on the local
	// slice header only; Entries() hands out a fresh copy each call.
	if len(entries) > 1 {
		for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
			entries[i], entries[j] = entries[j], entries[i]
		}
	}
	st.Entries = entries
	return st
}

// Subscribe registers a push channel. The channel is buffered so a slow client
// cannot block an appender; when the buffer is full the oldest message is
// dropped and the client is told it fell behind, because a dashboard that
// silently skips chain entries and still shows a green banner is worse than one
// that admits it is behind.
func (s *Source) Subscribe() (chan []byte, func()) {
	ch := make(chan []byte, 64)
	s.subMu.Lock()
	s.subs[ch] = struct{}{}
	s.subMu.Unlock()
	return ch, func() {
		s.subMu.Lock()
		delete(s.subs, ch)
		s.subMu.Unlock()
		close(ch)
	}
}

// Notify pushes a message to every subscriber.
func (s *Source) Notify(msg any) {
	b, err := json.Marshal(msg)
	if err != nil {
		return
	}
	s.subMu.Lock()
	defer s.subMu.Unlock()
	for ch := range s.subs {
		select {
		case ch <- b:
		default:
			// Drop the oldest message to make room and flag the gap, so the
			// client can re-fetch the chain rather than trust a partial stream.
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- []byte(`{"type":"audit.overflow"}`):
			default:
			}
		}
	}
}

// base64Std encodes with standard base64, matching how the audit chain encodes
// signatures, so one decoder serves both.
func base64Std(b []byte) string { return b64.EncodeToString(b) }
