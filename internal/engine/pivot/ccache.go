package pivot

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"time"
)

// Ccache principal name components for a TGT.
const keyVersionUnknown = 0xFFFFFFFF

// ccacheEntry is one credential record written to the ccache.
type ccacheEntry struct {
	Realm       string
	Principal   []string
	ClientRealm string
	ClientParts []string
	TicketDER   []byte
	SessionKey  []byte
	KeyType     int
	StartTime   time.Time
	EndTime     time.Time
}

// WriteCcache serializes credentials in the MIT Kerberos FILE format
// (version 0x0504) suitable for KRB5CCNAME.
// Layout: header (12 bytes) + principals + credential records.
// Strings are u32-length prefixed; times are u32 big-endian unix.
func WriteCcache(path string, info *ASREPInfo, clientPrincipal string) error {
	if info == nil || info.TicketDER == nil {
		return fmt.Errorf("no ticket to write")
	}

	var buf bytesBuffer

	// Header: AB 12-bit magic "0504", version, tag len.
	buf.Write([]byte{0x05, 0x04})
	buf.Write(u32be(0)) // version 0 (classic file)
	buf.Write(u32be(0)) // no header tags

	// Primary principal: REALM + components.
	writePrincipal(&buf, info.CRealm, principalParts(clientPrincipal))

	// Credential record.
	start := uint32(time.Now().Unix())
	end := start + 36000 // 10h validity
	writeU32(&buf, start)
	writeU32(&buf, start)             // authtime
	writeU32(&buf, start)             // starttime
	writeU32(&buf, end)
	writeU8(&buf, 0)                  // renew_till not set flag byte (simplified)
	writeU8(&buf, 0)                  // is_skey

	// Ticket flags (forwardable|renewable|preauthenticated).
	writeU32(&buf, 0x40000000|0x00800000|0x00200000)

	// keyblock: keytype, etype, keylen, key. The session key is not
	// extractable from a cloud AS-REP; emit a placeholder keyblock so
	// klist/pkinit tooling can still read the ticket.
	writeU16(&buf, 18) // enctype aes256
	writeU16(&buf, 18)
	writeU16(&buf, 32)
	buf.Write(make([]byte, 32))

	// Server principal = krbtgt/<REALM>@<REALM>.
	writePrincipal(&buf, info.CRealm, []string{"krbtgt", info.CRealm})

	// Ticket DER (u32 length prefixed).
	writeU32(&buf, uint32(len(info.TicketDER)))
	buf.Write(info.TicketDER)

	// Second ticket: none.
	writeU32(&buf, 0)

	return os.WriteFile(path, buf.Bytes(), 0o600)
}

func writePrincipal(buf *bytesBuffer, realm string, parts []string) {
	writeU32(buf, uint32(len(realm)))
	buf.WriteString(realm)
	writeU32(buf, uint32(len(parts)))
	for _, p := range parts {
		writeU32(buf, uint32(len(p)))
		buf.WriteString(p)
	}
}

func principalParts(principal string) []string {
	if principal == "" {
		return []string{"unknown"}
	}
	// strip realm suffix if present
	if i := strings.Index(principal, "@"); i >= 0 {
		principal = principal[:i]
	}
	return strings.Split(principal, "/")
}

func u32be(v uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return b
}

func writeU32(buf *bytesBuffer, v uint32) { buf.Write(u32be(v)) }

func writeU16(buf *bytesBuffer, v uint16) {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, v)
	buf.Write(b)
}

func writeU8(buf *bytesBuffer, v byte) { buf.Write([]byte{v}) }

// bytesBuffer is a minimal indirection over bytes.Buffer.
type bytesBuffer struct {
	data []byte
}

func (b *bytesBuffer) Write(p []byte) { b.data = append(b.data, p...) }
func (b *bytesBuffer) WriteString(s string) { b.data = append(b.data, s...) }
func (b *bytesBuffer) Bytes() []byte { return b.data }
