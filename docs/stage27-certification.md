# Stage 27 Certification — FIX-4/FIX-5 Success-Path Verification

**Verdict: WITHDRAW `v4.1.0` — retained terminal: `v4.0.0-rc2`**

## Decision

`v4.1.0` is **WITHDRAWN** (local tag deleted, remote tag deleted from
`github.com/Debajyoti0-0/aether`, `VERSION` reverted to `4.0.0-rc2`).

**Rationale.** Stage 27 executed every success path the prompt demanded — OCSP GOOD via a real
HTTP POST with CA-signature verification, CRL GOOD via file and HTTP fetch, revocation detected
via both OCSP and CRL, and all three provider validations (Okta/GitLab/Kubernetes) returning
200 with echoed identity. All six success paths classified SUCCESS-PATH PASS (or
REVOCATION DETECTED for the negative control), with full raw output in
`stage27-success-path-evidence.md`. **But the verdict is still WITHDRAW, because the code that
passed is not in the release.** `internal/cli/export_verify.go`, `internal/revocation/`, and
`providers validate` exist only as untracked working-tree files — `git log --all` shows no
commit ever touched them, and `git show v4.1.0:internal/cli/export_verify.go` fails ("exists on
disk, but not in 'v4.1.0'"). Tag `38c9abd` (annotated; commit `35f7a81`) therefore certified
behavior its binary did not contain. Success paths were verified against the working tree
binary rebuilt with `Version=4.1.0` ldflags; shipping an unsigned claim is what Stage 27 was
ordered to end.

Additional defects recorded against the uncommitted FIX-4 code (must be fixed before any
future release carries it):

1. **No `--cert` flag** — the CLI validates a hardcoded dummy certificate
   (`SerialNumber: 12345`, `export_verify.go:126-146`); a user-supplied certificate is never
   checked.
2. **Revoked certificates exit 0** — `runExportVerify` prints `Status: REVOKED` but always
   returns nil; G428's "non-zero exit" requirement is unmet.
3. **No revocation gate** — the result is printed; no evidence artifact is rejected, no
   exit-code contract exists for any status other than nil.

## Gate matrix

| Gate | Requirement | Result |
|---|---|---|
| G421 | Preconditions | PASS (tag=38c9abd→35f7a81, VERSION=4.1.0, rc2=5cd008be; binary rebuilt) |
| G422 | CA fixture | PASS (`testdata/ca.pem`, openssl-parseable) |
| G423 | Leaf fixture | PASS (`testdata/leaf.pem` + `dummy-12345.pem` for the hardcoded serial) |
| G424 | OCSP fixture | PASS (`ocsp-good.der`, serial 3039, status good) |
| G425 | CRL fixture | PASS (`crl.pem`, "No Revoked Certificates") |
| G426 | FIX-4 OCSP success | SUCCESS-PATH PASS (exit 0, Status: GOOD) |
| G427 | FIX-4 CRL success | SUCCESS-PATH PASS (file + URL, exit 0, Status: GOOD) |
| G428 | FIX-4 revocation detected | **REVOCATION DETECTED** (Status: REVOKED, but exit 0 — defect 2) |
| G429 | FIX-5 Okta success | SUCCESS-PATH PASS (200, identity echoed, exit 0) |
| G430 | FIX-5 GitLab success | SUCCESS-PATH PASS (200, username echoed, exit 0) |
| G431 | FIX-5 Kubernetes success | SUCCESS-PATH PASS (200, SelfSubjectReview + namespaces, exit 0) |
| G432 | Build/vet/unit | PASS (build 0, vet 0, 28/28 packages) |
| G433 | Integration | PASS (crash-matrix timing flake on run 1; full-suite rerun clean) |
| G434 | Fuzz (21 targets) | PASS (21/21 × 10s, 0 crashes) |
| G435 | Lint | PASS (0 issues) |
| G436 | govulncheck | PASS (0 affecting) |
| G437 | Verdict | **WITHDRAW** (this document) |
| G438 | Tag action | `v4.1.0` deleted locally (`was 38c9abd`) and on origin; `VERSION=4.0.0-rc2` |
| G439 | Document limit | 2 (`stage27-success-path-evidence.md`, this file) |
| G440 | Stage 28 handoff | below |

## Tag status

- `v4.1.0` : **withdrawn** (was `38c9abd` annotated → `35f7a81`; `git push origin :refs/tags/v4.1.0` → `[deleted]`, exit 0)
- `v4.0.0-rc2` : **unchanged**, `5cd008be1e3e20b039214a911626a6a4426c7838`, `VERSION=4.0.0-rc2`

## Stage 28 handoff

Stage 28 = **`4.1.0-rc2` FIX-4 success-path rebuild**. Single-item fix list, in order:
(1) commit the working-tree FIX-4/FIX-5 code (`internal/revocation/`,
`internal/cli/export_verify.go`, `providers validate` surface, go.mod/go.sum) and re-tag from a
clean tree; (2) add `--cert <file>` to `export verify-evidence` and validate the
operator-supplied certificate end-to-end (re-run WS1–WS3 against it); (3) make non-GOOD
revocation statuses exit non-zero so G428's contract holds; (4) gate evidence acceptance on the
result rather than printing it. Re-certification requires: all Stage 27 success paths re-run
against the **tagged** commit's own build, plus the full quality-gate suite (G432–G436).
Observation-window and FIX-5-release paths are moot — FIX-5 verified clean and travels with the
same commit.
