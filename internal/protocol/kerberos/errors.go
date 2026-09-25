package kerberos

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidPrincipal        = errors.New("invalid principal name")
	ErrInvalidRealm            = errors.New("invalid realm")
	ErrMalformedPacket         = errors.New("malformed Kerberos packet")
	ErrUnsupportedEncryption   = errors.New("unsupported encryption type")
	ErrPreauthRequired         = errors.New("pre-authentication required")
	ErrClockSkew               = errors.New("clock skew too great")
	ErrKDCUnavailable          = errors.New("KDC unavailable")
	ErrAuthenticationFailed    = errors.New("authentication failed")
	ErrAuthorizationDenied     = errors.New("authorization denied")
	ErrTicketInvalid           = errors.New("invalid ticket")
	ErrTicketExpired           = errors.New("ticket expired")
	ErrCCacheInvalid           = errors.New("invalid ccache")
	ErrEngagementExpired       = errors.New("engagement expired")
	ErrTargetOutsideBoundary   = errors.New("target outside engagement boundary")
	ErrCapabilityDenied        = errors.New("capability denied")
	ErrRateLimited             = errors.New("rate limit exceeded")
	ErrWrongPassword           = errors.New("wrong password")
	ErrWrongKey                = errors.New("wrong key")
	ErrFutureTicket            = errors.New("ticket not yet valid")
	ErrDNSFailure              = errors.New("DNS resolution failed")
	ErrConnectionRefused       = errors.New("connection refused")
	ErrUnsupportedEnctype      = errors.New("unsupported encryption type")
	ErrUnknownUser             = errors.New("unknown user")
	ErrMalformedASN1           = errors.New("malformed ASN.1")
	ErrTruncatedPacket         = errors.New("truncated packet")
	ErrInvalidDERLength        = errors.New("invalid DER length")
	ErrInvalidTag              = errors.New("invalid ASN.1 tag")
	ErrUnknownCriticalStruct   = errors.New("unknown critical structure")
	ErrInvalidEnumValue        = errors.New("invalid enum value")
	ErrOversizedField          = errors.New("oversized field")
	ErrMalformedCiphertext     = errors.New("malformed ciphertext")
	ErrInvalidPrincipalName    = errors.New("invalid principal name")
	ErrInvalidRealmName        = errors.New("invalid realm name")
)

const (
	KDC_ERR_NONE                  = 0
	KDC_ERR_NAME_EXP              = 1
	KDC_ERR_SERVICE_EXP           = 2
	KDC_ERR_BAD_PVNO              = 3
	KDC_ERR_C_OLD_MAST_KVNO       = 4
	KDC_ERR_S_OLD_MAST_KVNO       = 5
	KDC_ERR_C_PRINCIPAL_UNKNOWN   = 6
	KDC_ERR_S_PRINCIPAL_UNKNOWN   = 7
	KDC_ERR_PRINCIPAL_NOT_UNIQUE  = 8
	KDC_ERR_NULL_KEY              = 9
	KDC_ERR_CANNOT_POSTDATE       = 10
	KDC_ERR_NEVER_VALID           = 11
	KDC_ERR_POLICY                = 12
	KDC_ERR_BADOPTION             = 13
	KDC_ERR_ETYPE_NOSUPP          = 14
	KDC_ERR_SUMTYPE_NOSUPP        = 15
	KDC_ERR_PADATA_TYPE_NOSUPP    = 16
	KDC_ERR_TRTYPE_NOSUPP         = 17
	KDC_ERR_CLIENT_REVOKED        = 18
	KDC_ERR_SERVICE_REVOKED       = 19
	KDC_ERR_TGT_REVOKED           = 20
	KDC_ERR_CLIENT_NOTYET         = 21
	KDC_ERR_SERVICE_NOTYET        = 22
	KDC_ERR_KEY_EXP               = 23
	KDC_ERR_PREAUTH_FAILED        = 24
	KDC_ERR_PREAUTH_REQUIRED      = 25
	KDC_ERR_SERVER_NOMATCH        = 26
	KDC_ERR_MUST_USE_USER2USER    = 27
	KDC_ERR_PATH_NOT_ACCEPTED     = 28
	KDC_ERR_SVC_UNAVAILABLE       = 29
	KRB_AP_ERR_BAD_INTEGRITY      = 31
	KRB_AP_ERR_TKT_EXPIRED        = 32
	KRB_AP_ERR_TKT_NYV            = 33
	KRB_AP_ERR_REPEAT             = 34
	KRB_AP_ERR_NOT_US             = 35
	KRB_AP_ERR_BADMATCH           = 36
	KRB_AP_ERR_SKEW               = 37
	KRB_AP_ERR_BADADDR            = 38
	KRB_AP_ERR_BADVERSION         = 39
	KRB_AP_ERR_MSG_TYPE           = 40
	KRB_AP_ERR_MODIFIED           = 41
	KRB_AP_ERR_BADORDER           = 42
	KRB_AP_ERR_BADKEYVER          = 44
	KRB_AP_ERR_NOKEY              = 45
	KRB_AP_ERR_MUTEX              = 46
	KRB_AP_ERR_BADDIRECTION       = 47
	KRB_AP_ERR_METHOD             = 48
	KRB_AP_ERR_BADSEQ             = 49
	KRB_AP_ERR_INAPP_CKSUM        = 50
	KRB_AP_PATH_NOT_ACCEPTED      = 51
	KRB_ERR_RESPONSE_TOO_BIG      = 52
	KRB_ERR_GENERIC               = 60
	KRB_ERR_FIELD_TOOLONG         = 61
	KDC_ERROR_CLIENT_NOT_TRUSTED  = 62
	KDC_ERROR_KDC_NOT_TRUSTED     = 63
)

var kdcErrorMessages = map[int32]string{
	KDC_ERR_NONE:                  "No error",
	KDC_ERR_NAME_EXP:              "Client's entry in database has expired",
	KDC_ERR_SERVICE_EXP:           "Server's entry in database has expired",
	KDC_ERR_BAD_PVNO:              "Requested protocol version not supported",
	KDC_ERR_C_OLD_MAST_KVNO:       "Client's key encrypted in old master key",
	KDC_ERR_S_OLD_MAST_KVNO:       "Server's key encrypted in old master key",
	KDC_ERR_C_PRINCIPAL_UNKNOWN:   "Client not found in Kerberos database",
	KDC_ERR_S_PRINCIPAL_UNKNOWN:   "Server not found in Kerberos database",
	KDC_ERR_PRINCIPAL_NOT_UNIQUE:  "Multiple principal entries in database",
	KDC_ERR_NULL_KEY:              "Client or server has a null key",
	KDC_ERR_CANNOT_POSTDATE:       "Ticket not eligible for postdating",
	KDC_ERR_NEVER_VALID:           "Requested start time is later than end time",
	KDC_ERR_POLICY:                "KDC policy rejects request",
	KDC_ERR_BADOPTION:             "KDC can't fulfill requested option",
	KDC_ERR_ETYPE_NOSUPP:          "Unsupported encryption type",
	KDC_ERR_SUMTYPE_NOSUPP:        "Unsupported checksum type",
	KDC_ERR_PADATA_TYPE_NOSUPP:    "Unsupported pre-authentication type",
	KDC_ERR_TRTYPE_NOSUPP:         "Unsupported transited type",
	KDC_ERR_CLIENT_REVOKED:        "Client's credentials have been revoked",
	KDC_ERR_SERVICE_REVOKED:       "Server's credentials have been revoked",
	KDC_ERR_TGT_REVOKED:           "TGT has been revoked",
	KDC_ERR_CLIENT_NOTYET:         "Client not yet valid",
	KDC_ERR_SERVICE_NOTYET:        "Server not yet valid",
	KDC_ERR_KEY_EXP:               "Password has expired",
	KDC_ERR_PREAUTH_FAILED:        "Pre-authentication information was invalid",
	KDC_ERR_PREAUTH_REQUIRED:      "Additional pre-authentication required",
	KDC_ERR_SERVER_NOMATCH:        "Requested server and ticket don't match",
	KDC_ERR_MUST_USE_USER2USER:    "Server principal valid for user2user only",
	KDC_ERR_PATH_NOT_ACCEPTED:     "KDC policy rejects requested path",
	KDC_ERR_SVC_UNAVAILABLE:       "Service unavailable",
	KRB_AP_ERR_BAD_INTEGRITY:      "Integrity check on decrypted field failed",
	KRB_AP_ERR_TKT_EXPIRED:        "Ticket expired",
	KRB_AP_ERR_TKT_NYV:            "Ticket not yet valid",
	KRB_AP_ERR_REPEAT:             "Request is a replay",
	KRB_AP_ERR_NOT_US:             "Ticket not for us",
	KRB_AP_ERR_BADMATCH:           "Ticket and authenticator don't match",
	KRB_AP_ERR_SKEW:               "Clock skew too great",
	KRB_AP_ERR_BADADDR:            "Incorrect net address",
	KRB_AP_ERR_BADVERSION:         "Protocol version mismatch",
	KRB_AP_ERR_MSG_TYPE:           "Invalid message type",
	KRB_AP_ERR_MODIFIED:           "Message modified",
	KRB_AP_ERR_BADORDER:           "Message out of order",
	KRB_AP_ERR_BADKEYVER:          "Key version mismatch",
	KRB_AP_ERR_NOKEY:              "Service key not available",
	KRB_AP_ERR_MUTEX:              "Mutual authentication failed",
	KRB_AP_ERR_BADDIRECTION:       "Incorrect message direction",
	KRB_AP_ERR_METHOD:             "Alternative authentication method required",
	KRB_AP_ERR_BADSEQ:             "Incorrect sequence number",
	KRB_AP_ERR_INAPP_CKSUM:        "Inappropriate checksum type",
	KRB_AP_PATH_NOT_ACCEPTED:      "Policy rejects transited path",
	KRB_ERR_RESPONSE_TOO_BIG:      "Response too big for UDP",
	KRB_ERR_GENERIC:               "Generic error",
	KRB_ERR_FIELD_TOOLONG:         "Field too long",
	KDC_ERROR_CLIENT_NOT_TRUSTED:  "Client not trusted for delegation",
	KDC_ERROR_KDC_NOT_TRUSTED:     "KDC not trusted for delegation",
}

type KDCError struct {
	Code    int32
	Message string
	Realm   string
	SName   string
	EText   string
	EData   []byte
}

func (e *KDCError) Error() string {
	msg, ok := kdcErrorMessages[e.Code]
	if !ok {
		msg = "Unknown error"
	}
	if e.Message != "" {
		return fmt.Sprintf("KDC error %d: %s (%s)", e.Code, msg, e.Message)
	}
	return fmt.Sprintf("KDC error %d: %s", e.Code, msg)
}

func NewKDCError(code int32) *KDCError {
	return &KDCError{Code: code}
}

// NewKDCErrorWithDetail builds a classified KDCError from a raw KRB-ERROR
// message, extracting e-text and sname where present.
func NewKDCErrorWithDetail(code int32, rawKRBError []byte) *KDCError {
	e := &KDCError{Code: code}
	if parsed, err := ParseKRBError(rawKRBError); err == nil {
		e.EText = parsed.EText
		for _, c := range parsed.SName.NameString {
			e.SName += "/" + c
		}
		if len(e.SName) > 0 {
			e.SName = e.SName[1:]
		}
	}
	if msg, ok := kdcErrorMessages[code]; ok {
		e.Message = msg
	} else {
		e.Message = "Unknown"
	}
	return e
}

func IsKDCError(err error) (*KDCError, bool) {
	var ke *KDCError
	if errors.As(err, &ke) {
		return ke, true
	}
	return nil, false
}

func ClassifyError(code int32) error {
	switch code {
	case KDC_ERR_PREAUTH_REQUIRED:
		return ErrPreauthRequired
	case KDC_ERR_C_PRINCIPAL_UNKNOWN, KDC_ERR_S_PRINCIPAL_UNKNOWN:
		return ErrUnknownUser
	case KDC_ERR_PREAUTH_FAILED:
		return ErrWrongPassword
	case KDC_ERR_ETYPE_NOSUPP:
		return ErrUnsupportedEncryption
	case KDC_ERR_KEY_EXP:
		return ErrTicketExpired
	case KDC_ERR_CLIENT_REVOKED, KDC_ERR_SERVICE_REVOKED, KDC_ERR_TGT_REVOKED:
		return ErrAuthorizationDenied
	case KRB_AP_ERR_SKEW:
		return ErrClockSkew
	case KRB_AP_ERR_TKT_EXPIRED:
		return ErrTicketExpired
	case KRB_AP_ERR_TKT_NYV:
		return ErrFutureTicket
	case KRB_AP_ERR_BAD_INTEGRITY:
		return ErrMalformedCiphertext
	case KRB_AP_ERR_NOT_US:
		return ErrTicketInvalid
	default:
		return &KDCError{Code: code}
	}
}