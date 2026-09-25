package ldap

import (
	"bytes"
	"encoding/asn1"
	"testing"
)

// Stage 46g regression: wire evidence showed Samba AD dropping the connection
// (0 bytes + EOF) on Aether's SearchRequest. The captured bytes were:
//
//	30 38 02 01 02 63 33 04 00 02 01 02 02 01 00 02 01 01 02 01 1e
//	01 01 00 30 19 02 01 03 bf 81 23 12 30 10 13 0b objectClass 04 01 2a
//	30 05 30 03 0c 01 2a
//
// Defects: (1) encodeFilter returned an empty RawValue, so the Filter marshaled
// as SEQUENCE{INTEGER, ANY} instead of a context-specific choice;
// (2) "(objectClass=*)" parsed as equality-to-literal-"*" instead of a presence
// filter; (3) attributeSelection was double-wrapped. These tests pin the fix.

func TestSearchRequestEncoding_FormerConnectionDrop(t *testing.T) {
	filter := ParseFilter("(objectClass=*)")
	if filter == nil {
		t.Fatal("filter should parse")
	}
	if filter.FilterType != LDAP_FILTER_PRESENT {
		t.Fatalf("\"(objectClass=*)\" must be a presence filter, got type %d", filter.FilterType)
	}
	// Presence filter must encode as primitive context-specific tag 7 (0x87).
	rv, err := encodeFilter(*filter)
	if err != nil {
		t.Fatalf("encodeFilter: %v", err)
	}
	if rv.FullBytes != nil {
		t.Fatalf("encodeFilter must not set FullBytes (would emit wrapper): % x", rv.FullBytes)
	}
	if rv.Class != asn1.ClassContextSpecific || rv.Tag != 7 || rv.IsCompound {
		t.Fatalf("presence filter RawValue = %+v, want primitive context tag 7", rv)
	}
	if !bytes.Equal(rv.Bytes, []byte("objectClass")) {
		t.Fatalf("presence filter body = %q, want objectClass", rv.Bytes)
	}
}

func TestEncodeSearchRequest_AcceptedByWireShape(t *testing.T) {
	// Mirrors internal/engine/ad/ldap GetRootDSE options.
	req := SearchRequest{
		BaseObject: "",
		Scope:      LDAP_SCOPE_BASE,
		Filter:     *ParseFilter("(objectClass=*)"),
		Attributes: []string{"*"},
		SizeLimit:  1,
		TimeLimit:  30,
	}
	data, err := EncodeSearchRequest(2, req, nil)
	if err != nil {
		t.Fatalf("EncodeSearchRequest: %v", err)
	}
	// Outer LDAPMessage: SEQUENCE { INTEGER 2, [APPLICATION 3] ... }
	if data[0] != 0x30 {
		t.Fatalf("outer tag = %#02x, want SEQUENCE 0x30", data[0])
	}
	if !bytes.Contains(data, []byte{0x63}) {
		t.Fatalf("missing [APPLICATION 3] SearchRequest tag 0x63 in % x", data)
	}
	// The presence filter must appear as 0x87 0x0b objectClass —
	// the pre-fix binary produced 30 19 02 01 03 bf 81 23 ... instead.
	if !bytes.Contains(data, []byte{0x87, 0x0b, 'o', 'b', 'j', 'e', 'c', 't', 'C', 'l', 'a', 's', 's'}) {
		t.Fatalf("malformed/present-filter absent from SearchRequest: % x", data)
	}
	// attributeSelection must be 30 03 04 01 2a ("*") — pre-fix produced
	// 30 05 30 03 0c 01 2a (double-wrapped UTF8String).
	if !bytes.Contains(data, []byte{0x30, 0x03, 0x04, 0x01, 0x2a}) {
		t.Fatalf("attributeSelection not SEQUENCE OF OCTET STRING: % x", data)
	}
	// Scope must be ENUMERATED 0 (base) per RFC 4511 — pre-fix sent INTEGER.
	if !bytes.Contains(data, []byte{0x0a, 0x01, 0x00}) {
		t.Fatalf("scope not ENUMERATED 0 (base): % x", data)
	}
}

func TestParseFilter_EqualityStillWorks(t *testing.T) {
	f := ParseFilter("(sAMAccountName=user1)")
	if f == nil {
		t.Fatal("equality filter should parse")
	}
	if f.FilterType != LDAP_FILTER_EQUALITY {
		t.Fatalf("want EQUALITY, got %d", f.FilterType)
	}
	rv, err := encodeFilter(*f)
	if err != nil {
		t.Fatalf("encodeFilter: %v", err)
	}
	if rv.Class != asn1.ClassContextSpecific || rv.Tag != 3 || !rv.IsCompound {
		t.Fatalf("equality filter RawValue = %+v, want constructed context tag 3", rv)
	}
	// AttributeValueAssertion: SEQUENCE { attr, value }
	if !bytes.Contains(rv.Bytes, []byte("sAMAccountName")) || !bytes.Contains(rv.Bytes, []byte("user1")) {
		t.Fatalf("equality assertion bytes malformed: % x", rv.Bytes)
	}
}

// Stage 46g live defect: EqualityFilter kept the SEQUENCE TLV inside [3]
// (A3 L 30 L 04 attr 04 value). Samba rejects the illegal extra layer and
// closes the connection, surfacing as "receive search response: EOF".
// The [3] tag must REPLACE the SEQUENCE tag: A3 L 04 attr 04 value.
func TestEqualityFilter_WireShapeNoSequenceWrapper(t *testing.T) {
	f := ParseFilter("(objectClass=user)")
	if f == nil {
		t.Fatal("filter should parse")
	}
	tlv := mustMarshalFilter(*f)
	// a3 13 04 0b objectClass 04 04 user
	want := []byte{0xa3, 0x13, 0x04, 0x0b, 'o', 'b', 'j', 'e', 'c', 't', 'C', 'l', 'a', 's', 's', 0x04, 0x04, 'u', 's', 'e', 'r'}
	if !bytes.Equal(tlv, want) {
		t.Fatalf("equality filter TLV = % x, want % x", tlv, want)
	}
	if bytes.Contains(tlv, []byte{0x30}) {
		t.Fatalf("equality filter must not contain SEQUENCE wrapper: % x", tlv)
	}
}

// Composite filters must embed their children's context-specific choice TLVs
// verbatim. mustMarshalFilter used to emit SEQUENCE{INTEGER, ANY} per child,
// corrupting every AND/OR/NOT on the wire.
func TestMustMarshalFilter_ChoiceTLVNotSequence(t *testing.T) {
	tlv := mustMarshalFilter(*ParseFilter("(objectClass=*)"))
	want := append([]byte{0x87, 0x0b}, []byte("objectClass")...)
	if !bytes.Equal(tlv, want) {
		t.Fatalf("presence TLV = % x, want % x", tlv, want)
	}
}

func TestAndFilter_ChildrenAreChoiceTLVs(t *testing.T) {
	and := AndFilter(
		*ParseFilter("(objectClass=user)"),
		*ParseFilter("(!(objectClass=computer))"),
	)
	tlv := mustMarshalFilter(and)
	// and [0] SET OF Filter → 0xa0 per RFC 4511 4.5.1.7.
	if len(tlv) == 0 || tlv[0] != 0xa0 {
		t.Fatalf("AND TLV = % x, want leading 0xa0", tlv)
	}
	// First child must be the equality choice TLV (a3 ...), not
	// 30 L 02 01 03 (SEQUENCE{INTEGER, ANY}).
	if tlv[2] != 0xa3 {
		t.Fatalf("AND first child = %#02x, want 0xa3 choice TLV; full: % x", tlv[2], tlv)
	}
	// The NOT child must be A2-wrapped choice TLV, not SEQUENCE{INTEGER 2, ANY}.
	if !bytes.Contains(tlv, []byte{0xa2}) {
		t.Fatalf("AND must contain a2 (NOT) child: % x", tlv)
	}
	// Regression: NOT parsing must not leak parentheses into the AVA
	// (attr "(objectClass" / value "computer)").
	not := ParseFilter("(!(objectClass=computer))")
	if not == nil || not.FilterType != LDAP_FILTER_NOT {
		t.Fatalf("NOT filter should parse as NOT")
	}
	notTLV := mustMarshalFilter(*not)
	if bytes.Contains(notTLV, []byte{'('}) {
		t.Fatalf("NOT child leaked parens into assertion: % x", notTLV)
	}
}

// SubstringsFilter must emit A4 L 04 type 30 L (80|81|82) ... with
// context-specific CONSTRUCTED substring choices per RFC 4511 4.5.1.7.
func TestSubstringsFilter_WireShape(t *testing.T) {
	f := SubstringsFilter("sAMAccountName", "user", "", "")
	tlv := mustMarshalFilter(f)
	if len(tlv) == 0 || tlv[0] != 0xa4 {
		t.Fatalf("substrings TLV = % x, want leading 0xa4", tlv)
	}
	// type OCTET STRING then SEQUENCE then 0x80 (initial) constructed.
	if !bytes.Contains(tlv, []byte{0x04}) || !bytes.Contains(tlv, []byte{0x30}) || !bytes.Contains(tlv, []byte{0x80}) {
		t.Fatalf("substrings shape wrong: % x", tlv)
	}
	// Substring element must be constructed context tag 0x80 with the value
	// bytes directly inside (no INTEGER discriminator, no UTF8String tag 0x0c).
	if !bytes.Contains(tlv, []byte{'u', 's', 'e', 'r'}) {
		t.Fatalf("substring value missing: % x", tlv)
	}
}

func TestEncodeSearchRequest_SubtreeScopeUnchanged(t *testing.T) {
	req := SearchRequest{
		BaseObject: "DC=aether,DC=test",
		Scope:      LDAP_SCOPE_SUBTREE,
		Filter:     *ParseFilter("(objectClass=group)"),
		Attributes: []string{"cn"},
		SizeLimit:  1000,
		TimeLimit:  30,
	}
	data, err := EncodeSearchRequest(1, req, nil)
	if err != nil {
		t.Fatalf("EncodeSearchRequest: %v", err)
	}
	// Subtree scope 2 as ENUMERATED.
	if !bytes.Contains(data, []byte{0x0a, 0x01, 0x02}) {
		t.Fatalf("subtree scope not preserved: % x", data)
	}
	// Base DN present as OCTET STRING.
	if !bytes.Contains(data, []byte("DC=aether,DC=test")) {
		t.Fatalf("base DN missing: % x", data)
	}
}
