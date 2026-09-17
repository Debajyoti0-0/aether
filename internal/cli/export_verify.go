package cli

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/revocation"
)

// Exit-code contract for `export verify-evidence`:
//
//	0  - GOOD: revocation status good, evidence accepted
//	1  - configuration error (missing/invalid flag, unreadable or
//	     malformed certificate), invalid response, unavailable
//	     responder/CRL, timeout, or internal error
//	2  - REVOKED: evidence rejected
//	3  - UNKNOWN: revocation status could not be determined,
//	     evidence rejected
//
// Evidence acceptance is gated on a GOOD status only. A non-zero exit
// always means the evidence was NOT accepted.
var (
	// ErrEvidenceRevoked is returned when the certificate under
	// evaluation is revoked. The CLI maps it to exit code 2.
	ErrEvidenceRevoked = errors.New("evidence rejected: certificate is revoked")
	// ErrEvidenceUnknown is returned when the revocation status could
	// not be determined. The CLI maps it to exit code 3.
	ErrEvidenceUnknown = errors.New("evidence rejected: revocation status unknown")
	// ErrEvidenceCheckFail is returned when the revocation check
	// itself failed (invalid response, unavailable responder, CRL
	// error). The CLI maps it to exit code 1.
	ErrEvidenceCheckFail = errors.New("evidence rejected: revocation check failed")
)

type verifyEvidenceJSON struct {
	Status          string `json:"status"`
	Accepted        bool   `json:"accepted"`
	Evidence        string `json:"evidence"`
	CertificateSubj string `json:"certificate_subject,omitempty"`
	CertificateFP   string `json:"certificate_sha256,omitempty"`
	Serial          string `json:"serial,omitempty"`
	Reason          string `json:"reason,omitempty"`
	Error           string `json:"error,omitempty"`
}

var (
	evRevocationMode string
	evIssuerCert     string
	evCertFile       string
	evResponderURL   string
	evCRLURL         string
	evCRLFile        string
	evRevocationFile string
	evSkipSigVerify  bool
	evTimeout        string
	evCacheTTL       string
	evFailClosed     bool
	evJSON           bool
)

func init() {
	exportCmd.AddCommand(exportVerifyCmd)

	exportVerifyCmd.Flags().StringVar(&evRevocationMode, "revocation", "none", "Revocation checking mode: none, ocsp, crl, file")
	exportVerifyCmd.Flags().StringVar(&evCertFile, "cert", "", "Certificate PEM file to verify (required for ocsp/crl/file modes)")
	exportVerifyCmd.Flags().StringVar(&evIssuerCert, "issuer", "", "Issuer certificate PEM file (required for ocsp/crl)")
	exportVerifyCmd.Flags().StringVar(&evResponderURL, "responder", "", "OCSP responder URL (optional, from cert AIA if omitted)")
	exportVerifyCmd.Flags().StringVar(&evCRLURL, "crl-url", "", "CRL distribution point URL (optional, from cert CDP if omitted)")
	exportVerifyCmd.Flags().StringVar(&evCRLFile, "crl-file", "", "Local CRL file path (for crl mode)")
	exportVerifyCmd.Flags().StringVar(&evRevocationFile, "revocation-file", "", "File-based revocation list (serial numbers, one per line; required for file mode)")
	exportVerifyCmd.Flags().BoolVar(&evSkipSigVerify, "skip-sig-verify", false, "Skip signature verification (testing only)")
	exportVerifyCmd.Flags().StringVar(&evTimeout, "timeout", "10s", "HTTP timeout for OCSP/CRL requests")
	exportVerifyCmd.Flags().StringVar(&evCacheTTL, "cache-ttl", "5m", "Cache TTL for OCSP/CRL responses")
	exportVerifyCmd.Flags().BoolVar(&evFailClosed, "fail-closed", true, "Fail closed on network/validation errors")
	exportVerifyCmd.Flags().BoolVar(&evJSON, "json", false, "Emit JSON output")
}

var exportVerifyCmd = &cobra.Command{
	Use:   "verify-evidence",
	Short: "Verify evidence with revocation checking",
	Long: `Verify evidence artifacts with certificate revocation checking.

The certificate under evaluation must be supplied with --cert. There is
no default certificate: without --cert the command fails.

Exit codes:
  0 - GOOD (evidence accepted)
  1 - configuration/parse/network error (evidence rejected)
  2 - REVOKED (evidence rejected)
  3 - UNKNOWN status (evidence rejected)

Revocation modes:
  none     - No revocation checking (default)
  ocsp     - OCSP responder checking (requires --cert and --issuer)
  crl      - CRL checking (requires --cert, --issuer, and --crl-url or --crl-file)
  file     - File-based revocation list (requires --cert and --revocation-file)

Examples:
  aether export verify-evidence --revocation=none
  aether export verify-evidence --revocation=ocsp --cert leaf.pem --issuer ca.pem
  aether export verify-evidence --revocation=crl --cert leaf.pem --issuer ca.pem --crl-file crl.pem
  aether export verify-evidence --revocation=file --cert leaf.pem --revocation-file revoked.txt`,
	RunE: runExportVerify,
}

// loadCertificatePEM reads and parses a single PEM-encoded certificate.
func loadCertificatePEM(path, what string) (*x509.Certificate, error) {
	if path == "" {
		return nil, fmt.Errorf("--%s is required", what)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s cert: %w", what, err)
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("no PEM CERTIFICATE block in %s cert file %s", what, path)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse %s cert: %w", what, err)
	}
	return cert, nil
}

func runExportVerify(cmd *cobra.Command, args []string) error {
	// Parse timeout
	timeout, err := time.ParseDuration(evTimeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}

	// Parse cache TTL
	cacheTTL, err := time.ParseDuration(evCacheTTL)
	if err != nil {
		return fmt.Errorf("invalid cache TTL: %w", err)
	}

	mode := revocation.RevocationMode(evRevocationMode)
	switch mode {
	case revocation.ModeNone, revocation.ModeOCSP, revocation.ModeCRL, revocation.ModeFile:
	default:
		return fmt.Errorf("invalid revocation mode %q: must be none, ocsp, crl, or file", evRevocationMode)
	}

	// Build revocation config
	config := revocation.DefaultConfig()
	config.Mode = mode
	config.Timeout = timeout
	config.CacheTTL = cacheTTL
	config.FailClosed = evFailClosed
	config.SkipSignature = evSkipSigVerify

	var leaf *x509.Certificate
	var issuer *x509.Certificate

	if mode != revocation.ModeNone {
		// The certificate under evaluation is operator-supplied. There
		// is no fallback: a missing, unreadable, or malformed
		// certificate is a configuration error.
		leaf, err = loadCertificatePEM(evCertFile, "cert")
		if err != nil {
			return err
		}
	}

	switch mode {
	case revocation.ModeOCSP:
		config.OCSPResponder = evResponderURL
		issuer, err = loadCertificatePEM(evIssuerCert, "issuer")
		if err != nil {
			return err
		}
	case revocation.ModeCRL:
		config.CRLURL = evCRLURL
		config.CRLFile = evCRLFile
		if config.CRLURL == "" && config.CRLFile == "" {
			return fmt.Errorf("--crl-url or --crl-file is required for crl mode")
		}
		issuer, err = loadCertificatePEM(evIssuerCert, "issuer")
		if err != nil {
			return err
		}
	case revocation.ModeFile:
		if evRevocationFile == "" {
			return fmt.Errorf("--revocation-file is required for file mode")
		}
		config.RevocationFile = evRevocationFile
	}

	checker := revocation.NewChecker(config)

	fmt.Fprintf(os.Stderr, "Revocation check mode: %s\n", mode)
	if leaf != nil {
		sum := sha256.Sum256(leaf.Raw)
		fmt.Fprintf(os.Stderr, "Certificate: %s (serial %s, sha256 %s)\n",
			leaf.Subject.String(), leaf.SerialNumber.String(), hex.EncodeToString(sum[:8]))
	}
	if evIssuerCert != "" {
		fmt.Fprintf(os.Stderr, "Issuer: %s\n", evIssuerCert)
	}
	if evResponderURL != "" {
		fmt.Fprintf(os.Stderr, "OCSP responder: %s\n", evResponderURL)
	}
	if evCRLURL != "" {
		fmt.Fprintf(os.Stderr, "CRL URL: %s\n", evCRLURL)
	}
	if evCRLFile != "" {
		fmt.Fprintf(os.Stderr, "CRL file: %s\n", evCRLFile)
	}
	if evRevocationFile != "" {
		fmt.Fprintf(os.Stderr, "Revocation file: %s\n", evRevocationFile)
	}

	var result revocation.Result
	if mode == revocation.ModeNone {
		result = revocation.Result{Status: revocation.StatusGood, Reason: "revocation checking disabled"}
	} else {
		result = checker.Check(context.Background(), leaf, issuer)
	}

	// Evidence acceptance gate: only a GOOD revocation status accepts
	// the evidence. Everything else rejects it with a distinct exit
	// code (see the contract at the top of this file).
	out := verifyEvidenceJSON{
		Status:   result.Status.String(),
		Accepted: result.Status == revocation.StatusGood,
		Evidence: "rejected",
		Serial:   result.Serial,
		Reason:   result.Reason,
	}
	if leaf != nil {
		sum := sha256.Sum256(leaf.Raw)
		out.CertificateSubj = leaf.Subject.String()
		out.CertificateFP = hex.EncodeToString(sum[:])
		out.Serial = leaf.SerialNumber.String()
	}

	var runErr error
	switch result.Status {
	case revocation.StatusGood:
		out.Evidence = "accepted"
	case revocation.StatusRevoked:
		runErr = fmt.Errorf("%w (certificate %s, serial %s)", ErrEvidenceRevoked, leaf.Subject.String(), out.Serial)
	case revocation.StatusUnknown:
		runErr = ErrEvidenceUnknown
		if result.Error != nil {
			runErr = fmt.Errorf("%w: %v", ErrEvidenceUnknown, result.Error)
		}
	default: // StatusError and anything else: fail closed
		runErr = ErrEvidenceCheckFail
		if result.Error != nil {
			runErr = fmt.Errorf("%w: %v", ErrEvidenceCheckFail, result.Error)
		}
	}
	if result.Error != nil && runErr == nil {
		runErr = result.Error
	}
	if runErr != nil {
		out.Error = runErr.Error()
	}

	if evJSON {
		enc := json.NewEncoder(os.Stdout)
		if err := enc.Encode(out); err != nil {
			return fmt.Errorf("encode JSON output: %w", err)
		}
	} else {
		printRevocationResult(result)
	}

	if runErr != nil {
		fmt.Fprintf(os.Stderr, "EVIDENCE REJECTED: %v\n", runErr)
		return runErr
	}
	fmt.Fprintln(os.Stderr, "EVIDENCE ACCEPTED")
	return nil
}

func printRevocationResult(result revocation.Result) {
	fmt.Printf("Status: %s\n", result.Status)
	fmt.Printf("Evidence: ")
	if result.Status == revocation.StatusGood {
		fmt.Printf("accepted\n")
	} else {
		fmt.Printf("rejected\n")
	}
	if result.Serial != "" {
		fmt.Printf("Serial: %s\n", result.Serial)
	}
	if result.Reason != "" {
		fmt.Printf("Reason: %s\n", result.Reason)
	}
	if !result.ThisUpdate.IsZero() {
		fmt.Printf("ThisUpdate: %s\n", result.ThisUpdate.Format(time.RFC3339))
	}
	if !result.NextUpdate.IsZero() {
		fmt.Printf("NextUpdate: %s\n", result.NextUpdate.Format(time.RFC3339))
	}
	if result.Error != nil {
		fmt.Printf("Error: %v\n", result.Error)
	}
}
