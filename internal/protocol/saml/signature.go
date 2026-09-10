package saml

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
)

// Signer signs SAML assertions with an RSA key (XML-DSig style detached
// signing of the canonicalized Assertion element).
type Signer struct {
	Key        *rsa.PrivateKey
	CertBase64 string // base64 DER of the X.509 certificate
}

// NewSigner builds a signer from a PEM/DER private key and certificate.
func NewSigner(keyPEM []byte, certBase64 string) (*Signer, error) {
	key, err := x509.ParsePKCS1PrivateKey(keyPEM)
	if err != nil {
		// Try PKCS8 as a fallback.
		var k any
		k, err2 := x509.ParsePKCS8PrivateKey(keyPEM)
		if err2 != nil {
			return nil, fmt.Errorf("parse private key (pkcs1: %v, pkcs8: %v)", err, err2)
		}
		rk, ok := k.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is %T, want RSA", k)
		}
		key = rk
	}
	return &Signer{Key: key, CertBase64: certBase64}, nil
}

// SignedInfoDigest computes the SHA-256 digest of the assertion body,
// then signs it — the core operation behind enveloped XML signatures.
// Returns the base64 SignatureValue to embed in a <ds:Signature> block.
func (s *Signer) SignDigest(canonicalXML []byte) (sigValueB64, digestB64 string, err error) {
	sum := sha256.Sum256(canonicalXML)
	digestB64 = base64.StdEncoding.EncodeToString(sum[:])

	sig, err := rsa.SignPKCS1v15(rand.Reader, s.Key, crypto.SHA256, sum[:])
	if err != nil {
		return "", "", fmt.Errorf("sign digest: %w", err)
	}
	return base64.StdEncoding.EncodeToString(sig), digestB64, nil
}

// VerifyDigest verifies a signature over a canonical assertion body.
func VerifyDigest(pub *rsa.PublicKey, canonicalXML []byte, sigB64 string) error {
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	sum := sha256.Sum256(canonicalXML)
	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sig)
}

// KeyInfoXML renders the <ds:KeyInfo> block for embedding in signatures.
func (s *Signer) KeyInfoXML() string {
	if s.CertBase64 == "" {
		return ""
	}
	return fmt.Sprintf(`<ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#"><ds:X509Data><ds:X509Certificate>%s</ds:X509Certificate></ds:X509Data></ds:KeyInfo>`, s.CertBase64)
}
