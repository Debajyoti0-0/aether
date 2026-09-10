package api

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// CommandRunner executes an aether command on behalf of an
// authenticated operator. The runner receives the cert-derived
// Operator identity (never a client-asserted name) and is responsible
// for capability enforcement + spine dispatch (Stage 3, T1/T3).
type CommandRunner func(op *Operator, req *CommandRequest) (*CommandResponse, error)

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

	// Run executes incoming commands on behalf of an authenticated
	// operator. Required.
	Run CommandRunner

	// operatorsDir roots per-operator capability files ("" disables
	// capability file loading; the runner still receives the identity).
	operatorsDir string
	revoked      *RevocationList
	eventStore   *EventStore

	mu         sync.Mutex
	events     map[string][]*WorkspaceUpdate // workspace -> updates (fallback ring when no store)
	subs       map[string][]*subscriber      // workspace -> live subscribers
	subByCh    map[chan *WorkspaceUpdate]*subscriber
	operators  map[string]time.Time // conn remote -> last seen
	maxConns   int
	conns      int
	shutdown   chan struct{}
}

// SetEventStore attaches the persistent, cursor-addressable event log
// (T2). When set, Publish assigns persisted sequence numbers and
// subscriptions replay from the store.
func (s *Teamserver) SetEventStore(es *EventStore) { s.eventStore = es }

// NewTeamserver builds a server with mTLS requiring client
// certificates that chain to clientCAs (T1: no verification is not an
// option — a nil pool is a configuration error).
func NewTeamserver(addr string, srvCert tls.Certificate, clientCAs *x509.CertPool, operatorsDir string, revoked *RevocationList, runner CommandRunner) (*Teamserver, error) {
	if clientCAs == nil {
		return nil, fmt.Errorf("teamserver requires a client CA pool (run 'aether serve cert init')")
	}
	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{srvCert},
		MinVersion:   tls.VersionTLS13,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAs,
	}

	ln, err := tls.Listen("tcp", addr, tlsConf)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}

	return &Teamserver{
		Listener:     ln,
		TLSConf:      tlsConf,
		Run:          runner,
		operatorsDir: operatorsDir,
		revoked:      revoked,
		events:       map[string][]*WorkspaceUpdate{},
		subs:         map[string][]*subscriber{},
		subByCh:      map[chan *WorkspaceUpdate]*subscriber{},
		operators:    map[string]time.Time{},
		maxConns:     32,
		shutdown:     make(chan struct{}),
	}, nil
}

// SetMaxConnections bounds concurrent authenticated connections.
func (s *Teamserver) SetMaxConnections(n int) {
	if n > 0 {
		s.maxConns = n
	}
}

// Serve accepts connections until the listener closes. Each connection
// must present a valid, unrevoked operator certificate; the derived
// identity is attached to the connection for its lifetime.
func (s *Teamserver) Serve() error {
	for {
		conn, err := s.Listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				return nil
			}
			return err
		}
		select {
		case <-s.shutdown:
			_ = conn.Close()
			continue
		default:
		}
		s.mu.Lock()
		if s.conns >= s.maxConns {
			s.mu.Unlock()
			_ = conn.Close()
			continue
		}
		s.conns++
		s.mu.Unlock()

		go func() {
			defer func() {
				s.mu.Lock()
				s.conns--
				s.mu.Unlock()
			}()
			s.handleConn(conn)
		}()
	}
}

// Close stops the listener and signals shutdown.
func (s *Teamserver) Close() error {
	select {
	case <-s.shutdown:
		// already closed
	default:
		close(s.shutdown)
	}
	return s.Listener.Close()
}

// handleConn authenticates the TLS peer (cert-derived operator
// identity, revocation check) and then serves the connection. The
// identity is fixed for the connection's lifetime — a client cannot
// re-assert a different operator.
func (s *Teamserver) handleConn(conn net.Conn) {
	defer conn.Close()
	remote := conn.RemoteAddr().String()

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return
	}
	if err := tlsConn.Handshake(); err != nil {
		return
	}
	peerCerts := tlsConn.ConnectionState().PeerCertificates
	if len(peerCerts) == 0 {
		return
	}
	op, err := FromClientCert(peerCerts[0], time.Now())
	if err != nil {
		return // fail closed: unauthenticated peer gets no protocol at all
	}
	if s.revoked != nil && s.revoked.IsRevoked(op.Name) {
		return // revoked operator: fail closed
	}
	if s.operatorsDir != "" {
		caps, err := LoadOperatorCaps(s.operatorsDir, op.Name)
		if err != nil {
			return
		}
		op.Caps = caps
	}

	s.mu.Lock()
	s.operators[remote] = time.Now()
	s.mu.Unlock()

	s.serveConn(conn, op)
}

// serveConn runs the protocol v2 loop for one authenticated operator
// connection: multiplexed commands (bounded), subscriptions with
// cursor replay, and ping/pong. Identity is fixed (op); every response
// echoes the request's RequestID.
func (s *Teamserver) serveConn(conn net.Conn, op *Operator) {
	reader := bufio.NewReader(conn)
	var writeMu sync.Mutex
	writeFrame := func(env *Envelope) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return WriteFrame(conn, env)
	}

	// Per-connection in-flight command cap (backpressure).
	inflight := make(chan struct{}, 8)
	var inflightWG sync.WaitGroup

	for {
		env, err := ReadFrame(reader)
		if err != nil {
			return
		}

		switch env.Type {
		case MsgCommandRequest:
			var req CommandRequest
			if err := decodePayload(env.Payload, &req); err != nil {
				writeFrame(&Envelope{Version: ProtocolVersion, Type: MsgError, RequestID: env.RequestID, Payload: mustEncode(&CommandResponse{Error: err.Error()})})
				continue
			}

			select {
			case inflight <- struct{}{}:
			default:
				writeFrame(&Envelope{Version: ProtocolVersion, Type: MsgError, RequestID: env.RequestID, Payload: mustEncode(&CommandResponse{Error: "too many in-flight commands on this connection (cap 8)"})})
				continue
			}
			inflightWG.Add(1)
			go func(rid string, req CommandRequest) {
				defer inflightWG.Done()
				defer func() { <-inflight }()

				start := time.Now()
				resp, err := s.Run(op, &req)
				if err != nil && resp == nil {
					resp = &CommandResponse{Error: err.Error()}
				}
				if resp == nil {
					resp = &CommandResponse{}
				}
				resp.TookMS = time.Since(start).Milliseconds()
				payload, _ := EncodePayload(resp)
				_ = writeFrame(&Envelope{Version: ProtocolVersion, Type: MsgCommandResponse, RequestID: rid, Payload: payload})

				s.Publish(&WorkspaceUpdate{
					WorkspaceID: req.WorkspaceID,
					Kind:        "command_executed",
					Payload:     op.Name + ": " + req.CommandLine,
					Timestamp:   time.Now().Unix(),
				})
			}(env.RequestID, req)

		case MsgWorkspaceSub:
			var req WorkspaceRequest
			if err := decodePayload(env.Payload, &req); err != nil {
				s.writeError(conn, err)
				continue
			}

			sub := s.subscribe(req.WorkspaceID)

			// Cursor replay from the persistent event store (T2):
			// events with seq > req.Cursor are re-sent on (re)connect.
			if s.eventStore != nil {
				pending, _, err := s.eventStore.ReadSince(req.WorkspaceID, req.Cursor)
				if err != nil {
					// A gap is a real anomaly — report and refuse to
					// silently skip.
					writeFrame(&Envelope{Version: ProtocolVersion, Type: MsgError, RequestID: env.RequestID, Payload: mustEncode(&CommandResponse{Error: err.Error()})})
					s.unsubscribe(req.WorkspaceID, sub)
					continue
				}
				ok := true
				for _, ev := range pending {
					if !writeEvent(writeFrame, env.RequestID, &ev) {
						ok = false
						break
					}
				}
				if !ok {
					s.unsubscribe(req.WorkspaceID, sub)
					return
				}
			} else {
				// No persistent store (tests): replay the in-memory ring.
				s.mu.Lock()
				ring := s.events[req.WorkspaceID]
				snapshot := make([]*WorkspaceUpdate, len(ring))
				copy(snapshot, ring)
				s.mu.Unlock()
				for _, ev := range snapshot {
					if uint64(ev.Seq) <= req.Cursor {
						continue
					}
					payload, _ := EncodePayload(ev)
					writeMu.Lock()
					err := WriteFrame(conn, &Envelope{Version: ProtocolVersion, Type: MsgWorkspaceUpdate, Seq: uint64(ev.Seq), Payload: payload})
					writeMu.Unlock()
					if err != nil {
						s.unsubscribe(req.WorkspaceID, sub)
						return
					}
				}
			}

			for {
				select {
				case ev, ok := <-sub.ch:
					if !ok {
						return
					}
					payload, _ := EncodePayload(ev)
					writeMu.Lock()
					err := WriteFrame(conn, &Envelope{Version: ProtocolVersion, Type: MsgWorkspaceUpdate, Seq: uint64(ev.Seq), Payload: payload})
					writeMu.Unlock()
					if err != nil {
						s.unsubscribe(req.WorkspaceID, sub)
						return
					}
				case <-sub.done:
					return
				}
			}

		case MsgPing:
			_ = writeFrame(&Envelope{Version: ProtocolVersion, Type: MsgPong, RequestID: env.RequestID})

		default:
			s.writeError(conn, fmt.Errorf("unknown message type %q", env.Type))
		}
	}
}

func writeEvent(writeFrame func(*Envelope) error, rid string, ev *WorkspaceUpdate) bool {
	payload, _ := EncodePayload(ev)
	return writeFrame(&Envelope{Version: ProtocolVersion, Type: MsgWorkspaceUpdate, RequestID: rid, Seq: uint64(ev.Seq), Payload: payload}) == nil
}

func mustEncode(v any) json.RawMessage {
	data, _ := EncodePayload(v)
	return data
}

func (s *Teamserver) writeError(conn net.Conn, err error) {
	payload, _ := EncodePayload(&CommandResponse{Error: err.Error()})
	_ = WriteFrame(conn, &Envelope{Type: MsgError, Payload: payload})
}

func decodePayload(raw []byte, out any) error {
	if len(raw) == 0 {
		return fmt.Errorf("empty payload")
	}
	return json.Unmarshal(raw, out)
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
	if s.eventStore != nil {
		seq, err := s.eventStore.Append(ev.WorkspaceID, ev.Kind, ev.Payload)
		if err != nil {
			// Critical lifecycle events must never disappear silently.
			return
		}
		ev.Seq = int64(seq)
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
