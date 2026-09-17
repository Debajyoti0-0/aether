# Stage 28 — Release Recovery Evidence and Certification: 4.1.0-rc2

Companion to `docs/stage28-baseline-lock.md` (freeze, 69-path resolution,
RCA). Every result below was produced against the tagged commit's own
clean-checkout build — not the working tree — in `/c/dev/clean-room-aether`
(fresh `git clone`, `git checkout v4.1.0-rc2`, zero dirty/untracked paths).

## 1. Lineage

| Item | Value |
| --- | --- |
| Version | `4.1.0-rc2` |
| Branch | `master` |
| Release commit | `bc674af1ab9ac49c8ec154fda103e9e558457301` |
| Tag | `v4.1.0-rc2` (annotated, object `e8416b5849c26b37a53493782aacfc9bb41f236c`, dereferenced commit `bc674af1…`) |
| Tag content proof | `git show v4.1.0-rc2:internal/revocation/checker.go` ✓; `git show v4.1.0-rc2:internal/cli/export_verify.go` ✓; `git show v4.1.0-rc2:VERSION` = `4.1.0-rc2` |
| Baseline | `v4.0.0-rc2` unchanged: object `779cdb6f…`, commit `5cd008be1e3e20b039214a911626a6a4426c7838`, `VERSION=4.0.0-rc2` |
| Withdrawn tag | `v4.1.0` (Stage 27) — absent locally and on `origin`; NOT recreated, NOT reused |
| Clean room | `/c/dev/clean-room-aether` @ `bc674af1`, `git status --porcelain` = 0 entries, 0 untracked |
| Tagged build | `go build -ldflags "-X …internal/version.Version=4.1.0-rc2 -X …Commit=bc674af" -o bin/aether-tagged ./cmd/aether`; `./bin/aether-tagged --version` → `aether version 4.1.0-rc2` |

Gate-path note: the Stage 28 gate matrix names `internal/revocation/ocsp.go`
and `internal/providers/validate.go`; the actual implementation files are
`internal/revocation/checker.go` (OCSP/CRL/file checker, one file) and the
`providers validate` command in `internal/cli/v3.go`. The gates are
evaluated against the real paths above; `internal/revocation/ocsp.go` does
not exist at any commit (verified).

## 2. Gate matrix (G441–G460)

| Gate | Requirement | Result | Evidence (all raw output from the tagged build) |
| --- | --- | --- | --- |
| G441 | 69 paths bucketed; tree clean | PASS | Bucket table in baseline-lock §3; `git status --porcelain` empty |
| G442 | FIX code committed (`internal/revocation/`) | PASS | commit `05ba793`; `git show HEAD:internal/revocation/checker.go` succeeds |
| G443 | FIX code committed (`internal/cli/export_verify.go`) | PASS | commits `05ba793` + `267f797` |
| G444 | FIX-5 committed (providers validate) | PASS | commit `dd72f1e` (`internal/cli/v3.go`, command `providers validate`) |
| G445 | `--cert` in help | PASS | help lists `--cert` (12 flags total on verify-evidence) |
| G446 | leaf's real serial used, not 12345 | PASS | OCSP GOOD output: `Serial: 635766432577322893714060966593525603860886919607` (= hex 6F5CC0B8…); JSON `certificate_subject: CN=Aether Test Leaf,O=Aether` |
| G447 | missing `--cert` exits non-zero | PASS | `Error: --cert is required`, exit 1 |
| G448 | REVOKED exits non-zero | PASS | OCSP REVOKED exit 2; CRL REVOKED exit 2 |
| G449 | evidence gate enforced | PASS | `Evidence: rejected` on every non-GOOD result; `EVIDENCE REJECTED` on stderr; non-GOOD never returns exit 0 |
| G450 | tag created | PASS | `git rev-list -n1 v4.1.0-rc2` = `bc674af1…` |
| G451 | tag contains code | PASS | `git show v4.1.0-rc2:internal/revocation/checker.go` succeeds (see gate-path note) |
| G452 | tagged build version | PASS | `bin/aether-tagged --version` = `4.1.0-rc2`; tracked `bin/aether.exe` (same source) also reports `4.1.0-rc2` |
| G453 | FIX-4 OCSP on tagged build | SUCCESS-PATH PASS | GOOD → exit 0, evidence accepted (mock responder, fixture for leaf serial) |
| G454 | FIX-4 CRL on tagged build | SUCCESS-PATH PASS | GOOD → exit 0 (local CRL file, signature-verified against issuer) |
| G455 | FIX-4 revocation on tagged build | REVOCATION DETECTED + non-zero exit | OCSP REVOKED exit 2; CRL REVOKED exit 2; file-mode REVOKED exit 2 |
| G456 | FIX-5 all three providers on tagged build | SUCCESS-PATH PASS (mock) | okta/gitlab/kubernetes 200 paths exit 0 with identity echo (§4) |
| G457 | quality gates on tagged commit | PASS with one BLOCKED item | §5 (race = BLOCKED: no C toolchain, Docker daemon down) |
| G458 | `v4.0.0-rc2` untouched | PASS | object `779cdb6f…`, commit `5cd008be…`, `VERSION=4.0.0-rc2` — re-verified after all work |
| G459 | document limit ≤ 3 | PASS | 2 documents (`stage28-baseline-lock.md`, `stage28-certification.md`) |
| G460 | Stage 29 handoff embedded | PASS | §7 |

## 3. FIX-4 full matrix — `bin/aether-tagged` (tagged commit's own build)

| Scenario | Status | Exit | Evidence gate |
| --- | --- | --- | --- |
| OCSP GOOD (leaf, real serial) | GOOD | 0 | accepted |
| OCSP REVOKED | REVOKED | 2 | rejected |
| OCSP UNKNOWN | UNKNOWN | 3 | rejected |
| OCSP wrong certificate (dummy-12345 vs leaf response) | ERROR | 1 | rejected |
| OCSP unavailable responder (127.0.0.1:1) | ERROR | 1 | rejected |
| CRL GOOD (empty CRL, signature-verified) | GOOD | 0 | accepted |
| CRL REVOKED (leaf on CRL) | REVOKED | 2 | rejected |
| CRL wrong issuer (leaf presented as issuer) | ERROR | 1 | rejected |
| missing `--cert` | config error | 1 | rejected |
| malformed `--cert` (non-PEM) | config error | 1 | rejected |
| file mode REVOKED (serial 12345 in list) | REVOKED | 2 | rejected |
| file mode GOOD (serial not listed) | GOOD | 0 | accepted |
| `--json` output | valid JSON | 0 | accepted, identifies subject + SHA-256 |

Exit-code contract (implemented in `internal/cli/export_verify.go`, mapped
in `cmd/aether/main.go`): GOOD→0; REVOKED→2; UNKNOWN→3; configuration/
parse/network/internal errors→1. This contract is enforced by returning
sentinel errors (`ErrEvidenceRevoked`, `ErrEvidenceUnknown`,
`ErrEvidenceCheckFail`) from `RunE`; nothing on a non-GOOD path returns
`nil`. (During verification the first tagged build exited 1 on all error
classes because `main.go` did not map the sentinels; this was fixed in
commit `267f797` before the tag was finalized — see baseline-lock §5.)

Certificate-input contract: `--cert <file>` is required for ocsp/crl/file
modes; the certificate is read, PEM-decoded, parsed, and its own serial
drives the OCSP/CRL/file lookup. There is no fallback: the hardcoded dummy
certificate (serial 12345) and the demo `CheckSerial("12345")` path were
removed. Fixtures were regenerated for the real leaf serial (baseline-lock
§4). The CRL path additionally verifies the CRL signature against the
`--issuer` certificate and fails closed on mismatch — this check caught a
real fixture defect (test CA lacking `cRLSign`) during development.

## 4. FIX-5 provider validation — `bin/aether-tagged`

Mock infrastructure: `testdata/okta-200.py` (8081→8091),
`testdata/gitlab-200.py` (8082→8092), `testdata/k8s-200.py` (8083→8093).
(Ports shifted because a stale, unresponsive mock process from a previous
session — PID 29284 — held the original ports; it was left untouched.)

| Provider | Request | Response | Result |
| --- | --- | --- | --- |
| Okta | GET `/api/v1/users/me` (SSWS token) → 200; GET `/.well-known/openid-configuration` → 200 | identity JSON + discovery document | MOCK SUCCESS-PATH VERIFIED, exit 0 |
| GitLab | GET `/api/v4/user` (Bearer) → 200 | `{"id":1,"username":"testuser",…}` | MOCK SUCCESS-PATH VERIFIED, exit 0 |
| Kubernetes | POST SelfSubjectReview → 201; GET `/api/v1/namespaces` (Bearer) → 200 | userInfo + NamespaceList | MOCK SUCCESS-PATH VERIFIED, exit 0 |

Error paths (all exit non-zero): missing `--domain`; missing `--token`;
unknown provider (`plugin "foobar" not registered`); unreachable endpoint;
GitLab HTTP 401; Kubernetes HTTP 403; Okta discovery 401; Okta malformed
discovery body. No real credentials used anywhere.

Qualification label: **MOCK SUCCESS-PATH VERIFIED** only. LIVE
INTEROPERABILITY VERIFIED: **not claimed** (no live Okta/GitLab/Kubernetes
or Entra/IMDS access in this lab).

## 5. Quality gates (tagged commit, clean room)

| Gate | Result | Detail |
| --- | --- | --- |
| build (`go build ./...`) | PASS | |
| vet (`go vet ./...`) | PASS | |
| unit (`go test -count=1 ./...`) | PASS | 29 packages ok |
| integration (`go test -tags=integration ./test/integration/...`) | PASS | 16.3s |
| race (`go test -race ./...`) | **BLOCKED** | `-race` requires cgo; no C compiler on host (`gcc not found`), Docker daemon not running (repo's `Dockerfile.race` unavailable). NOT RUN — not counted as PASS |
| fuzz (23 targets × 60s) | PASS | 0 failures, 0 crashes; ~85k–137k execs/sec per target |
| lint (`golangci-lint run ./...`) | PASS | 0 issues |
| govulncheck (`govulncheck ./...`) | PASS | 0 vulnerabilities affecting code (1 unreachable vuln in required modules) |

Flake protocol: no test required a rerun; first runs are the recorded runs.

## 6. Blocker register

| ID | Finding | Severity | Status |
| --- | --- | --- | --- |
| B-28-1 | Hardcoded dummy certificate (serial 12345) in `export verify-evidence` | critical | RESOLVED (commit `05ba793`) |
| B-28-2 | REVOKED printed but exited 0 | critical | RESOLVED (commits `05ba793`, `267f797`) |
| B-28-3 | Evidence acceptance not gated on revocation result | critical | RESOLVED (commit `05ba793`) |
| B-28-4 | FIX-4/FIX-5 implementation uncommitted (Stage 27 root cause) | critical | RESOLVED (10 commits, tag contains code) |
| B-28-5 | Local CRL files never signature-verified | major | RESOLVED (commit `05ba793`; gate now also rejects the unsigned-issuer case) |
| B-28-6 | Revocation fixtures built for the withdrawn dummy serial | major | RESOLVED (regenerated for leaf serial; baseline-lock §4) |
| L-28-1 | Race detector not executable on this host | limitation | OPEN (BLOCKED, §5) — run `Dockerfile.race` where a C toolchain exists before GA |
| L-28-2 | Provider interoperability verified against local mocks only | limitation | OPEN — live Entra/IMDS/provider validation deferred to Stage 29 window |
| L-28-3 | Test PKI keys (`testdata/`, `test_pki/`) committed for fixture reproduction | accepted | Throwaway local material, no production value |

Open mandatory blockers: **none**.

## 7. Stage 29 handoff

`4.1.0-rc2` (tag `v4.1.0-rc2`, commit `bc674af1…`) is certified from the
tagged commit's own clean-checkout build; Stage 29 opens a 30-day
observation window for `4.1.0-rc2` in the expert lab and the authorized
internal team, with two standing items: (1) run the race suite (repo
`Dockerfile.race`) on a host with a C toolchain or running Docker daemon —
the only BLOCKED quality gate this stage; (2) live Entra/IMDS and live
provider (Okta/GitLab/Kubernetes) interoperability validation opens when
access is available, before 2027-06-30; live success must never be inferred
from this stage's mock evidence. `v4.0.0-rc2` remains the terminal retained
baseline should any observation-window finding force withdrawal.

## 8. Verdict

**OPTION A — 4.1.0-rc2 QUALIFIED (with explicitly recorded limitations
L-28-1..L-28-3, none blocking).**

> The code that passed is the code that was committed (`05ba793`,
> `dd72f1e`, `267f797`), built (`bc674af1` clean checkout), tagged
> (`v4.1.0-rc2` → `bc674af1`), and independently verified from the tag's
> own build.
