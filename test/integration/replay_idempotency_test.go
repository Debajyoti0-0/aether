//go:build integration

// Stage 5 backfill replay/idempotency evidence (B5-G12): a 10-scenario
// idempotency regression against the real workspace idempotency store
// (atomic PutRecordIfAbsent in the vault). Unknown fields are preserved
// byte-exact; replays return the ORIGINAL outcome; deterministic
// outcomes across repeated runs and reconnects.
package integration

import (
	"bytes"
	"testing"

	"github.com/Debajyoti0-0/aether/internal/workspace"
)

func replayWS(t *testing.T) *workspace.Workspace {
	t.Helper()
	return newWorkspace(t, "replay-idem")
}

func TestReplayIdempotency(t *testing.T) {
	const orig = `{"status":"completed","operation_id":"op-1","result":{"vm":"vm-1"}}`
	const replayed = `{"status":"completed","operation_id":"op-REPLAY","result":{"vm":"vm-CHANGED"}}`

	t.Run("scenario01_first_execution_wins", func(t *testing.T) {
		ws := replayWS(t)
		existing, err := ws.IdempotencyPutIfAbsent("op-key-1", []byte(orig))
		if err != nil || existing != nil {
			t.Fatalf("existing=%v err=%v", existing, err)
		}
	})
	t.Run("scenario02_replay_returns_original", func(t *testing.T) {
		ws := replayWS(t)
		if _, err := ws.IdempotencyPutIfAbsent("op-key-2", []byte(orig)); err != nil {
			t.Fatal(err)
		}
		existing, err := ws.IdempotencyPutIfAbsent("op-key-2", []byte(replayed))
		if err != nil || existing == nil {
			t.Fatalf("existing=%v err=%v", existing, err)
		}
		if !bytes.Equal(existing, []byte(orig)) {
			t.Fatalf("replay mutated outcome: %s", existing)
		}
	})
	t.Run("scenario03_get_deterministic_across_reads", func(t *testing.T) {
		ws := replayWS(t)
		if err := ws.IdempotencyPut("op-key-3", []byte(orig)); err != nil {
			t.Fatal(err)
		}
		a, err := ws.IdempotencyGet("op-key-3")
		if err != nil {
			t.Fatal(err)
		}
		b, err := ws.IdempotencyGet("op-key-3")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a, b) || !bytes.Equal(a, []byte(orig)) {
			t.Fatalf("nondeterministic reads: %s vs %s", a, b)
		}
	})
	t.Run("scenario04_unknown_fields_preserved_byte_exact", func(t *testing.T) {
		ws := replayWS(t)
		payload := []byte(`{"status":"completed","unknown_future_field":{"nested":[1,2,3]},"x-trailer":true}`)
		if err := ws.IdempotencyPut("op-key-4", payload); err != nil {
			t.Fatal(err)
		}
		got, err := ws.IdempotencyGet("op-key-4")
		if err != nil || !bytes.Equal(got, payload) {
			t.Fatalf("unknown fields not preserved: %s err=%v", got, err)
		}
	})
	t.Run("scenario05_independent_keys_independent_outcomes", func(t *testing.T) {
		ws := replayWS(t)
		if err := ws.IdempotencyPut("op-key-5a", []byte(orig)); err != nil {
			t.Fatal(err)
		}
		if err := ws.IdempotencyPut("op-key-5b", []byte(replayed)); err != nil {
			t.Fatal(err)
		}
		a, _ := ws.IdempotencyGet("op-key-5a")
		b, _ := ws.IdempotencyGet("op-key-5b")
		if !bytes.Equal(a, []byte(orig)) || !bytes.Equal(b, []byte(replayed)) {
			t.Fatalf("cross-contamination: %s / %s", a, b)
		}
	})
	t.Run("scenario06_replay_after_reconnect", func(t *testing.T) {
		name := "replay-reconnect-" + sanitizeName(t.Name())
		ws, err := workspace.Create(name, "integration-pass")
		if err != nil {
			t.Fatal(err)
		}
		if err := ws.IdempotencyPut("op-key-6", []byte(orig)); err != nil {
			t.Fatal(err)
		}
		if err := ws.Close(); err != nil {
			t.Fatal(err)
		}
		w2, err := workspace.Open(name, "integration-pass")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = w2.Close() })
		got, err := w2.IdempotencyGet("op-key-6")
		if err != nil || !bytes.Equal(got, []byte(orig)) {
			t.Fatalf("post-reconnect replay: %s err=%v", got, err)
		}
	})
	t.Run("scenario07_replay_of_existing_via_putifabsent", func(t *testing.T) {
		ws := replayWS(t)
		if _, err := ws.IdempotencyPutIfAbsent("op-key-7", []byte(orig)); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 3; i++ {
			existing, err := ws.IdempotencyPutIfAbsent("op-key-7", []byte(replayed))
			if err != nil || existing == nil || !bytes.Equal(existing, []byte(orig)) {
				t.Fatalf("replay %d: existing=%s err=%v", i, existing, err)
			}
		}
	})
	t.Run("scenario08_negative_invalid_key_rejected", func(t *testing.T) {
		ws := replayWS(t)
		if err := ws.IdempotencyPut("", []byte(orig)); err == nil {
			t.Fatal("empty key accepted")
		}
		if err := ws.IdempotencyPut("../traversal", []byte(orig)); err == nil {
			t.Fatal("traversal key accepted")
		}
	})
	t.Run("scenario09_negative_missing_key", func(t *testing.T) {
		ws := replayWS(t)
		if _, err := ws.IdempotencyGet("op-key-missing"); err == nil {
			t.Fatal("missing key returned without error")
		}
	})
	t.Run("scenario10_deterministic_outcomes_repeated_runs", func(t *testing.T) {
		// Two separate workspaces executing the "same" operation must
		// produce byte-identical idempotency records (determinism).
		ws1 := newWorkspace(t, "replay-idem-a")
		ws2 := newWorkspace(t, "replay-idem-b")
		for _, ws := range []*workspace.Workspace{ws1, ws2} {
			if err := ws.IdempotencyPut("op-key-10", []byte(orig)); err != nil {
				t.Fatal(err)
			}
		}
		a, err := ws1.IdempotencyGet("op-key-10")
		if err != nil {
			t.Fatal(err)
		}
		b, err := ws2.IdempotencyGet("op-key-10")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a, b) {
			t.Fatalf("outcomes diverge: %s vs %s", a, b)
		}
	})
}
