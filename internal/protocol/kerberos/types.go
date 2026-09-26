package kerberos

import (
	"encoding/asn1"
	"strings"
	"time"
)

const (
	KRB5_AS_REQ  = 10
	KRB5_AS_REP  = 11
	KRB5_TGS_REQ = 12
	KRB5_TGS_REP = 13
	KRB5_AP_REQ  = 14
	KRB5_AP_REP  = 15
	KRB5_ERROR   = 30
)

const (
	PA_DATA_TYPE_NONE           = 0
	PA_DATA_TYPE_ENC_TIMESTAMP  = 2
	PA_DATA_TYPE_PW_SALT        = 3
	PA_DATA_TYPE_ETYPE_INFO     = 11
	PA_DATA_TYPE_ETYPE_INFO2    = 19
	PA_DATA_TYPE_PK_AS_REQ      = 16
	PA_DATA_TYPE_PK_AS_REP      = 17
	PA_DATA_TYPE_PA_PAC_REQUEST = 128
)

const (
	ETYPE_DES_CBC_CRC             = 1
	ETYPE_DES_CBC_MD4             = 2
	ETYPE_DES_CBC_MD5             = 3
	ETYPE_AES128_CTS_HMAC_SHA1_96 = 17
	ETYPE_AES256_CTS_HMAC_SHA1_96 = 18
	ETYPE_RC4_HMAC                = 23
	ETYPE_RC4_HMAC_EXP            = 24
)

const (
	NAME_TYPE_UNKNOWN        = 0
	NAME_TYPE_PRINCIPAL      = 1
	NAME_TYPE_SRV_INST       = 2
	NAME_TYPE_SRV_HST        = 3
	NAME_TYPE_SRV_XHST       = 4
	NAME_TYPE_UID            = 5
	NAME_TYPE_X500_PRINCIPAL = 6
	NAME_TYPE_SMTP_NAME      = 7
	NAME_TYPE_ENTERPRISE     = 10
)

const (
	// RFC 4120 §5.4.1 KDCOptions is a 32-bit BIT STRING whose FIRST octet
	// carries bits 0-7 with bit 0 (reserved) as the most significant bit.
	// Aether's kdcOptionsBytes maps flag bit N to BIT STRING bit N, so these
	// constants must be the numeric masks MIT krb5 uses.
	//
	// The previous values were off by one and internally inconsistent: they
	// requested the RESERVED bit 31 for "forwardable" and put "renewable"
	// in undefined bits 24-25. Samba rejects an AS-REQ carrying undefined
	// option bits with KDC_ERR_BADOPTION (13), which is what Stage 46h
	// observed after the salt defect was fixed.
	KDC_OPT_RESERVED                = 0
	KDC_OPT_FORWARDABLE             = 1 << 30 // 0x40000000
	KDC_OPT_FORWARDED               = 1 << 29 // 0x20000000
	KDC_OPT_PROXIABLE               = 1 << 28 // 0x10000000
	KDC_OPT_PROXY                   = 1 << 27 // 0x08000000
	KDC_OPT_ALLOW_POSTDATE          = 1 << 26 // 0x04000000
	KDC_OPT_POSTDATED               = 1 << 25 // 0x02000000
	KDC_OPT_UNUSED7                 = 1 << 24 // 0x01000000
	KDC_OPT_RENEWABLE               = 1 << 23 // 0x00800000
	KDC_OPT_UNUSED9                 = 1 << 22
	KDC_OPT_UNUSED10                = 1 << 21
	KDC_OPT_ENC_TKT_IN_SKEY         = 1 << 20 // 0x00100000
	KDC_OPT_RENEW                   = 1 << 19 // 0x00080000
	KDC_OPT_VALIDATE                = 1 << 18 // 0x00040000
	KDC_OPT_UNUSED14                = 1 << 17
	KDC_OPT_UNUSED15                = 1 << 16
	KDC_OPT_CANONICALIZE            = 1 << 15 // 0x00008000
	KDC_OPT_REQUEST_ANONYMOUS       = 1 << 14 // 0x00004000
	KDC_OPT_DISABLE_TRANSITED_CHECK = 1 << 13 // 0x00000020
	KDC_OPT_RENEWABLE_OK            = 1 << 12 // 0x00000010
)

const (
	KRB_AP_REQ_OPT_USE_SESSION_KEY = 1 << 0
	KRB_AP_REQ_OPT_MUTUAL_REQUIRED = 1 << 1
)

const (
	AD_IF_RELEVANT       = 1
	AD_INTENDED_FOR      = 2
	AD_KDC_ISSUED        = 4
	AD_AND_OR            = 5
	AD_MANDATORY_FOR_KDC = 7
	AD_WINDOWS_PAC       = 128
)

var (
	OID_KRB5_PRINCIPAL_NAME = asn1.ObjectIdentifier{1, 2, 840, 113554, 1, 2, 2, 1}
)

//	PrincipalName ::= SEQUENCE {
//	    name-type[0] Int32,
//	    name-string[1] SEQUENCE OF KerberosString
//	}
//
// NameString holds the principal components only; the realm travels in the
// message realm field (RFC 4120 §5.2.2).
type PrincipalName struct {
	NameType   int32
	NameString []string
}

func (p PrincipalName) String() string {
	return strings.Join(p.NameString, "/")
}

// Realm ::= KerberosString ::= GeneralString (RFC 4120 §5.2.1).
type Realm string

func (r Realm) String() string {
	return string(r)
}

func RealmFromString(s string) Realm {
	return Realm(s)
}

func (r Realm) MarshalASN1() ([]byte, error) {
	s := string(r)
	inner := []byte{0x1b, byte(len(s))}
	inner = append(inner, []byte(s)...)
	outer := []byte{0xa2, byte(len(inner))}
	return append(outer, inner...), nil
}

type KerberosTime struct {
	Time  time.Time
	IsSet bool
}

func NewKerberosTime(t time.Time) KerberosTime {
	return KerberosTime{Time: t, IsSet: true}
}

func (kt KerberosTime) MarshalASN1() ([]byte, error) {
	if !kt.IsSet {
		return nil, nil
	}
	return []byte(kt.Time.UTC().Format("20060102150405Z")), nil
}

func (kt *KerberosTime) UnmarshalASN1(data []byte) error {
	if len(data) == 0 {
		kt.IsSet = false
		return nil
	}
	s := string(data)
	t, err := time.Parse("20060102150405Z", s)
	if err != nil {
		return err
	}
	kt.Time = t
	kt.IsSet = true
	return nil
}

type EncryptionKey struct {
	KeyType  int32
	KeyValue []byte
}

type EncryptedData struct {
	EType  int32
	KVNO   int32
	Cipher []byte
}

type Checksum struct {
	CksumType int32
	Checksum  []byte
}

type AuthorizationData struct {
	AdType int32
	AdData []byte
}

type PAData struct {
	PADataType  int32
	PADataValue []byte
}

type Ticket struct {
	TicketVNO int32
	Realm     Realm
	SName     PrincipalName
	EncPart   EncryptedData
	// Raw is the complete DER KerberosTicket exactly as the KDC sent it, with
	// the [APPLICATION 1] tag included. A credential cache stores this
	// encoding in its ticket field; the decrypted enc-part is a different
	// structure and must never be written in its place.
	Raw []byte
}

type Authenticator struct {
	AuthenticatorVNO  int32
	CName             PrincipalName
	Crealm            Realm
	CTime             KerberosTime
	CUSec             int32
	ChecksumType      int32  // cksumtype for checksum[3]
	Checksum          []byte // RFC 4120 §5.5.1: REQUIRED in TGS-REQ authenticators
	Subkey            *EncryptionKey
	SeqNumber         *int32
	AuthorizationData []AuthorizationData
}

type TransitedEncoding struct {
	TrType   int32
	Contents []byte
}

// EncTicketPart is reused for AS-REP/TGS-REP enc-parts (EncKDCRepPart shares
// the key/flags/times layout consumed by our decrypt path).
type EncTicketPart struct {
	Flags             asn1.BitString
	Key               EncryptionKey
	Crealm            Realm
	CName             PrincipalName
	Transited         TransitedEncoding
	Authtime          KerberosTime
	Starttime         KerberosTime
	Endtime           KerberosTime
	RenewTill         KerberosTime
	CAddr             []HostAddress
	AuthorizationData []AuthorizationData
}

type HostAddress struct {
	AddrType int32
	Address  []byte
}

type LastReq struct {
	LrType  int32
	LrValue KerberosTime
}

type KRBError struct {
	PVNO      int32
	MsgType   int32
	CTime     KerberosTime
	CUSec     int32
	STime     KerberosTime
	SUSec     int32
	ErrorCode int32
	Crealm    Realm
	CName     PrincipalName
	Realm     Realm
	SName     PrincipalName
	EText     string
	EData     []byte
	PAData    []PAData
}

const (
	PAC_LOGON_INFO       = 1
	PAC_CREDENTIAL_TYPE  = 2
	PAC_SERVER_CHECKSUM  = 6
	PAC_PRIVSVR_CHECKSUM = 7
	PAC_CLIENT_INFO      = 10
	PAC_CLIENT_CLAIMS    = 11
	PAC_DEVICE_INFO      = 12
	PAC_DEVICE_CLAIMS    = 13
	PAC_UPN_DNS_INFO     = 14
)

type KDCREQ struct {
	PVNO    int32
	MsgType int32
	PAData  []PAData
	ReqBody KDCREQBody
}

type KDCREQBody struct {
	KDCOptions asn1.BitString
	CName      PrincipalName
	Realm      Realm
	SName      PrincipalName
	From       KerberosTime
	Till       KerberosTime
	Nonce      int32
	EType      []int32
}
