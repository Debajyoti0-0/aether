package pivot

import (
	"bytes"
	"context"
	"encoding/asn1"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildASREQStructure(t *testing.T) {
	der, err := BuildASREQ(ASREQOptions{
		Realm:      "INTERNAL.LOCAL",
		ClientName: "user",
		Token:      "cloud-token-blob",
	})
	if err != nil {
		t.Fatalf("build as-req: %v", err)
	}
	if len(der) == 0 {
		t.Fatal("empty as-req")
	}

	// Top tag must be APPLICATION-10 (0x6A).
	if der[0] != 0x6A {
		t.Errorf("app tag = %#x, want 0x6A", der[0])
	}

	// Body should embed realm and client name as GeneralStrings.
	if !strings.Contains(string(der), "INTERNAL.LOCAL") {
		t.Error("realm missing from as-req")
	}
	if !strings.Contains(string(der), "krbtgt") {
		t.Error("sname missing from as-req")
	}
	if !strings.Contains(string(der), "cloud-token-blob") {
		t.Error("token pre-auth missing from as-req")
	}
}

func TestBuildASREQValidation(t *testing.T) {
	if _, err := BuildASREQ(ASREQOptions{}); err == nil {
		t.Error("empty options should fail")
	}
	if _, err := BuildASREQ(ASREQOptions{Realm: "R"}); err == nil {
		t.Error("missing client should fail")
	}
}

func TestKKDCPWrapUnwrap(t *testing.T) {
	msg := []byte("hello-kerberos")
	wrapped := wrapKDCProxyMessage(msg)

	if got := binary.BigEndian.Uint32(wrapped[:4]); got != uint32(len(msg)) {
		t.Errorf("prefix = %d, want %d", got, len(msg))
	}
	unwrapped, err := unwrapKDCProxyMessage(wrapped)
	if err != nil {
		t.Fatalf("unwrap: %v", err)
	}
	if string(unwrapped) != string(msg) {
		t.Errorf("unwrapped = %q", unwrapped)
	}

	// Unwrapped passthrough.
	passthrough, err := unwrapKDCProxyMessage(msg)
	if err != nil {
		t.Fatalf("passthrough: %v", err)
	}
	if string(passthrough) != string(msg) {
		t.Errorf("passthrough = %q", passthrough)
	}
}

func TestKKDCPClientSend(t *testing.T) {
	sent := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		readFull(r, body)
		sent <- body

		// Reply with a wrapped synthetic AS-REP.
		asrep := buildSyntheticASREP("INTERNAL.LOCAL", "user")
		w.Write(wrapKDCProxyMessage(asrep))
	}))
	defer srv.Close()

	client := NewKKDCPClient(srv.URL, "INTERNAL.LOCAL", srv.Client())
	asreq, _ := BuildASREQ(ASREQOptions{Realm: "INTERNAL.LOCAL", ClientName: "user"})

	reply, err := client.Send(context.Background(), asreq)
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	got := <-sent
	if !bytes.HasPrefix(got, []byte{0, 0}) {
		t.Error("request should be length-prefixed")
	}

	info, err := ParseASREP(reply)
	if err != nil {
		t.Fatalf("parse reply: %v", err)
	}
	if info.CRealm != "INTERNAL.LOCAL" {
		t.Errorf("realm = %q", info.CRealm)
	}
}

func readFull(r *http.Request, buf []byte) {
	for n := 0; n < len(buf); {
		m, _ := r.Body.Read(buf[n:])
		if m == 0 {
			break
		}
		n += m
	}
}

// buildSyntheticASREP builds a minimal valid AS-REP with a ticket.
func buildSyntheticASREP(realm, user string) []byte {
	// crealm [3]
	crealm := ctxTag(3, primString(realm))
	// cname [4]
	cname := ctxTag(4, primString(user))
	// ticket [5]: outer Ticket { tkt-vno, realm, sname, enc-part }
	ticketInner := concat(
		ctxTag(0, mustMarshalInt(5)),
		ctxTag(1, primString(realm)),
		ctxTag(2, snameSeq(realm)),
		ctxTag(3, primString("enc-part-blob")),
	)
	ticket := ctxTag(5, seqDER(ticketInner))
	// enc-part [6]
	encPart := ctxTag(6, primString("enc-part"))

	content := concat(crealm, cname, ticket, encPart)
	header := []byte{0x60 | byte(ASREPTag)}
	header = append(header, derLen(len(content))...)
	return append(header, content...)
}

func ctxTag(tag int, content []byte) []byte {
	return append([]byte{0xA0 | byte(tag)}, append(derLen(len(content)), content...)...)
}

func primString(s string) []byte {
	return append([]byte{0x1B, byte(len(s))}, []byte(s)...)
}

func seqDER(content []byte) []byte {
	return append([]byte{0x30}, append(derLen(len(content)), content...)...)
}

func snameSeq(realm string) []byte {
	inner := concat(
		ctxTag(0, mustMarshalInt(1)),
		ctxTag(1, seqDER(concat(primString("krbtgt"), primString(realm)))),
	)
	return seqDER(inner)
}

func derLen(l int) []byte {
	switch {
	case l < 128:
		return []byte{byte(l)}
	case l < 256:
		return []byte{0x81, byte(l)}
	default:
		return []byte{0x82, byte(l >> 8), byte(l)}
	}
}

func TestParseASREP(t *testing.T) {
	asrep := buildSyntheticASREP("CORP.LOCAL", "alice")
	info, err := ParseASREP(asrep)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if info.CRealm != "CORP.LOCAL" {
		t.Errorf("realm = %q", info.CRealm)
	}
	if info.TicketDER == nil {
		t.Error("ticket missing")
	}
	// Ticket DER must be a well-formed SEQUENCE.
	var raw asn1.RawValue
	if _, err := asn1.Unmarshal(info.TicketDER, &raw); err != nil {
		t.Errorf("ticket der malformed: %v", err)
	}
}

func TestParseASREPNotARep(t *testing.T) {
	// AS-REQ tag instead of AS-REP.
	bad := buildSyntheticASREP("R", "u")
	bad[0] = 0x6A
	if _, err := ParseASREP(bad); err == nil {
		t.Error("expected tag mismatch error")
	}
}

func TestWriteCcache(t *testing.T) {
	asrep := buildSyntheticASREP("CORP.LOCAL", "alice")
	info, err := ParseASREP(asrep)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	path := filepath.Join(t.TempDir(), "aether.ccache")
	if err := WriteCcache(path, info, "alice@CORP.LOCAL"); err != nil {
		t.Fatalf("write ccache: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ccache: %v", err)
	}

	// Magic + version + taglen.
	if data[0] != 0x05 || data[1] != 0x04 {
		t.Errorf("magic = %#x %#x", data[0], data[1])
	}
	// Contains realm and principal.
	if !strings.Contains(string(data), "CORP.LOCAL") {
		t.Error("realm missing from ccache")
	}
	if !strings.Contains(string(data), "alice") {
		t.Error("principal missing from ccache")
	}
	if !strings.Contains(string(data), "krbtgt") {
		t.Error("server principal missing from ccache")
	}
}

func TestWriteCcacheNilTicket(t *testing.T) {
	if err := WriteCcache("x.ccache", nil, "u"); err == nil {
		t.Error("nil info should fail")
	}
	if err := WriteCcache("x.ccache", &ASREPInfo{}, "u"); err == nil {
		t.Error("no ticket should fail")
	}
}

func TestPrincipalParts(t *testing.T) {
	if got := principalParts("alice@CORP.LOCAL"); len(got) != 1 || got[0] != "alice" {
		t.Errorf("parts = %v", got)
	}
	if got := principalParts("host/dc1.corp.com"); len(got) != 2 {
		t.Errorf("parts = %v", got)
	}
}

func TestHexDump(t *testing.T) {
	long := make([]byte, 64)
	out := HexDump(long)
	if len(out) > 70 {
		t.Errorf("dump too long: %d", len(out))
	}
}

func TestExtractCloudTGTValidation(t *testing.T) {
	// nil client
	if _, err := ExtractCloudTGT(context.Background(), nil, ASREQOptions{}, "", ""); err == nil {
		t.Error("nil client should fail")
	}
	// unreachable endpoint fails at send stage
	client := NewKKDCPClient("http://127.0.0.1:1/nope", "R.LOCAL", http.DefaultClient)
	if _, err := ExtractCloudTGT(context.Background(), client, ASREQOptions{ClientName: "u"}, "u", ""); err == nil {
		t.Error("unreachable endpoint should fail")
	}
}
