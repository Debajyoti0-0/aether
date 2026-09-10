package api

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"time"
)

// Protocol v2 (Stage 3, T2): every frame is 4-byte big-endian length +
// JSON payload. Every envelope carries Version and RequestID so responses
// are always correlatable to their request — shared response channels
// and "first response belongs to the caller" assumptions are gone.
//
// Message types: command | response | subscribe | event | ping | pong | error

// ProtocolVersion is the only protocol version this build speaks.
// Envelopes with any other version are rejected (no silent downgrade).
const ProtocolVersion = 2

// CommandRequest asks the teamserver to run an aether command in a workspace.
// NOTE (Stage 3, T1): the client-asserted `operator` field was REMOVED —
// the operator identity is derived exclusively from the mTLS client
// certificate and attached server-side.
type CommandRequest struct {
	WorkspaceID string `json:"workspace_id"`
	CommandLine string `json:"command_line"` // full aether CLI syntax
}

// CommandResponse carries the governed spine outcome of a command.
type CommandResponse struct {
	ActionID    string `json:"aid,omitempty"`
	Status      string `json:"status,omitempty"` // completed | failed | aborted | completed_state_unknown
	Stage       string `json:"stage,omitempty"`  // last successful spine stage
	OperationID string `json:"opid,omitempty"`
	Output      string `json:"output,omitempty"`
	Error       string `json:"error,omitempty"`
	TookMS      int64  `json:"took_ms"`
}

// WorkspaceRequest subscribes to workspace updates. Cursor is the
// last event sequence the client has seen (0 = everything retained);
// the server replays everything newer from the persistent event store.
type WorkspaceRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Cursor      uint64 `json:"cursor,omitempty"`
}

// WorkspaceUpdate is one streamed change to a workspace. Seq carries
// the workspace's monotonic event sequence (gaps are detectable).
type WorkspaceUpdate struct {
	WorkspaceID string `json:"workspace_id"`
	Kind        string `json:"kind"` // token_added, event, path_validated, ...
	Payload     string `json:"payload,omitempty"`
	Timestamp   int64  `json:"timestamp"`
	Seq         int64  `json:"seq,omitempty"`
}

// MessageType discriminates frames on the wire.
type MessageType string

const (
	MsgCommandRequest  MessageType = "command"
	MsgCommandResponse MessageType = "response"
	MsgWorkspaceSub    MessageType = "subscribe"
	MsgWorkspaceUpdate MessageType = "event"
	MsgPing            MessageType = "ping"
	MsgPong            MessageType = "pong"
	MsgError           MessageType = "error"
)

// Envelope is the outer frame: protocol version, correlation ID,
// message type, optional per-workspace event sequence, payload.
type Envelope struct {
	Version   int             `json:"v"`
	Type      MessageType     `json:"type"`
	RequestID string          `json:"rid,omitempty"` // client-issued; server echoes verbatim
	Seq       uint64          `json:"seq,omitempty"` // monotonic per workspace (event envelopes)
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// Validate rejects protocol violations fail-closed: unknown versions
// are refused (no downgrade), and correlation-bearing messages must
// carry a RequestID.
func (e *Envelope) Validate() error {
	if e.Version != ProtocolVersion {
		return fmt.Errorf("protocol version %d unsupported (want %d)", e.Version, ProtocolVersion)
	}
	switch e.Type {
	case MsgCommandRequest, MsgCommandResponse:
		if e.RequestID == "" {
			return fmt.Errorf("%s envelope requires a request id", e.Type)
		}
	}
	return nil
}

// NewRequestID mints a client-side correlation identifier.
func NewRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("rid-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// WriteFrame writes a length-prefixed JSON envelope to w.
func WriteFrame(w io.Writer, env *Envelope) error {
	data, err := json.Marshal(env)
	if err != nil {
		return err
	}
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(data)))
	if _, err := w.Write(lenBuf[:]); err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// ReadFrame reads one length-prefixed JSON envelope from r and
// validates the protocol header (version, correlation).
func ReadFrame(r *bufio.Reader) (*Envelope, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lenBuf[:])
	if length > 8<<20 {
		return nil, fmt.Errorf("frame too large (%d bytes)", length)
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}

	env := &Envelope{}
	if err := json.Unmarshal(data, env); err != nil {
		return nil, fmt.Errorf("decode envelope: %w", err)
	}
	if err := env.Validate(); err != nil {
		return nil, err
	}
	return env, nil
}

// EncodePayload marshals v into an envelope payload.
func EncodePayload(v any) (json.RawMessage, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

// GenerateClientCert creates a self-signed client certificate for an
// operator (mTLS client auth).
func GenerateClientCert(cn string) (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}

	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, nil
}

// GenerateServerCert creates a self-signed TLS certificate for the
// teamserver (operators should replace with CA-signed certs in prod).
func GenerateServerCert(hosts []string) (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}

	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "aether-teamserver"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}

	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, err
	}

	cert := tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  key,
	}
	_ = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	_ = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return cert, nil
}
