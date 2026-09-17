# Stage 27 — FIX-4 / FIX-5 Success-Path Evidence

**Repo:** `C:\dev\aether` · **Code under test:** working tree at `35f7a81` + uncommitted FIX-4/FIX-5 files · **Binary:** `bin\aether.exe` rebuilt with `-ldflags "-X internal/version.Version=4.1.0 -X .../Commit=35f7a81"`.

## G421 — Preconditions

```
PS> git rev-list -n1 v4.1.0
35f7a81880fcfce8513cbed8f4f6b7baadcaf7bb
PS> git cat-file -p 38c9abd
object 35f7a81880fcfce8513cbed8f4f6b7baadcaf7bb
type commit
tag v4.1.0
tagger Debajyoti Haldar ... 1789635105 +0530
Stage 24 — 4.1.0 operations release (QA-verified with raw evidence)
PS> git show v4.1.0:VERSION   → 4.1.0
PS> git rev-list -n1 v4.0.0-rc2
5cd008be1e3e20b039214a911626a6a4426c7838     (unchanged through the stage)
PS> bin\aether.exe --version
aether version 4.1.0                          (after rebuild; shipped binary said "dev")
PS> git status --porcelain | Measure-Object -Line
66                                            (PRE-EXISTING dirty state, not Stage 27's)
```

**DECISIVE FINDING — the tag does not contain the fixes:**

```
PS> git log --all --oneline -- internal/cli/export_verify.go   → (empty)
PS> git log --all -S validateOkta --oneline                    → (empty)
PS> git log --all --oneline -- internal/revocation/            → (empty)
PS> git show v4.1.0:internal/cli/export_verify.go
fatal: path 'internal/cli/export_verify.go' exists on disk, but not in 'v4.1.0'
PS> git status --porcelain | Select-String 'revocation|export_verify'
?? internal/cli/export_verify.go
?? internal/revocation/
```

FIX-4 and FIX-5 exist **only as untracked/uncommitted files**. They were never committed to any branch.

## G422–G425 — Fixtures (testdata/)

`openssl` 3.5.7. Generated: `ca.pem/ca.key` (`-subj "/CN=Aether Test CA/O=Aether"`), `leaf.pem`
(serial `6F5CC0B8…9B6`), `dummy-12345.pem` (`-set_serial 12345`, serial `3039`) — required because
`export verify-evidence` hardcodes serial 12345 and accepts no `--cert` (see G426 note).

```
PS> openssl x509 -in dummy-12345.pem -noout -serial
serial=3039
PS> openssl ocsp -reqin ocsp-req.der -text -noverify   → Serial Number: 3039
PS> openssl ocsp -respin ocsp-good.der -text -noverify
    OCSP Response Status: successful (0x0)
    Responder Id: CN = Aether Test CA, O = Aether
    Serial Number: 3039
    Cert Status: good
    Next Update: Sep 24 10:48:25 2026 GMT
PS> openssl ocsp -respin ocsp-revoked.der -text -noverify
    Serial Number: 3039 (revoked)
PS> openssl crl -in crl.pem -noout -text
    Issuer: CN=Aether Test CA, O=Aether · Next Update: Oct 17 2026 · No Revoked Certificates.
PS> openssl crl -in crl-revoked.pem -noout -text
    Serial Number: 3039 · Revocation Date: Jan 1 2035
```

All four gates PASS. Note: an earlier attempt produced `ocsp-good.der` for the wrong serial
because `openssl ocsp -signer` mode ignored `-reqin`; the `-index` responder mode was used
instead (good entry `V` for 3039 in `index-good.txt`).

## G426–G428 — FIX-4 success paths

Mock: `testdata\ocsp-mock.py` (POST/GET aware, port 9999). A stale pre-Stage-27 responder
holding port 9999 was killed first (it returned 404/501; see process log).

```
PS> .\bin\aether.exe export verify-evidence --revocation=ocsp --issuer testdata/ca.pem --responder http://127.0.0.1:9999/ocsp-good.der
Revocation check mode: ocsp
Issuer: testdata/ca.pem
OCSP responder: http://127.0.0.1:9999/ocsp-good.der
Status: GOOD
Serial: 12345
Reason: GOOD
ThisUpdate: 2026-09-17T10:48:25Z
NextUpdate: 2026-09-24T10:48:25Z
G426 exit: 0                       → SUCCESS-PATH PASS (real POST + parse + CA-signature verify)
```

```
PS> .\bin\aether.exe export verify-evidence --revocation=crl --issuer testdata/ca.pem --crl-file testdata/crl.pem
Status: GOOD / Serial: 12345 / Reason: not revoked / NextUpdate: 2026-10-17T10:46:20Z
G427 (file) exit: 0                → SUCCESS-PATH PASS

PS> .\bin\aether.exe export verify-evidence --revocation=crl --issuer testdata/ca.pem --crl-url http://127.0.0.1:9999/crl.pem
Status: GOOD / Serial: 12345 / Reason: not revoked
G427 (url) exit: 0                 → SUCCESS-PATH PASS (HTTP fetch path exercised)
```

```
PS> .\bin\aether.exe export verify-evidence --revocation=crl --issuer testdata/ca.pem --crl-file testdata/crl-revoked.pem
Status: REVOKED
Serial: 12345
Reason: certificate revoked
G428 exit: 0   ← DEFECT: revocation detected but command exits 0 (RunE always returns nil)

PS> .\bin\aether.exe export verify-evidence --revocation=ocsp --issuer testdata/ca.pem --responder http://127.0.0.1:9999/ocsp-revoked.der
Status: REVOKED / Serial: 12345 / Reason: REVOKED
G428b exit: 0  ← same defect
```

Classification: **REVOCATION DETECTED** (classification machinery correct; exit-code
enforcement absent).

Prompt-literal command form, for the record:

```
PS> .\bin\aether.exe export verify-evidence --revocation=ocsp --issuer testdata/ca.pem --cert testdata/leaf.pem ...
Error: unknown flag: --cert        (exit 1)
```

The CLI has **no `--cert` flag**; `runExportVerify` checks a hardcoded dummy
`&x509.Certificate{SerialNumber: 12345}` ("For demo purposes, we check a dummy certificate",
`export_verify.go:126-146`). A user-supplied certificate is never validated.

## G429–G431 — FIX-5 success paths

Mocks: `testdata\okta-200.py` (:8081 — `/api/v1/users/me` + `/.well-known/openid-configuration`),
`testdata\gitlab-200.py` (:8082 — `/api/v4/user`), `testdata\k8s-200.py` (:8083 —
SelfSubjectReview POST + `/api/v1/namespaces`). The prompt's `--config yaml` form does not
exist (`Error: --domain is required`, exit 1 — captured); the real flags are `--domain/--token`.

```
PS> .\bin\aether.exe providers validate okta --domain http://127.0.0.1:8081 --token test-token-not-real
Token validation: OK
Validating Okta configuration...
OIDC discovery: OK
Okta validation: OK
G429 exit: 0                       → SUCCESS-PATH PASS

PS> .\bin\aether.exe providers users --domain http://127.0.0.1:8081 --token test-token-not-real
test@example.com         ACTIVE     00u_test
1 user(s)
exit: 0                            → identity echo confirmed (test@example.com)

PS> .\bin\aether.exe providers validate gitlab --domain http://127.0.0.1:8082 --token test-token-not-real
Token validation: OK
Validating GitLab configuration...
GitLab API connectivity: OK
GitLab validation: OK
G430 exit: 0                       → SUCCESS-PATH PASS

PS> .\bin\aether.exe providers validate kubernetes --domain http://127.0.0.1:8083 --token test-token-not-real
Token validation: OK
Validating Kubernetes configuration...
Kubernetes API connectivity: OK
Kubernetes validation: OK
G431 exit: 0                       → SUCCESS-PATH PASS
```

## G432–G436 — Quality gates (code under test = working tree)

```
go build ./...    → exit 0   (after archiving stray debris gen_ocsp.go / test_file_read.go
                             → testdata/debris/*.go.txt; they declared duplicate main())
go vet ./...      → exit 0
go test -count=1 ./...  → 28/28 packages ok, exit 0 (incl. internal/revocation, pkg/plugins)
go test -count=1 -tags=integration ./test/integration/...
                  → run 1: FAIL TestCrashMatrix/kill_after_entry_021 (crash-timing flake,
                    "committed entries = 23, want 20 or 21"); run 2 (full suite): ok, exit 0
golangci-lint run ./... → 0 issues (exit 0)
govulncheck ./... → 0 affecting (1 module-level vuln not called)
fuzz: 21/21 targets × 10s each → 0 crashes
  (api: ReadFrame, DecodePayload, EncodePayload, EnvelopeValidation; exec: IMDSIdentityToken,
   ParseInstanceMetadata; token: ParsePRT, ParseOAuthTokens, ValidatePRT; msoapx: DecodeKey,
   ComputeSessionKeyProof, URLValuesEncoding, DeriveNonce; saml: ParseAssertion,
   AssertionXMLMarshal; wstrust: ParseRSTR, ExtractAssertionWS, BuildRST, ParseMEX;
   workspace: LoadRecord, AuditLogEntry)
```

## Fixture/mock inventory

`testdata/`: ca.{pem,key}, leaf.{pem,key,csr}, dummy-12345.{pem,key,csr}, ocsp-req.der,
ocsp-good.der, ocsp-revoked.der, crl.pem, crl-revoked.pem, index.txt, index-good.txt,
crlnumber, openssl.cnf, ocsp-mock.py, okta-200.py, gitlab-200.py, k8s-200.py, debris/.


