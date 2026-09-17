package revocation

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ocsp"
)

// RevocationMode specifies the revocation checking mode.
type RevocationMode string

const (
	ModeNone RevocationMode = "none" // No revocation checking (file fallback)
	ModeOCSP RevocationMode = "ocsp" // OCSP checking
	ModeCRL  RevocationMode = "crl"  // CRL checking
	ModeFile RevocationMode = "file" // File-based revocation list
)

// RevocationStatus represents the result of a revocation check.
type RevocationStatus int

const (
	StatusGood RevocationStatus = iota
	StatusRevoked
	StatusUnknown
	StatusError
)

func (s RevocationStatus) String() string {
	switch s {
	case StatusGood:
		return "GOOD"
	case StatusRevoked:
		return "REVOKED"
	case StatusUnknown:
		return "UNKNOWN"
	case StatusError:
		return "ERROR"
	default:
		return "UNDEFINED"
	}
}

// Result holds the result of a revocation check.
type Result struct {
	Status     RevocationStatus
	Serial     string
	Reason     string
	ThisUpdate time.Time
	NextUpdate time.Time
	Error      error
}

// Config holds configuration for the revocation checker.
type Config struct {
	Mode           RevocationMode
	OCSPResponder  string        // OCSP responder URL (optional, from cert AIA if empty)
	CRLURL         string        // CRL distribution point URL (optional, from cert CDP if empty)
	CRLFile        string        // Local CRL file path
	RevocationFile string        // File-based revocation list (serial numbers, one per line)
	Timeout        time.Duration // HTTP timeout
	CacheTTL       time.Duration // Cache TTL for OCSP/CRL responses
	FailClosed     bool          // Fail-closed on network/validation errors
	SkipSignature  bool          // Skip signature verification (for testing)
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Mode:       ModeFile,
		Timeout:    10 * time.Second,
		CacheTTL:   5 * time.Minute,
		FailClosed: true,
	}
}

// Checker performs revocation checks.
type Checker struct {
	config Config
	cache  sync.Map // map[string]*cachedResponse
}

type cachedResponse struct {
	result    Result
	expiresAt time.Time
}

// NewChecker creates a new revocation checker.
func NewChecker(config Config) *Checker {
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 5 * time.Minute
	}
	return &Checker{config: config}
}

// Check checks the revocation status of a certificate against its issuer.
func (c *Checker) Check(ctx context.Context, cert, issuer *x509.Certificate) Result {
	switch c.config.Mode {
	case ModeOCSP:
		return c.checkOCSP(ctx, cert, issuer)
	case ModeCRL:
		return c.checkCRL(ctx, cert, issuer)
	case ModeFile:
		return c.checkFile(cert)
	case ModeNone:
		return Result{Status: StatusGood, Reason: "revocation checking disabled"}
	default:
		return Result{Status: StatusError, Error: fmt.Errorf("unknown revocation mode: %s", c.config.Mode)}
	}
}

// CheckSerial checks the revocation status by serial number.
func (c *Checker) CheckSerial(ctx context.Context, serial string, issuer *x509.Certificate) Result {
	switch c.config.Mode {
	case ModeOCSP:
		return c.checkOCSPBySerial(ctx, serial, issuer)
	case ModeCRL:
		return c.checkCRLBySerial(ctx, serial, issuer)
	case ModeFile:
		return c.checkFileBySerial(serial)
	case ModeNone:
		return Result{Status: StatusGood, Reason: "revocation checking disabled"}
	default:
		return Result{Status: StatusError, Error: fmt.Errorf("unknown revocation mode: %s", c.config.Mode)}
	}
}

// checkOCSP performs an OCSP check for a certificate.
func (c *Checker) checkOCSP(ctx context.Context, cert, issuer *x509.Certificate) Result {
	// Get OCSP responder URL
	responder := c.config.OCSPResponder
	if responder == "" {
		if len(cert.OCSPServer) > 0 {
			responder = cert.OCSPServer[0]
		}
		if responder == "" {
			if c.config.FailClosed {
				return Result{Status: StatusError, Error: errors.New("no OCSP responder URL available")}
			}
			return Result{Status: StatusUnknown, Reason: "no OCSP responder URL"}
		}
	}

	// Check cache
	cacheKey := "ocsp:" + cert.SerialNumber.String()
	if cached, ok := c.cache.Load(cacheKey); ok {
		cachedResp := cached.(*cachedResponse)
		if time.Now().Before(cachedResp.expiresAt) {
			return cachedResp.result
		}
	}

	// Create OCSP request
	opts := ocsp.RequestOptions{
		Hash: crypto.SHA256,
	}
	req, err := ocsp.CreateRequest(cert, issuer, &opts)
	if err != nil {
		return Result{Status: StatusError, Error: fmt.Errorf("create OCSP request: %w", err)}
	}

	// Send request
	httpClient := &http.Client{Timeout: c.config.Timeout}
	reqBody := bytes.NewReader(req)
	resp, err := httpClient.Post(responder, "application/ocsp-request", reqBody)
	if err != nil {
		if c.config.FailClosed {
			return Result{Status: StatusError, Error: fmt.Errorf("OCSP request failed: %w", err)}
		}
		return Result{Status: StatusUnknown, Reason: "OCSP request failed"}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if c.config.FailClosed {
			return Result{Status: StatusError, Error: fmt.Errorf("OCSP responder returned status %d", resp.StatusCode)}
		}
		return Result{Status: StatusUnknown, Reason: fmt.Sprintf("OCSP responder status %d", resp.StatusCode)}
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{Status: StatusError, Error: fmt.Errorf("read OCSP response: %w", err)}
	}

	// Parse OCSP response
	ocspResp, err := ocsp.ParseResponseForCert(respBody, cert, issuer)
	if err != nil {
		if c.config.FailClosed {
			return Result{Status: StatusError, Error: fmt.Errorf("parse OCSP response: %w", err)}
		}
		return Result{Status: StatusUnknown, Reason: "parse OCSP response failed"}
	}

	// Check signature if not skipped
	if !c.config.SkipSignature {
		if err := ocspResp.CheckSignatureFrom(issuer); err != nil {
			return Result{Status: StatusError, Error: fmt.Errorf("OCSP signature verification failed: %w", err)}
		}
	}

	// Check status
	var status RevocationStatus
	switch ocspResp.Status {
	case ocsp.Good:
		status = StatusGood
	case ocsp.Revoked:
		status = StatusRevoked
	case ocsp.Unknown:
		status = StatusUnknown
	default:
		status = StatusUnknown
	}

	result := Result{
		Status:     status,
		Serial:     cert.SerialNumber.String(),
		ThisUpdate: ocspResp.ThisUpdate,
		NextUpdate: ocspResp.NextUpdate,
		Reason:     status.String(),
	}

	// Cache the result
	ttl := c.config.CacheTTL
	if ocspResp.NextUpdate.After(time.Now()) {
		ttl = ocspResp.NextUpdate.Sub(time.Now())
		if ttl > c.config.CacheTTL {
			ttl = c.config.CacheTTL
		}
	}
	c.cache.Store(cacheKey, &cachedResponse{
		result:    result,
		expiresAt: time.Now().Add(ttl),
	})

	return result
}

// checkOCSPBySerial checks OCSP by serial number (requires issuer cert).
func (c *Checker) checkOCSPBySerial(ctx context.Context, serial string, issuer *x509.Certificate) Result {
	// Create a dummy cert with the serial number for OCSP request
	cert := &x509.Certificate{
		SerialNumber: new(big.Int),
		Issuer:       issuer.Subject,
	}
	cert.SerialNumber.SetString(serial, 10)

	return c.checkOCSP(context.Background(), cert, issuer)
}

// checkCRLBySerial checks CRL by serial number (requires issuer cert).
func (c *Checker) checkCRLBySerial(ctx context.Context, serial string, issuer *x509.Certificate) Result {
	// Create a dummy cert with the serial number for CRL check
	cert := &x509.Certificate{
		SerialNumber: new(big.Int),
		Issuer:       issuer.Subject,
	}
	cert.SerialNumber.SetString(serial, 10)

	return c.checkCRL(context.Background(), cert, issuer)
}

// checkCRL performs a CRL check for a certificate.
func (c *Checker) checkCRL(ctx context.Context, cert, issuer *x509.Certificate) Result {
	// Get CRL URL
	crlURL := c.config.CRLURL
	if crlURL == "" {
		if len(cert.CRLDistributionPoints) > 0 {
			crlURL = cert.CRLDistributionPoints[0]
		}
		if crlURL == "" {
			if c.config.CRLFile != "" {
				return c.checkCRLFile(cert, issuer)
			}
			if c.config.FailClosed {
				return Result{Status: StatusError, Error: errors.New("no CRL URL or file available")}
			}
			return Result{Status: StatusUnknown, Reason: "no CRL source"}
		}
	}

	// Check cache
	cacheKey := "crl:" + crlURL
	if cached, ok := c.cache.Load(cacheKey); ok {
		cachedResp := cached.(*cachedResponse)
		if time.Now().Before(cachedResp.expiresAt) {
			return cachedResp.result
		}
	}

	// Fetch CRL
	httpClient := &http.Client{Timeout: c.config.Timeout}
	resp, err := httpClient.Get(crlURL)
	if err != nil {
		if c.config.FailClosed {
			return Result{Status: StatusError, Error: fmt.Errorf("CRL fetch failed: %w", err)}
		}
		return Result{Status: StatusUnknown, Reason: "CRL fetch failed"}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if c.config.FailClosed {
			return Result{Status: StatusError, Error: fmt.Errorf("CRL fetch returned status %d", resp.StatusCode)}
		}
		return Result{Status: StatusUnknown, Reason: fmt.Sprintf("CRL fetch status %d", resp.StatusCode)}
	}

	crlData, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{Status: StatusError, Error: fmt.Errorf("read CRL: %w", err)}
	}

	return c.parseAndCheckCRL(cert, issuer, crlData, crlURL)
}

// checkCRLFile checks a local CRL file.
func (c *Checker) checkCRLFile(cert *x509.Certificate, issuer *x509.Certificate) Result {
	if c.config.CRLFile == "" {
		if c.config.FailClosed {
			return Result{Status: StatusError, Error: errors.New("no CRL file configured")}
		}
		return Result{Status: StatusUnknown, Reason: "no CRL file"}
	}

	data, err := os.ReadFile(c.config.CRLFile)
	if err != nil {
		if c.config.FailClosed {
			return Result{Status: StatusError, Error: fmt.Errorf("read CRL file: %w", err)}
		}
		return Result{Status: StatusUnknown, Reason: "CRL file read failed"}
	}

	return c.parseAndCheckCRL(cert, issuer, data, c.config.CRLFile)
}

// parseAndCheckCRL parses a CRL and checks the certificate serial.
func (c *Checker) parseAndCheckCRL(cert *x509.Certificate, issuer *x509.Certificate, data []byte, source string) Result {
	// Try parsing as DER first
	crl, err := x509.ParseRevocationList(data)
	if err != nil {
		// Try PEM
		block, _ := pem.Decode(data)
		if block != nil {
			crl, err = x509.ParseRevocationList(block.Bytes)
		}
		if err != nil {
			return Result{Status: StatusError, Error: fmt.Errorf("parse CRL from %s: %w", source, err)}
		}
	}

	// Verify the CRL signature against the issuer when one is provided.
	// A CRL that is not issued by the supplied issuer must never be
	// trusted for a revocation decision.
	if issuer != nil && !c.config.SkipSignature {
		if err := crl.CheckSignatureFrom(issuer); err != nil {
			return Result{
				Status: StatusError,
				Error:  fmt.Errorf("CRL signature verification failed: %w", err),
			}
		}
	}

	// Check if certificate is revoked
	serial := cert.SerialNumber.String()
	for _, revoked := range crl.RevokedCertificates {
		if revoked.SerialNumber.String() == serial {
			return Result{
				Status:     StatusRevoked,
				Serial:     serial,
				Reason:     "certificate revoked",
				ThisUpdate: crl.ThisUpdate,
				NextUpdate: crl.NextUpdate,
			}
		}
	}

	// Check CRL validity
	now := time.Now()
	if !crl.NextUpdate.IsZero() && now.After(crl.NextUpdate) {
		return Result{
			Status:     StatusError,
			Error:      errors.New("CRL has expired"),
			ThisUpdate: crl.ThisUpdate,
			NextUpdate: crl.NextUpdate,
		}
	}

	// Cache the result
	cacheKey := "crl:" + source
	ttl := time.Until(crl.NextUpdate)
	if ttl > c.config.CacheTTL {
		ttl = c.config.CacheTTL
	}
	if ttl <= 0 {
		ttl = c.config.CacheTTL
	}
	c.cache.Store(cacheKey, &cachedResponse{
		result: Result{
			Status:     StatusGood,
			Serial:     cert.SerialNumber.String(),
			Reason:     "not revoked",
			ThisUpdate: crl.ThisUpdate,
			NextUpdate: crl.NextUpdate,
		},
		expiresAt: time.Now().Add(ttl),
	})

	return Result{
		Status:     StatusGood,
		Serial:     cert.SerialNumber.String(),
		Reason:     "not revoked",
		ThisUpdate: crl.ThisUpdate,
		NextUpdate: crl.NextUpdate,
	}
}

// checkFile checks a file-based revocation list.
func (c *Checker) checkFile(cert *x509.Certificate) Result {
	return c.checkFileBySerial(cert.SerialNumber.String())
}

// checkFileBySerial checks a file-based revocation list by serial number.
func (c *Checker) checkFileBySerial(serial string) Result {
	if c.config.RevocationFile == "" {
		return Result{Status: StatusGood, Reason: "file fallback: no revocation file configured"}
	}

	data, err := os.ReadFile(c.config.RevocationFile)
	if err != nil {
		if c.config.FailClosed {
			return Result{Status: StatusError, Error: fmt.Errorf("read revocation file: %w", err)}
		}
		return Result{Status: StatusGood, Reason: "file fallback: revocation file not found"}
	}

	// Parse file (one serial per line, # comments)
	lines := strings.Split(string(data), "\n")
	revoked := make(map[string]bool)
	for i, line := range lines {
		line = strings.TrimSpace(line)
		// Strip UTF-8 BOM from first line if present
		if i == 0 && strings.HasPrefix(line, "\ufeff") {
			line = strings.TrimPrefix(line, "\ufeff")
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		revoked[line] = true
	}

	if revoked[serial] {
		return Result{Status: StatusRevoked, Serial: serial, Reason: "revoked in file list"}
	}
	return Result{Status: StatusGood, Serial: serial, Reason: "not revoked in file list"}
}

// ClearCache clears the revocation cache.
func (c *Checker) ClearCache() {
	c.cache = sync.Map{}
}
