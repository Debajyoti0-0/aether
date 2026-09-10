package api

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Operator is the cryptographically authenticated identity of a
// teamserver client. It is derived EXCLUSIVELY from the client
// certificate presented during the TLS handshake — never from payload
// fields (Stage 3, T1: the client-asserted `operator` JSON field was
// removed from CommandRequest).
type Operator struct {
	Name            string    // from the URI SAN aether:operator:<name>
	CertFingerprint string    // hex SHA-256 of the DER certificate
	Caps            CapabilitySet
	IssuedAt        time.Time
	ExpiresAt       time.Time
}

// OperatorURIPrefix is the SAN URI scheme that binds a certificate to
// an operator identity: aether:operator:<name>.
const OperatorURIPrefix = "aether:operator:"

// FromClientCert validates a client certificate and derives the
// operator identity from it. Fail-closed rules:
//   - no certificate → error;
//   - URI SAN missing or malformed → error;
//   - expired or not-yet-valid → error;
//   - name must pass identifier validation (no traversal/separators).
func FromClientCert(cert *x509.Certificate, now time.Time) (*Operator, error) {
	if cert == nil {
		return nil, fmt.Errorf("no client certificate presented")
	}
	if now.Before(cert.NotBefore) {
		return nil, fmt.Errorf("client certificate is not yet valid (notBefore %s)", cert.NotBefore)
	}
	if now.After(cert.NotAfter) {
		return nil, fmt.Errorf("client certificate is expired (notAfter %s)", cert.NotAfter)
	}

	name := ""
	for _, uri := range cert.URIs {
		if uri != nil && strings.HasPrefix(uri.String(), OperatorURIPrefix) {
			name = strings.TrimPrefix(uri.String(), OperatorURIPrefix)
			break
		}
	}
	if name == "" {
		return nil, fmt.Errorf("client certificate has no %s<name> URI SAN", OperatorURIPrefix)
	}
	if err := validateOperatorName(name); err != nil {
		return nil, err
	}

	sum := sha256.Sum256(cert.Raw)
	return &Operator{
		Name:            name,
		CertFingerprint: hex.EncodeToString(sum[:]),
		IssuedAt:        cert.NotBefore,
		ExpiresAt:       cert.NotAfter,
	}, nil
}

// validateOperatorName applies the workspace-name identifier policy to
// operator names (they become file paths under the operators dir).
func validateOperatorName(name string) error {
	if name == "" || name != strings.TrimSpace(name) {
		return fmt.Errorf("operator name is empty or has surrounding whitespace")
	}
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return fmt.Errorf("operator name %q contains path separators", name)
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("operator name %q contains control characters", name)
		}
	}
	return nil
}

// CanExecute reports whether the operator holds the capability required
// for a command. Unknown/missing capability ⇒ deny (fail closed).
func (o *Operator) CanExecute(capability string) bool {
	if o == nil {
		return false
	}
	return o.Caps.Has(capability)
}

// RevocationList is a plain-text name list checked at connection time
// (Stage 3 scope; OCSP/CRL distribution is Stage 4).
type RevocationList struct {
	revoked map[string]bool
}

// LoadRevocationList parses one operator name per line (# comments
// allowed). A missing file is an empty list.
func LoadRevocationList(data []byte) *RevocationList {
	rl := &RevocationList{revoked: map[string]bool{}}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		rl.revoked[line] = true
	}
	return rl
}

// IsRevoked reports whether the operator name is revoked.
func (rl *RevocationList) IsRevoked(name string) bool {
	return rl.revoked[name]
}
