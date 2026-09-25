package kerberos

import (
	"encoding/asn1"
)

// Test-only hooks (used by cmd/asn1probe and wire tests to verify the exact
// DER bytes produced by the package). Not for production use.

// TestKDCRealmMarshal returns the [2] EXPLICIT GeneralString realm encoding.
func TestKDCRealmMarshal(s string) ([]byte, error) {
	return derCtx(2, derGeneralString(s)).encode(), nil
}

// TestRealmMarshal mirrors TestKDCRealmMarshal for the bare GeneralString form.
func TestRealmMarshal(s string) TestRealmMarshalRet {
	return TestRealmMarshalRet{Bytes: derGeneralString(s).encode(), Err: nil}
}

type TestRealmMarshalRet struct {
	Bytes []byte
	Err   error
}

// TestEncodeKDCOptions returns the KDCOptions BIT STRING bytes for flags.
func TestEncodeKDCOptions(o uint32) asn1.RawValue {
	return asn1.RawValue{Class: 0, Tag: 3, IsCompound: false, Bytes: kdcOptionsBytes(o)}
}