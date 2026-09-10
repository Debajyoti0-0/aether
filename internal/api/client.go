package api

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

func jsonUnmarshalImpl(data []byte, out any) error {
	return json.Unmarshal(data, out)
}

// TeamClient connects an operator to the teamserver over mTLS.
// Protocol v2: requests are correlated by RequestID; a single
// connection carries multiplexed commands AND event subscriptions
// with no response cross-talk (the client's read loop dispatches by
// correlation ID, the server echoes it).
type TeamClient struct {
	conn   net.Conn
	reader *bufio.Reader

	writeMu sync.Mutex

	mu      sync.Mutex
	pending map[string]chan *Envelope // RequestID -> waiter
	subs    []*subStream              // event streams (matched by type)

	Next chan *Envelope // uncorrelated frames (tests/legacy consumers)

	nextID func() string
}

type subStream struct {
	workspace string
	ch        chan *WorkspaceUpdate
}

// Dial connects to the teamserver. clientCert is a CA-issued operator
// certificate (T1); serverCAs verifies the server identity (nil pool
// with insecureSkipVerify=true is the research-only escape hatch).
func Dial(addr string, clientCert tls.Certificate, serverCAs *x509.CertPool, insecureSkipVerify bool) (*TeamClient, error) {
	tlsConf := &tls.Config{
		Certificates:       []tls.Certificate{clientCert},
		RootCAs:            serverCAs,
		InsecureSkipVerify: insecureSkipVerify,
		MinVersion:         tls.VersionTLS13,
	}

	conn, err := tls.Dial("tcp", addr, tlsConf)
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", addr, err)
	}

	c := &TeamClient{
		conn:    conn,
		reader:  bufio.NewReader(conn),
		pending: map[string]chan *Envelope{},
		Next:    make(chan *Envelope, 64),
		nextID:  NewRequestID,
	}
	go c.readLoop()
	return c, nil
}

// readLoop is the single demultiplexer: every inbound frame is routed
// by its correlation identity — responses to their pending request
// waiter, events to matching subscriptions, everything else to Next.
func (c *TeamClient) readLoop() {
	defer func() {
		c.mu.Lock()
		for rid, ch := range c.pending {
			close(ch)
			delete(c.pending, rid)
		}
		c.mu.Unlock()
		for _, s := range c.subs {
			close(s.ch)
		}
		close(c.Next)
	}()
	for {
		env, err := ReadFrame(c.reader)
		if err != nil {
			return
		}
		c.dispatch(env)
	}
}

func (c *TeamClient) dispatch(env *Envelope) {
	switch env.Type {
	case MsgCommandResponse, MsgError:
		c.mu.Lock()
		ch, ok := c.pending[env.RequestID]
		if ok {
			delete(c.pending, env.RequestID)
		}
		c.mu.Unlock()
		if ok {
			ch <- env
			return
		}
	case MsgWorkspaceUpdate:
		var ev WorkspaceUpdate
		if decodePayload(env.Payload, &ev) == nil {
			c.mu.Lock()
			var matched bool
			for _, s := range c.subs {
				if s.workspace == ev.WorkspaceID {
					select {
					case s.ch <- &ev:
					default: // slow consumer: drop rather than block the read loop
					}
					matched = true
				}
			}
			c.mu.Unlock()
			if matched {
				return
			}
		}
	}
	// Uncorrelated / unknown frames: hand to Next (bounded; drops only
	// on a stalled legacy consumer).
	select {
	case c.Next <- env:
	default:
	}
}

// ExecuteCommand sends a command and awaits ITS response (correlated
// by RequestID — no ordering assumptions).
func (c *TeamClient) ExecuteCommand(req *CommandRequest) (*CommandResponse, error) {
	return c.ExecuteCommandCtx(context.Background(), req)
}

// ExecuteCommandCtx is the context-aware variant.
func (c *TeamClient) ExecuteCommandCtx(ctx context.Context, req *CommandRequest) (*CommandResponse, error) {
	rid := c.nextID()
	payload, err := EncodePayload(req)
	if err != nil {
		return nil, err
	}

	ch := make(chan *Envelope, 1)
	c.mu.Lock()
	c.pending[rid] = ch
	c.mu.Unlock()

	c.writeMu.Lock()
	err = WriteFrame(c.conn, &Envelope{Version: ProtocolVersion, Type: MsgCommandRequest, RequestID: rid, Payload: payload})
	c.writeMu.Unlock()
	if err != nil {
		return nil, err
	}

	select {
	case env, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("connection closed")
		}
		switch env.Type {
		case MsgError:
			resp := &CommandResponse{}
			_ = decodePayload(env.Payload, resp)
			return resp, fmt.Errorf("%s", resp.Error)
		default:
			resp := &CommandResponse{}
			if err := decodePayload(env.Payload, resp); err != nil {
				return nil, err
			}
			if resp.Error != "" {
				return resp, fmt.Errorf("%s", resp.Error)
			}
			return resp, nil
		}
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, rid)
		c.mu.Unlock()
		return nil, fmt.Errorf("cancelled waiting for response")
	case <-time.After(5 * time.Minute):
		c.mu.Lock()
		delete(c.pending, rid)
		c.mu.Unlock()
		return nil, fmt.Errorf("timeout waiting for command response")
	}
}

// StreamWorkspace subscribes to workspace events from cursor 0 (all
// retained history) and returns the ordered update channel.
func (c *TeamClient) StreamWorkspace(workspaceID string) (<-chan *WorkspaceUpdate, error) {
	return c.StreamWorkspaceFrom(workspaceID, 0)
}

// StreamWorkspaceFrom subscribes with an explicit cursor: the server
// replays every persisted event newer than lastSeq, then streams live
// events — the reconnect/resume path (T2).
func (c *TeamClient) StreamWorkspaceFrom(workspaceID string, lastSeq uint64) (<-chan *WorkspaceUpdate, error) {
	payload, err := EncodePayload(&WorkspaceRequest{WorkspaceID: workspaceID, Cursor: lastSeq})
	if err != nil {
		return nil, err
	}

	s := &subStream{workspace: workspaceID, ch: make(chan *WorkspaceUpdate, 128)}
	c.mu.Lock()
	c.subs = append(c.subs, s)
	c.mu.Unlock()

	c.writeMu.Lock()
	err = WriteFrame(c.conn, &Envelope{Version: ProtocolVersion, Type: MsgWorkspaceSub, Payload: payload})
	c.writeMu.Unlock()
	if err != nil {
		return nil, err
	}
	return s.ch, nil
}

// Ping performs a protocol-level liveness round trip.
func (c *TeamClient) Ping() error {
	rid := c.nextID()
	ch := make(chan *Envelope, 1)
	c.mu.Lock()
	c.pending[rid] = ch
	c.mu.Unlock()

	c.writeMu.Lock()
	err := WriteFrame(c.conn, &Envelope{Version: ProtocolVersion, Type: MsgPing, RequestID: rid})
	c.writeMu.Unlock()
	if err != nil {
		return err
	}
	select {
	case env, ok := <-ch:
		if !ok {
			return fmt.Errorf("connection closed")
		}
		if env.Type != MsgPong {
			return fmt.Errorf("unexpected ping response %q", env.Type)
		}
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("ping timeout")
	}
}

// Close terminates the connection.
func (c *TeamClient) Close() error {
	return c.conn.Close()
}
