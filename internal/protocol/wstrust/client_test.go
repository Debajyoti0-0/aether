package wstrust

import (
	"strings"
	"testing"
)

func TestBuildRST(t *testing.T) {
	rst := BuildRST("user@example.com", "s3cret", "urn:federation:MicrosoftOnline", "https://sts.example.com/adfs/services/trust")

	doc := string(rst)
	checks := []string{
		"user@example.com",
		"s3cret",
		"urn:federation:MicrosoftOnline",
		"https://sts.example.com/adfs/services/trust",
		WSTrust13Namespace + "/Issue",
		"RequestSecurityToken",
		"UsernameToken",
	}
	for _, c := range checks {
		if !strings.Contains(doc, c) {
			t.Errorf("RST missing %q", c)
		}
	}

	// Escaping of XML-sensitive credentials.
	rst = BuildRST("u", `<p&>"`, "a", "e")
	if !strings.Contains(string(rst), "&lt;p&amp;&gt;&quot;") {
		t.Errorf("password not escaped: %s", string(rst))
	}
}

func TestExtractAssertion(t *testing.T) {
	body := []byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope">
 <s:Body>
  <wst:RequestSecurityTokenResponseCollection xmlns:wst="http://docs.oasis-open.org/ws-sx/ws-trust/200512">
   <wst:RequestSecurityTokenResponse>
    <t:Lifecycle xmlns:t="urn:x">x</t:Lifecycle>
    <Assertion xmlns="urn:oasis:names:tc:SAML:2.0:assertion" ID="_abc" Version="2.0">
     <Issuer>https://sts</Issuer>
     <Subject><NameID>user@example.com</NameID></Subject>
    </Assertion>
   </wst:RequestSecurityTokenResponse>
  </wst:RequestSecurityTokenResponseCollection>
 </s:Body>
</s:Envelope>`)

	raw, err := ExtractAssertion(body)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}
	if raw.ID != "_abc" {
		t.Errorf("id = %q", raw.ID)
	}
	if raw.Namespace != "urn:oasis:names:tc:SAML:2.0:assertion" {
		t.Errorf("ns = %q", raw.Namespace)
	}
	if !strings.Contains(raw.XML, "user@example.com") {
		t.Errorf("assertion inner xml = %q", raw.XML)
	}
}

func TestParseRSTR(t *testing.T) {
	body := []byte(`<Envelope xmlns="http://www.w3.org/2003/05/soap-envelope"><Body>
<Assertion xmlns="urn:oasis:names:tc:SAML:2.0:assertion" ID="_id" Version="2.0">
<Issuer>https://sts</Issuer>
<Subject><NameID>u@x.com</NameID></Subject>
<Conditions NotBefore="2026-01-01T00:00:00Z" NotOnOrAfter="2026-01-01T00:05:00Z"><AudienceRestriction><Audience>urn:federation:MicrosoftOnline</Audience></AudienceRestriction></Conditions>
</Assertion>
</Body></Envelope>`)

	assertion, err := ParseRSTR(body)
	if err != nil {
		t.Fatalf("parse rstr failed: %v", err)
	}
	if assertion.ID != "_id" {
		t.Errorf("id = %q", assertion.ID)
	}
	if assertion.Issuer != "https://sts" {
		t.Errorf("issuer = %q", assertion.Issuer)
	}
	if assertion.Audience != "urn:federation:MicrosoftOnline" {
		t.Errorf("audience = %q", assertion.Audience)
	}
}

func TestExtractAssertionFault(t *testing.T) {
	body := []byte(`<Envelope xmlns="http://www.w3.org/2003/05/soap-envelope"><Body>
<Fault xmlns="http://www.w3.org/2003/05/soap-envelope"><Reason><Text xml:lang="en">Authentication failed</Text></Reason></Fault>
</Body></Envelope>`)

	_, err := ExtractAssertion(body)
	if err == nil || !strings.Contains(err.Error(), "Authentication failed") {
		t.Errorf("expected fault error, got %v", err)
	}
}
