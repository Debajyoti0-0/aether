package msoapx

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"time"
)

// RegistrationRequest describes a device registration attempt.
type RegistrationRequest struct {
	Tenant       string
	DeviceName   string
	DeviceType   string // e.g. "Windows"
	OSVersion    string
	Certificate  bool // request a MDM device certificate
}

// RegistrationResult carries the outputs of a device registration.
type RegistrationResult struct {
	DeviceID       string
	CertificatePEM string
	PrivateKeyPEM  string
}

// GenerateDeviceKeyPair creates the RSA key pair used for device
// registration (TPM-attested on real Windows hosts).
func GenerateDeviceKeyPair(bits int) (*rsa.PrivateKey, string, error) {
	if bits < 2048 {
		bits = 2048
	}
	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, "", fmt.Errorf("generate key: %w", err)
	}
	pemData := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return key, string(pemData), nil
}

// BuildCSR builds a device certificate signing request.
func BuildCSR(key *rsa.PrivateKey, req RegistrationRequest) (string, error) {
	name := req.DeviceName
	if name == "" {
		name = "aether-device"
	}

	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:         name,
			Organization:       []string{req.Tenant},
			OrganizationalUnit: []string{"Devices"},
		},
		SignatureAlgorithm: x509.SHA256WithRSA,
	}

	der, err := x509.CreateCertificateRequest(rand.Reader, template, key)
	if err != nil {
		return "", fmt.Errorf("create csr: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der})), nil
}

// BuildRegistrationPayload assembles the JSON body posted to the
// device registration service (Azure AD Join / Workplace Join).
func BuildRegistrationPayload(req RegistrationRequest, csr string) map[string]any {
	if req.DeviceType == "" {
		req.DeviceType = "Windows"
	}
	if req.OSVersion == "" {
		req.OSVersion = "10.0.19045.0"
	}
	return map[string]any{
		"certificateSigningRequest": csr,
		"deviceName":                req.DeviceName,
		"deviceType":                req.DeviceType,
		"osVersion":                 req.OSVersion,
		"joinType":                  "AzureAdJoin",
		"requestedStart":            time.Now().UTC().Format(time.RFC3339),
	}
}
