package api

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// CommandRunner executes an aether command line. The CLI wires this to
// the in-process command registry; tests substitute fakes.
type CommandRunner func(req *CommandRequest) (*CommandResponse, error)

// subscriber is one live workspace stream. Lifecycle ownership:
// the teamserver (unsubscribe) is the ONLY component that closes
// subscriber.done; the event channel ch is never closed by anyone.
// Publish sends are guarded by done, so once unsubscribe returns, no
// future Publish can deliver to this subscriber (invariant: a channel
// is never sent on after its owner has torn it down, and no channel is
// ever closed, eliminating send-on-closed-channel panics).
type subscriber struct {
	ch   chan *WorkspaceUpdate
	done chan struct{}
}

// Teamserver accepts mTLS connections from operator clients.
type Teamserver struct {
	Listener net.Listener
	TLSConf  *tls.Config

	// Run executes incoming commands. Required.
	Run CommandRunner

	mu         sync.Mutex
	events     map[string][]*WorkspaceUpdate  // workspace -> updates
	subs       map[string][]*subscriber       // workspace -> live subscribers
	subByCh    map[chan *WorkspaceUpdate]*subscriber
	operators  map[string]time.Time           // conn remote -> last seen
	sessionKey []byte
}

// NewTeamserver builds a server with an mTLS listener bound to addr.
func NewTeamserver(addr string, cert tls.Certificate, runner CommandRunner) (*Teamserver, error) {
	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}

	ln, err := tls.Listen("tcp", addr, tlsConf)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}

	return &Teamserver{
		Listener:  ln,
		TLSConf:   tlsConf,
		Run:       runner,
		events:    map[string][]*WorkspaceUpdate{},
		subs:      map[string][]*subscriber{},
		subByCh:   map[chan *WorkspaceUpdate]*subscriber{},
		operators: map[string]time.Time{},
	}, nil
}

// Serve accepts connections until the listener closes.
func (s *Teamserver) Serve() error {
	for {
		conn, err := s.Listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				return nil
			}
			return err
		}
		go s.handleConn(conn)
	}
}

// Close stops the listener.
func (s *Teamserver) Close() error {
	return s.Listener.Close()
}

func (s *Teamserver) handleConn(conn net.Conn) {
	defer conn.Close()
	remote := conn.RemoteAddr().String()

	s.mu.Lock()
	s.operators[remote] = time.Now()
	s.mu.Unlock()

	reader := bufio.NewReader(conn)

	for {
		env, err := ReadFrame(reader)
		if err != nil {
			return
		}

		switch env.Type {
		case MsgCommandRequest:
			var req CommandRequest
			if err := decodePayload(env.Payload, &req); err != nil {
				s.writeError(conn, err)
				continue
			}

			resp, err := s.Run(&req)
			if err != nil && resp == nil {
				resp = &CommandResponse{Error: err.Error()}
			}
			if resp == nil {
				resp = &CommandResponse{}
			}

			payload, _ := EncodePayload(resp)
			WriteFrame(conn, &Envelope{Type: MsgCommandResponse, Payload: payload})

			// Record as a workspace event.
			s.Publish(&WorkspaceUpdate{
				WorkspaceID: req.WorkspaceID,
				Kind:        "command_executed",
				Payload:     req.CommandLine,
				Timestamp:   time.Now().Unix(),
			})

		case MsgWorkspaceSub:
			var req WorkspaceRequest
			if err := decodePayload(env.Payload, &req); err != nil {
				s.writeError(conn, err)
				continue
			}

			// Snapshot recent events under the lock, then replay
			// outside it (no head-of-line blocking for publishers).
			s.mu.Lock()
			snapshot := make([]*WorkspaceUpdate, len(s.events[req.WorkspaceID]))
			copy(snapshot, s.events[req.WorkspaceID])
			s.mu.Unlock()

			sub := s.subscribe(req.WorkspaceID)
			for _, ev := range snapshot {
				payload, _ := EncodePayload(ev)
				if err := WriteFrame(conn, &Envelope{Type: MsgWorkspaceUpdate, Payload: payload}); err != nil {
					s.unsubscribe(req.WorkspaceID, sub)
					return
				}
			}

			for {
				select {
				case ev, ok := <-sub.ch:
					if !ok {
						return
					}
					payload, _ := EncodePayload(ev)
					if err := WriteFrame(conn, &Envelope{Type: MsgWorkspaceUpdate, Payload: payload}); err != nil {
						s.unsubscribe(req.WorkspaceID, sub)
						return
					}
				case <-sub.done:
					// The subscriber was torn down by its owner; stop
					// streaming and close the connection.
					return
				}
			}

		default:
			s.writeError(conn, fmt.Errorf("unknown message type %q", env.Type))
		}
	}
}

func (s *Teamserver) writeError(conn net.Conn, err error) {
	payload, _ := EncodePayload(&CommandResponse{Error: err.Error()})
	_ = WriteFrame(conn, &Envelope{Type: MsgError, Payload: payload})
}

func decodePayload(raw []byte, out any) error {
	if len(raw) == 0 {
		return fmt.Errorf("empty payload")
	}
	return jsonUnmarshal(raw, out)
}

// Publish records and broadcasts a workspace update to ALL subscribers
// of that workspace (fan-out). Slow subscribers drop frames instead of
// blocking the server. Sends are guarded by the subscriber's done
// channel: after unsubscribe removes and closes it, a racing Publish
// can no longer deliver (the done branch wins or the send is dropped);
// the event channel itself is never closed, so a send on it can never
// panic.
func (s *Teamserver) Publish(ev *WorkspaceUpdate) {
	if ev.Timestamp == 0 {
		ev.Timestamp = time.Now().Unix()
	}

	s.mu.Lock()
	s.events[ev.WorkspaceID] = append(s.events[ev.WorkspaceID], ev)
	if len(s.events[ev.WorkspaceID]) > 100 {
		s.events[ev.WorkspaceID] = s.events[ev.WorkspaceID][1:]
	}
	subs := make([]*subscriber, len(s.subs[ev.WorkspaceID]))
	copy(subs, s.subs[ev.WorkspaceID])
	s.mu.Unlock()

	for _, sub := range subs {
		select {
		case sub.ch <- ev:
		case <-sub.done:
		default:
			// Slow subscriber: drop rather than block the server.
		}
	}
}

func (s *Teamserver) subscribe(workspace string) *subscriber {
	sub := &subscriber{
		ch:   make(chan *WorkspaceUpdate, 16),
		done: make(chan struct{}),
	}

	s.mu.Lock()
	s.subs[workspace] = append(s.subs[workspace], sub)
	s.subByCh[sub.ch] = sub
	s.mu.Unlock()
	return sub
}

// unsubscribe is the single authoritative owner of subscriber teardown.
// It removes the subscriber from the registry and closes sub.done under
// the server lock (so concurrent unsubscribes cannot double-close); it
// never closes sub.ch.
func (s *Teamserver) unsubscribe(workspace string, sub *subscriber) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if list, ok := s.subs[workspace]; ok {
		for i, cur := range list {
			if cur == sub {
				s.subs[workspace] = append(list[:i], list[i+1:]...)
				break
			}
		}
		if len(s.subs[workspace]) == 0 {
			delete(s.subs, workspace)
		}
	}
	delete(s.subByCh, sub.ch)
	select {
	case <-sub.done:
		// already torn down
	default:
		close(sub.done)
	}
}
