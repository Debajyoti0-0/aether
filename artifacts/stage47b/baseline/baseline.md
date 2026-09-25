# Stage 47B — Forensic Baseline

```text
Captured: 2026-09-26
HEAD:     ebb8c8bfb64f52bc0205351366495088c3772e63  (matches the briefed ebb8c8b)
Branch:   master
Tree:     CLEAN  (git status --porcelain -> empty)
VERSION:  5.0.0-alpha1
Tags:     no v5.* tags
```

## 1. Repository integrity (G47B-01, G47B-02)

| Property | Value |
|---|---|
| HEAD | `ebb8c8bfb64f52bc0205351366495088c3772e63` |
| `git describe --tags --always` | `ebb8c8b` |
| Branch | `master` |
| `git status --porcelain` | empty (clean) |
| `VERSION` | `5.0.0-alpha1` |
| `git tag -l 'v5.*'` | empty |

Recent history:

```text
ebb8c8b docs(stage46i): certify provisioning assessment and owner handoff
c07338e chore(stage47): record entry-gate results and CA harness blocker
ad0c0ae feat(ad): deliver AD Kerberos + LDAP expansion and close Stage 46h defects
a887a0e chore(artifacts): rename stage*/ to release/, audit/, handoff/
264f04c chore(artifacts): remove PROCESS-HISTORY pointer file; fix RELEASING.md references
```

## 2. Toolchain

| Component | Version / state |
|---|---|
| OS | Microsoft Windows 11 Pro |
| Architecture | AMD64 |
| Host | DARKPURPLE-01 |
| User | Debajyoti0-0 |
| Elevated | **No** — `IsAdministrator = False` |
| Go | `go1.27.1 windows/amd64` |
| C compiler (for `-race`) | gcc, `mingw-winlibs` |
| Docker | 29.7.2, daemon `linux`, context `desktop-linux` |
| Python | 3.14.x |
| Node | present (not used — no Node in the release build) |
| Chrome | `C:\Program Files\Google\Chrome\Application\chrome.exe` (on disk, not on PATH) |
| Edge | `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe` (on disk, not on PATH) |
| Network | reachable (static.rust-lang.org, unpkg.com, proxy.golang.org, github.com all HTTP 200) |
| Rust (installed this stage) | `rustc 1.98.1 (48a229cea 2026-09-01)`, `cargo 1.98.1` |
| Rust target (installed this stage) | `wasm32-unknown-unknown` |
| wasm-pack (installed this stage) | `wasm-pack 0.13.1` |

The Rust toolchain was absent at Stage 46i and was installed during Stage 47B
for the Stage 52 WASM workstream. It is recorded here because §3 requires the
dependency state at baseline, and because a later reader must not assume it was
always present.

## 3. AD lab state (G3404)

```text
aether-ad-lab        Up 2 hours (healthy)   172.18.0.2   realm AETHER.TEST
aether-test-client   Up about an hour                        172.18.0.4
```

The lab is a **Samba4 AD DC**. It is a real Kerberos/LDAP/DNS environment and it
was live-qualified in Stage 45b/46d/46f/46g/46h. It is **not** a Windows
environment and it has **no AD CS**. See `artifacts/stage47b/adcs/surface-reprobe.txt`.

## 4. Quality gates at baseline

| Gate | Command | Result |
|---|---|---|
| build | `go build ./...` | exit 0, no output |
| vet | `go vet ./...` | exit 0, no findings |
| test | `go test -count=1 ./...` | exit 0 — **31** packages ok, **0** FAIL, **14** no-test |
| race | `go test -race -count=1 ./...` | see `artifacts/stage47b/race/go-test-race.txt` |

Full test output: `artifacts/stage47b/baseline/go-test-output.txt`.
Build/vet/test summary: `artifacts/stage47b/baseline/quality-gates.txt`.

## 5. Environment variables

No credential-bearing or path-rewriting environment variable is required by the
build or the test suite. The relevant observed values:

```text
PATH           includes C:\Program Files\Go\bin, scoop shims, Python314
GOPATH/GOBIN   go defaults
CARGO_HOME     C:\Users\Debajyoti0-0\.cargo        (created this stage)
RUSTUP_HOME    C:\Users\Debajyoti0-0\.rustup       (created this stage)
DOCKER_HOST    unset (Docker Desktop default context)
```

## 6. Stage 46i evidence integrity (G47B-03, G47B-04)

All five Stage 46i / Stage 47 documents are present and unmodified in
substance. SHA-256 prefixes at capture time:

| File | Bytes | sha256 (first 16) |
|---|---|---|
| `docs/stage46i-provisioning-attempt.md` | 11162 | `8336A5E9E96B872F` |
| `docs/stage46i-certification.md` | 18810 | `127EC2A0C8FD3444` |
| `artifacts/stage47/baseline/baseline-correction.md` | 6480 | `713589DF1B5568EB` |
| `docs/stage46h-certification.md` | 27071 | `84148370D192A6D3` |
| `artifacts/stage47/BLOCKED-WITH-OWNER.md` | 6022 | `2F6CEB26CB2355B1` |

The historical 2026-09-21 baseline files in `artifacts/stage47/baseline/`
(`baseline-summary.md`, `package-inventory.json`, `repository-status.json`,
`dependency-state.json`, `gate-B3-G00.json`, `stage45-correlation.json`,
`stage46-correlation.json`) are **untouched** and remain forensic records. They
are superseded by `baseline-correction.md`, not overwritten.

## 7. Corrections this stage found in prior evidence

Recorded rather than silently fixed. Full detail and reasoning in
`artifacts/stage47b/adcs/surface-reprobe.txt` §4.

```text
1. Stage 46i recorded "pKICertificateTemplate classSchema in schema -> absent".
   ACTUAL: the classSchema IS present in the Samba forest:
     dn: CN=PKI-Certificate-Template,CN=Schema,CN=Configuration,DC=aether,DC=test
     lDAPDisplayName: pKICertificateTemplate
   The classes pKIEnrollmentService, certificationAuthority and
   pKIExtendedKeyUsage ARE absent.

2. Stage 46i listed only CN=Public Key Services as present. ACTUAL:
   CN=Enrollment Services and CN=Certificate Templates are also present
   beneath it. All three are empty of children.

Neither correction changes the G3206 verdict. Both make it more precise: the
absence of a CA in this lab is a runtime/servicing absence, not a schema
absence, which is exactly why a schema and empty containers cannot be
substituted for a real Windows AD CS deployment.
```
