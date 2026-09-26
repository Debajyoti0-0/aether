package kerberos

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidTGSREQ = errors.New("invalid TGS-REQ")
	ErrInvalidTGSREP = errors.New("invalid TGS-REP")
)

// BuildTGSREQWithChecksum builds an RFC 4120 TGS-REQ carrying an AP-REQ
// whose authenticator contains the RFC-required checksum over the req-body
// (checksum[3], key usage 6). Samba/AD reject TGS authenticators without it.
// sessionKey is the TGT session key; logKeyEtype its etype.
func BuildTGSREQWithChecksum(
	clientPrincipal PrincipalName,
	clientRealm Realm,
	serverPrincipal PrincipalName,
	ticket Ticket,
	etypes []int32,
	nonce int32,
	till time.Time,
	sessionKey []byte,
	logKeyEtype int32,
) ([]byte, error) {
	// Build the req-body first so the authenticator can checksum it.
	body, err := buildKDCReqBody(clientPrincipal, clientRealm, serverPrincipal, etypes, nonce, till)
	if err != nil {
		return nil, err
	}
	bodyBytes := body.encode()

	cksum, err := ComputeChecksum(logKeyEtype, sessionKey, KeyUsage_TGS_REQ_AUTH_CKSUM, bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("authenticator checksum: %w", err)
	}
	// Checksum type per etype (RFC 3961 §5.4 / RFC 4757): 15 = hmac-sha1-96
	// for AES enctypes, 7 = HMAC-MD5 for RC4.
	cksumType := int32(15)
	if logKeyEtype == ETYPE_RC4_HMAC {
		cksumType = 7
	}

	auth := Authenticator{
		AuthenticatorVNO: 5,
		CName:            clientPrincipal,
		Crealm:           clientRealm,
		CTime:            NewKerberosTime(time.Now().UTC()),
		CUSec:            0,
		ChecksumType:     cksumType,
		Checksum:         cksum,
	}
	authBytes, err := MarshalAuthenticator(auth)
	if err != nil {
		return nil, fmt.Errorf("marshal authenticator: %w", err)
	}
	// RFC 4120 §5.5.1: the AP-REQ authenticator is encrypted with usage 7
	// (AP_REQ_AUTHENTICATOR) — the TGS session key is the key.
	encryptedAuth, err := Encrypt(logKeyEtype, sessionKey, KeyUsage_AP_REQ_AUTHENTICATOR, authBytes)
	if err != nil {
		return nil, fmt.Errorf("encrypt authenticator: %w", err)
	}

	return buildTGSREQFromParts(clientPrincipal, clientRealm, serverPrincipal,
		ticket, etypes, nonce, till, encryptedAuth)
}

// BuildTGSREQ builds a TGS-REQ with a caller-supplied (already encrypted)
// authenticator. Retained for callers that construct the AP-REQ themselves.
func BuildTGSREQ(
	clientPrincipal PrincipalName,
	clientRealm Realm,
	serverPrincipal PrincipalName,
	ticket Ticket,
	etypes []int32,
	nonce int32,
	till time.Time,
	sessionKey []byte,
	authenticator []byte,
) ([]byte, error) {
	return buildTGSREQFromParts(clientPrincipal, clientRealm, serverPrincipal,
		ticket, etypes, nonce, till, authenticator)
}

func buildTGSREQFromParts(
	clientPrincipal PrincipalName,
	clientRealm Realm,
	serverPrincipal PrincipalName,
	ticket Ticket,
	etypes []int32,
	nonce int32,
	till time.Time,
	authenticator []byte,
) ([]byte, error) {
	body, err := buildKDCReqBody(clientPrincipal, clientRealm, serverPrincipal, etypes, nonce, till)
	if err != nil {
		return nil, err
	}

	// padata[3] SEQUENCE { PA-DATA { padata-type[1] = 1 (PA-TGS-REQ),
	//                                padata-value[2] = AP-REQ bytes } }
	paValue := BuildAPREQ(ticket, authenticator)
	pa := derSeq(
		derCtx(1, derInt(1)),          // PA-TGS-REQ
		derCtx(2, derOctets(paValue)), // AP-REQ bytes
	)

	// RFC 4120 §5.4.1: TGS-REQ ::= [APPLICATION 12] KDC-REQ — the app tag
	// wraps a UNIVERSAL SEQUENCE (see BuildASREQ note).
	req := derApp(12, derSeq(
		derCtx(1, derInt(krb5PVNO)),
		derCtx(2, derInt(int64(KRB5_TGS_REQ))),
		derCtx(3, derSeq(pa)),
		derCtx(4, body),
	))
	return req.encode(), nil
}

// MarshalAuthenticator encodes an RFC 4120 Authenticator
// (authenticator-vno[0] crealm[1] cname[2] cksum[3] ctime[4] cusec[5]
// subkey[6] seq-number[7]).
func MarshalAuthenticator(a Authenticator) ([]byte, error) {
	// Field order per RFC 4120 §5.5.1: authenticator-vno[0] crealm[1]
	// cname[2] checksum[3] ctime[4] cusec[5] subkey[6] seq-number[7].
	nodes := []*tlvNode{
		derCtx(0, derInt(int64(a.AuthenticatorVNO))),
		derCtx(1, derGeneralString(string(a.Crealm))),
		derCtx(2, principalNameTLV(a.CName)),
	}
	// checksum[3] Checksum REQUIRED for TGS-REQ authenticators
	// (RFC 4120 §5.5.1): SEQUENCE { cksumtype[0], checksum[1] }.
	if len(a.Checksum) > 0 {
		nodes = append(nodes, derCtx(3, derSeq(
			derCtx(0, derInt(int64(a.ChecksumType))),
			derCtx(1, derOctets(a.Checksum)),
		)))
	}
	nodes = append(nodes,
		derCtx(4, derGeneralizedTime(a.CTime.Time)),
		derCtx(5, derInt(int64(a.CUSec))),
	)
	if a.Subkey != nil {
		nodes = append(nodes, derCtx(6, derSeq(
			derCtx(0, derInt(int64(a.Subkey.KeyType))),
			derCtx(1, derOctets(a.Subkey.KeyValue)),
		)))
	}
	if a.SeqNumber != nil {
		nodes = append(nodes, derCtx(7, derInt(int64(*a.SeqNumber))))
	}
	return derSeq(nodes...).encode(), nil
}

// BuildAPREQ encodes a complete AP-REQ ([APPLICATION 14]) carrying the given
// ticket and encrypted authenticator — the PA-TGS-REQ value for TGS-REQ.
func BuildAPREQ(ticket Ticket, encryptedAuthenticator []byte) []byte {
	// RFC 4120 §5.5.1: AP-REQ ::= [APPLICATION 14] SEQUENCE { pvno[0],
	// msg-type[1], ap-options[2], ticket[3] Ticket, authenticator[4]
	// EncryptedData }.
	//
	// Ticket is an [APPLICATION 1] CHOICE type, so explicit tagging NESTS:
	// the wire form of ticket[3] is [3] wrapping [APPLICATION 1] wrapping the
	// inner SEQUENCE ("a3 … 61 … 30 …"). Encoding only [3] → SEQUENCE
	// (dropping the 61 layer) made Samba's decoder reject the PA-DATA and
	// answer TGS-REQs with KDC_ERR_GENERIC (60) — mirrored by our own parser,
	// which needs descendToSequence to unwrap ticket[5] → APP 1 → SEQUENCE in
	// KDC replies.
	return derApp(14, derSeq(
		derCtx(0, derInt(krb5PVNO)),
		derCtx(1, derInt(int64(KRB5_AP_REQ))),
		derCtx(2, derBitString([]byte{0})), // ap-options: none
		derCtx(3, derApp(1, derSeq( // Ticket: [3] → [APPLICATION 1] → SEQUENCE
			derCtx(0, derInt(int64(ticket.TicketVNO))),
			derCtx(1, derGeneralString(string(ticket.Realm))),
			derCtx(2, principalNameTLV(ticket.SName)),
			derCtx(3, encryptedDataTLV(ticket.EncPart)),
		))),
		derCtx(4, encryptedDataTLV(EncryptedData{Cipher: encryptedAuthenticator})),
	)).encode()
}

// BuildAPREQBody encodes an AP-REQ body with an empty authenticator carrier
// (used only when no session key is available yet).
func BuildAPREQBody(ticket Ticket) []byte {
	return BuildAPREQ(ticket, nil)
}

// encryptedDataTLV encodes EncryptedData (etype[0] kvno[1] cipher[2]).
func encryptedDataTLV(ed EncryptedData) *tlvNode {
	nodes := []*tlvNode{derCtx(0, derInt(int64(ed.EType)))}
	if ed.KVNO > 0 {
		nodes = append(nodes, derCtx(1, derInt(int64(ed.KVNO))))
	}
	nodes = append(nodes, derCtx(2, derOctets(ed.Cipher)))
	return derSeq(nodes...)
}

// ParseTGSREP parses a TGS-REP ([APPLICATION 13] KDC-REP).
func ParseTGSREP(data []byte) (*ASREP, error) {
	// Classify KRB-ERROR replies instead of masking them as "invalid AS-REP".
	if code, ok := ParseKRBErrorCode(data); ok {
		return nil, NewKDCErrorWithDetail(code, data)
	}
	return parseKDCReply(data, 13, KRB5_TGS_REP)
}
