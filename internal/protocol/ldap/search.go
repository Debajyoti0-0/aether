package ldap

import (
	"encoding/asn1"
	"fmt"
	"strings"
)

func EncodeSearchRequest(msgID int, req SearchRequest, controls []Control) ([]byte, error) {
	filterRV, err := encodeFilter(req.Filter)
	if err != nil {
		return nil, fmt.Errorf("encode filter: %w", err)
	}

	// RFC 4511 4.5.1.8: attributeSelection is SEQUENCE OF LDAPString, where
	// LDAPString is an OCTET STRING. asn1.Marshal([]string) emits UTF8String
	// elements (0x0c), which Samba/AD reject, so marshal [][]byte instead.
	attrBytes := make([][]byte, len(req.Attributes))
	for i, a := range req.Attributes {
		attrBytes[i] = []byte(a)
	}
	attrsTLV, err := asn1.Marshal(attrBytes)
	if err != nil {
		return nil, fmt.Errorf("marshal attributes: %w", err)
	}

	searchReq := struct {
		BaseObject   []byte
		Scope        asn1.Enumerated // ENUMERATED per RFC 4511 4.5.1
		DerefAliases asn1.Enumerated // ENUMERATED per RFC 4511 4.5.1
		SizeLimit    int
		TimeLimit    int
		TypesOnly    bool
		Filter       asn1.RawValue // encoded Filter choice, emitted verbatim
		Attributes   asn1.RawValue // full TLV of SEQUENCE OF OCTET STRING, emitted verbatim
	}{
		BaseObject:   []byte(req.BaseObject),
		Scope:        asn1.Enumerated(req.Scope),
		DerefAliases: asn1.Enumerated(req.DerefAliases),
		SizeLimit:    req.SizeLimit,
		TimeLimit:    req.TimeLimit,
		TypesOnly:    req.TypesOnly,
		Filter:       filterRV,
		Attributes:   asn1.RawValue{FullBytes: attrsTLV},
	}

	searchReqBytes, err := asn1.Marshal(searchReq)
	if err != nil {
		return nil, fmt.Errorf("marshal search request: %w", err)
	}

	searchReqContent, err := stripSequenceTag(searchReqBytes)
	if err != nil {
		return nil, fmt.Errorf("strip search request sequence: %w", err)
	}

	msg := LDAPMessage{
		MessageID: msgID,
		ProtocolOp: asn1.RawValue{
			Tag:        LDAP_SEARCH_REQUEST,
			Class:      asn1.ClassApplication,
			IsCompound: true,
			Bytes:      searchReqContent,
		},
		Controls: controls,
	}

	return asn1.Marshal(msg)
}

func encodeFilter(f Filter) (asn1.RawValue, error) {
	var bytes []byte

	switch f.FilterType {
	case LDAP_FILTER_AND, LDAP_FILTER_OR:
		// FilterValue.Bytes already contains the concatenated marshaled sub-filters
		bytes = f.FilterValue.Bytes
	case LDAP_FILTER_NOT:
		// FilterValue.Bytes contains the marshaled sub-filter
		bytes = f.FilterValue.Bytes
	case LDAP_FILTER_EQUALITY:
		// FilterValue.Bytes contains the marshaled AttributeValueAssertion
		bytes = f.FilterValue.Bytes
	case LDAP_FILTER_SUBSTRINGS:
		// FilterValue.Bytes contains the marshaled SubstringFilter
		bytes = f.FilterValue.Bytes
	case LDAP_FILTER_GE, LDAP_FILTER_LE:
		// FilterValue.Bytes contains the marshaled AttributeValueAssertion
		bytes = f.FilterValue.Bytes
	case LDAP_FILTER_PRESENT:
		// FilterValue.Bytes contains the attribute type (OCTET STRING)
		bytes = f.FilterValue.Bytes
	case LDAP_FILTER_APPROX:
		// FilterValue.Bytes contains the marshaled AttributeValueAssertion
		bytes = f.FilterValue.Bytes
	case LDAP_FILTER_EXTENSIBLE:
		// FilterValue.Bytes contains the marshaled MatchingRuleAssertion
		bytes = f.FilterValue.Bytes
	default:
		return asn1.RawValue{}, fmt.Errorf("unknown filter type: %d", f.FilterType)
	}

	tag := byte(f.FilterType) | 0xA0
	return asn1.RawValue{
		Class: asn1.ClassContextSpecific,
		Tag:   int(tag & 0x1F),
		// Presence filters are primitive (0x87); all other choices are constructed.
		IsCompound: f.FilterType != LDAP_FILTER_PRESENT,
		Bytes:      bytes,
	}, nil
}

// marshalFilterTLV encodes a Filter as its context-specific choice TLV
// (RFC 4511 4.5.1.7). Composite filters must contain the *encoded TLVs* of
// their children — asn1.Marshal on the Filter struct would wrap each child
// in a SEQUENCE{INTEGER, ANY}, which Samba/AD reject or misinterpret.
func marshalFilterTLV(f Filter) ([]byte, error) {
	rv, err := encodeFilter(f)
	if err != nil {
		return nil, err
	}
	return asn1.Marshal(rv)
}

func EqualityFilter(attr, value string) Filter {
	av := struct {
		Type  []byte
		Value []byte
	}{Type: []byte(attr), Value: []byte(value)}
	bytes, _ := asn1.Marshal(av)
	// RFC 4511 4.5.1.7: equalityMatch [3] carries the AttributeValueAssertion
	// content directly (A3 L 04 attr 04 value). The context tag REPLACES the
	// SEQUENCE tag; keeping the SEQUENCE TLV inside [3] produces an illegal
	// extra layer that Samba/AD reject by closing the connection (EOF).
	if content, err := stripSequenceTag(bytes); err == nil {
		bytes = content
	}
	return Filter{
		FilterType: LDAP_FILTER_EQUALITY,
		FilterValue: asn1.RawValue{
			Tag:        LDAP_FILTER_EQUALITY & 0x1F,
			Class:      asn1.ClassContextSpecific,
			IsCompound: true,
			Bytes:      bytes,
		},
	}
}

func AndFilter(filters ...Filter) Filter {
	var filterBytes []byte
	for _, f := range filters {
		fb := mustMarshalFilter(f)
		filterBytes = append(filterBytes, fb...)
	}
	return Filter{
		FilterType: LDAP_FILTER_AND,
		FilterValue: asn1.RawValue{
			Tag:        LDAP_FILTER_AND & 0x1F,
			Class:      asn1.ClassContextSpecific,
			IsCompound: true,
			Bytes:      filterBytes,
		},
	}
}

func OrFilter(filters ...Filter) Filter {
	var filterBytes []byte
	for _, f := range filters {
		fb := mustMarshalFilter(f)
		filterBytes = append(filterBytes, fb...)
	}
	return Filter{
		FilterType: LDAP_FILTER_OR,
		FilterValue: asn1.RawValue{
			Tag:        LDAP_FILTER_OR & 0x1F,
			Class:      asn1.ClassContextSpecific,
			IsCompound: true,
			Bytes:      filterBytes,
		},
	}
}

func NotFilter(filter Filter) Filter {
	// RFC 4511 4.5.1.7: not [2] contains exactly one child Filter — the child's
	// context-specific choice TLV verbatim (A2 L <child TLV>), NOT a
	// SEQUENCE{INTEGER, ANY} wrapper.
	bytes := mustMarshalFilter(filter)
	return Filter{
		FilterType: LDAP_FILTER_NOT,
		FilterValue: asn1.RawValue{
			Tag:        LDAP_FILTER_NOT & 0x1F,
			Class:      asn1.ClassContextSpecific,
			IsCompound: true,
			Bytes:      bytes,
		},
	}
}

func SubstringsFilter(attr string, initial, any, final string) Filter {
	type sub struct {
		typ   int
		value []byte
	}
	var subs []sub
	if initial != "" {
		subs = append(subs, sub{0, []byte(initial)})
	}
	for _, a := range strings.Split(any, "*") {
		if a != "" {
			subs = append(subs, sub{1, []byte(a)})
		}
	}
	if final != "" {
		subs = append(subs, sub{2, []byte(final)})
	}

	// RFC 4511 4.5.1.7: substrings [4] carries the SubstringFilter content
	// directly: A4 L 04 type 30 L 80 initial / 81 any / 82 final. Substring
	// choices are context-specific CONSTRUCTED tags (0x80/0x81/0x82 with
	// IsCompound), not INTEGER-discriminated SEQUENCEs.
	var subTLVs []byte
	for _, s := range subs {
		rv := asn1.RawValue{
			Class: asn1.ClassContextSpecific,
			Tag:   s.typ,
			// initial/any/final are implicitly-tagged AssertionValues
			// (OCTET STRING content) — primitive: 80 04 user.
			IsCompound: false,
			Bytes:      s.value,
		}
		tlv, err := asn1.Marshal(rv)
		if err != nil {
			continue
		}
		subTLVs = append(subTLVs, tlv...)
	}
	substringsSeq, err := asn1.Marshal(asn1.RawValue{
		Class:      asn1.ClassUniversal,
		Tag:        asn1.TagSequence,
		IsCompound: true,
		Bytes:      subTLVs,
	})
	if err != nil {
		substringsSeq = nil
	}
	// inner = type OCTET STRING || substrings SEQUENCE
	typeTLV, _ := asn1.Marshal(asn1.RawValue{
		Class: asn1.ClassUniversal, Tag: asn1.TagOctetString, Bytes: []byte(attr),
	})
	var inner []byte
	inner = append(inner, typeTLV...)
	inner = append(inner, substringsSeq...)
	return Filter{
		FilterType: LDAP_FILTER_SUBSTRINGS,
		FilterValue: asn1.RawValue{
			Tag:        LDAP_FILTER_SUBSTRINGS & 0x1F,
			Class:      asn1.ClassContextSpecific,
			IsCompound: true,
			Bytes:      inner,
		},
	}
}

func PresenceFilter(attr string) Filter {
	return Filter{
		FilterType: LDAP_FILTER_PRESENT,
		FilterValue: asn1.RawValue{
			Tag:        LDAP_FILTER_PRESENT & 0x1F,
			Class:      asn1.ClassContextSpecific,
			IsCompound: false,
			Bytes:      []byte(attr),
		},
	}
}

func DecodeSearchResponse(data []byte) ([]*SearchResultEntry, *SearchResultDone, []*SearchResultReference, error) {
	var msg LDAPMessage
	rest, err := asn1.Unmarshal(data, &msg)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("unmarshal LDAP message: %w", err)
	}
	_ = rest

	if msg.ProtocolOp.Class != asn1.ClassApplication {
		return nil, nil, nil, fmt.Errorf("unexpected protocolOp class %d tag %d", msg.ProtocolOp.Class, msg.ProtocolOp.Tag)
	}

	// msg.ProtocolOp.Bytes is the CONTENT of the [APPLICATION N] TLV, not a
	// stream of tagged elements. The tag itself selects the response type
	// (RFC 4511 4.5.2 / 4.5.3): a single entry, a single done, or references.
	switch msg.ProtocolOp.Tag {
	case LDAP_SEARCH_RESULT_ENTRY:
		entry, err := decodeSearchEntryContent(msg.ProtocolOp.Bytes)
		if err != nil {
			return nil, nil, nil, err
		}
		return []*SearchResultEntry{entry}, nil, nil, nil

	case LDAP_SEARCH_RESULT_DONE:
		done, err := decodeSearchDoneContent(msg.ProtocolOp.Bytes)
		if err != nil {
			return nil, nil, nil, err
		}
		return nil, done, nil, nil

	case LDAP_SEARCH_RESULT_REFERENCE:
		refs, err := decodeSearchRefContent(msg.ProtocolOp.Bytes)
		if err != nil {
			return nil, nil, nil, err
		}
		return nil, nil, refs, nil

	default:
		return nil, nil, nil, fmt.Errorf("unexpected search protocolOp tag %d", msg.ProtocolOp.Tag)
	}
}

// decodeSearchEntryContent parses the content of [APPLICATION 4]:
//
//	SearchResultEntry ::= SEQUENCE { objectName LDAPDN,
//	                                 attributes PartialAttributeList }
func decodeSearchEntryContent(content []byte) (*SearchResultEntry, error) {
	entry := &SearchResultEntry{}

	var nameRaw asn1.RawValue
	rest, err := asn1.Unmarshal(content, &nameRaw)
	if err != nil {
		return nil, fmt.Errorf("unmarshal search entry objectName: %w", err)
	}
	if nameRaw.Tag != asn1.TagOctetString {
		return nil, fmt.Errorf("expected OCTET STRING for objectName, got tag %d", nameRaw.Tag)
	}
	entry.ObjectName = nameRaw.Bytes

	var attrsSeq asn1.RawValue
	rest, err = asn1.Unmarshal(rest, &attrsSeq)
	if err != nil {
		return nil, fmt.Errorf("unmarshal search entry attributes: %w", err)
	}
	if len(rest) > 0 {
		// Optional [0] controls may follow; ignore them here.
		_ = rest
	}
	if attrsSeq.Tag == asn1.TagSequence && attrsSeq.Class == asn1.ClassUniversal {
		var attrs []PartialAttribute
		if _, err := asn1.Unmarshal(attrsSeq.FullBytes, &attrs); err != nil {
			return nil, fmt.Errorf("unmarshal partial attributes: %w", err)
		}
		entry.Attributes = attrs
	}

	return entry, nil
}

// decodeSearchDoneContent parses the content of [APPLICATION 5] (LDAPResult):
//
//	ENUMERATED resultCode, LDAPDN matchedDN, LDAPString diagnosticMessage,
//	optional [3] referral
func decodeSearchDoneContent(content []byte) (*SearchResultDone, error) {
	done := &SearchResultDone{}
	rest := content

	var rcRaw asn1.RawValue
	rest, err := asn1.Unmarshal(rest, &rcRaw)
	if err != nil {
		return nil, fmt.Errorf("unmarshal resultCode: %w", err)
	}
	if rcRaw.Tag != asn1.TagEnum {
		return nil, fmt.Errorf("expected ENUMERATED for resultCode, got tag %d", rcRaw.Tag)
	}
	done.ResultCode = decodeEnumerated(rcRaw.Bytes)

	var dnRaw asn1.RawValue
	rest, err = asn1.Unmarshal(rest, &dnRaw)
	if err != nil {
		return nil, fmt.Errorf("unmarshal matchedDN: %w", err)
	}
	if dnRaw.Tag != asn1.TagOctetString {
		return nil, fmt.Errorf("expected OCTET STRING for matchedDN, got tag %d", dnRaw.Tag)
	}
	done.MatchedDN = string(dnRaw.Bytes)

	var msgRaw asn1.RawValue
	rest, err = asn1.Unmarshal(rest, &msgRaw)
	if err != nil {
		return nil, fmt.Errorf("unmarshal errorMessage: %w", err)
	}
	if msgRaw.Tag != asn1.TagOctetString {
		return nil, fmt.Errorf("expected OCTET STRING for errorMessage, got tag %d", msgRaw.Tag)
	}
	done.ErrorMessage = string(msgRaw.Bytes)

	// Optional [3] referral sequence.
	if len(rest) > 0 {
		var refRaw asn1.RawValue
		rest2, err := asn1.Unmarshal(rest, &refRaw)
		if err == nil && refRaw.Class == asn1.ClassContextSpecific && refRaw.Tag == 3 {
			done.Referral = decodeURIList(refRaw.Bytes)
			_ = rest2
		}
	}

	return done, nil
}

// decodeSearchRefContent parses the content of [APPLICATION 19]:
// SEQUENCE OF LDAPURL (concatenated URI OCTET STRING TLVs).
func decodeSearchRefContent(content []byte) ([]*SearchResultReference, error) {
	refs := []*SearchResultReference{{
		Referral: decodeURIList(content),
	}}
	return refs, nil
}

func decodeURIList(content []byte) []string {
	var uris []string
	rest := content
	for len(rest) > 0 {
		var rv asn1.RawValue
		var err error
		rest, err = asn1.Unmarshal(rest, &rv)
		if err != nil {
			break
		}
		uris = append(uris, string(rv.Bytes))
	}
	return uris
}

func decodeEnumerated(b []byte) int {
	v := 0
	for _, x := range b {
		v = v<<8 | int(x)
	}
	// Sign-extend for negative ENUMERATED values.
	if len(b) > 0 && b[0]&0x80 != 0 {
		for i := len(b); i < 8; i++ {
			v |= -1 << (8 * i)
		}
	}
	return v
}

func DecodeSearchEntry(data []byte) (*SearchResultEntry, error) {
	var entry SearchResultEntry
	_, err := asn1.Unmarshal(data, &entry)
	if err != nil {
		return nil, fmt.Errorf("unmarshal search entry: %w", err)
	}
	return &entry, nil
}

func DecodeSearchDone(data []byte) (*SearchResultDone, error) {
	var done SearchResultDone
	_, err := asn1.Unmarshal(data, &done)
	if err != nil {
		return nil, fmt.Errorf("unmarshal search done: %w", err)
	}
	return &done, nil
}

func (e *SearchResultEntry) GetAttributeValues(attr string) []string {
	for _, a := range e.Attributes {
		if strings.EqualFold(string(a.Type), attr) {
			values := make([]string, len(a.Values))
			for i, v := range a.Values {
				values[i] = string(v)
			}
			return values
		}
	}
	return nil
}

func (e *SearchResultEntry) GetAttributeRawValues(attr string) [][]byte {
	for _, a := range e.Attributes {
		if strings.EqualFold(string(a.Type), attr) {
			return a.Values
		}
	}
	return nil
}

func (e *SearchResultEntry) GetFirstAttributeValue(attr string) string {
	values := e.GetAttributeValues(attr)
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

func ParseFilter(filterStr string) *Filter {
	filterStr = strings.TrimSpace(filterStr)
	if !strings.HasPrefix(filterStr, "(") || !strings.HasSuffix(filterStr, ")") {
		return nil
	}

	inner := filterStr[1 : len(filterStr)-1]
	inner = strings.TrimSpace(inner)

	if strings.HasPrefix(inner, "&") {
		return parseAndOrFilter(inner[1:], LDAP_FILTER_AND)
	}
	if strings.HasPrefix(inner, "|") {
		return parseAndOrFilter(inner[1:], LDAP_FILTER_OR)
	}
	if strings.HasPrefix(inner, "!") {
		// inner[1:] already includes the child's own parentheses, e.g.
		// "(!(a=b))" → "(a=b)". Re-wrapping produced "((a=b))" and leaked
		// parens into the AttributeValueAssertion (attr "(objectClass").
		f := ParseFilter(inner[1:])
		if f == nil {
			return nil
		}
		return &Filter{
			FilterType: LDAP_FILTER_NOT,
			FilterValue: asn1.RawValue{
				Tag:        LDAP_FILTER_NOT & 0x1F,
				Class:      asn1.ClassContextSpecific,
				IsCompound: true,
				Bytes:      mustMarshalFilter(*f),
			},
		}
	}

	// RFC 4511 4.5.1.7: a filter of the form "(attr=*)" is the presence
	// filter [7] PRLM (0x87), not an equality match against the literal "*".
	if strings.HasSuffix(inner, "=*") && !strings.Contains(inner[:len(inner)-2], "=") {
		f := PresenceFilter(strings.TrimSpace(inner[:len(inner)-2]))
		return &f
	}

	parts := strings.SplitN(inner, "=", 2)
	if len(parts) == 2 {
		attr := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		f := EqualityFilter(attr, value)
		return &f
	}

	return nil
}

func parseAndOrFilter(inner string, filterType int) *Filter {
	var filters []*Filter
	depth := 0
	start := 0

	for i := 0; i < len(inner); i++ {
		if inner[i] == '(' {
			depth++
		} else if inner[i] == ')' {
			depth--
			if depth == 0 {
				subFilter := ParseFilter(inner[start : i+1])
				if subFilter != nil {
					filters = append(filters, subFilter)
				}
				start = i + 1
			}
		}
	}

	var filterBytes []byte
	for _, f := range filters {
		fb := mustMarshalFilter(*f)
		filterBytes = append(filterBytes, fb...)
	}

	tag := byte(filterType) | 0xA0
	return &Filter{
		FilterType: filterType,
		FilterValue: asn1.RawValue{
			Tag:        int(tag),
			Class:      asn1.ClassContextSpecific,
			IsCompound: true,
			Bytes:      filterBytes,
		},
	}
}

func mustMarshalFilter(f Filter) []byte {
	// RFC 4511 4.5.1.7: a Filter marshals as its context-specific choice TLV
	// (e.g. 87 0B objectClass). Marshaling the Filter struct directly emits
	// SEQUENCE{INTEGER choice, ANY}, which corrupts every composite filter
	// (AND/OR/NOT children) on the wire.
	rv, err := encodeFilter(f)
	if err != nil {
		return nil
	}
	b, err := asn1.Marshal(rv)
	if err != nil {
		return nil
	}
	return b
}
