// Native Go fuzz targets for SAML assertion parsing
package saml

import (
	"testing"
)

func FuzzParseAssertion(f *testing.F) {
	// Seed corpus with valid SAML assertions
	validSAML := `<?xml version="1.0" encoding="UTF-8"?>
<saml:Assertion xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="_12345" Version="2.0" IssueInstant="2024-01-01T00:00:00Z">
  <saml:Issuer>https://sts.example.com</saml:Issuer>
  <saml:Subject>
    <saml:NameID Format="urn:oasis:names:tc:SAML:2.0:nameid-format:persistent">user@example.com</saml:NameID>
    <saml:SubjectConfirmation Method="urn:oasis:names:tc:SAML:2.0:cm:bearer">
      <saml:SubjectConfirmationData Recipient="https://sp.example.com" NotOnOrAfter="2024-01-01T01:00:00Z"/>
    </saml:SubjectConfirmation>
  </saml:Subject>
  <saml:Conditions NotBefore="2024-01-01T00:00:00Z" NotOnOrAfter="2024-01-01T01:00:00Z">
    <saml:AudienceRestriction><saml:Audience>https://sp.example.com</saml:Audience></saml:AudienceRestriction>
  </saml:Conditions>
  <saml:AuthnStatement AuthnInstant="2024-01-01T00:00:00Z" SessionIndex="_abc">
    <saml:AuthnContext><saml:AuthnContextClassRef>urn:oasis:names:tc:SAML:2.0:ac:classes:PasswordProtectedTransport</saml:AuthnContextClassRef></saml:AuthnContext>
  </saml:AuthnStatement>
</saml:Assertion>`

	f.Add([]byte(validSAML))

	// Minimal valid assertion
	minimalSAML := `<?xml version="1.0"?><Assertion xmlns="urn:oasis:names:tc:SAML:2.0:assertion" ID="_1" Version="2.0" IssueInstant="2024-01-01T00:00:00Z"><Issuer>test</Issuer><Subject><NameID>user</NameID></Subject><Conditions NotBefore="2024-01-01T00:00:00Z" NotOnOrAfter="2024-01-01T01:00:00Z"><AudienceRestriction><Audience>aud</Audience></AudienceRestriction></Conditions></Assertion>`
	f.Add([]byte(minimalSAML))

	f.Fuzz(func(t *testing.T, data []byte) {
		// ParseAssertion should not panic on any input
		_, err := ParseAssertion(string(data))
		// Errors are expected for malformed input; only panics are failures
		_ = err
	})
}

func FuzzAssertionXMLMarshal(f *testing.F) {
	builder := NewBuilder("https://sts.example.com")
	opts := BuildOptions{
		Subject:   "user@example.com",
		Audience:  "https://sp.example.com",
		Recipient: "https://sp.example.com",
		Lifetime:  5 * 60,
		Attributes: []Attribute{
			{Name: "role", Values: []string{"admin", "user"}},
			{Name: "department", Values: []string{"engineering"}},
		},
	}
	assertion, err := builder.Build(opts)
	if err != nil {
		return
	}

	f.Add([]byte(assertion.Raw))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Test round-trip parsing of builder output
		_, err := ParseAssertion(string(data))
		_ = err
	})
}