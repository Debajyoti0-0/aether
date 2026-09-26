// asn1probe dumps the actual wire bytes produced by the kerberos package
// (Stage 46R Phase 0 evidence — hex dump of the real AS-REQ).
package main

import (
	"encoding/asn1"
	"fmt"
	"os"
	"time"

	krb "github.com/Debajyoti0-0/aether/internal/protocol/kerberos"
)

func walk(b []byte, depth int) {
	for i := 0; i < len(b); {
		if i+2 > len(b) {
			break
		}
		tag := b[i]
		l := int(b[i+1])
		hdr := 2
		if l&0x80 != 0 {
			n := int(l & 0x7f)
			if i+2+n > len(b) {
				break
			}
			l = 0
			for k := 0; k < n; k++ {
				l = l<<8 | int(b[i+2+k])
			}
			hdr = 2 + n
		}
		if i+hdr+l > len(b) {
			fmt.Printf("%stag=0x%02x len=%d (truncated)\n", indent(depth), tag, l)
			break
		}
		class := "UNIV"
		switch tag >> 6 {
		case 1:
			class = "APPL"
		case 2:
			class = "CTX"
		}
		fmt.Printf("%stag=0x%02x (%s tlv=%d) len=%d bytes=%x\n",
			indent(depth), tag, class, l, l, b[i+hdr:i+hdr+l])
		if tag&0x20 != 0 { // constructed
			walk(b[i+hdr:i+hdr+l], depth+1)
		}
		i += hdr + l
	}
}

func indent(d int) string {
	s := ""
	for i := 0; i < d; i++ {
		s += "  "
	}
	return s
}

func main() {
	fmt.Fprintln(os.Stderr, "=== PROBE 1: KDCRealm.MarshalASN1 direct ===")
	enc, err := krb.TestKDCRealmMarshal("AETHER.TEST")
	fmt.Fprintf(os.Stderr, "KDCRealm bytes: %x err=%v\n", enc, err)

	fmt.Fprintln(os.Stderr, "=== PROBE 2: Realm.MarshalASN1 direct ===")
	r2 := krb.TestRealmMarshal("AETHER.TEST")
	fmt.Fprintf(os.Stderr, "Realm bytes: %x err=%v\n", r2.Bytes, r2.Err)

	fmt.Fprintln(os.Stderr, "=== PROBE 3: full AS-REQ wire dump ===")
	asreq, err := krb.BuildASREQ(
		krb.MakeUserPrincipal("user1", "AETHER.TEST"),
		krb.Realm("AETHER.TEST"),
		krb.MakeUserPrincipal("krbtgt", "AETHER.TEST"),
		[]int32{krb.ETYPE_AES256_CTS_HMAC_SHA1_96, krb.ETYPE_AES128_CTS_HMAC_SHA1_96, krb.ETYPE_RC4_HMAC},
		12345,
		time.Now().UTC().Add(5*time.Minute),
		nil,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "BuildASREQ error:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "AS-REQ %d bytes\n%x\n", len(asreq), asreq)
	fmt.Fprintln(os.Stderr, "--- tree ---")
	walk(asreq, 0)

	fmt.Fprintln(os.Stderr, "=== PROBE 4: RawValue universal-tag behavior ===")
	v := struct {
		R asn1.RawValue
	}{R: asn1.RawValue{Tag: 27, Class: 0, Bytes: []byte("AETHER.TEST")}}
	out, err := asn1.Marshal(v)
	fmt.Fprintf(os.Stderr, "RawValue{Tag:27,Class:0} -> %x err=%v\n", out, err)

	inner := []byte{0x1b, 0x0b}
	inner = append(inner, []byte("AETHER.TEST")...)
	rv := asn1.RawValue{Tag: 2, Class: 2, IsCompound: true, Bytes: inner}
	out2, err := asn1.Marshal(rv)
	fmt.Fprintf(os.Stderr, "RawValue{Tag:2,CTX,constructed} -> %x err=%v\n", out2, err)
}
