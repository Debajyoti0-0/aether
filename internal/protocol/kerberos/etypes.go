package kerberos

import (
	"errors"
	"strings"
)

var (
	ErrUnsupportedEtype = errors.New("unsupported encryption type")
)

type EtypeInfo struct {
	Etype       int32
	Salt        string
	S2KParams   string
}

type EtypeInfo2 struct {
	Etype      int32
	Salt       string
	S2KParams  string
}

func SupportedEtypes() []int32 {
	return []int32{
		ETYPE_AES256_CTS_HMAC_SHA1_96,
		ETYPE_AES128_CTS_HMAC_SHA1_96,
		ETYPE_RC4_HMAC,
	}
}

func PreferredEtype(offered []int32) int32 {
	preferred := SupportedEtypes()
	for _, p := range preferred {
		for _, o := range offered {
			if p == o {
				return p
			}
		}
	}
	return 0
}

func IsSupported(etype int32) bool {
	for _, e := range SupportedEtypes() {
		if e == etype {
			return true
		}
	}
	return false
}

func KeySize(etype int32) (int, error) {
	switch etype {
	case ETYPE_AES128_CTS_HMAC_SHA1_96:
		return 16, nil
	case ETYPE_AES256_CTS_HMAC_SHA1_96:
		return 32, nil
	case ETYPE_RC4_HMAC:
		return 16, nil
	default:
		return 0, ErrUnsupportedEtype
	}
}

func ChecksumSize(etype int32) (int, error) {
	switch etype {
	case ETYPE_AES128_CTS_HMAC_SHA1_96, ETYPE_AES256_CTS_HMAC_SHA1_96:
		return 12, nil
	case ETYPE_RC4_HMAC:
		return 12, nil
	default:
		return 0, ErrUnsupportedEtype
	}
}

func EtypeName(etype int32) string {
	switch etype {
	case ETYPE_DES_CBC_CRC:
		return "des-cbc-crc"
	case ETYPE_DES_CBC_MD4:
		return "des-cbc-md4"
	case ETYPE_DES_CBC_MD5:
		return "des-cbc-md5"
	case ETYPE_AES128_CTS_HMAC_SHA1_96:
		return "aes128-cts-hmac-sha1-96"
	case ETYPE_AES256_CTS_HMAC_SHA1_96:
		return "aes256-cts-hmac-sha1-96"
	case ETYPE_RC4_HMAC:
		return "rc4-hmac"
	case ETYPE_RC4_HMAC_EXP:
		return "rc4-hmac-exp"
	default:
		return "unknown"
	}
}

// etypeInfoEntries unwraps the SEQUENCE OF wrapper of ETYPE-INFO /
// ETYPE-INFO2 and returns the entry nodes.
//
//	ETYPE-INFO2 ::= SEQUENCE OF ETYPE-INFO2-ENTRY
//	ETYPE-INFO2-ENTRY ::= SEQUENCE { etype[0], salt[1], s2kparams[2] }
//
// Aether's decoder walks children of a raw buffer, so it must first drop the
// outer SEQUENCE header — otherwise each entry is off by one nesting level and
// every field decodes as empty. Live Samba ETYPE-INFO2 (Stage 46h):
//
//	30 2b { 30 29 { a0 03 02 01 12, a1 1a 1b 18 "AETHER.TESTAdministrator",
//	                a2 06 04 04 00 00 10 00 } }
func etypeInfoEntries(data []byte) ([]tlvRaw, error) {
	if top, _, err := parseTLV(data); err == nil {
		if top.class == derClassUniversal && top.tag == (tagSequence&0x1f) {
			data = top.value
		}
	}
	return parseChildren(data)
}

// derStringValue peels the context tag(s) and the inner string/OCTET-STRING
// header off a tagged node and returns the raw content octets.
//
// ETHER's ETYPE-INFO2 carries salt as [1] GeneralString and s2kparams as
// [2] OCTET STRING, so `a1 1a 1b 18 <salt>` and `a2 06 04 04 <params>`. Reading
// the context node's .value directly would leave the `1b 18` / `04 04` header
// in the string, corrupting both the salt and the PBKDF2 iteration count.
func derStringValue(r tlvRaw) []byte {
	inner := unwrapCtx(r)
	// The context node's value is itself a complete TLV (GeneralString 0x1b or
	// OCTET STRING 0x04) because the module is EXPLICIT TAGS.
	if h, _, err := parseTLV(inner.value); err == nil &&
		h.class == derClassUniversal && (h.tag == tagOctetString&0x1f || h.tag == 0x1b) {
		return h.value
	}
	return inner.value
}

func ParseEtypeInfo(data []byte) ([]EtypeInfo, error) {
	// ETYPE-INFO entries are context-tagged sequences; we surface raw entries.
	var result []EtypeInfo
	kids, err := etypeInfoEntries(data)
	if err != nil {
		return nil, err
	}
	for _, k := range kids {
		entryKids, derr := parseChildren(k.value)
		if derr != nil {
			continue
		}
		var e EtypeInfo
		if x := findCtx(entryKids, 0); x != nil {
			if v, derr := decodeInt(*x); derr == nil {
				e.Etype = int32(v)
			}
		}
		if x := findCtx(entryKids, 1); x != nil {
			e.Salt = string(derStringValue(*x))
		}
		result = append(result, e)
	}
	return result, nil
}

func ParseEtypeInfo2(data []byte) ([]EtypeInfo2, error) {
	var result []EtypeInfo2
	kids, err := etypeInfoEntries(data)
	if err != nil {
		return nil, err
	}
	for _, k := range kids {
		entryKids, derr := parseChildren(k.value)
		if derr != nil {
			continue
		}
		var e EtypeInfo2
		if x := findCtx(entryKids, 0); x != nil {
			if v, derr := decodeInt(*x); derr == nil {
				e.Etype = int32(v)
			}
		}
		if x := findCtx(entryKids, 1); x != nil {
			e.Salt = string(derStringValue(*x))
		}
		// RFC 4120 §5.4.5: ETYPE-INFO2-ENTRY s2kparams [2] is an OCTET
		// STRING (for AES256 it carries the PBKDF2 iteration count).
		if x := findCtx(entryKids, 2); x != nil {
			e.S2KParams = string(derStringValue(*x))
		}
		result = append(result, e)
	}
	return result, nil
}

func MakeSalt(realm Realm, principal PrincipalName) string {
	// RFC 4120 §4 (pre-authentication salt): the default salt for a
	// principal is the concatenation of the realm and the principal name
	// components, e.g. "AETHER.TEST" + "user1" -> "AETHER.TESTuser1".
	var sb strings.Builder
	sb.WriteString(string(realm))
	for _, c := range principal.NameString {
		// The realm itself is not a name component (see MakeUserPrincipal),
		// so every remaining component is appended verbatim.
		sb.WriteString(c)
	}
	return sb.String()
}