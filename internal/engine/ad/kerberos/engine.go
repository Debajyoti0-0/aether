package kerberos

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"time"

	"github.com/Debajyoti0-0/aether/internal/protocol/kerberos"
)

type Engine struct {
	dc        string
	kdcPort   int
	ldapPort  int
	realm     string
	domain    string
	transport *kerberosTransport
	engagement *EngagementConfig
}

type EngagementConfig struct {
	AllowedRealms     []string
	AllowedUsers      []string
	AllowedDCs        []string
	AllowedCapabilities []string
	ExpiresAt         time.Time
}

type TGT struct {
	SessionKey []byte
	Ticket     *Ticket
}

type Ticket struct {
	Flags     int32
	AuthTime  time.Time
	EndTime   time.Time
	StartTime time.Time
	RenewTill time.Time
	SName     *kerberos.PrincipalName
	Realm     kerberos.Realm
	EncPart   kerberos.EncryptedData
}

type TGS struct {
	Ticket *Ticket
}

type KerberoastBlob struct {
	Format        string
	HashAlgorithm string
	Hash          string
}

type ASREPRoastBlob struct {
	Format        string
	HashAlgorithm string
	Hash          string
}

const (
	CcacheFormatMIT     = "MIT"
	CcacheFormatHeimdal = "Heimdal"
)

func NewEngine(dc string, kdcPort, ldapPort int, realm, domain string) *Engine {
	return &Engine{
		dc:       dc,
		kdcPort:  kdcPort,
		ldapPort: ldapPort,
		realm:    realm,
		domain:   domain,
	}
}

func NewEngineWithEngagement(dc string, kdcPort, ldapPort int, realm, domain string, engagement *EngagementConfig) *Engine {
	e := NewEngine(dc, kdcPort, ldapPort, realm, domain)
	e.engagement = engagement
	return e
}

func (e *Engine) Close() error {
	if e.transport != nil {
		return e.transport.Close()
	}
	return nil
}

func (e *Engine) checkEngagement(username, capability string) error {
	if e.engagement == nil {
		return nil
	}
	if time.Now().After(e.engagement.ExpiresAt) {
		return fmt.Errorf("engagement expired")
	}
	realmMatch := false
	for _, r := range e.engagement.AllowedRealms {
		if r == e.realm {
			realmMatch = true
			break
		}
	}
	if !realmMatch {
		return fmt.Errorf("realm not allowed")
	}
	userMatch := false
	for _, u := range e.engagement.AllowedUsers {
		if u == username {
			userMatch = true
			break
		}
	}
	if !userMatch {
		return fmt.Errorf("user not allowed")
	}
	dcMatch := false
	for _, d := range e.engagement.AllowedDCs {
		if d == e.dc {
			dcMatch = true
			break
		}
	}
	if !dcMatch {
		return fmt.Errorf("DC not allowed")
	}
	if len(e.engagement.AllowedCapabilities) > 0 {
		capMatch := false
		for _, c := range e.engagement.AllowedCapabilities {
			if c == capability {
				capMatch = true
				break
			}
		}
		if !capMatch {
			return fmt.Errorf("capability not granted: %s", capability)
		}
	}
	return nil
}

func (e *Engine) getTransport() (*kerberosTransport, error) {
	if e.transport != nil {
		return e.transport, nil
	}
	// Use 127.0.0.1 explicitly to avoid IPv6 issues on Windows
	addr := "127.0.0.1"
	if e.dc != "localhost" {
		addr = e.dc
	}
	// Use UDP first (like kinit), fallback to TCP if needed
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", addr, e.kdcPort))
	if err != nil {
		return nil, fmt.Errorf("resolve UDP addr: %w", err)
	}
	udpConn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, fmt.Errorf("dial UDP: %w", err)
	}
	e.transport = &kerberosTransport{udpConn: udpConn, dc: e.dc, domain: e.domain, kdcAddr: udpAddr}
	return e.transport, nil
}

func (e *Engine) RequestTGT(ctx context.Context, username, password string) (*TGT, error) {
	if err := e.checkEngagement(username, "kerberos:tgt"); err != nil {
		return nil, err
	}

	clientPrincipal := kerberos.MakeUserPrincipal(username, e.realm)
	clientRealm := kerberos.Realm(e.realm)
	serverPrincipal := kerberos.MakeUserPrincipal("krbtgt", e.realm)

	salt := kerberos.MakeSalt(clientRealm, clientPrincipal)
	sessionKey, err := kerberos.StringToKey(kerberos.ETYPE_AES256_CTS_HMAC_SHA1_96, password, salt, "")
	if err != nil {
		return nil, fmt.Errorf("key derivation: %w", err)
	}

	transport, err := e.getTransport()
	if err != nil {
		return nil, err
	}

	asreq, err := kerberos.BuildASREQWithPreauth(clientPrincipal, clientRealm, serverPrincipal, kerberos.SupportedEtypes(), 12345, time.Now().Add(5*time.Minute), password)
	if err != nil {
		return nil, fmt.Errorf("build AS-REQ: %w", err)
	}

	fmt.Printf("DEBUG: Sending AS-REQ (%d bytes)\n", len(asreq))
	fmt.Printf("DEBUG: AS-REQ hex: %x\n", asreq)
	if err := transport.Send(asreq); err != nil {
		return nil, fmt.Errorf("send AS-REQ: %w", err)
	}

	resp, err := transport.Recv()
	fmt.Printf("DEBUG: Received response (%d bytes, err=%v)\n", len(resp), err)
	if err != nil {
		return nil, fmt.Errorf("recv AS-REP: %w", err)
	}

	rep, err := kerberos.ParseASREP(resp)
	if err != nil {
		if kerr, ok := kerberos.IsKDCError(err); ok {
			return nil, fmt.Errorf("KDC error: %v", kerr)
		}
		return nil, fmt.Errorf("parse AS-REP: %w", err)
	}

	encPart, err := kerberos.DecryptASREPEncPart(rep, sessionKey, kerberos.ETYPE_AES256_CTS_HMAC_SHA1_96)
	if err != nil {
		return nil, fmt.Errorf("decrypt AS-REP: %w", err)
	}

	ticket := &Ticket{
		Flags:     int32(encPart.Flags.Bytes[0]),
		AuthTime:  encPart.Authtime.Time,
		EndTime:   encPart.Endtime.Time,
		StartTime: encPart.Starttime.Time,
		RenewTill: encPart.RenewTill.Time,
		SName:     &rep.Ticket.SName,
		Realm:     rep.Ticket.Realm,
		EncPart:   rep.Ticket.EncPart,
	}

	return &TGT{
		SessionKey: sessionKey,
		Ticket:     ticket,
	}, nil
}

func (e *Engine) RequestTGTWithKeytab(ctx context.Context, username, keytabPath string) (*TGT, error) {
	if err := e.checkEngagement(username, "kerberos:tgt"); err != nil {
		return nil, err
	}

	clientPrincipal := kerberos.MakeUserPrincipal(username, e.realm)
	clientRealm := kerberos.Realm(e.realm)
	serverPrincipal := kerberos.MakeUserPrincipal("krbtgt", e.realm)

	keytab, err := kerberos.ReadKeytab(keytabPath)
	if err != nil {
		return nil, fmt.Errorf("read keytab: %w", err)
	}

	// Find the appropriate key (prefer AES256)
	entry, err := keytab.FindEntry(clientPrincipal.FullName(clientRealm), clientRealm, kerberos.ETYPE_AES256_CTS_HMAC_SHA1_96)
	if err != nil {
		// Try RC4-HMAC as fallback
		entry, err = keytab.FindEntry(clientPrincipal.FullName(clientRealm), clientRealm, kerberos.ETYPE_RC4_HMAC)
		if err != nil {
			return nil, fmt.Errorf("no suitable key in keytab: %w", err)
		}
	}

	sessionKey := entry.Key

	transport, err := e.getTransport()
	if err != nil {
		return nil, err
	}

	asreq, err := kerberos.BuildASREQWithKeytab(clientPrincipal, clientRealm, serverPrincipal, kerberos.SupportedEtypes(), 12345, time.Now().Add(5*time.Minute), entry.Key)
	if err != nil {
		return nil, fmt.Errorf("build AS-REQ: %w", err)
	}

	if err := transport.Send(asreq); err != nil {
		return nil, fmt.Errorf("send AS-REQ: %w", err)
	}

	resp, err := transport.Recv()
	if err != nil {
		return nil, fmt.Errorf("recv AS-REP: %w", err)
	}

	rep, err := kerberos.ParseASREP(resp)
	if err != nil {
		if kerr, ok := kerberos.IsKDCError(err); ok {
			return nil, fmt.Errorf("KDC error: %v", kerr)
		}
		return nil, fmt.Errorf("parse AS-REP: %w", err)
	}

	encPart, err := kerberos.DecryptASREPEncPart(rep, sessionKey, kerberos.ETYPE_AES256_CTS_HMAC_SHA1_96)
	if err != nil {
		return nil, fmt.Errorf("decrypt AS-REP: %w", err)
	}

	ticket := &Ticket{
		Flags:     int32(encPart.Flags.Bytes[0]),
		AuthTime:  encPart.Authtime.Time,
		EndTime:   encPart.Endtime.Time,
		StartTime: encPart.Starttime.Time,
		RenewTill: encPart.RenewTill.Time,
		SName:     &rep.Ticket.SName,
		Realm:     rep.Ticket.Realm,
		EncPart:   rep.Ticket.EncPart,
	}

	return &TGT{
		SessionKey: sessionKey,
		Ticket:     ticket,
	}, nil
}

func (e *Engine) RequestTGS(ctx context.Context, tgt *TGT, spn string) (*TGS, error) {
	if err := e.checkEngagement("", "kerberos:tgs"); err != nil {
		return nil, err
	}

	parsedSPN, err := kerberos.ParseSPN(spn)
	if err != nil {
		return nil, fmt.Errorf("invalid SPN: %w", err)
	}

	clientPrincipal := tgt.Ticket.SName
	clientRealm := tgt.Ticket.Realm
	serverPrincipal := kerberos.MakeSPNPrincipal(parsedSPN.ServiceClass, parsedSPN.Host, e.realm)

	auth := kerberos.Authenticator{
		AuthenticatorVNO: 5,
		CName:            *clientPrincipal,
		Crealm:           clientRealm,
		CTime:            kerberos.NewKerberosTime(time.Now().UTC()),
		CUSec:            0,
	}

	authBytes, err := kerberos.MarshalAuthenticator(auth)
	if err != nil {
		return nil, fmt.Errorf("marshal authenticator: %w", err)
	}

	encryptedAuth, err := kerberos.Encrypt(kerberos.ETYPE_AES256_CTS_HMAC_SHA1_96, tgt.SessionKey, kerberos.KeyUsage_AP_REQ_AUTHENTICATOR, authBytes)
	if err != nil {
		return nil, fmt.Errorf("encrypt authenticator: %w", err)
	}

	transport, err := e.getTransport()
	if err != nil {
		return nil, err
	}

	apreq := kerberos.BuildAPREQ(toProtoTicket(tgt.Ticket), encryptedAuth)
	tgsreq, err := kerberos.BuildTGSREQ(
		*clientPrincipal, clientRealm, serverPrincipal,
		toProtoTicket(tgt.Ticket), kerberos.SupportedEtypes(), 12345,
		time.Now().Add(5*time.Minute), tgt.SessionKey, apreq,
	)
	if err != nil {
		return nil, fmt.Errorf("build TGS-REQ: %w", err)
	}

	if err := transport.Send(tgsreq); err != nil {
		return nil, fmt.Errorf("send TGS-REQ: %w", err)
	}

	resp, err := transport.Recv()
	if err != nil {
		return nil, fmt.Errorf("recv TGS-REP: %w", err)
	}

	rep, err := kerberos.ParseTGSREP(resp)
	if err != nil {
		if kerr, ok := kerberos.IsKDCError(err); ok {
			return nil, fmt.Errorf("TGS-REP error: %v", kerr)
		}
		return nil, fmt.Errorf("parse TGS-REP: %w", err)
	}

	ticket := &Ticket{
		Flags:     int32(rep.Ticket.EncPart.Cipher[0]),
		SName:     &rep.Ticket.SName,
		Realm:     rep.Ticket.Realm,
		EncPart:   rep.Ticket.EncPart,
	}

	return &TGS{Ticket: ticket}, nil
}

// toProtoTicket converts the engine-level Ticket to the protocol-level Ticket
// for wire encoding (the engine type carries parsed metadata; the protocol
// type is the DER structure).
func toProtoTicket(t *Ticket) kerberos.Ticket {
	if t == nil {
		return kerberos.Ticket{}
	}
	return kerberos.Ticket{
		TicketVNO: 5,
		Realm:     t.Realm,
		SName:     *t.SName,
		EncPart:   t.EncPart,
	}
}

func (e *Engine) ExtractKerberoastBlob(tgs *TGS) (*KerberoastBlob, error) {
	return &KerberoastBlob{
		Format:        "krb5tgs",
		HashAlgorithm: "sha256",
		Hash:          fmt.Sprintf("%x", sha256.Sum256(tgs.Ticket.EncPart.Cipher)),
	}, nil
}

func (e *Engine) RequestASREPNoPreauth(ctx context.Context, username string) ([]byte, error) {
	clientPrincipal := kerberos.MakeUserPrincipal(username, e.realm)
	clientRealm := kerberos.Realm(e.realm)
	serverPrincipal := kerberos.MakeUserPrincipal("krbtgt", e.realm)

	transport, err := e.getTransport()
	if err != nil {
		return nil, err
	}

	asreq, err := kerberos.BuildASREQ(clientPrincipal, clientRealm, serverPrincipal, kerberos.SupportedEtypes(), 12345, time.Now().Add(5*time.Minute), nil)
	if err != nil {
		return nil, fmt.Errorf("build AS-REQ: %w", err)
	}

	if err := transport.Send(asreq); err != nil {
		return nil, fmt.Errorf("send AS-REQ: %w", err)
	}

	resp, err := transport.Recv()
	if err != nil {
		return nil, fmt.Errorf("recv AS-REP: %w", err)
	}

	_, err = kerberos.ParseASREP(resp)
	if err != nil {
		if kerr, ok := kerberos.IsKDCError(err); ok && kerr.Code == kerberos.KDC_ERR_PREAUTH_REQUIRED {
			return resp, nil
		}
		return nil, err
	}

	return resp, nil
}

func (e *Engine) ExtractASREPRoastBlob(asrep []byte) (*ASREPRoastBlob, error) {
	return &ASREPRoastBlob{
		Format:        "krb5asrep",
		HashAlgorithm: "sha256",
		Hash:          fmt.Sprintf("%x", sha256.Sum256(asrep)),
	}, nil
}

func (e *Engine) ExportCcache(tgt *TGT, format string) ([]byte, error) {
	ccache := kerberos.NewCCache()
	ccache.AddEntry(kerberos.CCacheEntry{
		ClientPrincipal: *tgt.Ticket.SName,
		ClientRealm:     tgt.Ticket.Realm,
		ServerPrincipal: kerberos.MakeUserPrincipal("krbtgt", e.realm),
		ServerRealm:     kerberos.Realm(e.realm),
		Key:             kerberos.EncryptionKey{KeyType: kerberos.ETYPE_AES256_CTS_HMAC_SHA1_96, KeyValue: tgt.SessionKey},
		AuthTime:        tgt.Ticket.AuthTime,
		StartTime:       tgt.Ticket.StartTime,
		EndTime:         tgt.Ticket.EndTime,
		RenewTill:       tgt.Ticket.RenewTill,
		IsSKey:          false,
		TicketFlags:     tgt.Ticket.Flags,
		Ticket:          tgt.Ticket.EncPart.Cipher,
	})

	return ccache.Bytes()
}

func (e *Engine) ParseCcache(data []byte) (*TGT, error) {
	ccache, err := kerberos.ReadCCacheFromBytes(data)
	if err != nil {
		return nil, err
	}
	entry := ccache.GetDefaultEntry()
	if entry == nil {
		return nil, fmt.Errorf("no default entry in ccache")
	}
	return &TGT{
		SessionKey: entry.Key.KeyValue,
		Ticket: &Ticket{
			Flags:     entry.TicketFlags,
			AuthTime:  entry.AuthTime,
			StartTime: entry.StartTime,
			EndTime:   entry.EndTime,
			RenewTill: entry.RenewTill,
			SName:     &entry.ServerPrincipal,
			Realm:     entry.ServerRealm,
		},
	}, nil
}

func (e *Engine) CompareTickets(t1, t2 *Ticket) bool {
	if t1 == nil || t2 == nil {
		return false
	}
	if t1.Realm != t2.Realm {
		return false
	}
	if t1.SName == nil || t2.SName == nil {
		return false
	}
	if t1.SName.FullName(t1.Realm) != t2.SName.FullName(t2.Realm) {
		return false
	}
	if !t1.AuthTime.Equal(t2.AuthTime) || !t1.EndTime.Equal(t2.EndTime) {
		return false
	}
	return true
}