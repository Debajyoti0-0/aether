package web

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

// A minimal, push-only RFC 6455 server.
//
// Why hand-rolled: the dashboard's hard rule is zero runtime dependencies and a
// single binary. A WebSocket library would be the only new module in go.mod for
// a feature that is one handshake plus one frame writer. The surface actually
// needed is tiny and is enumerated below, which is what makes a hand-rolled
// implementation reviewable rather than reckless:
//
//   - handshake: Sec-WebSocket-Key + the RFC 6455 magic GUID, SHA-1, base64
//   - write: text frames only, always masked-off (a server never masks)
//   - read: only enough to honour close and ping, and to notice a client that
//     has gone away
//
// What is deliberately absent: fragmentation, extensions (permessage-deflate),
// binary frames, and any inbound application message. A client cannot send a
// command, so there is no command-injection or CSRF surface here. Inbound frames
// are counted only to detect and drop a peer that misbehaves, and the count is
// surfaced in the server log so an attempt is visible.

// wsGUID is the RFC 6455 handshake constant.
const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// wsMaxClientFrame bounds a single inbound frame. The dashboard never expects
// client data, so a small limit is correct and a large one would only widen the
// attack surface.
const wsMaxClientFrame = 1024

// Opcodes used by this implementation.
const (
	opContinuation = 0x0
	opText         = 0x1
	opBinary       = 0x2
	opClose        = 0x8
	opPing         = 0x9
	opPong         = 0xA
)

// wsConn is a minimal server-side WebSocket connection.
type wsConn struct {
	rw   net.Conn
	br   *bufio.Reader
	wmu  chan struct{} // serialises writes; a channel used as a mutex keeps the type trivially copy-safe
	dead bool
}

func newWSConn(rw net.Conn, br *bufio.Reader) *wsConn {
	return &wsConn{rw: rw, br: br, wmu: make(chan struct{}, 1)}
}

// upgrade performs the RFC 6455 handshake.
//
// It refuses anything that is not a well-formed GET upgrade carrying a
// Sec-WebSocket-Key, and it echoes back the client's Sec-WebSocket-Protocol only
// if the server supports it. No subprotocol is offered, because the dashboard
// needs none; an unrequested subprotocol is a protocol-confusion risk.
func upgradeWebSocket(w http.ResponseWriter, r *http.Request) (*wsConn, error) {
	if r.Method != http.MethodGet {
		return nil, fmt.Errorf("websocket upgrade requires GET")
	}
	if !headerContainsToken(r.Header, "Connection", "upgrade") {
		return nil, fmt.Errorf("missing Connection: Upgrade")
	}
	if !strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket") {
		return nil, fmt.Errorf("missing Upgrade: websocket")
	}
	if v := r.Header.Get("Sec-WebSocket-Version"); v != "13" {
		return nil, fmt.Errorf("unsupported Sec-WebSocket-Version %q, only 13 is implemented", v)
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, fmt.Errorf("missing Sec-WebSocket-Key")
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, fmt.Errorf("response writer does not support hijacking")
	}
	conn, brw, err := hj.Hijack()
	if err != nil {
		return nil, fmt.Errorf("hijack: %w", err)
	}
	sum := sha1.Sum([]byte(key + wsGUID))
	accept := base64.StdEncoding.EncodeToString(sum[:])
	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
	if _, err := conn.Write([]byte(resp)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("write handshake: %w", err)
	}
	_ = conn.SetDeadline(time.Time{}) // clear any deadline inherited from the request
	return newWSConn(conn, brw.Reader), nil
}

func headerContainsToken(h http.Header, name, token string) bool {
	for _, v := range h.Values(name) {
		for _, part := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(part), token) {
				return true
			}
		}
	}
	return false
}

// WriteText sends a complete, unfragmented text frame.
func (c *wsConn) WriteText(payload []byte) error {
	return c.writeFrame(opText, payload)
}

func (c *wsConn) writeFrame(opcode byte, payload []byte) error {
	c.wmu <- struct{}{}
	defer func() { <-c.wmu }()
	if c.dead {
		return net.ErrClosed
	}
	if len(payload) > 1<<20 {
		return fmt.Errorf("outbound frame of %d bytes exceeds the 1 MiB limit", len(payload))
	}
	// A server never masks, so the mask bit is clear and there is no key.
	var hdr [10]byte
	n := 2
	hdr[0] = 0x80 | opcode // FIN set
	switch {
	case len(payload) < 126:
		hdr[1] = byte(len(payload))
	case len(payload) <= 0xFFFF:
		hdr[1] = 126
		binary.BigEndian.PutUint16(hdr[2:4], uint16(len(payload)))
		n = 4
	default:
		hdr[1] = 127
		binary.BigEndian.PutUint64(hdr[2:10], uint64(len(payload)))
		n = 10
	}
	_ = c.rw.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if _, err := c.rw.Write(hdr[:n]); err != nil {
		c.dead = true
		return err
	}
	if len(payload) > 0 {
		if _, err := c.rw.Write(payload); err != nil {
			c.dead = true
			return err
		}
	}
	return nil
}

// Close sends a close frame with a status code and then closes the socket.
func (c *wsConn) Close(code uint16, reason string) {
	payload := make([]byte, 2, 2+len(reason))
	binary.BigEndian.PutUint16(payload, code)
	payload = append(payload, reason...)
	_ = c.writeFrame(opClose, payload)
	c.dead = true
	_ = c.rw.Close()
}

// readFrame reads one frame, returning the opcode and payload.
//
// It is used only for protocol housekeeping (close, ping) and to notice a
// disconnect. Application payloads are counted and discarded; nothing a client
// sends can change server state.
func (c *wsConn) readFrame() (opcode byte, payload []byte, err error) {
	var h [2]byte
	if _, err := c.br.Read(h[:]); err != nil {
		return 0, nil, err
	}
	opcode = h[0] & 0x0F
	masked := h[1]&0x80 != 0
	length := uint64(h[1] & 0x7F)
	switch length {
	case 126:
		var b [2]byte
		if _, err := c.br.Read(b[:]); err != nil {
			return 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(b[:]))
	case 127:
		var b [8]byte
		if _, err := c.br.Read(b[:]); err != nil {
			return 0, nil, err
		}
		length = binary.BigEndian.Uint64(b[:])
	}
	if length > wsMaxClientFrame {
		return opcode, nil, fmt.Errorf("client frame of %d bytes exceeds the %d byte limit", length, wsMaxClientFrame)
	}
	var mask [4]byte
	if masked {
		if _, err := c.br.Read(mask[:]); err != nil {
			return 0, nil, err
		}
	}
	payload = make([]byte, length)
	if length > 0 {
		if _, err := readFull(c.br, payload); err != nil {
			return opcode, nil, err
		}
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	if !utf8.Valid(payload) && opcode == opText {
		return opcode, nil, fmt.Errorf("text frame payload is not valid UTF-8")
	}
	return opcode, payload, nil
}

func readFull(r *bufio.Reader, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := r.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// handleWebSocket streams chain updates to the browser.
//
// Push-only. There is no read loop that interprets client messages, so no
// client can cause a state change, and there is no endpoint here that a
// cross-site page could usefully target.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgradeWebSocket(w, r)
	if err != nil {
		http.Error(w, "websocket upgrade failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	ch, unsubscribe := s.src.Subscribe()
	defer unsubscribe()

	// Announce the current head so a freshly opened dashboard is not blank
	// until the next append arrives.
	if head, ok := s.firstEntry(); ok {
		if b, err := jsonBytes(map[string]any{
			"type": "audit.head", "seq": head.Seq, "hash": head.Hash, "total": s.src.Status().EntryCount,
		}); err == nil {
			_ = conn.WriteText(b)
		}
	}

	ctx := r.Context()
	discarded := 0
	go func() {
		defer conn.Close(1000, "server closing")
		for {
			select {
			case <-ctx.Done():
				return
			case msg, open := <-ch:
				if !open {
					return
				}
				if err := conn.WriteText(msg); err != nil {
					return
				}
			}
		}
	}()

	// Read loop: honour close and ping, count and discard anything else. This
	// exists to detect a departed client promptly rather than to accept input.
	for {
		op, _, err := conn.readFrame()
		if err != nil {
			return
		}
		switch op {
		case opClose:
			return
		case opPing:
			_ = conn.writeFrame(opPong, nil)
		default:
			discarded++
			if discarded == 1 || discarded%1000 == 0 {
				// Surfaced so a client attempting to drive the server is visible
				// in the log rather than invisible.
				logf("aether dashboard: /ws discarded %d client frame(s); this endpoint is push-only", discarded)
			}
		}
	}
}

func (s *Server) firstEntry() (AuditEntry, bool) {
	e := s.src.Entries()
	if len(e) == 0 {
		return AuditEntry{}, false
	}
	return e[0], true
}
