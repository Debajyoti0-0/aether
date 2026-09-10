package pivot

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/asn1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"
)

// Kerberos constants (RFC 4120) for the subset we need.
const (
	krb5PVNO     = 5
	ASREQTag     = 10
	ASREPTag     = 11
	EnctypeAES256 = 18
	EnctypeAES128 = 17
	NameTypePrincipal = 1
)

// Azure AD Kerberos padata type carried in the AS-REQ/AS-REP.
const pADATAAzureAD = 130

// tagGeneralString is the ASN.1 GeneralString tag number (0x1B).
const tagGeneralString = 27

// KKDCPClient speaks MS-KKDCP (Kerberos Key Distribution Center Proxy
// Protocol) over HTTPS — the transport Azure AD exposes for hybrid
// Kerberos. Raw Kerberos messages are wrapped as
// 4-byte-big-endian length prefix + DER payload (MS-KKDCP 2.2.1).
type KKDCPClient struct {
	Endpoint   string // e.g. https://login.microsoftonline.com/common/Kerberos/api
	Domain     string // realm name, e.g. INTERNAL.LOCAL
	HTTPClient *http.Client
}

// NewKKDCPClient builds a KKDCP client.
func NewKKDCPClient(endpoint, domain string, hc *http.Client) *KKDCPClient {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &KKDCPClient{Endpoint: endpoint, Domain: domain, HTTPClient: hc}
}

// Send posts a raw Kerberos message and returns the raw reply body.
func (c *KKDCPClient) Send(ctx context.Context, krbMsg []byte) ([]byte, error) {
	if c.Endpoint == "" {
		return nil, fmt.Errorf("kkdcp endpoint is required")
	}

	wrapped := wrapKDCProxyMessage(krbMsg)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(wrapped))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/kerberos")
	req.Header.Set("Accept", "application/kerberos")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kkdcp request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kkdcp http %d: %s", resp.StatusCode, truncateBytes(body, 256))
	}
	return unwrapKDCProxyMessage(body)
}

func wrapKDCProxyMessage(msg []byte) []byte {
	out := make([]byte, 4+len(msg))
	binary.BigEndian.PutUint32(out[:4], uint32(len(msg)))
	copy(out[4:], msg)
	return out
}

func unwrapKDCProxyMessage(body []byte) ([]byte, error) {
	if len(body) > 4 {
		n := binary.BigEndian.Uint32(body[:4])
		if int(n) == len(body)-4 {
			return body[4:], nil
		}
	}
	return body, nil
}

// ---------------------------------------------------------------------------
// Minimal AS-REQ construction (RFC 4120 §5.4.1) with Azure AD Kerberos
// PA-DATA carrying the cloud token as pre-authentication material.
// ---------------------------------------------------------------------------

// ASREQOptions configures the TGT request.
type ASREQOptions struct {
	Realm      string // e.g. INTERNAL.LOCAL
	ClientName string // user principal (without realm)
	SName      string // default krbtgt/<Realm>
	Token      string // OAuth/PRT-bound token used as pre-auth blob
}

// contextTag wraps DER bytes in a context-specific constructed tag
// and returns the full DER encoding.
func contextTag(tag int, content []byte) []byte {
	header := []byte{0xA0 | byte(tag)} // constructed context tag
	l := len(content)
	switch {
	case l < 128:
		header = append(header, byte(l))
	case l < 256:
		header = append(header, 0x81, byte(l))
	default:
		header = append(header, 0x82, byte(l>>8), byte(l))
	}
	return append(header, content...)
}

// generalString emits a GeneralString primitive (DER).
func generalString(s string) []byte {
	return primitive(tagGeneralString, []byte(s))
}

// primitive emits a universal primitive tag with content.
func primitive(tag int, content []byte) []byte {
	header := []byte{byte(tag)}
	l := len(content)
	switch {
	case l < 128:
		header = append(header, byte(l))
	case l < 256:
		header = append(header, 0x81, byte(l))
	default:
		header = append(header, 0x82, byte(l>>8), byte(l))
	}
	return append(header, content...)
}

// generalizedTime emits a GeneralizedTime primitive (DER), tag 24.
func generalizedTime(t time.Time) []byte {
	return primitive(24, []byte(t.UTC().Format("20060102150405Z")))
}

// marshalBitString emits a BIT STRING (DER) with 32 unused-bit-free bytes.
func marshalBitString(bits []byte) []byte {
	content := append([]byte{0x00}, bits...) // 0 unused bits
	return primitive(3, content)
}

// padataItem is one PA-DATA (padata-type, padata-value) pair.
type padataItem struct {
	Type  int
	Value []byte
}

// marshalPADATA emits a SEQUENCE OF PA-DATA.
func marshalPADATA(items []padataItem) ([]byte, error) {
	var out []byte
	for _, it := range items {
		intEl := contextTag(1, mustMarshalInt(it.Type))
		valEl := contextTag(2, primitive(asn1.TagOctetString, it.Value))
		out = append(out, primitive(asn1.TagSequence, concat(intEl, valEl))...)
	}
	return primitive(asn1.TagSequence, out), nil
}

func mustMarshalInt(v int) []byte {
	der, _ := asn1.Marshal(v)
	return der
}

func mustMarshalRaw(rv asn1.RawValue) []byte {
	der, _ := asn1.Marshal(rv)
	return der
}

// BuildASREQ assembles a DER-encoded AS-REQ for a cloud TGT.
func BuildASREQ(opts ASREQOptions) ([]byte, error) {
	if opts.Realm == "" {
		return nil, fmt.Errorf("realm is required")
	}
	if opts.ClientName == "" {
		return nil, fmt.Errorf("client name is required")
	}
	sname := opts.SName
	if sname == "" {
		sname = "krbtgt/" + opts.Realm
	}

	// cname: [1] PrincipalName
	cnameInner := append(
		contextTag(0, mustMarshalInt(NameTypePrincipal)),
		contextTag(1, func() []byte {
			var names []byte
			for _, n := range []string{opts.ClientName} {
				names = append(names, generalString(n)...)
			}
			return names
		}())...,
	)
	cname := contextTag(1, cnameInner)

	// sname: [3] PrincipalName (krbtgt/REALM)
	snameParts := stringsSplit2(sname)
	var snameNames []byte
	for _, n := range snameParts {
		snameNames = append(snameNames, generalString(n)...)
	}
	snameEl := contextTag(3, append(
		contextTag(0, mustMarshalInt(NameTypePrincipal)),
		contextTag(1, snameNames)...,
	))

	// kdc-options: [0] BIT STRING (forwardable|renewable|canonicalize)
	kdcOptions := contextTag(0, marshalBitString(kdcOptionsBytes()))

	// realm: [2] GeneralString
	realmEl := contextTag(2, generalString(opts.Realm))

	// till: [6] GeneralizedTime
	till := contextTag(6, generalizedTime(time.Now().Add(10*time.Hour)))

	// ctime: [7] GeneralizedTime (KerberosTime)
	ctime := contextTag(7, generalizedTime(time.Now()))

	// cusec: [8] INTEGER
	cusec := contextTag(8, mustMarshalInt(time.Now().Nanosecond()/1000))

	// nonce: [9] INTEGER
	nonce := contextTag(9, mustMarshalInt(randomNonce()))

	// etypes: [10] SEQUENCE OF INTEGER
	var etypesContent []byte
	for _, et := range []int{EnctypeAES256, EnctypeAES128} {
		etypesContent = append(etypesContent, mustMarshalInt(et)...)
	}
	etypes := contextTag(10, primitive(asn1.TagSequence, etypesContent))

	body := concat(kdcOptions, cname, realmEl, snameEl, till, ctime, cusec, nonce, etypes)
	bodySeq := contextTag(2, primitive(asn1.TagSequence, body))

	// padata: [1] SEQUENCE OF PA-DATA (Azure AD Kerberos token pre-auth)
	var paEl []byte
	if opts.Token != "" {
		paSeq, err := marshalPADATA([]padataItem{{Type: pADATAAzureAD, Value: []byte(opts.Token)}})
		if err != nil {
			return nil, err
		}
		paEl = contextTag(1, paSeq)
	}

	appContent := concat(bodySeq, paEl)
	header := []byte{0x60 | byte(ASREQTag)} // APPLICATION-10, constructed
	l := len(appContent)
	switch {
	case l < 128:
		header = append(header, byte(l))
	case l < 256:
		header = append(header, 0x81, byte(l))
	default:
		header = append(header, 0x82, byte(l>>8), byte(l))
	}
	return append(header, appContent...), nil
}

func kdcOptionsBytes() []byte {
	bits := make([]byte, 4)
	setBit(bits, 1)  // forwardable
	setBit(bits, 8)  // renewable
	setBit(bits, 15) // canonicalize
	return bits
}

func setBit(b []byte, n int) {
	b[n/8] |= 1 << (7 - uint(n)%8)
}

func randomNonce() int {
	n, err := rand.Int(rand.Reader, big.NewInt(0x7fffffff))
	if err != nil {
		return int(time.Now().UnixNano() & 0x7fffffff)
	}
	return int(n.Int64())
}

func stringsSplit2(s string) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

func concat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// ASREPInfo carries the parsed essentials of an AS-REP.
type ASREPInfo struct {
	CRealm    string
	CName     []byte // raw DER of the cname sequence
	TicketDER []byte // full DER of the ticket (for the ccache)
	EncPart   []byte // raw DER of the encrypted part
}

// ParseASREP extracts the ticket and metadata from a DER AS-REP.
func ParseASREP(der []byte) (*ASREPInfo, error) {
	info := &ASREPInfo{}

	// The AS-REP is an APPLICATION-11 sequence; walk its children by
	// scanning context-tagged elements.
	rest := der
	// Skip the outer APPLICATION wrapper.
	var outer asn1.RawValue
	if _, err := asn1.Unmarshal(der, &outer); err != nil {
		return nil, fmt.Errorf("unmarshal as-rep: %w", err)
	}
	if outer.Tag != ASREPTag {
		return nil, fmt.Errorf("not an AS-REP (tag %d)", outer.Tag)
	}
	rest = outer.Bytes

	for len(rest) > 0 {
		var el asn1.RawValue
		next, err := asn1.Unmarshal(rest, &el)
		if err != nil {
			break
		}
		if el.Class == asn1.ClassContextSpecific {
			switch el.Tag {
			case 3: // crealm
				var realm asn1.RawValue
				if _, err := asn1.Unmarshal(el.Bytes, &realm); err == nil {
					info.CRealm = string(realm.Bytes)
				}
			case 4: // cname
				info.CName = el.Bytes
			case 5: // ticket
				full, err := fullDER(el.Bytes)
				if err == nil {
					info.TicketDER = full
				}
			case 6: // enc-part
				info.EncPart = el.Bytes
			}
		}
		rest = next
	}

	if info.TicketDER == nil {
		return nil, fmt.Errorf("as-rep contains no ticket")
	}
	if info.CRealm == "" {
		info.CRealm = "UNKNOWN.REALM"
	}
	return info, nil
}

// fullDER returns the complete DER encoding of the first element in b.
func fullDER(b []byte) ([]byte, error) {
	var raw asn1.RawValue
	rest, err := asn1.Unmarshal(b, &raw)
	if err != nil {
		return nil, err
	}
	return b[:len(b)-len(rest)], nil
}

// HexDump returns a short hex preview for logs.
func HexDump(b []byte) string {
	if len(b) > 32 {
		return hex.EncodeToString(b[:32]) + "..."
	}
	return hex.EncodeToString(b)
}

// truncateBytes shortens a byte slice for error messages.
func truncateBytes(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
