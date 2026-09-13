// Native Go fuzz targets for WS-Trust parsing
package wstrust

import (
	"testing"
)

func FuzzParseRSTR(f *testing.F) {
	// Valid RSTR with SAML assertion
	validRSTR := `<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope">
  <s:Body>
    <wst:RequestSecurityTokenResponse xmlns:wst="http://docs.oasis-open.org/ws-sx/ws-trust/200512">
      <wst:RequestedSecurityToken>
        <saml:Assertion xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="_123" Version="2.0" IssueInstant="2024-01-01T00:00:00Z">
          <saml:Issuer>https://sts.example.com</saml:Issuer>
          <saml:Subject><saml:NameID>user@example.com</saml:NameID></saml:Subject>
        </saml:Assertion>
      </wst:RequestedSecurityToken>
    </wst:RequestSecurityTokenResponse>
  </s:Body>
</s:Envelope>`

	f.Add([]byte(validRSTR))

	// SOAP 1.1 fault
	soap11Fault := `<?xml version="1.0"?>
<SOAP-ENV:Envelope xmlns:SOAP-ENV="http://schemas.xmlsoap.org/soap/envelope/">
  <SOAP-ENV:Body>
    <SOAP-ENV:Fault>
      <faultcode>SOAP-ENV:Client</faultcode>
      <faultstring>Invalid credentials</faultstring>
    </SOAP-ENV:Fault>
  </SOAP-ENV:Body>
</SOAP-ENV:Envelope>`
	f.Add([]byte(soap11Fault))

	// SOAP 1.2 fault
	soap12Fault := `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope">
  <s:Body>
    <s:Fault>
      <s:Code><s:Value>s:Sender</s:Value></s:Code>
      <s:Reason><s:Text>Authentication failed</s:Text></s:Reason>
    </s:Fault>
  </s:Body>
</s:Envelope>`
	f.Add([]byte(soap12Fault))

	f.Fuzz(func(t *testing.T, data []byte) {
		// ParseRSTR should not panic on any input
		_, err := ParseRSTR(data)
		_ = err
	})
}

func FuzzExtractAssertionWS(f *testing.F) {
	f.Add([]byte(`<Assertion ID="_1" xmlns="urn:oasis:names:tc:SAML:2.0:assertion"><Issuer>test</Issuer></Assertion>`))
	f.Add([]byte(`<saml:Assertion xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="_1"><saml:Issuer>test</saml:Issuer></saml:Assertion>`))
	f.Add([]byte(`<InvalidElement>not an assertion</InvalidElement>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		_, err := ExtractAssertion(data)
		_ = err
	})
}

func FuzzBuildRST(f *testing.F) {
	f.Add([]byte("user@example.com"))
	f.Add([]byte("password123"))
	f.Add([]byte(""))
	f.Add([]byte("very long username that might cause issues with buffer handling in the XML builder"))

	f.Fuzz(func(t *testing.T, username []byte) {
		// BuildRST should not panic on any username/password input
		rst := BuildRST(string(username), "password", "urn:federation:MicrosoftOnline", "https://sts.example.com")
		_ = rst
	})
}

func FuzzParseMEX(f *testing.F) {
	validMEX := `<?xml version="1.0" encoding="UTF-8"?>
<wsdl:definitions xmlns:wsdl="http://schemas.xmlsoap.org/wsdl/" xmlns:wsp="http://schemas.xmlsoap.org/ws/2004/09/policy" xmlns:wst="http://docs.oasis-open.org/ws-sx/ws-trust/200512" targetNamespace="https://sts.example.com">
  <wsdl:import namespace="http://docs.oasis-open.org/ws-sx/ws-trust/200512" location="https://sts.example.com/mex"/>
  <wsp:Policy wsu:Id="UsernameMixedPolicy">
    <wsp:ExactlyOne>
      <wsp:All>
        <sp:TransportBinding xmlns:sp="http://schemas.xmlsoap.org/ws/2005/07/securitypolicy">
          <wsp:Policy>
            <sp:TransportToken>
              <wsp:Policy>
                <sp:HttpsToken RequireClientCertificate="false"/>
              </wsp:Policy>
            </sp:TransportToken>
          </wsp:Policy>
        </sp:TransportBinding>
      </wsp:All>
    </wsp:ExactlyOne>
  </wsp:Policy>
</wsdl:definitions>`

	f.Add([]byte(validMEX))
	f.Add([]byte(`<definitions></definitions>`))
	f.Add([]byte(`not xml at all`))

	f.Fuzz(func(t *testing.T, data []byte) {
		_, feasible, err := DetectDowngradeOpportunity(data)
		_ = feasible
		_ = err
	})
}