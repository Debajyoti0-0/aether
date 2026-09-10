package api

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

func parseURI(raw string) (*url.URL, error) {
	return url.Parse(raw)
}

// Teamserver PKI (Stage 3, T1): a real CA hierarchy replaces the
// self-signed client/server certificates. `serve cert init` creates
// the CA and server pair; `serve cert issue` mints operator client
// certificates bound to the URI SAN aether:operator:<name>.
//
// Algorithm note: ECDSA P-256 is used (instead of the directive's
// Ed25519 sketch) for maximal interop with non-Go TLS tooling; the
// trust properties are identical. Documented in stage3-evidence.md.

// CertPaths describes the four files a live teamserver requires.
type CertPaths struct {
	CAKey    string // teamserver-ca.key
	CACert   string // teamserver-ca.crt
	SrvKey   string // teamserver-server.key
	SrvCert  string // teamserver-server.crt
}

// InitCA creates the teamserver CA (CA=true, pathlen=1) and a server
// certificate signed by it (SANs: 127.0.0.1, localhost + extras).
// Existing files are never overwritten (fail closed).
func InitCA(dir string, extraHosts []string) (*CertPaths, error) {
	paths := &CertPaths{
		CAKey:   filepath.Join(dir, "teamserver-ca.key"),
		CACert:  filepath.Join(dir, "teamserver-ca.crt"),
		SrvKey:  filepath.Join(dir, "teamserver-server.key"),
		SrvCert: filepath.Join(dir, "teamserver-server.crt"),
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	for _, p := range []string{paths.CAKey, paths.CACert, paths.SrvKey, paths.SrvCert} {
		if _, err := os.Stat(p); err == nil {
			return nil, fmt.Errorf("refusing to overwrite existing %s (delete it explicitly to rotate)", p)
		}
	}

	// CA.
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          bigSerial(),
		Subject:               pkix.Name{CommonName: "Aether Teamserver CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, err
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return nil, err
	}
	if err := writeKeyPEM(paths.CAKey, caKey); err != nil {
		return nil, err
	}
	if err := writeCertPEM(paths.CACert, caDER); err != nil {
		return nil, err
	}

	// Server certificate signed by the CA.
	hosts := append([]string{"127.0.0.1", "localhost"}, extraHosts...)
	srvKey, srvDER, err := issueCert(caCert, caKey, "aether-teamserver", nil, hosts, 365)
	if err != nil {
		return nil, err
	}
	if err := writeKeyPEM(paths.SrvKey, srvKey); err != nil {
		return nil, err
	}
	if err := writeCertPEM(paths.SrvCert, srvDER); err != nil {
		return nil, err
	}
	return paths, nil
}

// IssueOperatorCert mints a client certificate for an operator:
// CN=<name>, URI SAN aether:operator:<name>, ExtKeyUsageClientAuth.
// Returns the PEM cert + key bytes (the CLI writes them to disk).
func IssueOperatorCert(caCertPath, caKeyPath, name string, days int) (certPEM, keyPEM []byte, err error) {
	if err := validateOperatorName(name); err != nil {
		return nil, nil, err
	}
	caCert, caKey, err := loadCA(caCertPath, caKeyPath)
	if err != nil {
		return nil, nil, err
	}
	if days <= 0 || days > 3650 {
		days = 365
	}
	uri := OperatorURIPrefix + name
	key, der, err := issueCert(caCert, caKey, name, []string{uri}, nil, days)
	if err != nil {
		return nil, nil, err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM, nil
}

// LoadServerTLS assembles the server-side TLS inputs: the server
// keypair and the CA pool used to verify client certificates.
func LoadServerTLS(srvCert, srvKey, caCert string) (tls.Certificate, *x509.CertPool, error) {
	cert, err := os.ReadFile(srvCert)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("read server cert: %w", err)
	}
	key, err := os.ReadFile(srvKey)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("read server key: %w", err)
	}
	pair, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("load server keypair: %w", err)
	}
	caPEM, err := os.ReadFile(caCert)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("read CA cert: %w", err)
	}
	p := x509.NewCertPool()
	if !p.AppendCertsFromPEM(caPEM) {
		return tls.Certificate{}, nil, fmt.Errorf("CA cert %s contains no certificates", caCert)
	}
	return pair, p, nil
}

// LoadOperatorTLS assembles the client-side TLS inputs: the operator
// keypair and the CA pool used to verify the server certificate.
func LoadOperatorTLS(certPath, keyPath, caCert string) (tls.Certificate, *x509.CertPool, error) {
	cert, err := os.ReadFile(certPath)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("read operator cert: %w", err)
	}
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("read operator key: %w", err)
	}
	pair, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("load operator keypair: %w", err)
	}
	caPEM, err := os.ReadFile(caCert)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("read server CA cert: %w", err)
	}
	p := x509.NewCertPool()
	if !p.AppendCertsFromPEM(caPEM) {
		return tls.Certificate{}, nil, fmt.Errorf("server CA cert %s contains no certificates", caCert)
	}
	return pair, p, nil
}

func loadCA(caCertPath, caKeyPath string) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	der, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read CA cert: %w", err)
	}
	block, _ := pem.Decode(der)
	if block == nil {
		return nil, nil, fmt.Errorf("CA cert %s is not PEM", caCertPath)
	}
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, err
	}
	keyPEM, err := os.ReadFile(caKeyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read CA key: %w", err)
	}
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("CA key %s is not PEM", caKeyPath)
	}
	caKey, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}
	return caCert, caKey, nil
}

func issueCert(ca *x509.Certificate, caKey *ecdsa.PrivateKey, cn string, uris, hosts []string, days int) (*ecdsa.PrivateKey, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          bigSerial(),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(0, 0, days),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	for _, u := range uris {
		parsed, err := parseURI(u)
		if err != nil {
			return nil, nil, err
		}
		tmpl.URIs = append(tmpl.URIs, parsed)
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}
	return key, der, nil
}

func writeKeyPEM(path string, key *ecdsa.PrivateKey) error {
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), 0o600)
}

func writeCertPEM(path string, der []byte) error {
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600)
}

func bigSerial() *big.Int {
	s, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	return s
}
