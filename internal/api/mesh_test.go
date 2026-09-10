package api

import (
	"sync"
	"testing"
	"time"
)

func TestPublishFansOutToAllSubscribers(t *testing.T) {
	// Two independently issued operators (real CA hierarchy, T1).
	srvCert, clientCAs, _, caDir := testPKI(t)
	srv, err := NewTeamserver("127.0.0.1:0", srvCert, clientCAs, "", nil, func(op *Operator, req *CommandRequest) (*CommandResponse, error) {
		return &CommandResponse{Status: "completed"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve()
	defer srv.Close()

	pairA := issueOperatorPair(t, caDir, "op-a")
	pairB := issueOperatorPair(t, caDir, "op-b")
	op1, err := Dial(srv.Listener.Addr().String(), pairA, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	defer op1.Close()
	op2, err := Dial(srv.Listener.Addr().String(), pairB, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	defer op2.Close()

	up1, err := op1.StreamWorkspace("MeshWS")
	if err != nil {
		t.Fatal(err)
	}
	up2, err := op2.StreamWorkspace("MeshWS")
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(150 * time.Millisecond)
	srv.Publish(&WorkspaceUpdate{WorkspaceID: "MeshWS", Kind: "mesh_event", Payload: "hello-both"})

	deadline := time.After(3 * time.Second)
	got1, got2 := false, false
	for !(got1 && got2) {
		select {
		case ev, ok := <-up1:
			if !ok {
				t.Fatal("op1 stream closed")
			}
			if ev.Kind == "mesh_event" {
				got1 = true
			}
		case ev, ok := <-up2:
			if !ok {
				t.Fatal("op2 stream closed")
			}
			if ev.Kind == "mesh_event" {
				got2 = true
			}
		case <-deadline:
			t.Fatalf("fan-out incomplete: op1=%t op2=%t", got1, got2)
		}
	}
}
func TestSubscribeReplacesOldChannel(t *testing.T) {
	// Subscribers accumulate per workspace and can be removed
	// individually without affecting others.
	srv := &Teamserver{
		events:  map[string][]*WorkspaceUpdate{},
		subs:    map[string][]*subscriber{},
		subByCh: map[chan *WorkspaceUpdate]*subscriber{},
	}

	sub1 := srv.subscribe("WS")
	sub2 := srv.subscribe("WS")

	// Both subscribers receive the publish.
	srv.Publish(&WorkspaceUpdate{WorkspaceID: "WS", Kind: "mesh"})
	for _, sub := range []*subscriber{sub1, sub2} {
		select {
		case ev, ok := <-sub.ch:
			if !ok || ev.Kind != "mesh" {
				t.Errorf("ev = %+v ok=%t", ev, ok)
			}
		default:
			t.Fatal("subscriber did not receive publish")
		}
	}

	// Unsubscribing one leaves the other live, and the unsubscribed
	// channel must never receive a publish again.
	srv.unsubscribe("WS", sub1)
	srv.Publish(&WorkspaceUpdate{WorkspaceID: "WS", Kind: "second"})
	select {
	case ev, ok := <-sub2.ch:
		if !ok || ev.Kind != "second" {
			t.Errorf("ev = %+v ok=%t", ev, ok)
		}
	default:
		t.Fatal("sub2 stopped receiving after sub1 unsubscribed")
	}
	select {
	case ev := <-sub1.ch:
		t.Fatalf("unsubscribed sub1 received publish: %+v", ev)
	default:
	}
}

// TestPublishUnsubscribeRace hammers Publish against subscribe/
// unsubscribe cycles. The historical bug â€” Publish sending on a channel
// closed by unsubscribe â€” crashed the process with a send-on-closed-
// channel panic. The lifecycle fix (single closure owner, done-guarded
// sends, never-closed event channels) must survive this hammering under
// -race.
func TestPublishUnsubscribeRace(t *testing.T) {
	srv := &Teamserver{
		events:  map[string][]*WorkspaceUpdate{},
		subs:    map[string][]*subscriber{},
		subByCh: map[chan *WorkspaceUpdate]*subscriber{},
	}

	const publishers = 100
	const churners = 100
	const publishes = 50

	var wg sync.WaitGroup
	for i := 0; i < publishers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < publishes; j++ {
				srv.Publish(&WorkspaceUpdate{WorkspaceID: "RaceWS", Kind: "hammer", Payload: "x"})
			}
		}()
	}
	for i := 0; i < churners; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				sub := srv.subscribe("RaceWS")
				srv.Publish(&WorkspaceUpdate{WorkspaceID: "RaceWS", Kind: "hammer", Payload: "y"})
				srv.unsubscribe("RaceWS", sub)
				// Repeated unsubscribe must be safe (idempotent teardown).
				srv.unsubscribe("RaceWS", sub)
			}
		}()
	}

	// Concurrent unsubscribe during active publication of live subs.
	wg.Add(1)
	go func() {
		defer wg.Done()
		live := srv.subscribe("RaceWS")
		for j := 0; j < publishes; j++ {
			srv.Publish(&WorkspaceUpdate{WorkspaceID: "RaceWS", Kind: "hammer", Payload: "z"})
		}
		srv.unsubscribe("RaceWS", live)
	}()

	wg.Wait()

	// Invariant: registry is empty after all churners unsubscribed.
	srv.mu.Lock()
	_, stillSubscribed := srv.subs["RaceWS"]
	srv.mu.Unlock()
	if stillSubscribed {
		t.Fatal("subscriber registry not empty after all unsubscribes")
	}
}
