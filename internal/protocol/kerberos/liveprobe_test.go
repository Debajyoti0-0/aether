package kerberos

import (
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"testing"
	"time"
)

// TestLivePreAuthProbe is a diagnostic probe, not an assertion of protocol
// behaviour. It is skipped unless AETHER_LIVE_KDC is set, so the normal suite
// stays hermetic. Run it inside the AD lab:
//
//	AETHER_LIVE_KDC=172.18.0.2 go test -run TestLivePreAuthProbe -v ./internal/protocol/kerberos
func TestLivePreAuthProbe(t *testing.T) {
	dc := os.Getenv("AETHER_LIVE_KDC")
	if dc == "" {
		t.Skip("set AETHER_LIVE_KDC to run the live pre-auth probe")
	}
	user := os.Getenv("AETHER_LIVE_USER")
	if user == "" {
		user = "administrator"
	}
	realm := Realm("AETHER.TEST")
	cp := MakeUserPrincipal(user, string(realm))
	sp := MakeUserPrincipal("krbtgt", string(realm))

	req, err := BuildASREQ(cp, realm, sp, SupportedEtypes(), 12345, time.Now().Add(5*time.Minute), nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	addr, err := net.ResolveUDPAddr("udp", dc+":88")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	c, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	if _, err := c.Write(req); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = c.SetReadDeadline(time.Now().Add(10 * time.Second))
	buf := make([]byte, 65535)
	n, err := c.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	resp := buf[:n]
	fmt.Printf("PROBE user=%q respLen=%d\n", user, n)
	fmt.Printf("PROBE raw=%s\n", hex.EncodeToString(resp))

	e, perr := ParseKRBError(resp)
	if perr != nil {
		t.Fatalf("ParseKRBError: %v", perr)
	}
	fmt.Printf("PROBE errorCode=%d (%s)\n", e.ErrorCode, KDCErrorName(e.ErrorCode))
	fmt.Printf("PROBE eDataLen=%d\n", len(e.EData))
	fmt.Printf("PROBE eData=%s\n", hex.EncodeToString(e.EData))

	pads, derr := parsePADataTLV(e.EData)
	if derr != nil {
		fmt.Printf("PROBE parsePADataTLV error=%v\n", derr)
	}
	for i, p := range pads {
		fmt.Printf("PROBE paData[%d] type=%d len=%d value=%s\n", i, p.PADataType, len(p.PADataValue), hex.EncodeToString(p.PADataValue))
	}
	infos2, ierr := ParseEtypeInfo2(padValue(pads, PA_DATA_TYPE_ETYPE_INFO2))
	fmt.Printf("PROBE etypeInfo2=%+v err=%v\n", infos2, ierr)
	infos1, ierr1 := ParseEtypeInfo(padValue(pads, PA_DATA_TYPE_ETYPE_INFO))
	fmt.Printf("PROBE etypeInfo=%+v err=%v\n", infos1, ierr1)

	hint, herr := ParsePreAuthHint(resp)
	fmt.Printf("PROBE hint=%+v err=%v\n", hint, herr)
}

func padValue(pads []PAData, want int32) []byte {
	for _, p := range pads {
		if p.PADataType == want {
			return p.PADataValue
		}
	}
	return nil
}
