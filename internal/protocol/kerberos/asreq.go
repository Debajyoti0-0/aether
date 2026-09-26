package kerberos

import (
	"encoding/asn1"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidASREQ = errors.New("invalid AS-REQ")
	ErrInvalidASREP = errors.New("invalid AS-REP")
)

// ASREP is a parsed AS-REP/TGS-REP. KDC-REP field numbering restarts at [0]
// (RFC 4120 §5.4.2): pvno[0] msg-type[1] padata[2] crealm[3] cname[4]
// ticket[5] enc-part[6].
type ASREP struct {
	PVNO    int32
	MsgType int32
	PAData  []PAData
	CRealm  Realm
	CName   PrincipalName
	Ticket  *Ticket
	EncPart EncryptedData
}

// KrbSalt derives the RFC 4120 §4 default string-to-key salt:
// realm followed by the principal name components.
func KrbSalt(realm string, principal PrincipalName) string {
	var sb strings.Builder
	sb.WriteString(realm)
	for _, c := range principal.NameString {
		sb.WriteString(c)
	}
	return sb.String()
}

// BuildASREQ builds an RFC 4120 AS-REQ ([APPLICATION 10] EXPLICIT TAGS DER)
// with PA-ENC-TIMESTAMP pre-authentication material supplied by the caller.
func BuildASREQ(
	clientPrincipal PrincipalName,
	clientRealm Realm,
	serverPrincipal PrincipalName,
	etypes []int32,
	nonce int32,
	till time.Time,
	paData []PAData,
) ([]byte, error) {
	body, err := buildKDCReqBody(clientPrincipal, clientRealm, serverPrincipal, etypes, nonce, till)
	if err != nil {
		return nil, err
	}

	// RFC 4120 §5.4.1: AS-REQ ::= [APPLICATION 10] KDC-REQ and
	// KDC-REQ ::= SEQUENCE { pvno[1], msg-type[2], padata[3] OPTIONAL,
	// req-body[4] }. The APPLICATION tag wraps a UNIVERSAL SEQUENCE; the
	// previous builder put the fields directly under the app tag, which
	// Samba/AD DER parsers reject by closing the connection (live Stage 46g
	// wire evidence: MIT capture "6a 81 bc 30 81 b9 ..." vs Aether
	// "6a 7d a1 03 ..." — missing the 30 SEQUENCE layer).
	fields := []*tlvNode{
		derCtx(1, derInt(krb5PVNO)),           // pvno [1]
		derCtx(2, derInt(int64(KRB5_AS_REQ))), // msg-type [2]
	}
	if len(paData) > 0 {
		var pads []*tlvNode
		for _, pa := range paData {
			pads = append(pads, derSeq(
				derCtx(1, derInt(int64(pa.PADataType))),
				derCtx(2, derOctets(pa.PADataValue)),
			))
		}
		fields = append(fields, derCtx(3, derSeq(pads...))) // padata [3]
	}
	fields = append(fields, derCtx(4, body)) // req-body [4]
	req := derApp(10, derSeq(fields...))
	return req.encode(), nil
}

// buildKDCReqBody encodes KDC-REQ-BODY in schema field order:
// [0] kdc-options, [1] cname, [2] realm, [3] sname, [5] till, [7] nonce, [8] etype.
func buildKDCReqBody(
	clientPrincipal PrincipalName,
	clientRealm Realm,
	serverPrincipal PrincipalName,
	etypes []int32,
	nonce int32,
	till time.Time,
) (*tlvNode, error) {
	if len(etypes) == 0 {
		return nil, ErrInvalidASREQ
	}
	// MIT krb5's default request: forwardable, and renewable for a further
	// period. Both bits are now at their RFC 4120 §5.4.1 positions.
	opts := uint32(KDC_OPT_FORWARDABLE | KDC_OPT_RENEWABLE)
	if clientPrincipal.NameType == NAME_TYPE_SRV_INST {
		opts |= KDC_OPT_CANONICALIZE
	}
	fields := []*tlvNode{
		derCtx(0, derBitString(kdcOptionsBytes(opts))),        // kdc-options [0] BIT STRING
		derCtx(1, principalNameTLV(clientPrincipal)),          // cname [1]
		derCtx(2, derGeneralString(string(clientRealm))),      // realm [2] GeneralString
		derCtx(3, principalNameTLV(serverPrincipal)),          // sname [3]
		derCtx(5, derGeneralizedTime(till)),                   // till [5] KerberosTime
		// RFC 4120 §5.4.2: rtime[6] is REQUIRED when RENEWABLE is requested
		// in kdc-options. Omitting it made Samba answer KDC_ERR_BADOPTION.
		derCtx(6, derGeneralizedTime(till.Add(renewLifetime))), // rtime [6] KerberosTime
		derCtx(7, derInt(int64(nonce))),                        // nonce [7]
		etypeSeqTLV(etypes),                                    // etype [8] SEQUENCE OF Int32
	}
	return derSeq(fields...), nil
}

// renewLifetime is the extra window requested for ticket renewal.
const renewLifetime = 7 * 24 * time.Hour

const krb5PVNO = 5

// buildPAEncTimestampPlaintext encodes the PA-ENC-TIMESTAMP inner SEQUENCE:
//
//	SEQUENCE { patimestamp[0] KerberosTime, pausec[1] Int32 OPTIONAL }
func buildPAEncTimestampPlaintext(now time.Time) []byte {
	now = now.UTC()
	return derSeq(
		derCtx(0, derGeneralizedTime(now)),
		derCtx(1, derInt(int64(now.Nanosecond()/1000))),
	).encode()
}

// buildPAEncTimestampWithKey wraps the encrypted timestamp into an
// EncryptedData SEQUENCE as required by RFC 4120 PA-ENC-TIMESTAMP.
func buildPAEncTimestampWithKey(etype int32, key []byte, now time.Time) (PAData, error) {
	if len(key) == 0 {
		return PAData{}, ErrWrongKey
	}
	plain := buildPAEncTimestampPlaintext(now)
	ct, err := Encrypt(etype, key, KeyUsage_AS_REQ_PA_ENC_TIMESTAMP, plain)
	if err != nil {
		return PAData{}, fmt.Errorf("encrypt timestamp: %w", err)
	}
	encData := derSeq(
		derCtx(0, derInt(int64(etype))),   // etype [0]
		derCtx(2, derOctets(ct)),          // cipher [2]
	).encode()
	return PAData{
		PADataType:  PA_DATA_TYPE_ENC_TIMESTAMP,
		PADataValue: encData,
	}, nil
}

// ---------------------------------------------------------------------------
// Response parsing: AS-REP ([APPLICATION 11] KDC-REP), KRB-ERROR
// ---------------------------------------------------------------------------

// ParseKRBError parses a KRB-ERROR ([APPLICATION 30] SEQUENCE) and returns
// the error-code (field [6]). KRB-ERROR fields: pvno[0] msg-type[1] ctime[2]
// cusec[3] susec[4] error-code[6] crealm[7] cname[8] realm[9] sname[10]
// e-data[12]. The previous code had no KRB-ERROR parser at all, so every
// KDC rejection surfaced as the misleading "parse AS-REP: invalid AS-REP".
func ParseKRBError(data []byte) (*KRBError, error) {
	top, _, err := parseTLV(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedASN1, err)
	}
	if top.class != derClassApplication || top.tag != 30 || !top.constructed {
		return nil, ErrInvalidASREP
	}
	// KRB-ERROR ::= [APPLICATION 30] KRB-ERROR-BODY (a SEQUENCE) — unwrap it.
	// parseTLV returns the low-5-bit tag: SEQUENCE (0x30) parses as 0x10.
	seq, _, err := parseTLV(top.value)
	if err != nil || seq.class != derClassUniversal || seq.tag != (tagSequence&0x1f) {
		return nil, fmt.Errorf("%w: KRB-ERROR body is not a SEQUENCE", ErrMalformedASN1)
	}
	kids, err := parseChildren(seq.value)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedASN1, err)
	}
	e := &KRBError{}
	if r := findCtx(kids, 6); r != nil {
		if v, derr := decodeInt(*r); derr == nil {
			e.ErrorCode = int32(v)
		}
	}
	if r := findCtx(kids, 7); r != nil {
		if s, derr := decodeKerberosString(*r); derr == nil {
			e.Crealm = Realm(s)
		}
	}
	if r := findCtx(kids, 10); r != nil {
		if pn, derr := parsePrincipalNameTLV(*r); derr == nil {
			e.SName = pn
		}
	}
	// RFC 4120 §3.2.3: KRB-ERROR has NO padata field. For
	// KDC_ERR_PREAUTH_REQUIRED the KDC carries its pre-authentication hints in
	// the e-data [12] OCTET STRING, as a PA-DATA SEQUENCE containing
	// ETYPE-INFO2. (Field [11] is e-text, a KerberosString — reading it as
	// padata is why the KDC salt was never found and every non-canonical
	// account fell back to a locally guessed salt.)
	if r := findCtx(kids, 12); r != nil {
		e.EData = r.value
		if e.ErrorCode == KDC_ERR_PREAUTH_REQUIRED {
			if pads, perr := parsePADataTLV(e.EData); perr == nil {
				e.PAData = pads
			}
		}
	}
	return e, nil
}

// parsePADataTLV parses a PA-DATA SEQUENCE OF entries:
//
//	PA-DATA ::= SEQUENCE { padata-type [1] Int32, padata-value [2] OCTET STRING }
func parsePADataTLV(data []byte) ([]PAData, error) {
	top, _, err := parseTLV(data)
	if err != nil {
		return nil, err
	}
	// KRB-ERROR e-data is `[12] OCTET STRING`, and the field is EXPLICIT
	// tagged, so what arrives here is usually an OCTET STRING TLV wrapping the
	// PA-DATA SEQUENCE. Peel any OCTET STRING layer before looking for the
	// SEQUENCE. Live Samba evidence (Stage 46h, error-code 25 e-data):
	//   04 5b 30 59 30 09 a1 03 02 01 02 ...  a2 06 04 04 00 00 10 00
	// i.e. OCTET STRING(91) { SEQUENCE(89) { PA-DATA(type=19, len=45) {
	// etype=18, salt="AETHER.TESTAdministrator", s2kparams=00 00 10 00 } } }
	if top.class == derClassUniversal && top.tag == tagOctetString&0x1f {
		inner, _, ierr := parseTLV(top.value)
		if ierr != nil {
			return nil, ierr
		}
		top = inner
	}
	seq := unwrapCtx(top)
	if seq.class != derClassUniversal || seq.tag != (tagSequence&0x1f) {
		return nil, ErrMalformedASN1
	}
	kids, err := parseChildren(seq.value)
	if err != nil {
		return nil, err
	}
	out := make([]PAData, 0, len(kids))
	for _, k := range kids {
		entryKids, derr := parseChildren(k.value)
		if derr != nil {
			return nil, derr
		}
		var pa PAData
		if x := findCtx(entryKids, 1); x != nil {
			if v, ierr := decodeInt(*x); ierr == nil {
				pa.PADataType = int32(v)
			}
		}
		if x := findCtx(entryKids, 2); x != nil {
			pa.PADataValue = unwrapCtx(*x).value
		}
		out = append(out, pa)
	}
	return out, nil
}

// PreAuthHint is the KDC-supplied pre-authentication parameter set a client
// needs to derive a long-term key: which etype the KDC wants, and the
// authoritative salt and s2kparams for that etype.
type PreAuthHint struct {
	Etype     int32
	Salt      string
	S2KParams string
	HasSalt   bool
}

// ParsePreAuthHint extracts ETYPE-INFO2 (preferred) or ETYPE-INFO from a
// KDC_ERR_PREAUTH_REQUIRED reply and selects the strongest etype the client
// supports. RFC 4120 §3.2.3: the salt MUST come from the KDC, never from a
// locally computed default — Active Directory uses the account name exactly as
// stored in the directory, so "administrator" and "Administrator" have
// different salts and only the KDC knows which is right.
func ParsePreAuthHint(data []byte) (*PreAuthHint, error) {
	e, err := ParseKRBError(data)
	if err != nil {
		return nil, err
	}
	if e.ErrorCode != KDC_ERR_PREAUTH_REQUIRED {
		return nil, NewKDCErrorWithDetail(e.ErrorCode, data)
	}

	// ETYPE-INFO2 is SEQUENCE OF ETYPE-INFO2-ENTRY; prefer it over ETYPE-INFO.
	for _, pa := range e.PAData {
		if pa.PADataType != PA_DATA_TYPE_ETYPE_INFO2 {
			continue
		}
		infos, ierr := ParseEtypeInfo2(pa.PADataValue)
		if ierr == nil && len(infos) > 0 {
			return selectHint(len(infos), func(i int) (int32, string, string) {
				return infos[i].Etype, infos[i].Salt, infos[i].S2KParams
			}), nil
		}
	}
	for _, pa := range e.PAData {
		if pa.PADataType != PA_DATA_TYPE_ETYPE_INFO {
			continue
		}
		infos, ierr := ParseEtypeInfo(pa.PADataValue)
		if ierr == nil && len(infos) > 0 {
			return selectHint(len(infos), func(i int) (int32, string, string) {
				return infos[i].Etype, infos[i].Salt, infos[i].S2KParams
			}), nil
		}
	}

	// Some KDCs omit ETYPE-INFO entirely. Fall back to the RFC 4120 §4
	// default salt, but uppercase the realm: Kerberos realms are canonically
	// upper case and AD derives the salt from the realm as the KDC stores it.
	return &PreAuthHint{
		Etype: ETYPE_AES256_CTS_HMAC_SHA1_96,
		Salt:  "",
	}, nil
}

// selectHint picks the strongest client-supported etype from the KDC's offer.
func selectHint(n int, get func(int) (int32, string, string)) *PreAuthHint {
	best := (*PreAuthHint)(nil)
	bestRank := -1
	for i := 0; i < n; i++ {
		etype, salt, params := get(i)
		if !IsSupported(etype) {
			continue
		}
		rank := 0
		switch etype {
		case ETYPE_AES256_CTS_HMAC_SHA1_96:
			rank = 3
		case ETYPE_AES128_CTS_HMAC_SHA1_96:
			rank = 2
		case ETYPE_RC4_HMAC:
			rank = 1
		}
		if rank > bestRank {
			bestRank = rank
			best = &PreAuthHint{Etype: etype, Salt: salt, S2KParams: params, HasSalt: salt != ""}
		}
	}
	if best == nil {
		// KDC offered only etypes we do not support.
		return &PreAuthHint{Etype: ETYPE_AES256_CTS_HMAC_SHA1_96}
	}
	return best
}

// ParseKRBErrorCode extracts just the error-code from a KRB-ERROR reply.
// Returns (code, true) if data is a KRB-ERROR, (0, false) otherwise.
func ParseKRBErrorCode(data []byte) (int32, bool) {
	if len(data) == 0 || data[0] != 0x7e {
		return 0, false
	}
	e, err := ParseKRBError(data)
	if err != nil {
		return 0, false
	}
	return e.ErrorCode, true
}

// KDCErrorName returns the human name for a KDC error code.
func KDCErrorName(code int32) string {
	if msg, ok := kdcErrorMessages[code]; ok {
		return msg
	}
	return "Unknown"
}

func ParseASREP(data []byte) (*ASREP, error) {
	// A KRB-ERROR reply is not an AS-REP: parse and re-raise as a classified
	// KDC error so callers see "KDC error 25: ..." instead of a bare
	// "invalid AS-REP".
	if code, ok := ParseKRBErrorCode(data); ok {
		return nil, NewKDCErrorWithDetail(code, data)
	}
	return parseKDCReply(data, 11, KRB5_AS_REP)
}

// parseKDCReply parses AS-REP/TGS-REP. KDC-REP field numbering restarts at [0]
// (RFC 4120 §5.4.2): pvno[0] msg-type[1] padata[2] crealm[3] cname[4]
// ticket[5] enc-part[6].
func parseKDCReply(data []byte, appTag byte, wantMsgType int32) (*ASREP, error) {
	top, _, err := parseTLV(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedASN1, err)
	}
	if top.class != derClassApplication || top.tag != appTag || !top.constructed {
		return nil, ErrInvalidASREP
	}
	// RFC 4120 §5.4.2: AS-REP ::= [APPLICATION 11] KDC-REP where KDC-REP is
	// a SEQUENCE. Unwrap that SEQUENCE before reading fields — real KDC
	// replies carry "6b ... 30 ... a0 pvno ..." and parsing the app contents
	// as the field stream found no msg-type at all ("invalid AS-REP" on a
	// perfectly valid reply). Note parseTLV returns the low-5-bit tag, so
	// SEQUENCE (0x30) parses as tag 0x10.
	seq, _, err := parseTLV(top.value)
	if err != nil || seq.class != derClassUniversal || seq.tag != (tagSequence&0x1f) {
		return nil, fmt.Errorf("%w: KDC-REP body is not a SEQUENCE", ErrMalformedASN1)
	}
	kids, err := parseChildren(seq.value)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedASN1, err)
	}
	rep := &ASREP{}
	if r := findCtx(kids, 0); r != nil {
		if v, derr := decodeInt(*r); derr == nil {
			rep.PVNO = int32(v)
		}
	}
	if r := findCtx(kids, 1); r != nil {
		if v, derr := decodeInt(*r); derr == nil {
			rep.MsgType = int32(v)
		}
	}
	if rep.MsgType != wantMsgType {
		return nil, ErrInvalidASREP
	}
	if r := findCtx(kids, 2); r != nil {
		rep.PAData, err = parsePADataSeq(r.value)
		if err != nil {
			return nil, fmt.Errorf("%w: padata: %v", ErrMalformedASN1, err)
		}
	}
	if r := findCtx(kids, 3); r != nil {
		s, derr := decodeKerberosString(*r)
		if derr != nil {
			return nil, fmt.Errorf("%w: crealm: %v", ErrMalformedASN1, derr)
		}
		rep.CRealm = Realm(s)
	}
	if r := findCtx(kids, 4); r != nil {
		pn, derr := parsePrincipalNameTLV(*r)
		if derr != nil {
			return nil, fmt.Errorf("%w: cname: %v", ErrMalformedASN1, derr)
		}
		rep.CName = pn
	}
	if r := findCtx(kids, 5); r != nil {
		t, derr := parseTicketTLV(*r)
		if derr != nil {
			return nil, fmt.Errorf("%w: ticket: %v", ErrMalformedASN1, derr)
		}
		// Preserve the complete KerberosTicket ([APPLICATION 1] TLV included)
		// as the KDC sent it. A credential cache stores that encoding in its
		// ticket field, so without it the only bytes available are the
		// encrypted enc-part, which is a different structure entirely.
		if inner, _, ierr := parseTLV(r.value); ierr == nil {
			t.Raw = inner.full
		} else {
			t.Raw = r.full
		}
		rep.Ticket = &t
	}
	if r := findCtx(kids, 6); r != nil {
		ed, derr := parseEncryptedDataTLV(*r)
		if derr != nil {
			return nil, fmt.Errorf("%w: enc-part: %v", ErrMalformedASN1, derr)
		}
		rep.EncPart = ed
	}
	return rep, nil
}

// parsePADataSeq parses the content of a padata SEQUENCE OF PA-DATA.
func parsePADataSeq(content []byte) ([]PAData, error) {
	items, err := parseChildren(content)
	if err != nil {
		return nil, err
	}
	var out []PAData
	for _, item := range items {
		kids, err := parseChildren(item.value)
		if err != nil {
			return nil, err
		}
		var pa PAData
		if r := findCtx(kids, 1); r != nil {
			if v, derr := decodeInt(*r); derr == nil {
				pa.PADataType = int32(v)
			}
		}
		if r := findCtx(kids, 2); r != nil {
			pa.PADataValue = r.value
		}
		out = append(out, pa)
	}
	return out, nil
}

// parsePrincipalNameTLV parses a PrincipalName (name-type[0], name-string[1]).
// cname/sname fields EXPLICIT-wrap the PrincipalName SEQUENCE — descend first
// (the previous code parsed the wrapper content as the field stream and
// silently produced empty principal names).
func parsePrincipalNameTLV(r tlvRaw) (PrincipalName, error) {
	var pn PrincipalName
	seq, err := descendToSequence(r)
	if err != nil {
		return pn, err
	}
	kids, err := parseChildren(seq.value)
	if err != nil {
		return pn, err
	}
	if t := findCtx(kids, 0); t != nil {
		if v, derr := decodeInt(*t); derr == nil {
			pn.NameType = int32(v)
		}
	}
	if t := findCtx(kids, 1); t != nil {
		// name-string[1] wraps a SEQUENCE OF KerberosString: the child of the
		// [1] wrapper is the SEQUENCE itself — descend, then iterate elements.
		seqInner, err := descendToSequence(*t)
		if err != nil {
			return pn, err
		}
		inner, err := parseChildren(seqInner.value)
		if err != nil {
			return pn, err
		}
		for _, it := range inner {
			s, derr := decodeKerberosString(it)
			if derr != nil {
				return pn, derr
			}
			pn.NameString = append(pn.NameString, s)
		}
	}
	return pn, nil
}

// ParseTicketFromCCache re-parses a raw Ticket TLV stored in a ccache entry
// (ccaches keep the complete DER-encoded Ticket, [APPLICATION 1] included).
func ParseTicketFromCCache(raw []byte) (Ticket, error) {
	top, _, err := parseTLV(raw)
	if err != nil {
		return Ticket{}, err
	}
	return parseTicketTLV(top)
}

// parseTicketTLV parses a Ticket (tkt-vno[0] realm[1] sname[2] enc-part[3]).
// ticket[5] in KDC-REP is EXPLICIT-wrapped: [5] → [APPLICATION 1] → SEQUENCE.
func parseTicketTLV(r tlvRaw) (Ticket, error) {
	var t Ticket
	seq, err := descendToSequence(r)
	if err != nil {
		return t, err
	}
	kids, err := parseChildren(seq.value)
	if err != nil {
		return t, err
	}
	if x := findCtx(kids, 0); x != nil {
		if v, derr := decodeInt(*x); derr == nil {
			t.TicketVNO = int32(v)
		}
	}
	if x := findCtx(kids, 1); x != nil {
		s, derr := decodeKerberosString(*x)
		if derr != nil {
			return t, derr
		}
		t.Realm = Realm(s)
	}
	if x := findCtx(kids, 2); x != nil {
		pn, derr := parsePrincipalNameTLV(*x)
		if derr != nil {
			return t, derr
		}
		t.SName = pn
	}
	if x := findCtx(kids, 3); x != nil {
		ed, derr := parseEncryptedDataTLV(*x)
		if derr != nil {
			return t, derr
		}
		t.EncPart = ed
	}
	return t, nil
}

// parseEncryptedDataTLV parses EncryptedData (etype[0] kvno[1] cipher[2]).
// Strips EXPLICIT context wrappers (cipher[2] wraps an OCTET STRING).
func parseEncryptedDataTLV(r tlvRaw) (EncryptedData, error) {
	var ed EncryptedData
	seq, err := descendToSequence(r)
	if err != nil {
		return ed, err
	}
	kids, err := parseChildren(seq.value)
	if err != nil {
		return ed, err
	}
	if x := findCtx(kids, 0); x != nil {
		if v, derr := decodeInt(*x); derr == nil {
			ed.EType = int32(v)
		}
	}
	if x := findCtx(kids, 1); x != nil {
		if v, derr := decodeInt(*x); derr == nil {
			ed.KVNO = int32(v)
		}
	}
	if x := findCtx(kids, 2); x != nil {
		inner := unwrapCtx(*x)
		ed.Cipher = inner.value
	}
	return ed, nil
}

// BuildASREQWithPreauth builds an AS-REQ with PA-ENC-TIMESTAMP derived from
// the client password (AES-256 string-to-key with the RFC 4120 salt).
//
// Deprecated: this fallback derives the salt locally and therefore only works
// when the KDC happens to use the RFC 4120 §4 default salt with the exact
// account-name casing. Prefer BuildASREQWithHint, which takes the salt and
// etype the KDC itself supplied.
func BuildASREQWithPreauth(
	clientPrincipal PrincipalName,
	clientRealm Realm,
	serverPrincipal PrincipalName,
	etypes []int32,
	nonce int32,
	till time.Time,
	password string,
) ([]byte, error) {
	salt := KrbSalt(string(clientRealm), clientPrincipal)
	key, err := StringToKey(ETYPE_AES256_CTS_HMAC_SHA1_96, password, salt, "")
	if err != nil {
		return nil, fmt.Errorf("key derivation: %w", err)
	}
	pa, err := buildPAEncTimestampWithKey(ETYPE_AES256_CTS_HMAC_SHA1_96, key, time.Now())
	if err != nil {
		return nil, err
	}
	return BuildASREQ(clientPrincipal, clientRealm, serverPrincipal, etypes, nonce, till, []PAData{pa})
}

// BuildASREQWithHint builds the pre-authenticated AS-REQ using the etype, salt
// and s2kparams the KDC advertised in ETYPE-INFO2 (RFC 4120 §3.2.3). This is
// the correct path: the salt is the KDC's, so account-name casing and
// non-default AD salt formats are handled without guessing.
func BuildASREQWithHint(
	clientPrincipal PrincipalName,
	clientRealm Realm,
	serverPrincipal PrincipalName,
	etypes []int32,
	nonce int32,
	till time.Time,
	password string,
	hint *PreAuthHint,
) ([]byte, error) {
	etype := int32(ETYPE_AES256_CTS_HMAC_SHA1_96)
	salt, params := "", ""
	if hint != nil {
		if hint.Etype != 0 {
			etype = hint.Etype
		}
		if hint.HasSalt {
			salt = hint.Salt
			params = hint.S2KParams
		}
	}
	if salt == "" {
		salt = KrbSalt(CanonicalRealm(clientRealm), clientPrincipal)
	}
	key, err := StringToKey(etype, password, salt, params)
	if err != nil {
		return nil, fmt.Errorf("key derivation: %w", err)
	}
	pa, err := buildPAEncTimestampWithKey(etype, key, time.Now())
	if err != nil {
		return nil, err
	}
	// Offer the etype actually used for pre-auth first so the KDC cannot
	// choose one we did not derive a key for.
	offered := make([]int32, 0, len(etypes)+1)
	offered = append(offered, etype)
	for _, e := range etypes {
		if e != etype {
			offered = append(offered, e)
		}
	}
	return BuildASREQ(clientPrincipal, clientRealm, serverPrincipal, offered, nonce, till, []PAData{pa})
}

// CanonicalRealm upper-cases a Kerberos realm. Realm names are canonically
// upper case (RFC 4120 §5.2.1 realm is case-sensitive but every KDC stores it
// upper case), and the AD default string-to-key salt is built from the realm
// exactly as the KDC holds it.
func CanonicalRealm(r Realm) string {
	return strings.ToUpper(string(r))
}

// DecryptASREPEncPart decrypts and parses the AS-REP enc-part
// (EncASRepPart ::= EncKDCRepPart: key[0], ... flags[4], authtime[5],
// starttime[6], endtime[7], renew-till[8]).
func DecryptASREPEncPart(rep *ASREP, key []byte, etype int32) (*EncTicketPart, error) {
	plaintext, err := Decrypt(etype, key, KeyUsage_AS_REP_ENCPART, rep.EncPart.Cipher)
	if err != nil {
		return nil, err
	}
	return parseEncKDCRepPart(plaintext)
}

// parseEncKDCRepPart parses EncKDCRepPart/EncASRepPart/EncTGSRepPart plaintext.
// The plaintext is [APPLICATION 25]/[APPLICATION 26] wrapping the SEQUENCE —
// descend to the SEQUENCE before reading fields (the previous code parsed the
// app-wrapper content as the field stream and silently returned empty
// key/flags/times, producing an unusable ccache).
func parseEncKDCRepPart(plaintext []byte) (*EncTicketPart, error) {
	top, _, err := parseTLV(plaintext)
	if err != nil {
		return nil, fmt.Errorf("%w: enc-part: %v", ErrMalformedASN1, err)
	}
	seq, err := descendToSequence(top)
	if err != nil {
		return nil, fmt.Errorf("%w: enc-part: %v", ErrMalformedASN1, err)
	}
	kids, err := parseChildren(seq.value)
	if err != nil {
		return nil, fmt.Errorf("%w: enc-part: %v", ErrMalformedASN1, err)
	}
	out := &EncTicketPart{}
	if x := findCtx(kids, 0); x != nil { // key[0] EncryptionKey
		ek, derr := parseEncryptionKeyTLV(*x)
		if derr != nil {
			return nil, fmt.Errorf("%w: key: %v", ErrMalformedASN1, derr)
		}
		out.Key = ek
	}
	if x := findCtx(kids, 4); x != nil { // flags[4] TicketFlags BIT STRING
		if bs, derr := parseBitStringTLV(*x); derr == nil {
			out.Flags = bs
		}
	}
	if x := findCtx(kids, 5); x != nil { // authtime[5]
		if t, derr := decodeTime(*x); derr == nil {
			out.Authtime = KerberosTime{Time: t, IsSet: true}
		}
	}
	if x := findCtx(kids, 6); x != nil { // starttime[6] OPTIONAL
		if t, derr := decodeTime(*x); derr == nil {
			out.Starttime = KerberosTime{Time: t, IsSet: true}
		}
	}
	if x := findCtx(kids, 7); x != nil { // endtime[7]
		if t, derr := decodeTime(*x); derr == nil {
			out.Endtime = KerberosTime{Time: t, IsSet: true}
		}
	}
	if x := findCtx(kids, 8); x != nil { // renew-till[8] OPTIONAL
		if t, derr := decodeTime(*x); derr == nil {
			out.RenewTill = KerberosTime{Time: t, IsSet: true}
		}
	}
	if x := findCtx(kids, 9); x != nil { // srealm[9]
		if s, derr := decodeKerberosString(*x); derr == nil {
			out.Crealm = Realm(s)
		}
	}
	if x := findCtx(kids, 10); x != nil { // sname[10]
		if pn, derr := parsePrincipalNameTLV(*x); derr == nil {
			out.CName = pn
		}
	}
	return out, nil
}

// parseBitStringTLV decodes a BIT STRING content into an asn1.BitString.
// Strips EXPLICIT context wrappers first (RFC 4120 EXPLICIT TAGS).
func parseBitStringTLV(r tlvRaw) (asn1.BitString, error) {
	r = unwrapCtx(r)
	if r.tag != (tagBitString&0x1f) || r.constructed || len(r.value) < 1 {
		return asn1.BitString{}, ErrInvalidTag
	}
	unused := uint(r.value[0])
	if unused > 7 {
		return asn1.BitString{}, ErrInvalidDERLength
	}
	data := r.value[1:]
	if len(data) == 0 && unused > 0 {
		return asn1.BitString{}, ErrInvalidDERLength
	}
	return asn1.BitString{Bytes: data, BitLength: len(data)*8 - int(unused)}, nil
}

// parseEncryptionKeyTLV parses EncryptionKey (keytype[0], key-value[1]).
// key[0] EXPLICIT-wraps the EncryptionKey SEQUENCE and key-value[1]
// EXPLICIT-wraps an OCTET STRING — descend/unwrap before reading.
func parseEncryptionKeyTLV(r tlvRaw) (EncryptionKey, error) {
	var ek EncryptionKey
	seq, err := descendToSequence(r)
	if err != nil {
		return ek, err
	}
	kids, err := parseChildren(seq.value)
	if err != nil {
		return ek, err
	}
	if x := findCtx(kids, 0); x != nil {
		if v, derr := decodeInt(*x); derr == nil {
			ek.KeyType = int32(v)
		}
	}
	if x := findCtx(kids, 1); x != nil {
		inner := unwrapCtx(*x)
		ek.KeyValue = inner.value
	}
	return ek, nil
}

// BuildASREQWithKeytab builds an AS-REQ with PA-ENC-TIMESTAMP encrypted under
// an existing long-term key (e.g. from a keytab).
func BuildASREQWithKeytab(
	clientPrincipal PrincipalName,
	clientRealm Realm,
	serverPrincipal PrincipalName,
	etypes []int32,
	nonce int32,
	till time.Time,
	key []byte,
) ([]byte, error) {
	pa, err := buildPAEncTimestampWithKey(ETYPE_AES256_CTS_HMAC_SHA1_96, key, time.Now())
	if err != nil {
		return nil, err
	}
	return BuildASREQ(clientPrincipal, clientRealm, serverPrincipal, etypes, nonce, till, []PAData{pa})
}
