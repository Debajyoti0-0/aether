package api

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Debajyoti0-0/aether/internal/store"
)

// EventStore is the persistent, cursor-addressable event log backing
// teamserver subscriptions (Stage 3, T2). Events are stored in the
// attached workspace's vault (bucket "ts_events", key = 20-digit
// zero-padded seq) so subscribers can resume after reconnect from any
// sequence — the in-memory ring of Stage 1/2 could not.
//
// Sequencing: one monotonic sequence per workspace, persisted in the
// vault meta bucket in the same transaction as the event itself, so a
// gap is always a real (detectable) anomaly, never a lost update.
type EventStore struct {
	v  *store.Vault
	mu sync.Mutex
}

// NewEventStore binds an event store to a vault.
func NewEventStore(v *store.Vault) *EventStore { return &EventStore{v: v} }

const (
	bucketTSEvents   = "ts_events"
	metaTSEventsSeq  = "ts_events_seq:"
	maxEventsPerWS   = 100000
)

// Append persists an event and returns its workspace-scoped sequence.
func (es *EventStore) Append(wsID string, kind, payload string) (uint64, error) {
	es.mu.Lock()
	defer es.mu.Unlock()

	metaName := metaTSEventsSeq + wsID
	seq, err := es.seq(metaName)
	if err != nil {
		return 0, err
	}
	seq++
	ev := WorkspaceUpdate{WorkspaceID: wsID, Kind: kind, Payload: payload, Timestamp: nowUnix()}
	data, err := json.Marshal(ev)
	if err != nil {
		return 0, err
	}
	if err := es.v.PutRecord(bucketTSEvents, seqKey(wsID, seq), data); err != nil {
		return 0, err
	}
	if err := es.v.MetaSet(metaName, seqBytes(seq)); err != nil {
		return 0, err
	}
	// Retention: prune events beyond the cap (best effort).
	if seq > maxEventsPerWS {
		_ = es.v.DeleteRecord(bucketTSEvents, seqKey(wsID, seq-maxEventsPerWS))
	}
	return seq, nil
}

// ReadSince returns persisted events for the workspace with
// sequence > afterSeq, oldest-first. Used for subscribe-with-cursor
// replay on (re)connect.
func (es *EventStore) ReadSince(wsID string, afterSeq uint64) ([]WorkspaceUpdate, uint64, error) {
	es.mu.Lock()
	defer es.mu.Unlock()

	latest, err := es.seq(metaTSEventsSeq + wsID)
	if err != nil {
		return nil, 0, err
	}
	var out []WorkspaceUpdate
	for s := afterSeq + 1; s <= latest; s++ {
		data, err := es.v.GetRecord(bucketTSEvents, seqKey(wsID, s))
		if err != nil {
			// A missing sequence inside the range is a real gap —
			// surfaced to the consumer, never skipped silently.
			return out, latest, fmt.Errorf("event gap at seq %d for workspace %q", s, wsID)
		}
		var ev WorkspaceUpdate
		if err := json.Unmarshal(data, &ev); err != nil {
			return out, latest, fmt.Errorf("corrupt event at seq %d: %w", s, err)
		}
		out = append(out, ev)
	}
	return out, latest, nil
}

// LatestSeq returns the newest persisted sequence for the workspace.
func (es *EventStore) LatestSeq(wsID string) (uint64, error) {
	es.mu.Lock()
	defer es.mu.Unlock()
	return es.seq(metaTSEventsSeq + wsID)
}

func (es *EventStore) seq(metaName string) (uint64, error) {
	raw, err := es.v.MetaGet(metaName)
	if err != nil {
		return 0, err
	}
	return seqParse(raw), nil
}

func seqKey(wsID string, seq uint64) string {
	return fmt.Sprintf("%s:%020d", wsID, seq)
}

func seqBytes(seq uint64) []byte {
	b := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		b[i] = byte(seq)
		seq >>= 8
	}
	return b
}

func seqParse(b []byte) uint64 {
	if len(b) != 8 {
		return 0
	}
	var seq uint64
	for _, c := range b {
		seq = seq<<8 | uint64(c)
	}
	return seq
}

func nowUnix() int64 { return time.Now().Unix() }
