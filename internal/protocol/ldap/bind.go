package ldap

import (
	"encoding/asn1"
	"fmt"
)

// stripSequenceTag parses a BER SEQUENCE and returns the content bytes
// (i.e. everything after the SEQUENCE tag and length). This is needed because
// Go's asn1.Marshal always wraps struct fields in a SEQUENCE (tag 0x30), but
// LDAP protocol operations use the [APPLICATION N] tag directly as the
// constructed container (RFC 4511).
func stripSequenceTag(data []byte) ([]byte, error) {
	var raw asn1.RawValue
	rest, err := asn1.Unmarshal(data, &raw)
	if err != nil {
		return nil, err
	}
	if len(rest) > 0 {
		return nil, fmt.Errorf("unexpected trailing data after SEQUENCE")
	}
	return raw.Bytes, nil
}

func EncodeBindRequest(msgID int, dn, password string, controls []Control) ([]byte, error) {
	// Per RFC 4511, simple authentication is [0] OCTET STRING — the password
	// bytes are the direct value of the [0] tag, not wrapped in a separate
	// OCTET STRING TLV.
	auth := asn1.RawValue{
		Tag:          0, // Simple authentication [0]
		Class:        asn1.ClassContextSpecific,
		Bytes:        []byte(password),
		IsCompound:   false,
	}

	bindReq := BindRequest{
		Version:        3,
		Name:           []byte(dn),
		Authentication: auth,
	}

	bindReqBytes, err := asn1.Marshal(bindReq)
	if err != nil {
		return nil, fmt.Errorf("marshal bind request: %w", err)
	}

	// Strip the outer SEQUENCE tag/length: [APPLICATION 0] directly contains
	// the fields per RFC 4511, not an inner SEQUENCE.
	bindReqContent, err := stripSequenceTag(bindReqBytes)
	if err != nil {
		return nil, fmt.Errorf("strip bind request sequence: %w", err)
	}

	msg := LDAPMessage{
		MessageID: msgID,
		ProtocolOp: asn1.RawValue{
			Tag:          LDAP_BIND_REQUEST,
			Class:        asn1.ClassApplication,
			IsCompound:   true,
			Bytes:        bindReqContent,
		},
		Controls: controls,
	}

	return asn1.Marshal(msg)
}

func DecodeBindResponse(data []byte) (*BindResponse, error) {
	var msg LDAPMessage
	_, err := asn1.Unmarshal(data, &msg)
	if err != nil {
		return nil, fmt.Errorf("unmarshal LDAP message: %w", err)
	}

	if msg.ProtocolOp.Tag != LDAP_BIND_RESPONSE || msg.ProtocolOp.Class != asn1.ClassApplication {
		return nil, ErrInvalidBindRequest
	}

	// Manually parse the bind response to avoid asn1.Unmarshal issues with optional custom-tagged fields
	resp := &BindResponse{}
	rest := msg.ProtocolOp.Bytes

	// Parse ResultCode (ENUMERATED)
	var resultCodeRaw asn1.RawValue
	rest, err = asn1.Unmarshal(rest, &resultCodeRaw)
	if err != nil {
		return nil, fmt.Errorf("unmarshal resultCode: %w", err)
	}
	if resultCodeRaw.Tag != asn1.TagEnum {
		return nil, fmt.Errorf("expected ENUMERATED for resultCode, got tag %d", resultCodeRaw.Tag)
	}
	resp.ResultCode = int(resultCodeRaw.Bytes[0])

	// Parse MatchedDN (OCTET STRING)
	var matchedDNRaw asn1.RawValue
	rest, err = asn1.Unmarshal(rest, &matchedDNRaw)
	if err != nil {
		return nil, fmt.Errorf("unmarshal matchedDN: %w", err)
	}
	if matchedDNRaw.Tag != asn1.TagOctetString {
		return nil, fmt.Errorf("expected OCTET STRING for matchedDN, got tag %d", matchedDNRaw.Tag)
	}
	resp.MatchedDN = string(matchedDNRaw.Bytes)

	// Parse ErrorMessage (OCTET STRING)
	var errorMsgRaw asn1.RawValue
	rest, err = asn1.Unmarshal(rest, &errorMsgRaw)
	if err != nil {
		return nil, fmt.Errorf("unmarshal errorMessage: %w", err)
	}
	if errorMsgRaw.Tag != asn1.TagOctetString {
		return nil, fmt.Errorf("expected OCTET STRING for errorMessage, got tag %d", errorMsgRaw.Tag)
	}
	resp.ErrorMessage = string(errorMsgRaw.Bytes)

	// Parse optional Referral (context-specific tag 3)
	if len(rest) > 0 {
		var referralRaw asn1.RawValue
		rest, err = asn1.Unmarshal(rest, &referralRaw)
		if err == nil && referralRaw.Tag == 3 && referralRaw.Class == asn1.ClassContextSpecific {
			// Parse the referral SEQUENCE
			var referrals []string
			refRest := referralRaw.Bytes
			for len(refRest) > 0 {
				var refRaw asn1.RawValue
				refRest, err = asn1.Unmarshal(refRest, &refRaw)
				if err != nil {
					break
				}
				if refRaw.Tag == asn1.TagOctetString {
					referrals = append(referrals, string(refRaw.Bytes))
				}
			}
			resp.Referral = referrals
		} else if err != nil {
			// Not a referral, might be ServerSaslCreds
			if err != nil {
				// Ignore error, just check next
			}
		}

		// Parse optional ServerSaslCreds (tag 7)
		if len(rest) > 0 {
			var saslRaw asn1.RawValue
			rest, err = asn1.Unmarshal(rest, &saslRaw)
			if err == nil && saslRaw.Tag == 7 && saslRaw.Class == asn1.ClassContextSpecific {
				resp.ServerSaslCreds = saslRaw.Bytes
			}
		}
	}

	return resp, nil
}

func EncodeUnbindRequest(msgID int) ([]byte, error) {
	msg := LDAPMessage{
		MessageID: msgID,
		ProtocolOp: asn1.RawValue{
			Tag:          LDAP_UNBIND_REQUEST,
			Class:        asn1.ClassApplication,
			IsCompound:   false,
			Bytes:        nil,
		},
	}
	return asn1.Marshal(msg)
}

func EncodeSaslBindRequest(msgID int, mechanism, initialCreds string, controls []Control) ([]byte, error) {
	auth := asn1.RawValue{
		Tag:          3, // SASL
		Class:        asn1.ClassContextSpecific,
		IsCompound:   true,
		Bytes:        nil, // Will be filled by marshaling the inner sequence
	}

	// SASL credentials: mechanism + initial credentials
	saslCreds := struct {
		Mechanism   string
		Credentials string `asn1:"optional"`
	}{
		Mechanism:   mechanism,
		Credentials: initialCreds,
	}
	saslBytes, err := asn1.Marshal(saslCreds)
	if err != nil {
		return nil, fmt.Errorf("marshal sasl creds: %w", err)
	}
	auth.Bytes = saslBytes

	bindReq := BindRequest{
		Version:        3,
		Name:           nil, // SASL doesn't use DN in bind request
		Authentication: auth,
	}

	bindReqBytes, err := asn1.Marshal(bindReq)
	if err != nil {
		return nil, fmt.Errorf("marshal sasl bind request: %w", err)
	}

	bindReqContent, err := stripSequenceTag(bindReqBytes)
	if err != nil {
		return nil, fmt.Errorf("strip sasl bind request sequence: %w", err)
	}

	msg := LDAPMessage{
		MessageID: msgID,
		ProtocolOp: asn1.RawValue{
			Tag:          LDAP_BIND_REQUEST,
			Class:        asn1.ClassApplication,
			IsCompound:   true,
			Bytes:        bindReqContent,
		},
		Controls: controls,
	}

	return asn1.Marshal(msg)
}
