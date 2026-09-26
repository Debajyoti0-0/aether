# Stage 53R — `ms-wcce` Review (deferred to Stage 47)

`internal/protocol/ms-wcce/` existed only as uncommitted work on the
`C:\dev\aether` line. It was preserved on branch
`stranded-stage-45-46` and reviewed before any port decision. **Decision:
defer entirely to Stage 47. Not ported.**

## Why it matters

It is the only AD CS / certificate-template code in either line. The
canonical trunk reaches AD CS only through `internal/protocol/msoapx` and
`internal/protocol/wstrust` (MS SOAP / WS-Trust), a different protocol
family. For Stage 47 (AD CS, templates, ESC6, enrollment) this package is
the most relevant asset in the project.

## Verdict: it does not compile

Extracted to a scratch module outside the trunk and built:

```
ms-wcce\types.go:80:6:    CertRequest redeclared in this block
ms-wcce\request.go:84:6:  other declaration of CertRequest
ms-wcce\request.go:118:8: tmpl.ExtKeyUsage undefined (type *x509.CertificateRequest
                          has no field or method ExtKeyUsage)
ms-wcce\request.go:128:17: undefined: parseURI
```

Three files, 603 lines, zero tests, never built. It is a partial draft.

## Defect list for Stage 47

### D1 — `CertRequest` declared twice, with two different meanings

Not a trivial de-duplication. The two declarations are different concepts
that collided on one name:

| Location | Shape | Meaning |
|---|---|---|
| `types.go:80` | `Version, Subject, SerialNumber, Issuer, NotBefore, NotAfter, PublicKey, SignatureAlgorithm, Signature, Extensions, Attributes` | a **certificate** model |
| `request.go:84` | `Flags, Attributes, CSR []byte` (PKCS#10 DER) | an **outbound enrollment request** |

`BuildCertRequest`, `AttributesString` and `Thumbprint` are all methods on
the `request.go` type, so that is the one the rest of the code uses. The
`types.go` declaration is the orphan and needs renaming (e.g.
`IssuedCertificate`). Repair requires a naming decision, not a merge.

### D2 — EKU cannot be encoded in the CSR

`request.go:118` assigns `tmpl.ExtKeyUsage = opts.EKUs`, but
`x509.CertificateRequest` has no such field. Requested extended key usage
must be emitted as an entry in `ExtraExtensions` (OID `2.5.29.37`). This
is a silent functional gap on exactly the ESC1/ESC6 request-construction
path, not a cosmetic error.

### D3 — `parseURI` undefined

Called at `request.go:128`, defined nowhere in the package.

### D4 — documented transport does not exist

The package doc states:

> "Wire transport (DCOM/RPC for ICertRequestD) is defined as the Transport
> interface; the live DCOM transport requires a Windows AD CS environment
> and is reported via ErrTransportUnavailable rather than simulated."

Neither `Transport` nor `ErrTransportUnavailable` appears anywhere in the
three committed files. The "we do not simulate" claim is **unbacked by
implementation**. Stage 47 must either add the interface and the sentinel
error, or delete the claim. An unbacked honesty claim is worse than none.

### D5 — malformed SAN URIs are silently dropped

```go
for _, uri := range opts.SANs[SANKindURI] {
    if u, err := parseURI(uri); err == nil {
        tmpl.URIs = append(tmpl.URIs, u)
    }
}
```

The error is discarded. A malformed URI is dropped rather than reported, so
the operator receives a valid-looking request carrying **fewer identities
than they asked for**. On abuse-path construction (ESC1/ESC6) that is a
fail-open behaviour. It should return an error, consistent with the
fail-closed posture the rest of the safety spine takes.

## What is genuinely good and worth keeping

- Correct `CERTSRV_E_*` disposition/error constants
- Correct `PrivateKeyFlags`, `SubjectNameFlags`, `EnrollmentFlags` models
- Correct pKI LDAP attribute names (`msPKI-Enrollment-Flag`,
  `msPKI-Certificate-Name-Flag`, `cACertificateRevocationList*`, …)
- Correct MS UPCtrl SAN extension OID `1.3.6.1.4.1.311.20.2.3`, encoded as
  `SEQUENCE OF [0] IMPLICIT UTF8String`
- `BuildSANAttribute` rendering the multi-value SAN form used by ESC1/ESC6/ESC15
- `crypto/rand` for both `rsa.GenerateKey` and `x509.CreateCertificateRequest`
  (not `math/rand`)
- 2048-bit default key size
- No private key material in any output path
- `Thumbprint()` returns a SHA-256 CSR digest "without exposing key
  material" — correct evidence-correlation hygiene

## Repair scope estimate

One type rename, one missing helper, one incorrect x509 API usage, one
fail-open path, one unbacked documentation claim, and a protocol-structure
test suite. No wire transport, and no live DCOM.
