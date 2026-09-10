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

// Teamserver accepts mTLS connections from operator clients.
type Teamserver struct {
	Listener net.Listener
	TLSConf  *tls.Config

	// Run executes incoming commands. Required.
	Run CommandRunner

	mu         sync.Mutex
	events     map[string][]*WorkspaceUpdate // workspace -> updates
	subs       map[string]chan *WorkspaceUpdate
	operators  map[string]time.Time // conn remote -> last seen
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
		subs:      map[string]chan *WorkspaceUpdate{},
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

			// Replay recent events, then stream live ones.
			ch := s.subscribe(req.WorkspaceID)
			s.mu.Lock()
			for _, ev := range s.events[req.WorkspaceID] {
				payload, _ := EncodePayload(ev)
				WriteFrame(conn, &Envelope{Type: MsgWorkspaceUpdate, Payload: payload})
			}
			s.mu.Unlock()

			for ev := range ch {
				payload, _ := EncodePayload(ev)
				if err := WriteFrame(conn, &Envelope{Type: MsgWorkspaceUpdate, Payload: payload}); err != nil {
					s.unsubscribe(req.WorkspaceID, ch)
					return
				}
			}
			return

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

// Publish records and broadcasts a workspace update.
func (s *Teamserver) Publish(ev *WorkspaceUpdate) {
	if ev.Timestamp == 0 {
		ev.Timestamp = time.Now().Unix()
	}

	s.mu.Lock()
	s.events[ev.WorkspaceID] = append(s.events[ev.WorkspaceID], ev)
	if len(s.events[ev.WorkspaceID]) > 100 {
		s.events[ev.WorkspaceID] = s.events[ev.WorkspaceID][1:]
	}
	s.mu.Unlock()

	s.mu.Lock()
	if ch, ok := s.subs[ev.WorkspaceID]; ok {
		select {
		case ch <- ev:
		default:
		}
	}
	s.mu.Unlock()
}

func (s *Teamserver) subscribe(workspace string) chan *WorkspaceUpdate {
	ch := make(chan *WorkspaceUpdate, 16)
	s.mu.Lock()
	s.subs[workspace] = ch
	s.mu.Unlock()
	return ch
}

func (s *Teamserver) unsubscribe(workspace string, ch chan *WorkspaceUpdate) {
	s.mu.Lock()
	if cur, ok := s.subs[workspace]; ok && cur == ch {
		delete(s.subs, workspace)
	}
	s.mu.Unlock()
	close(ch)
}
