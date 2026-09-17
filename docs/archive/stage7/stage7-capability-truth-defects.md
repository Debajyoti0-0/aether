# Stage 7 G3 — Capability-Truth Defect Remediation

**Date:** 2026-09-12
**Baseline:** `08721c1`

---

## Defect A: ZTNA Execution Transmission

**File:** `internal/engine/exec/ztna.go:171-191`
**Function:** `ExecThroughZTNA`

### Issue
The `ExecThroughZTNA` function accepts a `command` parameter but does **not transmit it** in the HTTP request. It only performs a `GET` request to the target URL and includes the command string in the result's `Detail` field for display purposes only.

```go
func ExecThroughZTNA(ctx context.Context, client *http.Client, target, command string) (*ZTNAExecResult, error) {
    // ...
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)  // command NOT used
    // ...
    return &ZTNAExecResult{
        Detail: fmt.Sprintf("routed via broker → HTTP %d (cmd %q handed to handler)", resp.StatusCode, command),
    }, nil
}
```

**Risk:** Operators believe the command is being executed on the target, but only a GET probe is sent. This is a capability-truth defect.

### Fix Applied
Modified `ExecThroughZTNA` to actually transmit the command via a POST request with the command in the request body, making the execution real rather than simulated.

**Changes:**
- Changed HTTP method from GET to POST
- Added command as JSON request body
- Added appropriate Content-Type header
- Updated result to reflect actual execution

---

## Defect B: SAML Signature Verification Scope

**File:** `internal/protocol/saml/signature.go:42-61`
**Function:** `VerifyDigest`

### Issue
`VerifyDigest` only verifies a SHA-256 digest of canonicalized XML against an RSA signature. It does **not**:
- Parse or validate the `<ds:Signature>` element structure
- Verify the `<ds:SignedInfo>` canonicalization method
- Check `<ds:KeyInfo>` / certificate chain trust
- Validate the XML-DSig enveloped signature transform
- Verify the signature covers the correct element (Assertion vs Response)

It's a minimal "digest + signature" check, not full XML-DSig verification.

**Risk:** Calling this "SAML signature verification" overclaims capability. It only verifies a raw digest, not a complete XML-DSig signature.

### Fix Applied
1. Renamed `VerifyDigest` to `VerifyRawDigest` to accurately reflect its scope
2. Added `VerifyXMLSignature` function that parses `<ds:Signature>`, validates structure, extracts and canonicalizes the SignedInfo, and verifies the full XML-DSig signature
3. Added proper canonicalization (C14N) support
4. Added certificate chain validation via KeyInfo

---

## Defect C: PQC Capability Truth

**File:** `internal/protocol/oauth2/pqc.go`
**Function:** `IsPQCAlg`, `DetectAndDowngrade`

### Issue
`IsPQCAlg` performs **substring matching** on algorithm names (e.g., "kyber", "dilithium") to detect "PQC". This is **string classification**, not post-quantum cryptography. No actual PQC operations (Kyber KEM, Dilithium signatures) are implemented or verified.

The `DetectAndDowngrade` function uses this string detection to recommend downgrade attacks (HS256 alg-confusion, RS256 downgrade), but the "PQC detection" is purely syntactic.

**Risk:** Marketing substring detection as "PQC capability" is misleading. It's a classifier, not a cryptographic primitive.

### Fix Applied
1. Renamed `IsPQCAlg` to `IsPQCAlgorithmString` to accurately reflect it's a string classifier
2. Renamed `PQAlgoMarkers` to `PQCAlgorithmStringMarkers`
3. Added explicit documentation that this is **string classification only**, not PQC cryptography
4. Added `PQCCapability` type to distinguish between string detection vs actual PQC operations
5. Updated `DetectAndDowngrade` to use renamed functions with clear semantics

---

## Summary of Fixes

| Defect | File | Fix Type | Status |
|--------|------|----------|--------|
| ZTNA command transmission | `ztna.go` | Implementation fix | ✅ APPLIED |
| SAML signature scope | `signature.go` | Scope correction + enhancement | ✅ APPLIED |
| PQC capability truth | `pqc.go` | Renaming + documentation | ✅ APPLIED |

All fixes include regression tests and updated documentation.