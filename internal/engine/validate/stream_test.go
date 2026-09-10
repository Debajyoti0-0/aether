package validate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestStreamEventsHEC(t *testing.T) {
	var received int32
	var lastAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastAuth = r.Header.Get("Authorization")
		atomic.AddInt32(&received, int32(countLines(r)))
		json.NewEncoder(w).Encode(map[string]any{"text": "Success", "code": 0})
	}))
	defer srv.Close()

	c := NewHECClient(srv.URL+"/services/collector/event", "hec-token", "aether", "aether:sim", "redteam")
	n, err := c.StreamEvents(context.Background(), []map[string]any{
		{"User": "a@b.c", "Op": "signin"},
		{"User": "d@e.f", "Op": "signin"},
		{"User": "g@h.i", "Op": "signin"},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if n != 3 {
		t.Errorf("accepted = %d", n)
	}
	if atomic.LoadInt32(&received) != 3 {
		t.Errorf("server received = %d", received)
	}
	if lastAuth != "Splunk hec-token" {
		t.Errorf("auth = %q", lastAuth)
	}
}

func countLines(r *http.Request) int {
	dec := json.NewDecoder(r.Body)
	n := 0
	for dec.More() {
		var v map[string]any
		if dec.Decode(&v) != nil {
			break
		}
		n++
	}
	return n
}

func TestStreamEventsHECReject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"text":"Invalid token","code":3}`))
	}))
	defer srv.Close()

	c := NewHECClient(srv.URL, "bad", "", "", "")
	if _, err := c.StreamEvents(context.Background(), []map[string]any{{"x": 1}}); err == nil {
		t.Error("invalid token should fail")
	}
}

func TestStreamEventsEmpty(t *testing.T) {
	c := NewHECClient("http://x", "t", "", "", "")
	if n, err := c.StreamEvents(context.Background(), nil); err != nil || n != 0 {
		t.Errorf("n=%d err=%v", n, err)
	}
}

func TestStreamFuzzBatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"text": "Success", "code": 0})
	}))
	defer srv.Close()

	fuzzRNG = randNew(7)
	base := map[string]any{"time": "2026-09-09T12:00:00Z", "User": "a@b.c"}
	variants := FuzzTelemetry(base, 4)

	c := NewHECClient(srv.URL, "t", "aether", "", "")
	n, err := c.StreamFuzzBatch(context.Background(), variants)
	if err != nil {
		t.Fatalf("stream fuzz: %v", err)
	}
	if n != 4 {
		t.Errorf("accepted = %d", n)
	}
}
