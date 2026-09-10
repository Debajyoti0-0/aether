package saml

import (
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/Debajyoti0-0/aether/internal/types"
)

// Namespaces used in SAML 2.0 documents.
const (
	NSAssertion = "urn:oasis:names:tc:SAML:2.0:assertion"
	NSProtocol  = "urn:oasis:names:tc:SAML:2.0:protocol"
)

// Builder constructs SAML 2.0 assertions for forging and testing.
type Builder struct {
	Issuer      string
	Certificate string // base64 DER (optional, for signing)
}

// NewBuilder creates an assertion builder.
func NewBuilder(issuer string) *Builder {
	return &Builder{Issuer: issuer}
}

func randomID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "_" + fmt.Sprintf("%x", b)
}

// assertionXML is the template marshalled into the final document.
type assertionXML struct {
	XMLName xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion Assertion"`
	ID      string   `xml:"ID,attr"`
	Version string   `xml:"Version,attr"`
	IssueInstant string `xml:"IssueInstant,attr"`
	Issuer  string   `xml:"Issuer"`
	Subject struct {
		NameID struct {
			Value string `xml:",chardata"`
			Format string `xml:"Format,attr,omitempty"`
		} `xml:"NameID"`
		SubjectConfirmation struct {
			Method string `xml:"Method,attr"`
			SubjectConfirmationData struct {
				Recipient string `xml:"Recipient,attr,omitempty"`
				InResponseTo string `xml:"InResponseTo,attr,omitempty"`
				NotOnOrAfter string `xml:"NotOnOrAfter,attr,omitempty"`
			} `xml:"SubjectConfirmationData"`
		} `xml:"SubjectConfirmation"`
	} `xml:"Subject"`
	Conditions struct {
		NotBefore           string `xml:"NotBefore,attr"`
		NotOnOrAfter        string `xml:"NotOnOrAfter,attr"`
		AudienceRestriction struct {
			Audience string `xml:"Audience"`
		} `xml:"AudienceRestriction"`
	} `xml:"Conditions"`
	AuthnStatement struct {
		AuthnInstant string `xml:"AuthnInstant,attr"`
		SessionIndex string `xml:"SessionIndex,attr"`
		AuthnContext struct {
			AuthnContextClassRef string `xml:"AuthnContextClassRef"`
		} `xml:"AuthnContext"`
	} `xml:"AuthnStatement"`
	AttributeStatements []attributeStatementXML `xml:"AttributeStatement"`
}

type attributeStatementXML struct {
	Attributes []attributeXML `xml:"Attribute"`
}

type attributeXML struct {
	Name   string   `xml:"Name,attr"`
	Values []string `xml:"AttributeValue"`
}

// Attribute is a SAML attribute (name + values).
type Attribute struct {
	Name   string
	Values []string
}

// Build produces a signed-pending (unsigned) SAML assertion.
func (b *Builder) Build(opts BuildOptions) (*types.SAMLAssertion, error) {
	if b.Issuer == "" && opts.Issuer != "" {
		b.Issuer = opts.Issuer
	}
	if b.Issuer == "" {
		return nil, fmt.Errorf("issuer is required")
	}
	if opts.Subject == "" {
		return nil, fmt.Errorf("subject is required")
	}

	now := time.Now().UTC()
	lifetime := opts.Lifetime
	if lifetime <= 0 {
		lifetime = 5 * time.Minute
	}

	a := assertionXML{
		ID:           randomID(),
		Version:      "2.0",
		IssueInstant: now.Format(time.RFC3339),
	}
	a.Issuer = b.Issuer
	a.Subject.NameID.Value = opts.Subject
	a.Subject.NameID.Format = "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent"
	a.Subject.SubjectConfirmation.Method = "urn:oasis:names:tc:SAML:2.0:cm:bearer"
	a.Subject.SubjectConfirmation.SubjectConfirmationData.Recipient = opts.Recipient
	a.Subject.SubjectConfirmation.SubjectConfirmationData.InResponseTo = opts.InResponseTo
	a.Subject.SubjectConfirmation.SubjectConfirmationData.NotOnOrAfter = now.Add(lifetime).Format(time.RFC3339)
	a.Conditions.NotBefore = now.Format(time.RFC3339)
	a.Conditions.NotOnOrAfter = now.Add(lifetime).Format(time.RFC3339)
	a.Conditions.AudienceRestriction.Audience = opts.Audience
	a.AuthnStatement.AuthnInstant = now.Format(time.RFC3339)
	a.AuthnStatement.SessionIndex = randomID()
	a.AuthnStatement.AuthnContext.AuthnContextClassRef =
		"urn:oasis:names:tc:SAML:2.0:ac:classes:PasswordProtectedTransport"

	if len(opts.Attributes) > 0 {
		stmt := attributeStatementXML{}
		for _, attr := range opts.Attributes {
			stmt.Attributes = append(stmt.Attributes, attributeXML{
				Name:   attr.Name,
				Values: attr.Values,
			})
		}
		a.AttributeStatements = append(a.AttributeStatements, stmt)
	}

	raw, err := xml.MarshalIndent(a, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal assertion: %w", err)
	}

	doc := xml.Header[:len(xml.Header)-1] + "\n" + string(raw)
	return &types.SAMLAssertion{
		Raw:       doc,
		ID:        a.ID,
		Issuer:    b.Issuer,
		Audience:  opts.Audience,
		NotBefore: now,
		NotOnOrAfter: now.Add(lifetime),
		Claims:    claimsMap(opts.Attributes),
	}, nil
}

// BuildOptions configures assertion construction.
type BuildOptions struct {
	Issuer       string
	Subject      string // NameID
	Audience     string
	Recipient    string
	InResponseTo string
	Lifetime     time.Duration
	Attributes   []Attribute
}

func claimsMap(attrs []Attribute) map[string]string {
	m := map[string]string{}
	for _, a := range attrs {
		if len(a.Values) > 0 {
			m[a.Name] = strings.Join(a.Values, ",")
		}
	}
	return m
}

// ParseAssertion extracts core fields from a raw SAML assertion document.
func ParseAssertion(raw string) (*types.SAMLAssertion, error) {
	var parsed struct {
		ID           string `xml:"ID,attr"`
		Issuer       string `xml:"Issuer"`
		Subject      struct {
			NameID struct {
				Value string `xml:",chardata"`
			} `xml:"NameID"`
		} `xml:"Subject"`
		Conditions struct {
			NotBefore           string `xml:"NotBefore,attr"`
			NotOnOrAfter        string `xml:"NotOnOrAfter,attr"`
			AudienceRestriction struct {
				Audience string `xml:"Audience"`
			} `xml:"AudienceRestriction"`
		} `xml:"Conditions"`
	}

	if err := xml.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("parse assertion: %w", err)
	}

	out := &types.SAMLAssertion{
		Raw:      raw,
		ID:       parsed.ID,
		Issuer:   parsed.Issuer,
		Audience: parsed.Conditions.AudienceRestriction.Audience,
	}
	if t, err := time.Parse(time.RFC3339, parsed.Conditions.NotBefore); err == nil {
		out.NotBefore = t
	}
	if t, err := time.Parse(time.RFC3339, parsed.Conditions.NotOnOrAfter); err == nil {
		out.NotOnOrAfter = t
	}
	return out, nil
}
