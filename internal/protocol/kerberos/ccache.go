package kerberos

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

var (
	ErrInvalidCCache       = errors.New("invalid ccache format")
	ErrCCacheVersion       = errors.New("unsupported ccache version")
	ErrCCacheEntryNotFound = errors.New("ccache entry not found")
	ErrCorruptCCache       = errors.New("corrupt ccache")
)

const (
	// CCACHE_VERSION_MIT is the MIT krb5 / Heimdal v4 file format.
	// The previous code set the Heimdal constant to 0x0504 as well, so the
	// version guard accepted a value it could not actually decode.
	CCACHE_VERSION_MIT     = 0x0504
	CCACHE_VERSION_HEIMDAL = 0x0503
)

// ccacheMaxField bounds every length prefix read from the file so a corrupt or
// hostile ccache cannot drive an unbounded allocation. A Kerberos ticket is
// at most a few KiB; authdata and tickets are the largest fields.
const ccacheMaxField = 1 << 20 // 1 MiB

// ccacheMaxComponents / ccacheMaxEntries bound the two count prefixes.
const (
	ccacheMaxComponents = 1024
	ccacheMaxEntries    = 4096
)

// CCacheHeader is the v4 file header. Data holds the raw header field block so
// a parsed cache re-encodes byte-for-byte; MIT only defines one field today
// (tag 1, the KDC/client clock offset) and readers must ignore unknown tags,
// so the bytes are preserved rather than interpreted.
type CCacheHeader struct {
	Version   uint16
	HeaderLen uint16
	Data      []byte
}

type CCacheEntry struct {
	ClientPrincipal PrincipalName
	ClientRealm     Realm
	ServerPrincipal PrincipalName
	ServerRealm     Realm
	Key             EncryptionKey
	AuthTime        time.Time
	StartTime       time.Time
	EndTime         time.Time
	RenewTill       time.Time
	IsSKey          bool
	TicketFlags     int32
	Addresses       []HostAddress
	AuthData        []AuthorizationData
	Ticket          []byte
	SecondTicket    []byte
}

type CCache struct {
	Header           CCacheHeader
	Entries          []CCacheEntry
	DefaultPrincipal *PrincipalName
	// DefaultRealm is the realm of DefaultPrincipal, which the file format
	// stores as the last component of the default principal. The previous
	// model had no place for it, so every parse lost the realm and
	// ConvertMITToHeimdal could not round-trip a real file.
	DefaultRealm Realm
}

func NewCCache() *CCache {
	return &CCache{
		Header: CCacheHeader{
			Version:   CCACHE_VERSION_MIT,
			HeaderLen: 0,
		},
		Entries: []CCacheEntry{},
	}
}

func (c *CCache) AddEntry(entry CCacheEntry) {
	c.Entries = append(c.Entries, entry)
	if c.DefaultPrincipal == nil {
		cp := entry.ClientPrincipal
		c.DefaultPrincipal = &cp
		c.DefaultRealm = entry.ClientRealm
	}
}

func (c *CCache) GetEntry(clientPrincipal PrincipalName, clientRealm Realm, serverPrincipal PrincipalName, serverRealm Realm) *CCacheEntry {
	for i := range c.Entries {
		e := &c.Entries[i]
		if e.ClientPrincipal.Equal(clientPrincipal) && e.ClientRealm == clientRealm &&
			e.ServerPrincipal.Equal(serverPrincipal) && e.ServerRealm == serverRealm {
			return e
		}
	}
	return nil
}

// GetDefaultEntry returns the cache's default credential. Configuration
// entries share the default principal as their client, so a real kinit cache
// stores them FIRST; returning the first match handed callers a configuration
// record whose "ticket" is cache metadata rather than a Kerberos ticket.
func (c *CCache) GetDefaultEntry() *CCacheEntry {
	if c.DefaultPrincipal == nil || len(c.Entries) == 0 {
		return nil
	}
	for i := range c.Entries {
		e := &c.Entries[i]
		if e.IsConfigEntry() {
			continue
		}
		if e.ClientPrincipal.Equal(*c.DefaultPrincipal) {
			return e
		}
	}
	return nil
}

func (c *CCache) RemoveEntry(clientPrincipal PrincipalName, clientRealm Realm, serverPrincipal PrincipalName, serverRealm Realm) bool {
	for i := range c.Entries {
		e := &c.Entries[i]
		if e.ClientPrincipal.Equal(clientPrincipal) && e.ClientRealm == clientRealm &&
			e.ServerPrincipal.Equal(serverPrincipal) && e.ServerRealm == serverRealm {
			c.Entries = append(c.Entries[:i], c.Entries[i+1:]...)
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// MIT ccache v4 file format (MIT krb5 doc/formats/ccache_file_format.rst)
//
//	file:          uint16 version (0x0504),
//	               uint16 header_length, header[header_length],
//	               default principal, credentials... until EOF
//	header field:  uint16 tag, uint16 length, value[length]
//	principal:     uint32 name_type, uint32 num_components,
//	               data realm, data component[num_components]
//	keyblock:      uint16 enctype, data key
//	credential:    client princ, server princ, keyblock, authtime, starttime,
//	               endtime, renew_till, is_skey, ticket_flags, addresses,
//	               authdata, ticket, second_ticket
//
// Three properties of the real format that the previous implementation got
// wrong, and which together made every real MIT cache unreadable:
//
//  1. The principal carries a leading uint32 name_type before the component
//     count, and the REALM is the first data field, not the last component.
//     The old code read a bare component count and treated the final string
//     as the realm, desynchronising the stream by four bytes per principal.
//  2. The keyblock is uint16 enctype followed by a *counted* key, so the key
//     length is a uint32. Reading it as a second uint16 desynchronised every
//     credential by two bytes.
//  3. There is no credential count and no separate configuration section:
//     the credential sequence simply runs to end of file. MIT encodes cache
//     configuration entries as ordinary credentials whose server realm is
//     "X-CACHECONF:", so they must be filtered out rather than parsed as
//     tickets.
// ---------------------------------------------------------------------------

// configEntryRealm is the realm MIT reserves for credential-cache
// configuration entries. Such entries decode cleanly as credentials, so a
// parser that ignores this would hand callers a configuration record where
// they expect a ticket.
const configEntryRealm = Realm("X-CACHECONF:")

// IsConfigEntry reports whether the entry is a credential-cache configuration
// record rather than a real Kerberos credential. MIT documents that programs
// displaying a cache should not surface these, and that the ticket field is
// not a valid ticket encoding.
func (e CCacheEntry) IsConfigEntry() bool {
	return e.ServerRealm == configEntryRealm
}

// cursor is a bounds-checked big-endian reader. Every read is validated so a
// truncated or hostile ccache yields a controlled error instead of a panic or
// a multi-gigabyte allocation.
type cursor struct {
	r   io.Reader
	buf []byte
	pos int
}

func newCursor(r io.Reader) (*cursor, error) {
	// Slurp the file once: ccache files are small and this makes bounds
	// checking trivial.
	b, err := io.ReadAll(io.LimitReader(r, 8<<20))
	if err != nil {
		return nil, ErrCorruptCCache
	}
	return &cursor{r: r, buf: b}, nil
}

func (c *cursor) remaining() int { return len(c.buf) - c.pos }

func (c *cursor) u8() (uint8, error) {
	if c.remaining() < 1 {
		return 0, ErrCorruptCCache
	}
	v := c.buf[c.pos]
	c.pos++
	return v, nil
}

func (c *cursor) u16() (uint16, error) {
	if c.remaining() < 2 {
		return 0, ErrCorruptCCache
	}
	v := binary.BigEndian.Uint16(c.buf[c.pos : c.pos+2])
	c.pos += 2
	return v, nil
}

func (c *cursor) u32() (uint32, error) {
	if c.remaining() < 4 {
		return 0, ErrCorruptCCache
	}
	v := binary.BigEndian.Uint32(c.buf[c.pos : c.pos+4])
	c.pos += 4
	return v, nil
}

// blob reads a uint32-length-prefixed byte string with an upper bound.
func (c *cursor) blob(limit uint32) ([]byte, error) {
	n, err := c.u32()
	if err != nil {
		return nil, err
	}
	if n > limit {
		return nil, ErrCorruptCCache
	}
	if int(n) > c.remaining() {
		return nil, ErrCorruptCCache
	}
	out := make([]byte, n)
	copy(out, c.buf[c.pos:c.pos+int(n)])
	c.pos += int(n)
	return out, nil
}

// readPrincipal reads a ccache principal:
//
//	uint32 name_type
//	uint32 num_components      (name components only; the realm is separate)
//	data realm
//	data component[num_components]
//
// The realm is a field of its own, not the trailing component, and name_type
// precedes the count. The previous implementation read a bare component count
// and treated the last string as the realm, which consumed four bytes too few
// per principal and desynchronised every following field.
func (c *cursor) readPrincipal() (PrincipalName, Realm, error) {
	var pn PrincipalName

	nameType, err := c.u32()
	if err != nil {
		return pn, "", err
	}
	n, err := c.u32()
	if err != nil {
		return pn, "", err
	}
	if n > ccacheMaxComponents {
		return pn, "", ErrCorruptCCache
	}
	realm, err := c.blob(ccacheMaxField)
	if err != nil {
		return pn, "", err
	}
	comps := make([]string, 0, n)
	for i := uint32(0); i < n; i++ {
		s, err := c.blob(ccacheMaxField)
		if err != nil {
			return pn, "", err
		}
		comps = append(comps, string(s))
	}
	pn.NameType = int32(nameType)
	pn.NameString = comps
	return pn, Realm(realm), nil
}

func ReadCCache(filename string) (*CCache, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return ReadCCacheFromReader(f)
}

func ReadCCacheFromReader(r io.Reader) (*CCache, error) {
	c, err := newCursor(r)
	if err != nil {
		return nil, err
	}

	version, err := c.u16()
	if err != nil {
		return nil, ErrInvalidCCache
	}
	if version != CCACHE_VERSION_MIT && version != CCACHE_VERSION_HEIMDAL {
		return nil, fmt.Errorf("%w: 0x%04x", ErrCCacheVersion, version)
	}
	if version == CCACHE_VERSION_HEIMDAL {
		// Heimdal v4 uses a different credential layout (datasize-prefixed
		// fields). Reporting it explicitly is correct; the previous code
		// claimed Heimdal support while decoding MIT bytes.
		return nil, fmt.Errorf("%w: heimdal 0x%04x", ErrCCacheVersion, version)
	}

	headerLen, err := c.u16()
	if err != nil {
		return nil, ErrCorruptCCache
	}
	if int(headerLen) > c.remaining() {
		return nil, ErrCorruptCCache
	}
	// The header is a sequence of (uint16 tag, uint16 len, value) fields whose
	// only defined member today is the KDC clock offset. Readers must ignore
	// unknown tags, so the raw bytes are preserved for re-encoding and skipped
	// for interpretation. The length covers the fields themselves, not the
	// two bytes that hold it.
	headerData := make([]byte, int(headerLen))
	copy(headerData, c.buf[c.pos:c.pos+int(headerLen)])
	c.pos += int(headerLen)

	ccache := NewCCache()
	ccache.Header = CCacheHeader{Version: version, HeaderLen: headerLen, Data: headerData}

	// Default principal.
	dp, dRealm, err := c.readPrincipal()
	if err != nil {
		return nil, err
	}
	ccache.DefaultPrincipal = &dp
	ccache.DefaultRealm = dRealm

	// Credentials until EOF.
	for c.remaining() > 0 {
		if len(ccache.Entries) >= ccacheMaxEntries {
			return nil, ErrCorruptCCache
		}
		e, err := readCredential(c)
		if err != nil {
			return nil, err
		}
		ccache.Entries = append(ccache.Entries, e)
	}

	return ccache, nil
}

func readCredential(c *cursor) (CCacheEntry, error) {
	var e CCacheEntry

	var err error
	if e.ClientPrincipal, e.ClientRealm, err = c.readPrincipal(); err != nil {
		return e, err
	}
	if e.ServerPrincipal, e.ServerRealm, err = c.readPrincipal(); err != nil {
		return e, err
	}

	// keyblock: uint16 enctype followed by a counted key, so the key length is
	// a uint32. The previous code read a second uint16, stealing two bytes of
	// the following authtime field and corrupting every credential.
	enctype, err := c.u16()
	if err != nil {
		return e, err
	}
	key, err := c.blob(ccacheMaxField)
	if err != nil {
		return e, err
	}
	e.Key = EncryptionKey{KeyType: int32(enctype), KeyValue: key}

	// times: authtime, starttime, endtime, renew_till.
	times := make([]time.Time, 0, 4)
	for i := 0; i < 4; i++ {
		v, err := c.u32()
		if err != nil {
			return e, err
		}
		times = append(times, time.Unix(int64(v), 0).UTC())
	}
	e.AuthTime, e.StartTime, e.EndTime, e.RenewTill = times[0], times[1], times[2], times[3]

	if e.IsSKey, err = c.u8AsBool(); err != nil {
		return e, err
	}

	flags, err := c.u32()
	if err != nil {
		return e, err
	}
	e.TicketFlags = int32(flags)

	// addresses: uint32 count, then (uint16 type, uint32 len, bytes)*.
	addrCount, err := c.u32()
	if err != nil {
		return e, err
	}
	if addrCount > ccacheMaxComponents {
		return e, ErrCorruptCCache
	}
	for i := uint32(0); i < addrCount; i++ {
		at, err := c.u16()
		if err != nil {
			return e, err
		}
		a, err := c.blob(ccacheMaxField)
		if err != nil {
			return e, err
		}
		e.Addresses = append(e.Addresses, HostAddress{AddrType: int32(at), Address: a})
	}

	// authdata: uint32 count, then (uint16 type, uint32 len, bytes)*.
	adCount, err := c.u32()
	if err != nil {
		return e, err
	}
	if adCount > ccacheMaxComponents {
		return e, err
	}
	for i := uint32(0); i < adCount; i++ {
		at, err := c.u16()
		if err != nil {
			return e, err
		}
		a, err := c.blob(ccacheMaxField)
		if err != nil {
			return e, err
		}
		e.AuthData = append(e.AuthData, AuthorizationData{AdType: int32(at), AdData: a})
	}

	if e.Ticket, err = c.blob(ccacheMaxField); err != nil {
		return e, err
	}
	if e.SecondTicket, err = c.blob(ccacheMaxField); err != nil {
		return e, err
	}

	return e, nil
}

func (c *cursor) u8AsBool() (bool, error) {
	v, err := c.u8()
	if err != nil {
		return false, err
	}
	return v != 0, nil
}

func ReadCCacheFromBytes(data []byte) (*CCache, error) {
	return ReadCCacheFromReader(bytes.NewReader(data))
}

func (c *CCache) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := c.WriteToWriter(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *CCache) WriteToFile(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	return c.WriteToWriter(f)
}

// WriteToWriter emits a real MIT v4 ccache so that reference tools
// (klist, kvno, Samba) can read Aether's output. The previous writer emitted
// the invented TLV stream, so every round-trip test passed against Aether's
// own encoder while producing files no other implementation could open.
func (c *CCache) WriteToWriter(w io.Writer) error {
	var buf bytes.Buffer

	version := c.Header.Version
	if version == 0 {
		version = CCACHE_VERSION_MIT
	}
	if version != CCACHE_VERSION_MIT {
		return fmt.Errorf("%w: cannot write 0x%04x", ErrCCacheVersion, version)
	}
	writeU16(&buf, version)

	// header_length covers the header fields themselves; the fields are
	// emitted verbatim so a parsed cache round-trips byte-for-byte.
	writeU16(&buf, uint16(len(c.Header.Data)))
	buf.Write(c.Header.Data)

	// default principal (components include the realm)
	if c.DefaultPrincipal != nil {
		if err := writePrincipal(&buf, *c.DefaultPrincipal, c.DefaultRealm); err != nil {
			return err
		}
	} else if len(c.Entries) > 0 {
		e := c.Entries[0]
		if err := writePrincipal(&buf, e.ClientPrincipal, e.ClientRealm); err != nil {
			return err
		}
	} else {
		// A ccache with no credentials still needs a default principal.
		if err := writePrincipal(&buf, PrincipalName{NameType: NAME_TYPE_PRINCIPAL}, ""); err != nil {
			return err
		}
	}

	for _, e := range c.Entries {
		if err := writeCredential(&buf, e); err != nil {
			return err
		}
	}

	_, err := w.Write(buf.Bytes())
	return err
}

func writePrincipal(buf *bytes.Buffer, pn PrincipalName, realm Realm) error {
	// uint32 name_type, uint32 num_components, data realm, data component[].
	// A principal built in memory may never have had a name type assigned;
	// emit KRB5_NT_PRINCIPAL rather than a zero type MIT would report as an
	// unknown principal form.
	nameType := pn.NameType
	if nameType == NAME_TYPE_UNKNOWN {
		nameType = NAME_TYPE_PRINCIPAL
	}
	writeU32(buf, uint32(nameType))
	writeU32(buf, uint32(len(pn.NameString)))
	writeBlob(buf, []byte(realm))
	for _, s := range pn.NameString {
		writeBlob(buf, []byte(s))
	}
	return nil
}

func writeCredential(buf *bytes.Buffer, e CCacheEntry) error {
	if err := writePrincipal(buf, e.ClientPrincipal, e.ClientRealm); err != nil {
		return err
	}
	if err := writePrincipal(buf, e.ServerPrincipal, e.ServerRealm); err != nil {
		return err
	}

	// keyblock: uint16 enctype followed by a counted key.
	writeU16(buf, uint16(e.Key.KeyType))
	writeBlob(buf, e.Key.KeyValue)

	writeU32(buf, uint32(e.AuthTime.Unix()))
	writeU32(buf, uint32(e.StartTime.Unix()))
	writeU32(buf, uint32(e.EndTime.Unix()))
	writeU32(buf, uint32(e.RenewTill.Unix()))

	if e.IsSKey {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}

	writeU32(buf, uint32(e.TicketFlags))

	writeU32(buf, uint32(len(e.Addresses)))
	for _, a := range e.Addresses {
		writeU16(buf, uint16(a.AddrType))
		writeBlob(buf, a.Address)
	}

	writeU32(buf, uint32(len(e.AuthData)))
	for _, a := range e.AuthData {
		writeU16(buf, uint16(a.AdType))
		writeBlob(buf, a.AdData)
	}

	writeBlob(buf, e.Ticket)
	writeBlob(buf, e.SecondTicket)
	return nil
}

func writeU16(buf *bytes.Buffer, v uint16) {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], v)
	buf.Write(b[:])
}

func writeU32(buf *bytes.Buffer, v uint32) {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)
	buf.Write(b[:])
}

func writeBlob(buf *bytes.Buffer, b []byte) {
	writeU32(buf, uint32(len(b)))
	buf.Write(b)
}

// ConvertMITToHeimdal reads a MIT v4 ccache and writes a MIT v4 ccache.
// Heimdal's on-disk credential layout differs (datasize-prefixed fields), so
// this is a format-preserving re-encode, not a Heimdal translation; the CLI
// label is retained for compatibility and the behaviour is now honest about
// what it produces.
func ConvertMITToHeimdal(mitPath, heimdalPath string) error {
	ccache, err := ReadCCache(mitPath)
	if err != nil {
		return err
	}
	return ccache.WriteToFile(heimdalPath)
}

func ConvertHeimdalToMIT(heimdalPath, mitPath string) error {
	ccache, err := ReadCCache(heimdalPath)
	if err != nil {
		return err
	}
	return ccache.WriteToFile(mitPath)
}
