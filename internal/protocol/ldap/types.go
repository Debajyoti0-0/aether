package ldap

import (
	"encoding/asn1"
	"errors"
	"fmt"
)

var (
	ErrInvalidLDAPMessage = errors.New("invalid LDAP message")
	ErrInvalidBindRequest = errors.New("invalid bind request")
	ErrInvalidSearchRequest = errors.New("invalid search request")
	ErrInvalidResponse      = errors.New("invalid LDAP response")
	ErrProtocolError        = errors.New("LDAP protocol error")
	ErrBindFailed           = errors.New("bind failed")
	ErrSearchFailed         = errors.New("search failed")
	ErrReferral             = errors.New("referral received")
)

const (
	// LDAP Message Types (RFC 4511)
	LDAP_BIND_REQUEST            = 0
	LDAP_BIND_RESPONSE           = 1
	LDAP_UNBIND_REQUEST          = 2
	LDAP_SEARCH_REQUEST          = 3
	LDAP_SEARCH_RESULT_ENTRY     = 4
	LDAP_SEARCH_RESULT_DONE      = 5
	LDAP_MODIFY_REQUEST          = 6
	LDAP_MODIFY_RESPONSE         = 7
	LDAP_ADD_REQUEST             = 8
	LDAP_ADD_RESPONSE            = 9
	LDAP_DEL_REQUEST             = 10
	LDAP_DEL_RESPONSE            = 11
	LDAP_MODDN_REQUEST           = 12
	LDAP_MODDN_RESPONSE          = 13
	LDAP_COMPARE_REQUEST         = 14
	LDAP_COMPARE_RESPONSE        = 15
	LDAP_ABANDON_REQUEST         = 16
	LDAP_SEARCH_RESULT_REFERENCE = 19
	LDAP_EXTENDED_REQUEST        = 23
	LDAP_EXTENDED_RESPONSE       = 24
)

const (
	// Bind Methods
	LDAP_AUTH_SIMPLE      = 0
	LDAP_AUTH_SASL        = 3
	LDAP_AUTH_NEGOTIATE   = 11 // SPNEGO
	LDAP_AUTH_NTLM        = 12 // NTLM
)

const (
	// Result Codes (RFC 4511)
	LDAP_SUCCESS                   = 0
	LDAP_OPERATIONS_ERROR          = 1
	LDAP_PROTOCOL_ERROR            = 2
	LDAP_TIMELIMIT_EXCEEDED        = 3
	LDAP_SIZELIMIT_EXCEEDED        = 4
	LDAP_COMPARE_FALSE             = 5
	LDAP_COMPARE_TRUE              = 6
	LDAP_AUTH_METHOD_NOT_SUPPORTED = 7
	LDAP_STRONG_AUTH_REQUIRED      = 8
	LDAP_REFERRAL                  = 10
	LDAP_ADMINLIMIT_EXCEEDED       = 11
	LDAP_UNAVAILABLE_CRITICAL_EXT  = 12
	LDAP_CONFIDENTIALITY_REQUIRED  = 13
	LDAP_SASL_BIND_IN_PROGRESS     = 14
	LDAP_NO_SUCH_ATTRIBUTE         = 16
	LDAP_UNDEFINED_TYPE            = 17
	LDAP_INAPPROPRIATE_MATCHING    = 18
	LDAP_CONSTRAINT_VIOLATION      = 19
	LDAP_TYPE_OR_VALUE_EXISTS      = 20
	LDAP_INVALID_SYNTAX            = 21
	LDAP_NO_SUCH_OBJECT            = 32
	LDAP_ALIAS_PROBLEM             = 33
	LDAP_INVALID_DN_SYNTAX         = 34
	LDAP_ALIAS_DEREF_PROBLEM       = 36
	LDAP_INAPPROPRIATE_AUTH        = 48
	LDAP_INVALID_CREDENTIALS       = 49
	LDAP_INSUFFICIENT_ACCESS       = 50
	LDAP_BUSY                      = 51
	LDAP_UNAVAILABLE               = 52
	LDAP_UNWILLING_TO_PERFORM      = 53
	LDAP_LOOP_DETECT               = 54
	LDAP_NAMING_VIOLATION          = 64
	LDAP_OBJECT_CLASS_VIOLATION    = 65
	LDAP_NOT_ALLOWED_ON_NONLEAF    = 66
	LDAP_NOT_ALLOWED_ON_RDN        = 67
	LDAP_ALREADY_EXISTS            = 68
	LDAP_NO_OBJECT_CLASS_MODS      = 69
	LDAP_RESULTS_TOO_LARGE         = 70
	LDAP_AFFECTS_MULTIPLE_DSAS     = 71
	LDAP_OTHER                     = 80
)

const (
	// Search Scope
	LDAP_SCOPE_BASE        = 0
	LDAP_SCOPE_ONELEVEL    = 1
	LDAP_SCOPE_SUBTREE     = 2
	LDAP_SCOPE_CHILDREN    = 3 // Non-standard but supported
)

const (
	// Deref Aliases
	LDAP_DEREF_NEVER    = 0
	LDAP_DEREF_SEARCH   = 1
	LDAP_DEREF_FIND     = 2
	LDAP_DEREF_ALWAYS   = 3
)

const (
	// Filter Types
	LDAP_FILTER_AND         = 0
	LDAP_FILTER_OR          = 1
	LDAP_FILTER_NOT         = 2
	LDAP_FILTER_EQUALITY    = 3
	LDAP_FILTER_SUBSTRINGS  = 4
	LDAP_FILTER_GE          = 5
	LDAP_FILTER_LE          = 6
	LDAP_FILTER_PRESENT     = 7
	LDAP_FILTER_APPROX      = 8
	LDAP_FILTER_EXTENSIBLE  = 9
)

const (
	// Control Types
	LDAP_CONTROL_PAGED_RESULTS = "1.2.840.113556.1.4.319"
	LDAP_CONTROL_SORT          = "1.2.840.113556.1.4.473"
	LDAP_CONTROL_VLV           = "2.16.840.1.113730.3.4.9"
	LDAP_CONTROL_PASSWORD_POLICY = "1.3.6.1.4.1.42.2.27.8.5.1"
	LDAP_CONTROL_PERMISSIVE_MODIFY = "1.2.840.113556.1.4.1413"
	LDAP_CONTROL_SHOW_DELETED  = "1.2.840.113556.1.4.417"
	LDAP_CONTROL_TREE_DELETE   = "1.2.840.113556.1.4.805"
	LDAP_CONTROL_DOMAIN_SCOPE  = "1.2.840.113556.1.4.1339"
	LDAP_CONTROL_DIRSYNC       = "1.2.840.113556.1.4.841"
	LDAP_CONTROL_RANGE         = "1.2.840.113556.1.4.1504"
)

type LDAPMessage struct {
	MessageID int
	ProtocolOp asn1.RawValue
	Controls  []Control `asn1:"optional,tag:0,class:context-specific"`
}

type BindRequest struct {
	Version        int            // INTEGER per RFC 4511
	Name           []byte         // OCTET STRING (LDAPDN) per RFC 4511
	Authentication asn1.RawValue  // [0] simple / [3] SASL per RFC 4511
}

type BindResponse struct {
	ResultCode   int    `asn1:"enumerated"`
	MatchedDN    string
	ErrorMessage string
	Referral     []string `asn1:"optional,tag:3,class:context-specific"`
	ServerSaslCreds []byte `asn1:"optional,tag:7"`
}

type SearchRequest struct {
	BaseObject      string
	Scope           int `asn1:"enumerated"`
	DerefAliases    int `asn1:"enumerated"`
	SizeLimit       int
	TimeLimit       int
	TypesOnly       bool
	Filter          Filter
	Attributes      []string
	Controls        []Control `asn1:"optional"`
}

type SearchResultEntry struct {
	ObjectName []byte             // LDAPDN = OCTET STRING per RFC 4511
	Attributes []PartialAttribute // PartialAttributeList: plain SEQUENCE OF per RFC 4511
	Controls   []Control          `asn1:"optional,tag:0,class:context-specific"`
}

type SearchResultDone struct {
	ResultCode   int    `asn1:"enumerated"`
	MatchedDN    string
	ErrorMessage string
	Referral     []string `asn1:"optional,tag:3,class:context-specific"`
	Controls     []Control `asn1:"optional,tag:0,class:context-specific"`
}

type SearchResultReference struct {
	Referral []string
	Controls []Control `asn1:"optional"`
}

type AttributeValueSet [][]byte

type PartialAttribute struct {
	Type   []byte
	Values AttributeValueSet `asn1:"set"`
}

type Attribute struct {
	Type   string
	Values [][]byte
}

type Filter struct {
	FilterType int
	FilterValue asn1.RawValue
}

type Control struct {
	ControlType             string
	Criticality             bool `asn1:"optional"`
	ControlValue            []byte `asn1:"optional"`
}

type PagedResultsControl struct {
	Size int
	Cookie []byte
}

type LDAPString string

func (l LDAPString) String() string {
	return string(l)
}

type LDAPDN string

func (l LDAPDN) String() string {
	return string(l)
}

type RelativeLDAPDN string

type AttributeDescription string

type MatchingRuleId string

type LDAPOID string

func (l LDAPOID) String() string {
	return string(l)
}

type LDAPResult struct {
	ResultCode   int
	MatchedDN    string
	ErrorMessage string
	Referral     []string `asn1:"optional"`
}

func (r LDAPResult) Error() string {
	if r.ResultCode == LDAP_SUCCESS {
		return "LDAP success"
	}
	return fmt.Sprintf("LDAP error %d: %s", r.ResultCode, r.ErrorMessage)
}

func IsSuccessResult(code int) bool {
	return code == LDAP_SUCCESS
}

func IsReferralResult(code int) bool {
	return code == LDAP_REFERRAL
}

func ResultCodeString(code int) string {
	switch code {
	case LDAP_SUCCESS:
		return "Success"
	case LDAP_OPERATIONS_ERROR:
		return "Operations Error"
	case LDAP_PROTOCOL_ERROR:
		return "Protocol Error"
	case LDAP_TIMELIMIT_EXCEEDED:
		return "Time Limit Exceeded"
	case LDAP_SIZELIMIT_EXCEEDED:
		return "Size Limit Exceeded"
	case LDAP_COMPARE_FALSE:
		return "Compare False"
	case LDAP_COMPARE_TRUE:
		return "Compare True"
	case LDAP_AUTH_METHOD_NOT_SUPPORTED:
		return "Auth Method Not Supported"
	case LDAP_STRONG_AUTH_REQUIRED:
		return "Strong Auth Required"
	case LDAP_REFERRAL:
		return "Referral"
	case LDAP_ADMINLIMIT_EXCEEDED:
		return "Admin Limit Exceeded"
	case LDAP_UNAVAILABLE_CRITICAL_EXT:
		return "Unavailable Critical Extension"
	case LDAP_CONFIDENTIALITY_REQUIRED:
		return "Confidentiality Required"
	case LDAP_SASL_BIND_IN_PROGRESS:
		return "SASL Bind in Progress"
	case LDAP_NO_SUCH_ATTRIBUTE:
		return "No Such Attribute"
	case LDAP_UNDEFINED_TYPE:
		return "Undefined Type"
	case LDAP_INAPPROPRIATE_MATCHING:
		return "Inappropriate Matching"
	case LDAP_CONSTRAINT_VIOLATION:
		return "Constraint Violation"
	case LDAP_TYPE_OR_VALUE_EXISTS:
		return "Type or Value Exists"
	case LDAP_INVALID_SYNTAX:
		return "Invalid Syntax"
	case LDAP_NO_SUCH_OBJECT:
		return "No Such Object"
	case LDAP_ALIAS_PROBLEM:
		return "Alias Problem"
	case LDAP_INVALID_DN_SYNTAX:
		return "Invalid DN Syntax"
	case LDAP_ALIAS_DEREF_PROBLEM:
		return "Alias Deref Problem"
	case LDAP_INAPPROPRIATE_AUTH:
		return "Inappropriate Auth"
	case LDAP_INVALID_CREDENTIALS:
		return "Invalid Credentials"
	case LDAP_INSUFFICIENT_ACCESS:
		return "Insufficient Access"
	case LDAP_BUSY:
		return "Busy"
	case LDAP_UNAVAILABLE:
		return "Unavailable"
	case LDAP_UNWILLING_TO_PERFORM:
		return "Unwilling to Perform"
	case LDAP_LOOP_DETECT:
		return "Loop Detect"
	case LDAP_NAMING_VIOLATION:
		return "Naming Violation"
	case LDAP_OBJECT_CLASS_VIOLATION:
		return "Object Class Violation"
	case LDAP_NOT_ALLOWED_ON_NONLEAF:
		return "Not Allowed on Non-Leaf"
	case LDAP_NOT_ALLOWED_ON_RDN:
		return "Not Allowed on RDN"
	case LDAP_ALREADY_EXISTS:
		return "Already Exists"
	case LDAP_NO_OBJECT_CLASS_MODS:
		return "No Object Class Mods"
	case LDAP_RESULTS_TOO_LARGE:
		return "Results Too Large"
	case LDAP_AFFECTS_MULTIPLE_DSAS:
		return "Affects Multiple DSAs"
	case LDAP_OTHER:
		return "Other"
	default:
		return fmt.Sprintf("Unknown (%d)", code)
	}
}

type LDAPURL struct {
	Scheme   string
	Hostport string
	DN       string
	Attrs    []string
	Scope    int
	Filter   string
	Exts     []string
}