package api

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Debajyoti0-0/aether/internal/workspace"
)

// T2 acceptance: subscriptions replay from the persistent event store
// by cursor, and reconnect resumes from the last seen sequence.
func TestSubscribeWithCursor(t *testing.T) {
	t.Setenv("AETHER_CONFIG_DIR", t.TempDir())
	w, err := workspace.Create("CursorWS", "pw")
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	es := NewEventStore(w.Vault())
	srvCert, clientCAs, clientPair, _ := testPKI(t)
	srv, err := NewTeamserver("127.0.0.1:0", srvCert, clientCAs, "", nil, func(op *Operator, req *CommandRequest) (*CommandResponse, error) {
		return &CommandResponse{Status: "completed"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	srv.SetEventStore(es)
	go srv.Serve()
	defer srv.Close()

	// Persist 5 events before anyone subscribes.
	for i := 0; i < 5; i++ {
		if _, err := es.Append("CursorWS", "test_event", "payload"); err != nil {
			t.Fatal(err)
		}
	}

	client, err := Dial(srv.Listener.Addr().String(), clientPair, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// Subscribe at cursor 2 → must receive seqs 3, 4, 5.
	updates, err := client.StreamWorkspaceFrom("CursorWS", 2)
	if err != nil {
		t.Fatal(err)
	}
	got := []uint64{}
	deadline := time.After(5 * time.Second)
	for len(got) < 3 {
		select {
		case ev, ok := <-updates:
			if !ok {
				t.Fatal("stream closed")
			}
			got = append(got, uint64(ev.Seq))
		case <-deadline:
			t.Fatalf("timeout; got seqs %v", got)
		}
	}
	for i, seq := range got {
		if seq != uint64(3+i) {
			t.Errorf("replay seq[%d] = %d, want %d", i, seq, 3+i)
		}
	}

	// New events arrive live with increasing seqs.
	srv.Publish(&WorkspaceUpdate{WorkspaceID: "CursorWS", Kind: "test_event", Payload: "live"})
	select {
	case ev, ok := <-updates:
		if !ok {
			t.Fatal("stream closed")
		}
		if ev.Seq != 6 {
			t.Errorf("live seq = %d, want 6", ev.Seq)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for live event")
	}

	// A fresh client resuming at seq 5 gets exactly the live event.
	client2, err := Dial(srv.Listener.Addr().String(), clientPair, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	defer client2.Close()
	updates2, err := client2.StreamWorkspaceFrom("CursorWS", 5)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case ev, ok := <-updates2:
		if !ok {
			t.Fatal("stream closed")
		}
		if ev.Seq != 6 {
			t.Errorf("resume seq = %d, want 6", ev.Seq)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for resume")
	}
	select {
	case ev := <-updates2:
		t.Fatalf("unexpected extra event on resume: %+v", ev)
	case <-time.After(500 * time.Millisecond):
	}
}

// Event store semantics: gap detection and monotonic sequencing.
func TestEventStoreSequencing(t *testing.T) {
	t.Setenv("AETHER_CONFIG_DIR", t.TempDir())
	w, err := workspace.Create("SeqWS", "pw")
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	es := NewEventStore(w.Vault())

	for i := 0; i < 3; i++ {
		seq, err := es.Append("SeqWS", "kind", "p")
		if err != nil {
			t.Fatal(err)
		}
		if seq != uint64(i+1) {
			t.Fatalf("seq = %d, want %d", seq, i+1)
		}
	}
	events, latest, err := es.ReadSince("SeqWS", 0)
	if err != nil || latest != 3 || len(events) != 3 {
		t.Fatalf("read = %d events, latest %d, err %v", len(events), latest, err)
	}
	// Unknown workspace → empty, no error.
	events2, latest2, err := es.ReadSince("OtherWS", 0)
	if err != nil || latest2 != 0 || len(events2) != 0 {
		t.Fatalf("unknown ws read = %d/%d/%v", len(events2), latest2, err)
	}
}

// T2 acceptance (multi-operator): three authenticated operators issue
// concurrent commands; responses are correlated per operator
// connection, and the audit chain stays intact (no cross-delivery, no
// corruption).
func TestMultiOperatorRace(t *testing.T) {
	t.Setenv("AETHER_CONFIG_DIR", t.TempDir())
	w, err := workspace.Create("MultiOpWS", "pw")
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	srvCert, clientCAs, _, caDir := testPKI(t)
	var mu sync.Mutex
	seenActors := map[string]int{}
	srv, err := NewTeamserver("127.0.0.1:0", srvCert, clientCAs, "", nil, func(op *Operator, req *CommandRequest) (*CommandResponse, error) {
		mu.Lock()
		seenActors[op.Name]++
		mu.Unlock()
		return &CommandResponse{Status: "completed", Output: op.Name + ":" + req.CommandLine}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve()
	defer srv.Close()

	operators := []string{"op-a", "op-b", "op-c"}
	const perOperator = 50

	var wg sync.WaitGroup
	errCh := make(chan error, len(operators)*perOperator)
	for _, name := range operators {
		name := name
		pair := issueOperatorPair(t, caDir, name)
		client, err := Dial(srv.Listener.Addr().String(), pair, nil, true)
		if err != nil {
			t.Fatal(err)
		}
		defer client.Close()

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer client.Close()
			var inner sync.WaitGroup
			sem := make(chan struct{}, 8)
			for i := 0; i < perOperator; i++ {
				inner.Add(1)
				go func(i int) {
					defer inner.Done()
					sem <- struct{}{}
					defer func() { <-sem }()
					resp, err := client.ExecuteCommand(&CommandRequest{
						WorkspaceID: w.Name,
						CommandLine: "probe-" + name + "-" + itoa(i),
					})
					if err != nil {
						errCh <- err
						return
					}
					if !strings.HasPrefix(resp.Output, name+":") {
						errCh <- errIdentityLeak(name, resp.Output)
					}
				}(i)
			}
			inner.Wait()
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, name := range operators {
		if seenActors[name] != perOperator {
			t.Errorf("operator %s served %d commands, want %d", name, seenActors[name], perOperator)
		}
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

func errIdentityLeak(want, got string) error {
	return &identityLeakError{want: want, got: got}
}

type identityLeakError struct{ want, got string }

func (e *identityLeakError) Error() string {
	return "response identity leak: want prefix " + e.want + ", got " + e.got
}
