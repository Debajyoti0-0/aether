package wstrust

import (
	"strings"
	"testing"
)

func TestBuildDowngradeRST(t *testing.T) {
	rst := BuildDowngradeRST(DowngradeRequest{
		Username:    "user@corp.com",
		DeviceClaim: "device-guid-123",
		Endpoint:    "https://sts.corp.com/adfs/services/trust/2005/usernamemixed",
	})

	doc := string(rst)
	checks := []string{
		AuthClassPassword,
		ClaimAuthMethod,
		ClaimName,
		"user@corp.com",
		"device-guid-123",
		"usernamemixed",
		"<wst:AuthMethod>",
		"<wst:Claims>",
	}
	for _, c := range checks {
		if !strings.Contains(doc, c) {
			t.Errorf("downgrade RST missing %q", c)
		}
	}
}

func TestDowngradeRSTHelper(t *testing.T) {
	rst := DowngradeRST("user@corp.com", "")
	doc := string(rst)
	if !strings.Contains(doc, AuthClassPassword) {
		t.Error("default auth class should be password")
	}
	if strings.Contains(doc, "immutableid") {
		t.Error("no device claim should omit immutableid")
	}
}

func TestDowngradeRSTXMLEscaping(t *testing.T) {
	rst := DowngradeRST(`u&ser<>`, "")
	if !strings.Contains(string(rst), "u&amp;ser&lt;&gt;") {
		t.Error("username not escaped")
	}
}

const sampleMEX = `<?xml version="1.0" encoding="utf-8"?>
<wsdl:definitions xmlns:wsdl="http://schemas.xmlsoap.org/wsdl/"
    xmlns:soap12="http://schemas.xmlsoap.org/wsdl/soap12/"
    xmlns:soap="http://schemas.xmlsoap.org/wsdl/soap/">
  <wsdl:service name="SecurityTokenService">
    <wsdl:port name="CustomBinding_IWSTrust13Async1">
      <soap12:address location="https://sts.corp.com/adfs/services/trust/2005/usernamemixed"/>
    </wsdl:port>
    <wsdl:port name="CustomBinding_IWSTrust13Async2">
      <soap12:address location="https://sts.corp.com/adfs/services/trust/13/windowstransport"/>
    </wsdl:port>
    <wsdl:port name="CustomBinding_IWSTrust1Async">
      <soap:address location="https://sts.corp.com/adfs/services/trust/2005/certificatemixed"/>
    </wsdl:port>
  </wsdl:service>
</wsdl:definitions>`

func TestParseMEX(t *testing.T) {
	doc, err := ParseMEX([]byte(sampleMEX))
	if err != nil {
		t.Fatalf("parse mex: %v", err)
	}
	if len(doc.TrustEndpoints) != 3 {
		t.Fatalf("endpoints = %d, want 3", len(doc.TrustEndpoints))
	}
	if doc.TrustEndpoints[0].Version != "2005" {
		t.Errorf("version = %q", doc.TrustEndpoints[0].Version)
	}
}

func TestDetectDowngradeOpportunity(t *testing.T) {
	endpoint, feasible, err := DetectDowngradeOpportunity([]byte(sampleMEX))
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if !feasible {
		t.Fatal("downgrade should be feasible with usernamemixed endpoint")
	}
	if endpoint != "https://sts.corp.com/adfs/services/trust/2005/usernamemixed" {
		t.Errorf("endpoint = %q", endpoint)
	}
}

func TestDetectDowngradeNoUsernamemixed(t *testing.T) {
	mex := strings.ReplaceAll(sampleMEX, "usernamemixed", "windowstransport")
	_, feasible, err := DetectDowngradeOpportunity([]byte(mex))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if feasible {
		t.Error("expected infeasible without usernamemixed endpoint")
	}
}

func TestParseMEXGarbage(t *testing.T) {
	if _, err := ParseMEX([]byte("this is not xml")); err == nil {
		t.Error("expected parse error")
	}
}
