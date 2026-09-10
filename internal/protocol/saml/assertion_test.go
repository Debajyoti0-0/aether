package saml

import (
	"strings"
	"testing"
	"time"
)

func TestBuildAssertion(t *testing.T) {
	b := NewBuilder("https://sts.example.com/adfs/services/trust")
	a, err := b.Build(BuildOptions{
		Subject:   "user@example.com",
		Audience:  "urn:federation:MicrosoftOnline",
		Recipient: "https://login.microsoftonline.com/login.srf",
		Lifetime:  5 * time.Minute,
		Attributes: []Attribute{
			{Name: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/upn", Values: []string{"user@example.com"}},
		},
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	checks := []string{
		"<Assertion", "user@example.com", "urn:federation:MicrosoftOnline",
		"https://sts.example.com/adfs/services/trust", "upn", "user@example.com",
	}
	for _, c := range checks {
		if !strings.Contains(a.Raw, c) {
			t.Errorf("assertion missing %q", c)
		}
	}
	if a.ID == "" || a.Issuer != "https://sts.example.com/adfs/services/trust" {
		t.Errorf("meta = %+v", a)
	}
}

func TestBuildRequiresFields(t *testing.T) {
	b := NewBuilder("")
	if _, err := b.Build(BuildOptions{Subject: "u"}); err == nil {
		t.Error("expected issuer error")
	}
	b = NewBuilder("iss")
	if _, err := b.Build(BuildOptions{}); err == nil {
		t.Error("expected subject error")
	}
}

func TestParseAssertion(t *testing.T) {
	raw := `<Assertion xmlns="urn:oasis:names:tc:SAML:2.0:assertion" ID="_x" Version="2.0" IssueInstant="2026-01-01T00:00:00Z"><Issuer>iss</Issuer><Subject><NameID>u@x.com</NameID></Subject><Conditions NotBefore="2026-01-01T00:00:00Z" NotOnOrAfter="2026-01-01T00:05:00Z"><AudienceRestriction><Audience>aud</Audience></AudienceRestriction></Conditions></Assertion>`

	a, err := ParseAssertion(raw)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if a.ID != "_x" || a.Issuer != "iss" || a.Audience != "aud" {
		t.Errorf("parsed = %+v", a)
	}
	if a.NotOnOrAfter.IsZero() {
		t.Error("NotOnOrAfter should be parsed")
	}
}
