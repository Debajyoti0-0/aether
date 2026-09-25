package kerberos

// wire.go — minimal DER TLV codec for the RFC 4120 Kerberos ASN.1 module.
//
// The RFC 4120 module is DEFINITIONS EXPLICIT TAGS, so every context tag on
// the wire wraps a complete TLV (e.g. realm[2] GeneralString encodes as
// a2 LL 1b LL <bytes>). Go's encoding/asn1 cannot express GeneralString or
// forced-GeneralizedTime from struct tags, and it silently ignores custom
// MarshalASN1() methods — hence this codec.

import (
	"encoding/binary"
	"encoding/asn1"
	"errors"
	"fmt"
	"time"
)

const (
	derClassUniversal   = 0x00
	derClassApplication = 0x40
	derClassContext     = 0x80

	tagInteger         = 0x02
	tagBitString       = 0x03
	tagOctetString     = 0x04
	tagUTCTime         = 0x17
	tagGeneralizedTime = 0x18
	tagGeneralString   = 0x1b
	tagSequence        = 0x30
)

var (
	// derWireErrors are distinct from errors.go's protocol-level errors.
	ErrDERTruncated = errors.New("truncated DER packet")
	ErrDERLength    = errors.New("invalid DER length")
	ErrDERTag       = errors.New("unexpected DER tag")
)

// ---------------------------------------------------------------------------
// Encoding
// ---------------------------------------------------------------------------

// tlvNode is one DER TLV element: either constructed (children) or primitive (value).
type tlvNode struct {
	class       byte
	tag         byte
	constructed bool
	children    []*tlvNode
	value       []byte
}

func (n *tlvNode) add(c *tlvNode) *tlvNode {
	n.children = append(n.children, c)
	return n
}

func (n *tlvNode) encode() []byte {
	var content []byte
	if n.constructed {
		for _, c := range n.children {
			content = append(content, c.encode()...)
		}
	} else {
		content = n.value
	}
	tagByte := n.class | n.tag
	if n.constructed {
		tagByte |= 0x20
	}
	out := derHeader(tagByte, len(content))
	return append(out, content...)
}

func derHeader(tag byte, length int) []byte {
	if length < 0x80 {
		return []byte{tag, byte(length)}
	}
	var lenBytes []byte
	for v := length; v > 0; v >>= 8 {
		lenBytes = append([]byte{byte(v)}, lenBytes...)
	}
	out := []byte{tag, byte(0x80 | len(lenBytes))}
	return append(out, lenBytes...)
}

func derSeq(children ...*tlvNode) *tlvNode {
	return &tlvNode{class: derClassUniversal, tag: tagSequence, constructed: true, children: children}
}

// derApp builds an APPLICATION-tagged constructed node (e.g. AS-REQ = [APPLICATION 10]).
func derApp(tag byte, children ...*tlvNode) *tlvNode {
	return &tlvNode{class: derClassApplication, tag: tag, constructed: true, children: children}
}

// derCtx builds an EXPLICIT context-specific wrapper around a complete TLV.
func derCtx(tag byte, children ...*tlvNode) *tlvNode {
	return &tlvNode{class: derClassContext, tag: tag, constructed: true, children: children}
}

func derInt(v int64) *tlvNode {
	b, err := asn1.Marshal(v)
	if err != nil {
		b = []byte{tagInteger, 0x01, 0x00}
	}
	raw, _, perr := parseTLV(b)
	if perr != nil {
		return &tlvNode{class: derClassUniversal, tag: tagInteger, value: []byte{0}}
	}
	return &tlvNode{class: derClassUniversal, tag: tagInteger, value: raw.value}
}

func derOctets(b []byte) *tlvNode {
	return &tlvNode{class: derClassUniversal, tag: tagOctetString, value: b}
}

// derBitString encodes a BIT STRING with a multiple-of-8 bit length (unused bits = 0).
// Trailing zero octets are TRIMMED (DER minimality): a KDC-REQ-BODY must
// re-encode byte-identically inside Samba/Heimdal's KDC — it verifies the
// TGS-REQ authenticator checksum over its own re-encoded body, and Heimdal's
// BIT STRING encoder trims. A padded "03 05 00 60 00 00" KDCOptions made that
// verification fail with KDC_ERR_BADOPTION (12).
func derBitString(b []byte) *tlvNode {
	end := len(b)
	for end > 1 && b[end-1] == 0 {
		end--
	}
	value := make([]byte, 0, end+1)
	value = append(value, 0) // unused-bits byte
	value = append(value, b[:end]...)
	return &tlvNode{class: derClassUniversal, tag: tagBitString, value: value}
}

// derGeneralString encodes KerberosString ::= GeneralString (universal tag 27, 0x1b).
func derGeneralString(s string) *tlvNode {
	return &tlvNode{class: derClassUniversal, tag: tagGeneralString, value: []byte(s)}
}

// derGeneralizedTime encodes KerberosTime: always GeneralizedTime, seconds precision, 'Z'.
func derGeneralizedTime(t time.Time) *tlvNode {
	return &tlvNode{class: derClassUniversal, tag: tagGeneralizedTime,
		value: []byte(t.UTC().Format("20060102150405Z"))}
}

// kdcOptionsBytes maps RFC 4120 KDCOptions bit numbers onto wire bytes.
// Bit 0 is the most significant bit of the first octet (FORWARDABLE = 0x80).
// kdcOptionsBytes encodes KDCOptions (RFC 4120 §5.4.1) as the 4 content
// octets of a 32-bit BIT STRING.
//
// The BIT STRING content is the option mask in BIG-ENDIAN wire order: bit 0
// (reserved) is the MSB of the first octet and bit 31 is the LSB of the
// fourth. So the content octets are simply the big-endian bytes of the
// uint32 mask — e.g. FORWARDABLE (0x40000000) is 0x40 of the FIRST octet.
//
// The previous implementation instead treated the mask as an LSB-first bit
// INDEX and wrote data[i/8] |= 0x80>>i%8, which transposed every bit into the
// wrong octet: FORWARDABLE (1<<30) landed in the fourth octet as 0x02 and
// RENEWABLE (1<<23) in the third as 0x01. Samba decodes that as undefined
// option bits and answers KDC_ERR_BADOPTION (13).
func kdcOptionsBytes(flags uint32) []byte {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, flags)
	return data
}

// EncodeKDCOptions returns the KDCOptions BIT STRING wire bytes for flags.
func EncodeKDCOptions(flags uint32) []byte {
	return kdcOptionsBytes(flags)
}

// principalNameTLV encodes PrincipalName ::= SEQUENCE {
//
//	name-type[0] Int32, name-string[1] SEQUENCE OF KerberosString }
//
// NameString holds the components only; the realm travels in its own field.
func principalNameTLV(p PrincipalName) *tlvNode {
	var comps []*tlvNode
	for _, c := range p.NameString {
		comps = append(comps, derGeneralString(c))
	}
	return derSeq(
		derCtx(0, derInt(int64(p.NameType))),
		derCtx(1, derSeq(comps...)),
	)
}

func etypeSeqTLV(etypes []int32) *tlvNode {
	var nodes []*tlvNode
	for _, e := range etypes {
		nodes = append(nodes, derInt(int64(e)))
	}
	return derCtx(8, derSeq(nodes...))
}

// ---------------------------------------------------------------------------
// Parsing
// ---------------------------------------------------------------------------

type tlvRaw struct {
	class       byte
	tag         byte
	constructed bool
	value       []byte // content
	full        []byte // complete TLV bytes
}

func parseTLV(b []byte) (tlvRaw, []byte, error) {
	if len(b) < 2 {
		return tlvRaw{}, nil, ErrDERTruncated
	}
	tag := b[0]
	if tag&0x1f == 0x1f {
		return tlvRaw{}, nil, ErrDERTag // multi-byte tags not used by RFC 4120
	}
	l := int(b[1])
	hdr := 2
	if l&0x80 != 0 {
		n := l & 0x7f
		if n == 0 || n > 4 || len(b) < 2+n {
			return tlvRaw{}, nil, ErrDERLength
		}
		l = 0
		for i := 0; i < n; i++ {
			l = l<<8 | int(b[2+i])
		}
		hdr = 2 + n
		if l < 0x80 {
			return tlvRaw{}, nil, ErrDERLength // non-minimal length encoding
		}
	}
	if len(b) < hdr+l {
		return tlvRaw{}, nil, ErrDERTruncated
	}
	return tlvRaw{
		class:       tag & 0xc0,
		tag:         tag & 0x1f,
		constructed: tag&0x20 != 0,
		value:       b[hdr : hdr+l],
		full:        b[:hdr+l],
	}, b[hdr+l:], nil
}

func parseChildren(content []byte) ([]tlvRaw, error) {
	var out []tlvRaw
	rest := content
	for len(rest) > 0 {
		n, r, err := parseTLV(rest)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
		rest = r
	}
	return out, nil
}

func findCtx(children []tlvRaw, tag byte) *tlvRaw {
	for i := range children {
		if children[i].class == derClassContext && children[i].tag == tag {
			return &children[i]
		}
	}
	return nil
}

// unwrapCtx strips EXPLICIT context-tag wrappers: RFC 4120's module is
// EXPLICIT TAGS, so every field arrives as e.g. "a1 03 02 01 05" ([1] wrapping
// the INTEGER). Decoders must look at the inner universal TLV.
func unwrapCtx(r tlvRaw) tlvRaw {
	for r.class == derClassContext {
		inner, _, err := parseTLV(r.value)
		if err != nil {
			return r
		}
		r = inner
	}
	return r
}

// descendToSequence unwraps context/application tag layers until a universal
// SEQUENCE is reached (e.g. ticket[5] → [APPLICATION 1] → SEQUENCE).
func descendToSequence(r tlvRaw) (tlvRaw, error) {
	for {
		if r.class == derClassUniversal && r.tag == (tagSequence&0x1f) {
			return r, nil
		}
		if !r.constructed {
			return r, fmt.Errorf("expected SEQUENCE, got class %#x tag %d", r.class, r.tag)
		}
		inner, _, err := parseTLV(r.value)
		if err != nil {
			return r, err
		}
		r = inner
	}
}

func decodeInt(r tlvRaw) (int64, error) {
	r = unwrapCtx(r)
	if r.tag != tagInteger || r.constructed || len(r.value) == 0 {
		return 0, ErrDERTag
	}
	var v int64
	negative := r.value[0]&0x80 != 0
	for _, b := range r.value {
		v = v<<8 | int64(b)
	}
	if negative {
		v -= int64(1) << uint(len(r.value)*8)
	}
	return v, nil
}

// decodeKerberosString accepts GeneralString (RFC-mandated) plus the
// PrintableString/IA5String/UTF8String encodings observed from lenient peers.
func decodeKerberosString(r tlvRaw) (string, error) {
	r = unwrapCtx(r)
	if r.constructed {
		return "", ErrDERTag
	}
	switch r.tag {
	case tagGeneralString, 0x16, 0x13, 0x0c:
		return string(r.value), nil
	}
	return "", fmt.Errorf("expected string tag 27/22/19/12, got %d", r.tag)
}

func decodeTime(r tlvRaw) (time.Time, error) {
	r = unwrapCtx(r)
	if r.constructed {
		return time.Time{}, ErrDERTag
	}
	s := string(r.value)
	switch r.tag {
	case tagGeneralizedTime:
		return time.Parse("20060102150405Z", s)
	case tagUTCTime:
		return time.Parse("060102150405Z", s)
	}
	return time.Time{}, fmt.Errorf("expected time tag 24/23, got %d", r.tag)
}

