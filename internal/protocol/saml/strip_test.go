package saml

import (
	"strings"
	"testing"
)

const signedAssertion = `<Assertion xmlns="urn:oasis:names:tc:SAML:2.0:assertion" ID="_x">
<Issuer>https://sts</Issuer>
<ds:Signature xmlns:ds="http://www.w3.org/2000/09/xmldsig#"><ds:SignedInfo><ds:Reference/></ds:SignedInfo></ds:Signature>
<Subject><NameID>u@x.com</NameID></Subject>
</Assertion>`

func TestStripSignature(t *testing.T) {
	out, n, err := StripSignature(signedAssertion)
	if err != nil {
		t.Fatalf("strip: %v", err)
	}
	if n != 1 {
		t.Errorf("stripped = %d, want 1", n)
	}
	if strings.Contains(out, "ds:Signature") {
		t.Errorf("signature not removed: %s", out)
	}
	if !strings.Contains(out, "<NameID>u@x.com</NameID>") {
		t.Errorf("assertion body damaged: %s", out)
	}
}

func TestStripSignatureMultiple(t *testing.T) {
	doc := `<Response><Assertion ID="a">` +
		`<ds:Signature xmlns:ds="x"><a/></ds:Signature>` +
		`</Assertion><Assertion ID="b">` +
		`<ds:Signature xmlns:ds="x"><b/></ds:Signature>` +
		`</Assertion></Response>`

	out, n, err := StripSignature(doc)
	if err != nil {
		t.Fatalf("strip: %v", err)
	}
	if n != 2 {
		t.Errorf("stripped = %d, want 2", n)
	}
	if strings.Contains(out, "Signature") {
		t.Errorf("signatures remain: %s", out)
	}
}

func TestStripSelfClosingSignature(t *testing.T) {
	doc := `<Assertion ID="x"><ds:Signature xmlns:ds="y"/></Assertion>`
	out, n, err := StripSignature(doc)
	if err != nil {
		t.Fatalf("strip: %v", err)
	}
	if n != 1 {
		t.Errorf("stripped = %d", n)
	}
	if strings.Contains(out, "Signature") {
		t.Errorf("self-closing not removed: %s", out)
	}
}

func TestStripNoSignature(t *testing.T) {
	out, n, err := StripSignature(signedAssertion)
	_ = out
	_ = err
	_ = n
	// Re-run against an unsigned doc.
	unsigned := `<Assertion ID="y"><Issuer>iss</Issuer></Assertion>`
	out, n, err = StripSignature(unsigned)
	if err != nil {
		t.Fatalf("strip unsigned: %v", err)
	}
	if n != 0 {
		t.Errorf("stripped = %d, want 0", n)
	}
	if out != unsigned {
		t.Errorf("unsigned doc modified: %s", out)
	}
}

func TestStripNotSAML(t *testing.T) {
	if _, _, err := StripSignature("<html><body>nope</body></html>"); err == nil {
		t.Error("non-SAML input should fail")
	}
}

func TestStripDocumentMeta(t *testing.T) {
	res, err := StripDocument(signedAssertion)
	if err != nil {
		t.Fatal(err)
	}
	if res.Stripped != 1 || !res.HasAssertion {
		t.Errorf("result = %+v", res)
	}
}
