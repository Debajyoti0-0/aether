package wstrust

import (
	"bytes"
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Debajyoti0-0/aether/internal/protocol/saml"
	"github.com/Debajyoti0-0/aether/internal/types"
)

// BuildRST constructs a WS-Trust 1.3 RequestSecurityToken SOAP envelope
// with username/password credentials for the given applies-to realm.
func BuildRST(username, password, appliesTo, endpoint string) []byte {
	created := time.Now().UTC().Format(time.RFC3339)
	expires := time.Now().UTC().Add(5 * time.Minute).Format(time.RFC3339)
	messageID := fmt.Sprintf("urn:uuid:%s", randomUUID())
	wsaTo := endpoint
	if wsaTo == "" {
		wsaTo = "https://sts.example.com/adfs/services/trust/2005/usernamemixed"
	}

	var b strings.Builder

	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString(`<s:Envelope xmlns:s="` + SOAPEnvelopeNS + `" xmlns:wsa="` + AddressingNS + `" xmlns:wsu="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd" xmlns:wsse="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd" xmlns:wst="` + WSTrust13Namespace + `">`)
	b.WriteString(`<s:Header>`)
	b.WriteString(`<wsa:Action s:mustUnderstand="1">` + WSTrust13Namespace + `/RST/Issue</wsa:Action>`)
	b.WriteString(`<wsa:MessageID>` + messageID + `</wsa:MessageID>`)
	b.WriteString(`<wsa:ReplyTo><wsa:Address>http://www.w3.org/2005/08/addressing/anonymous</wsa:Address></wsa:ReplyTo>`)
	b.WriteString(`<wsa:To s:mustUnderstand="1">` + wsaTo + `</wsa:To>`)
	b.WriteString(`<wsse:Security s:mustUnderstand="1">`)
	b.WriteString(`<wsu:Timestamp wsu:Id="_0">`)
	b.WriteString(`<wsu:Created>` + created + `</wsu:Created>`)
	b.WriteString(`<wsu:Expires>` + expires + `</wsu:Expires>`)
	b.WriteString(`</wsu:Timestamp>`)
	b.WriteString(`<wsse:UsernameToken wsu:Id="` + randomID() + `">`)
	b.WriteString(`<wsse:Username>` + escapeXML(username) + `</wsse:Username>`)
	b.WriteString(`<wsse:Password>` + escapeXML(password) + `</wsse:Password>`)
	b.WriteString(`</wsse:UsernameToken>`)
	b.WriteString(`</wsse:Security>`)
	b.WriteString(`</s:Header>`)
	b.WriteString(`<s:Body>`)
	b.WriteString(`<wst:RequestSecurityToken>`)
	b.WriteString(`<wst:TokenType>urn:oasis:names:tc:SAML:2.0:assertion</wst:TokenType>`)
	b.WriteString(`<wst:RequestType>` + WSTrust13Namespace + `/Issue</wst:RequestType>`)
	b.WriteString(`<wsp:AppliesTo xmlns:wsp="http://schemas.xmlsoap.org/ws/2004/09/policy"><wsa:EndpointReference><wsa:Address>` + escapeXML(appliesTo) + `</wsa:Address></wsa:EndpointReference></wsp:AppliesTo>`)
	b.WriteString(`<wst:KeyType>` + WSTrust13Namespace + `/SymmetricKey</wst:KeyType>`)
	b.WriteString(`<wst:Lifetime><wsu:Created>` + created + `</wsu:Created><wsu:Expires>` + expires + `</wsu:Expires></wst:Lifetime>`)
	b.WriteString(`</wst:RequestSecurityToken>`)
	b.WriteString(`</s:Body>`)
	b.WriteString(`</s:Envelope>`)

	return []byte(b.String())
}

func escapeXML(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;", "<", "&lt;", ">", "&gt;",
		`"`, "&quot;", "'", "&apos;",
	)
	return r.Replace(s)
}

func randomUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	// Set version 4 and RFC 4122 variant bits.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// AssertionRaw is the raw assertion XML plus its declared namespace.
type AssertionRaw struct {
	Namespace string
	ID        string
	XML       string
}

// ExtractAssertion finds the first Assertion element in an RSTR body
// using a token-level scan, tolerating any envelope/fault layout and
// both SAML 1.1 and 2.0 namespaces.
func ExtractAssertion(body []byte) (*AssertionRaw, error) {
	decoder := xml.NewDecoder(bytes.NewReader(body))

	var (
		depth     int
		inAssert  bool
		startLine int
		assertNS  string
		assertID  string
		buf       bytes.Buffer
	)

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse soap response: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if inAssert {
				// Keep the inner element verbatim.
				buf.WriteByte('<')
				buf.WriteString(t.Name.Local)
				for _, attr := range t.Attr {
					buf.WriteByte(' ')
					buf.WriteString(attr.Name.Local)
					buf.WriteString(`="`)
					buf.WriteString(attr.Value)
					buf.WriteString(`"`)
				}
				buf.WriteByte('>')
				continue
			}
			if strings.HasSuffix(t.Name.Local, "Assertion") {
				inAssert = true
				startLine = depth
				assertNS = t.Name.Space
				for _, attr := range t.Attr {
					if attr.Name.Local == "ID" || attr.Name.Local == "AssertionID" {
						assertID = attr.Value
					}
				}
				buf.Reset()
				buf.WriteByte('<')
				buf.WriteString(t.Name.Local)
				for _, attr := range t.Attr {
					buf.WriteByte(' ')
					buf.WriteString(attr.Name.Local)
					buf.WriteString(`="`)
					buf.WriteString(attr.Value)
					buf.WriteString(`"`)
				}
				buf.WriteByte('>')
			}
		case xml.EndElement:
			if inAssert {
				buf.WriteString("</" + t.Name.Local + ">")
				if depth == startLine {
					return &AssertionRaw{Namespace: assertNS, ID: assertID, XML: buf.String()}, nil
				}
				depth--
				continue
			}
			depth--
		case xml.CharData:
			if inAssert {
				buf.Write(t)
			}
		case xml.ProcInst, xml.Comment, xml.Directive:
			// ignored
		}
	}

	// No assertion found: surface a SOAP fault message if present.
	if msg := extractFaultText(body); msg != "" {
		return nil, fmt.Errorf("sts fault: %s", msg)
	}
	return nil, fmt.Errorf("no assertion in rstr response")
}

// extractFaultText pulls a human-readable message out of a SOAP fault.
func extractFaultText(body []byte) string {
	// SOAP 1.2: Envelope > Body > Fault > Reason > Text
	var soap12 struct {
		Body struct {
			Fault struct {
				Reason struct {
					Text string `xml:"Text"`
				} `xml:"Reason"`
			} `xml:"Fault"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(body, &soap12); err == nil && strings.TrimSpace(soap12.Body.Fault.Reason.Text) != "" {
		return strings.TrimSpace(soap12.Body.Fault.Reason.Text)
	}

	// SOAP 1.1: Envelope > Body > Fault > faultstring
	var soap11 struct {
		Body struct {
			Fault struct {
				FaultString string `xml:"faultstring"`
			} `xml:"Fault"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(body, &soap11); err == nil && strings.TrimSpace(soap11.Body.Fault.FaultString) != "" {
		return strings.TrimSpace(soap11.Body.Fault.FaultString)
	}
	return ""
}

// ParseRSTR parses the extracted assertion into typed metadata.
func ParseRSTR(body []byte) (*types.SAMLAssertion, error) {
	raw, err := ExtractAssertion(body)
	if err != nil {
		return nil, err
	}

	parsed, err := saml.ParseAssertion(raw.XML)
	if err != nil {
		return nil, err
	}
	if parsed.ID == "" {
		parsed.ID = raw.ID
	}
	return parsed, nil
}

func nsAttr(ns string) string {
	if ns == "" {
		return ` xmlns="` + saml.NSAssertion + `"`
	}
	return ` xmlns="` + ns + `"`
}
