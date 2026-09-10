package api

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

func jsonUnmarshal(data []byte, out any) error {
	return json.Unmarshal(data, out)
}

// TeamClient connects an operator to the teamserver over mTLS.
type TeamClient struct {
	conn   net.Conn
	reader *bufio.Reader
	mu     sync.Mutex
	Next   chan *Envelope
}

// Dial connects to the teamserver. serverCert is the pinned CA/server
// cert (nil pool = use system roots); clientCert authenticates the operator.
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
		conn:   conn,
		reader: bufio.NewReader(conn),
		Next:   make(chan *Envelope, 64),
	}
	go c.readLoop()
	return c, nil
}

func (c *TeamClient) readLoop() {
	defer close(c.Next)
	for {
		env, err := ReadFrame(c.reader)
		if err != nil {
			return
		}
		c.Next <- env
	}
}

// ExecuteCommand sends a command and awaits its response.
func (c *TeamClient) ExecuteCommand(req *CommandRequest) (*CommandResponse, error) {
	payload, err := EncodePayload(req)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	err = WriteFrame(c.conn, &Envelope{Type: MsgCommandRequest, Payload: payload})
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}

	deadline := time.After(60 * time.Second)
	for {
		select {
		case env, ok := <-c.Next:
			if !ok {
				return nil, fmt.Errorf("connection closed")
			}
			switch env.Type {
			case MsgCommandResponse:
				resp := &CommandResponse{}
				if err := decodePayload(env.Payload, resp); err != nil {
					return nil, err
				}
				if resp.Error != "" {
					return resp, fmt.Errorf("%s", resp.Error)
				}
				return resp, nil
			case MsgWorkspaceUpdate:
				continue // unrelated stream traffic
			case MsgError:
				resp := &CommandResponse{}
				_ = decodePayload(env.Payload, resp)
				return nil, fmt.Errorf("server error: %s", resp.Error)
			}
		case <-deadline:
			return nil, fmt.Errorf("timeout waiting for command response")
		}
	}
}

// StreamWorkspace subscribes to workspace updates and returns the channel.
func (c *TeamClient) StreamWorkspace(workspaceID string) (<-chan *WorkspaceUpdate, error) {
	payload, err := EncodePayload(&WorkspaceRequest{WorkspaceID: workspaceID})
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	err = WriteFrame(c.conn, &Envelope{Type: MsgWorkspaceSub, Payload: payload})
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}

	updates := make(chan *WorkspaceUpdate, 64)
	go func() {
		defer close(updates)
		for env := range c.Next {
			if env.Type == MsgWorkspaceUpdate {
				ev := &WorkspaceUpdate{}
				if decodePayload(env.Payload, ev) == nil {
					updates <- ev
				}
			}
		}
	}()
	return updates, nil
}

// Close terminates the connection.
func (c *TeamClient) Close() error {
	return c.conn.Close()
}
