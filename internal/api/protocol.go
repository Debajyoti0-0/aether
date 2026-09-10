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
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"time"
)

// Protocol: every frame is 4-byte big-endian length + JSON payload.
// Messages are the RPC contract of the Aether teamserver:
//
//   ExecuteCommand  (CommandRequest  -> CommandResponse)
//   StreamWorkspace (WorkspaceRequest -> stream WorkspaceUpdate)

// CommandRequest asks the teamserver to run an aether command in a workspace.
type CommandRequest struct {
	WorkspaceID string `json:"workspace_id"`
	CommandLine string `json:"command_line"` // full aether CLI syntax
	Operator    string `json:"operator,omitempty"`
}

// CommandResponse carries the result of an executed command.
type CommandResponse struct {
	OK      bool   `json:"ok"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
	TookMS  int64  `json:"took_ms"`
}

// WorkspaceRequest subscribes to workspace updates.
type WorkspaceRequest struct {
	WorkspaceID string `json:"workspace_id"`
}

// WorkspaceUpdate is one streamed change to a workspace.
type WorkspaceUpdate struct {
	WorkspaceID string `json:"workspace_id"`
	Kind        string `json:"kind"` // token_added, event, path_validated, ...
	Payload     string `json:"payload,omitempty"`
	Timestamp   int64  `json:"timestamp"`
}

// MessageType discriminates frames on the wire.
type MessageType string

const (
	MsgCommandRequest  MessageType = "command_request"
	MsgCommandResponse MessageType = "command_response"
	MsgWorkspaceSub    MessageType = "workspace_sub"
	MsgWorkspaceUpdate MessageType = "workspace_update"
	MsgError           MessageType = "error"
)

// Envelope is the outer frame: type + payload.
type Envelope struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
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

// ReadFrame reads one length-prefixed JSON envelope from r.
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
