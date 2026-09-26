# Stage 53W — G53R-26 Security Review

Captured (UTC): 2026-09-26. HEAD under review: `4364ab4`.
Branch: `reconciliation/stage53r`. Repository: `C:\dev\aether`.

Scope: the 13-item checklist from the Stage 53W charter. Every item was executed
against the current tree with raw evidence. No item was closed by assertion.

**Outcome: G53R-26 = `BLOCKED-WITH-OWNER`.** The review was performed and it
found a P1. A security review that surfaces a P1 cannot also certify the tree.

---

## Checklist results

| # | Area | Result | Evidence |
| --- | --- | --- | --- |
| 1 | Crypto primitives | **PASS** | §1 below |
| 2 | Key handling | **PASS** | §2 below |
| 3 | Secret hygiene | **PASS** | §3 below |
| 4 | Input validation | **PASS** | §4 below |
| 5 | Injection | **PASS** | §5 below |
| 6 | Authorization coverage | **PASS** | §6 below |
| 7 | Engagement boundary | **PASS** | §7 below |
| 8 | Transport / TLS | **FAIL — P1** | §8 below, SEC-53W-01 |
| 9 | Race conditions | **FLAKY** | §9 below, SEC-53W-03 |
| 10 | Resource exhaustion | **PASS** | §10 below |
| 11 | Error disclosure / panic | **PASS** | §11 below |
| 12 | Dependency risk | **PARTIAL** | §12 below, SEC-53W-02 |
| 13 | Network exposure | **PASS** | §13 below |

---

### 1. Crypto primitives — PASS

| Primitive | Files | Adjudication |
| --- | ---: | --- |
| `crypto/ed25519` | 10 | Correct for the audit chain and signatures |
| `argon2` | 2 | Correct for password-derived key material |
| `crypto/rand` | 22 | Used for all security-relevant randomness |
| `math/rand` | 6 | **Not** security-relevant — see below |
| `crypto/md5`, `crypto/sha1` | 3 | **Protocol-mandated**, not weaknesses |

`math/rand` appears only in `internal/behavior/timing.go`,
`internal/transport/stealth.go`, `internal/transport/retry.go`, `internal/rl/rng.go`,
`internal/rl/agent.go`, `internal/engine/validate/fuzz.go` — timing jitter, backoff
jitter, RL sampling and fuzz input generation. None of these produce a token, key,
nonce or session identifier.

The MD5/SHA1 uses are all compelled by specifications, and treating them as
findings would be wrong:

- `internal/web/ws.go:96` — SHA-1 for the WebSocket `Sec-WebSocket-Accept`
  handshake, mandated by RFC 6455 §4.2.2.
- `internal/protocol/kerberos/crypto.go`, `pac.go` — MD4/MD5/SHA-1 in Kerberos
  string-to-key and RFC 4757 key derivation, mandated by RFC 4120.

Replacing these would break interoperability, which is the opposite of the goal.

### 2. Key handling — PASS

- Audit ledger and key written via `osWriteFile` / `os.OpenFile` at mode
  `0o600` (`internal/store/audit_io.go:7`, `internal/store/audit.go:60`).
- `aether-audit.key` and `aether-audit.jsonl` are gitignored:
  `git check-ignore -v` resolves both to `.gitignore:58` and `.gitignore:59`.
- The PKI root `0700` control recovered in an earlier stage is intact.

### 3. Secret hygiene — PASS

Searched non-test code for `slog`/`log`/`fmt.Fprint*` calls referencing
`BindPass`, `password`, `token`, `secret`: **no call sites**. Credentials are not
routed through the logging surface.

### 4. Input validation — PASS

79 `json.Unmarshal` and 29 `asn1.Unmarshal` sites in non-test code. Decoders that
read from a stream are bounded: `internal/protocol/kerberos/ccache.go:197` wraps
the reader in `io.LimitReader(r, 8<<20)`. The LDAP search decoder tracks nesting
depth explicitly (`internal/protocol/ldap/search.go:546-554`).

### 5. Injection — PASS

- No `fmt.Sprintf` constructing SQL (`SELECT`/`INSERT`/`UPDATE`/`DELETE`): no hits.
- No shell-interpolated `exec.Command` against `sh`/`bash`/`cmd`/`powershell`: no
  hits.

### 6. Authorization coverage — PASS

`requireEngagementScope(` has **15 call sites** across
`internal/cli/ad/{ldap,enum,roast,tgt}.go`, matching the 15 gated commands
recorded by Stage 53S. `IsAuthorized(` has 2 non-test sites
(`internal/engagement/engagement.go`, `internal/cli/ad/scope.go`).

### 7. Engagement boundary — PASS, and ordered before network

`requireEngagementScope` is the **first statement in `RunE`**, before any engine
construction or dial. Verified at three call sites in `internal/cli/ad/ldap.go`
(lines 182, 287, 392), each of the form:

```go
RunE: func(cmd *cobra.Command, args []string) error {
    if err := requireEngagementScope(engFile, domain, dc, ldapCapability(cmd)); err != nil {
        return err
    }
    ...
```

This is structural ordering, not a log-based inference. Behaviourally confirmed in
Stage 53V: six commands re-run with all required flags supplied reached the
engagement control and refused with a specific engagement error, with no network
activity.

### 8. Transport / TLS — **FAIL (P1)**

See **SEC-53W-01** below.

### 9. Race conditions — **FLAKY**

See **SEC-53W-03** below.

### 10. Resource exhaustion — PASS

Every `io.ReadAll` in non-test code is wrapped in `io.LimitReader` with an
explicit bound: `internal/intel/kev.go:67` (16 MiB),
`internal/cli/providers_exec.go:42` (4 MiB),
`internal/engine/exec/imds.go:68,121,166` (4 KiB, 1 MiB, 1 MiB),
`internal/engine/exec/github.go:81,108` (1 MiB, 4 MiB),
`internal/engine/exec/azure.go:78` (8 MiB). No unbounded read found.

### 11. Error disclosure / panic freedom — PASS

- `panic(` in non-test `internal/` code: **0 sites**.
- `os.Exit(` in non-test `internal/` code: **0 sites** — library code returns
  errors; only `cmd/aether/main.go` terminates the process, with a documented
  exit-code contract (2 revoked, 3 status unknown, 1 otherwise).

### 12. Dependency risk — **PARTIAL**, see SEC-53W-02

`go mod verify`: all modules verified. `govulncheck ./...`: 0 affecting, 0 in
imported packages; GO-2026-5932 (`x/crypto/openpgp`) verified unimported,
`Fixed in: N/A`, recorded as accepted no-action risk. 14 direct dependencies, none
flagged deprecated by `go list -m all`.

**Not performed:** licence audit (GPL/AGPL/copyleft) across the dependency graph.
Recorded as a gap rather than a pass.

### 13. Network exposure — PASS

The dashboard defaults to loopback: `internal/web/server.go:292` returns
`127.0.0.1:0`, and `internal/web/web_test.go:187-188` asserts the default address
is `127.0.0.1:8443`. A non-loopback bind must be requested explicitly. The
teamserver is constructed with an explicit address and mTLS
(`internal/api/teamserver.go`, exercised in tests with client CAs).

---

## Findings

### SEC-53W-01 — LDAP transport never verifies the server certificate (P1)

**Area:** Transport. **Component:** `internal/engine/ad/ldap/engine.go`.

**Observed.** Both TLS paths in the LDAP engine hardcode
`InsecureSkipVerify: true`:

```go
// line 91-94, LDAPS
if e.useTLS {
    conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
        InsecureSkipVerify: true,
    })

// line 116-119, StartTLS
config := &tls.Config{
    ServerName:         e.host,
    InsecureSkipVerify: true,
}
```

**Compounding.** The `Engine` struct (`engine.go:13-25`) has no CA pool, no
`Insecure` toggle and no verification field. The CLI exposes only `--tls` and
`--starttls` on all 14 LDAP/AD commands — no `--ca-file`, no `--insecure`, no
`--verify`. `ServerName` is set on the StartTLS path but is inert while
verification is disabled.

**Expected.** Certificate verification enabled by default, with an explicit
operator opt-out and a way to supply a CA for lab-issued certificates.

**Impact.** `Bind(ctx, dn, password)` at `engine.go:125` transmits bind
credentials over a channel whose peer is unauthenticated. An operator running
`aether ad ldap bind --tls` against a network position controlled by an adversary
has their directory credentials captured, and the tool cannot detect it. For a
tool whose entire purpose is authorized assessment of identity infrastructure,
an unconditional, non-overridable trust bypass on the credential path is a
release-blocking weakness.

**Severity:** P1.

**Disposition: `DEFERRED` — fix required, with an owner.** Not waived and not
`ACCEPTED`. The fix was not applied here for a specific, stated reason rather than
omission:

The obvious remedy — flip the default to verifying — would break **every**
existing `--tls` / `--starttls` invocation against self-signed or lab-issued AD CS
certificates, which is exactly the environment this project's own Stage 47/48/49
qualification depends on. Stage 53W's charter also carries a hard "no regression"
rule and a "no new features" rule. Choosing the security-correct default is a
posture decision with a real blast radius, and it needs an explicit owner, not a
drive-by change buried in a review commit.

**Required remediation:**

1. Add `caPool *x509.CertPool` and `insecureSkipVerify bool` to `ldap.Engine`.
2. Verify by default using the system root pool; honour `--ca-file` for a
   supplied CA.
3. Add an explicit `--insecure` opt-in, warn loudly on stderr when used, and
   record the fact in the audit entry so a captured bind is attributable.
4. Plumb across all 14 LDAP/AD commands.
5. Add a regression test proving a self-signed peer is rejected without
   `--insecure` and accepted with `--ca-file`.

**Owner:** AD/Kerberos/LDAP owner (same owner as G53R-08 and G3206).
**Blocks:** Stage 54 production qualification. A P1 credential-exposure path on
the primary protocol surface cannot be carried into a GA claim.

---

### SEC-53W-02 — Untracked "Stage 54 results" artifacts contain figures that do not reproduce (P2)

**Area:** Evidence integrity. **Component:** `artifacts/stage54/security-results.json`,
`artifacts/stage54/browser-runs.json`.

**Observed.** Both files exist on disk, are **untracked by git** (confirmed:
`git ls-files --error-unmatch` matches neither), and assert specific results
attributed to commit `5bb26bd`. Two of those figures do not reproduce:

| Claim in `security-results.json` | Measured at `4364ab4` |
| --- | --- |
| `"packages": 34, "passed": 34` (unit + race) | **35** packages contain test files; **52** packages total under `./...` |
| `"fuzzing": {"targets": 10}` | **9** `Fuzz*` functions exist in the tree |

Additionally `browser-runs.json` asserts 65 browser runs with 0 failures and 0
leaks, and `security-results.json` asserts 10 historical defects re-tested with 0
regressions. **None of that was reproduced here**, and the files are not part of
the repository.

One claim in the same file is also scope-ambiguous in exactly the way Stage 53V
corrected: `"golangci_lint": {"issues": 0, "status": "PASS"}` at `5bb26bd` was
produced by a config enabling only `ineffassign`.

**Impact.** These files look like Stage 54 evidence and are positioned exactly
where a reviewer would look for it. Two of their numbers are wrong. Adopting them
would import fabricated results into the qualification record — the specific
failure mode Stage 54 §43 exists to prevent.

**Severity:** P2 (evidence integrity, not a runtime defect).

**Disposition: `ACCEPTED` as quarantined.** Left on disk, explicitly **not**
adopted, **not** committed, and **not** cited. Recorded here so that if a later
stage encounters them it knows they are unverified and partly inaccurate rather
than mistaking them for prior work.

**Owner:** QA manager. **Action:** either regenerate from executed runs with
correct figures, or delete.

---

### SEC-53W-03 — One unexplained race-suite failure, not reproduced (P3, RCA-INCOMPLETE)

**Area:** Concurrency. **Component:** unknown — the output was not captured.

**Observed.** During Stage 53W, a full `go test -race -count=1 ./...` returned
exit 1 with a race-or-failure marker. It did not reproduce in **10** subsequent
full runs (4 sequential + 6 background), all of which exited 0.

**Process defect, disclosed.** The failing run's output was consumed by a summary
expression and discarded rather than written to a file. The failing package,
test name and stack are therefore **lost**, and no root cause can be assigned. That
is my error in evidence handling, and it is recorded as such rather than papered
over with a clean re-run.

**Severity:** P3 (no reproduction, no identified defect) — but the ceiling is
P1 if the failure was a genuine data race in production code.

**Disposition: `DEFERRED` — RCA-INCOMPLETE.** Per Stage 54 §39, an unknown root
cause is recorded as RCA-INCOMPLETE and not written speculatively.

**Required remediation:** re-run the race suite in a loop with per-run output
retained, until either the failure reproduces and is captured, or a materially
larger sample (suggested: 50 runs) passes and the event is recorded as a
one-off with its capture gap disclosed.

**Owner:** QA manager.

---

## Findings summary

| ID | Severity | Area | Disposition | Owner |
| --- | --- | --- | --- | --- |
| SEC-53W-01 | **P1** | Transport — LDAP TLS verification disabled | **DEFERRED** (fix required; blocks Stage 54) | AD/Kerberos/LDAP |
| SEC-53W-02 | P2 | Evidence integrity — inaccurate untracked result files | **ACCEPTED** (quarantined, not adopted) | QA manager |
| SEC-53W-03 | P3 | Concurrency — unexplained race failure | **DEFERRED** (RCA-INCOMPLETE) | QA manager |

Every finding carries a disposition. None is left undispositioned.

## G53R-26 verdict

The gate requires a security review with a defined scope, executed, with findings
disposed. That has now happened: 13 items executed, 10 PASS, 1 FAIL, 1 FLAKY,
1 PARTIAL, 3 findings dispositioned.

**The gate is `BLOCKED-WITH-OWNER`, not RESOLVED.** A review that finds a P1
credential-exposure path on the primary protocol surface has not produced a clean
result, and the charter is explicit that an internal, fixable blocker must not be
waived for convenience. SEC-53W-01 is internal and fixable; it is deferred with an
owner and a five-step remediation, not waved through.

What Stage 53T may take from this review: the checklist exists, it was executed,
and 10 of 13 areas are clean with evidence. What it may not take: a claim that
Aether's transport security is sound.

## Not performed by this review

- Live AD/Kerberos/LDAP protocol testing (no authorized directory available).
- AD CS / MS-WCCE / PKINIT (G3206, external owner).
- Teamserver mTLS, capability and revocation testing.
- Licence/copyleft audit of the dependency graph.
- Fuzzing campaign (existing `Fuzz*` targets were not re-run here; Stage 53S
  recorded a 20s sweep that this review did not reproduce).
- Browser security review (see `docs/stage53w-browser-scope.md`).
