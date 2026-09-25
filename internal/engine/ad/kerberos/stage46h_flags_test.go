package kerberos

import (
	"encoding/asn1"
	"testing"
)

// TestTicketFlagsSafePreservesAll32Bits is the Stage 46h D3 companion
// regression test. EncKDCRepPart.TicketFlags is a 32-bit BIT STRING, but the
// previous implementation returned only the first octet, so a credential cache
// written by Aether recorded 0x00000040 instead of the real 0x40e00000
// (forwardable|renewable|pre-authenticated|initial). The value below is the
// TicketFlags observed in the live Stage 46h AS-REP and matches the flags
// recorded by MIT kinit in testdata/mit-kinit-user1.ccache.
func TestTicketFlagsSafePreservesAll32Bits(t *testing.T) {
	// forwardable | renewable | pre-authenticated | initial
	raw := []byte{0x40, 0xe0, 0x00, 0x00}
	bs := asn1.BitString{Bytes: raw, BitLength: len(raw) * 8}

	got := ticketFlagsSafe(bs)
	if want := int32(0x40e00000); got != want {
		t.Fatalf("ticketFlagsSafe = %#08x, want %#08x", uint32(got), uint32(want))
	}
}

// TestTicketFlagsSafeAbsentField pins the OPTIONAL-field guard: an absent or
// empty BIT STRING must yield 0 and never panic (the Stage 46g live defect).
func TestTicketFlagsSafeAbsentField(t *testing.T) {
	for name, bs := range map[string]asn1.BitString{
		"zero value":  {},
		"no bytes":    {Bytes: nil, BitLength: 32},
		"zero length": {Bytes: []byte{}, BitLength: 32},
		"zero bits":   {Bytes: []byte{0x40, 0xe0, 0x00, 0x00}, BitLength: 0},
	} {
		if got := ticketFlagsSafe(bs); got != 0 {
			t.Fatalf("%s: ticketFlagsSafe = %#08x, want 0", name, uint32(got))
		}
	}
}

// TestTicketFlagsSafeMasksShortBitString ensures a BIT STRING shorter than a
// full word does not contribute stray bits from the final octet.
func TestTicketFlagsSafeMasksShortBitString(t *testing.T) {
	// 12 significant bits: the low 4 bits of the second octet are undefined
	// and must be cleared.
	bs := asn1.BitString{Bytes: []byte{0x12, 0x3f}, BitLength: 12}
	if got := ticketFlagsSafe(bs); got != 0x1230 {
		t.Fatalf("ticketFlagsSafe = %#08x, want %#08x", uint32(got), uint32(0x1230))
	}
}
