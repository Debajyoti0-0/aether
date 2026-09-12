# Aether — Development-Cycle Forensic Review

**Version reviewed:** 3.4.0-stage3 (actual) · **HEAD:** `b3ed72f` · **Branch:** `master`
**Review date:** 2026-09-12 · **Baseline doc:** `docs/development-cycle-review-baseline.md`
**Review prompt baseline (`C:\dev\aether`, `3.6.0-stage5`, `d250d52`):** **CONTRADICTED** — `BASELINE_ERROR` recorded; this review targets the actual repository.

---

## 1. Executive summary

Aether today is a **well-structured, genuinely engineered, single-developer
offensive-security platform at the end of Stage 3** — not a Stage 5 release
candidate. The review prompt's premise (a Stage 5 tree with conformance
artifacts, interop evidence, and a 15-item deferred register) does not exist in
this repository; what exists is a strong Stage 3 tree (208 Go files, ~29.6k LOC,
~389 test functions, 27 green test packages) plus a concurrent Stage 6
*documentation* effort that is carefully reconciling the real state.

The honest verdict: **Aether is a verified engineering system (M3) for its
offline/governed surface, and a functional prototype (M1–M2) for everything that
requires a live adversary-side identity fabric**. Its governance spine, storage
contract, and PKI teamserver are the strongest engineering in the tree. Its
protocol surface is broad but validated **only against itself** — every
"interop" test is a fixture or `httptest` loop, zero live evidence exists
anywhere in the tree, there are **zero native fuzz targets**, and three
capabilities (`ztna exec`, SAML XML-DSig forging, PQC detection) still carry the
v3.2.0 misrepresentation findings, two of them unmitigated in code.

## 2. Scope and methodology

Independent current-state assessment. All claims re-verified by local execution:
`go build`/`go vet`/`go test -count=1 ./...`/`go test -tags=integration`/
`govulncheck ./...` (all PASS — see baseline doc). Code inspection traced the
spine, audit chain, vault, teamserver identity/capabilities, protocol packages
(`msoapx`, `oauth2`, `saml`, `wstrust`), `engine/{token,cap,exec,pivot,validate,
watch,graph,orchestrate,orchestrator,mutation,rollback,spine,relay}`, plugins,
CI workflow, Makefile, README/CHANGELOG/SECURITY, and all stage documents.
The concurrent session's Stage 6 docs were conflict-checked, not inherited.

## 3. Baseline verification

See `docs/development-cycle-review-baseline.md` (R0 PASS). Key facts: 256
tracked files, zero tags, VERSION 3.4.0-stage3 consistent with the committed
binary, no `artifacts/` directory, `.kilo` stray worktree untracked, concurrent
session advancing HEAD during review.

## 4. Vision vs actual implementation

Vision: *"Impacket for the Hybrid Cloud Era."* Verified reality per module:

| Vision capability | Implementation (real) | Public surface | Verification | Level | Limitation |
|---|---|---|---|---|---|
| PRT | `internal/engine/token` (parse/import/extract, JWT alg-confusion), `internal/protocol/msoapx` (session-key proof, channel binding) | `aether prt`, `token` | FIXTURE_VERIFIED | 3 | No live-tenant validation; no broker interaction |
| Relay / WS-Trust / SAML | `internal/protocol/wstrust` (RST, downgrade), `internal/protocol/saml` (builder, strip, signer) | `aether relay` | FIXTURE_VERIFIED | 2 | XML-DSig self-consistent only (§7); no IdP interop |
| OAuth2/OIDC | `internal/protocol/oauth2` (client, JWKS, CAE handler) | via `relay`/`prt` | FIXTURE_VERIFIED | 2–3 | No real tenant flows |
| MS-OAPX | `internal/protocol/msoapx` | `aether prt` | FIXTURE_VERIFIED | 2 | Broker grant/proof deferred |
| IMDS | `internal/engine/exec` (httptest) | `exec imds` | FIXTURE_VERIFIED | 2 | No per-cloud split (deferred) |
| Kerberos pivot | `cloudkerberos.go` — reworked DER AS-REQ builder, structured AS-REP parser | `pivot cloud-to-onprem` | FIXTURE_VERIFIED | 2 | Never validated against a real KDC; placeholder ccache session key (honestly documented in README) |
| Exec providers | Azure RunCommand, AWS SSM (SigV4), GitHub, GCP, parallel | `exec` | FIXTURE_VERIFIED + spine | 3 | All via httptest doubles |
| **ZTNA exec** | `ztna.go:171-191` | listed capability | **CONTRADICTED** | 1 | **Command never transmitted** — HTTP GET to target; `command` interpolated into a status string (v3.2.0 RCA finding, UNRESOLVED) |
| Validation/risk | `validate` (risk, ATT&CK, PDF, executive, SOC predictor) | `validate`, `export` | TEST_VERIFIED | 3 | Heuristics; documented confidence caps |
| Teamserver | `internal/api` — mTLS, ECDSA P-256 CA, cert-bound identity, capability files, protocol v2, event store, spine dispatch | `serve`/`connect` | TEST_VERIFIED (fixture) | 3 | No live two-host evidence; file-based revocation |
| Evidence/audit | `store/audit.go` — Ed25519 hash chain, fsync per append, JSONL+bbolt | `audit` | TEST_VERIFIED | 3–4 | Trust-root distribution undefined |
| Storage | bbolt vault, cross-process lock, schema versioning, atomic rollback | workspace | TEST_VERIFIED | 4 | Crash-matrix artifacts never produced |
| Replay | replay command; **request idempotency CONFIRMED_MISSING** | `replay` | TEST_VERIFIED | 3 / absent | Retried mutations double-execute (documented) |
| Spine | 8-stage fail-closed pipeline, `completed_state_unknown` | internal | TEST_VERIFIED | 4 | Rollback states not unified into ActionState |
| **PQC** | `pqc.go` — substring detector + downgrade *analysis* | capability naming | **CONTRADICTED (as "PQC support")** | 1 | Zero post-quantum crypto performed (v3.2.0 RCA finding) |
| SAML signing | `signature.go:42-61` — signs `SHA256(raw bytes)` | relay forge | **CONTRADICTED (interop)** | 2 | No C14N/SignedInfo; verifies only against Aether's own verifier |
| Plugins | registry, optional SHA-256, 4 providers | `plugins`, `providers` | TEST_VERIFIED | 2 | Manifests unsigned; runtime loading absent |
| Watch | `engine/watch` daemon | `watch` | TEST_VERIFIED | 2 | No live-tenant evidence |

## 5. Architecture maturity

Dependency direction is clean: `cmd/aether` → `internal/cli` → engine/protocol/
store/workspace packages → `internal/{types,paths,version}`. `internal/store` is
the canonical persistence contract; `internal/engine/spine` is the canonical
mutation control plane; `internal/api` is the transport/authz boundary. CLI
mutation re-entry is eliminated via the `governance.go` intent whitelist (fail-
closed on unrepresentable commands). Side effects are explicit (`Mutation`
interface with BeforeState/AfterState/UndoRecipe); `ErrStateUnavailable` is
recorded, never fabricated. Configuration flows through `AETHER_CONFIG_DIR` with
a TestMain-owned pattern in tests (full DI deferred). Context propagation is
present on network paths (`http.NewRequestWithContext`).

**Subsystem classification:** spine `PRODUCTION-CAPABLE`; store/vault
`PRODUCTION-CAPABLE`; workspace crypto `PRODUCTION-CAPABLE`; api/teamserver
`STRUCTURED_IMPLEMENTATION`; protocol packages `STRUCTURED_IMPLEMENTATION`
(pivot, msoapx) to `FUNCTIONAL_PROTOTYPE` (ztna); plugins
`FUNCTIONAL_PROTOTYPE`; watch/orchestrate `FUNCTIONAL_PROTOTYPE`; release
tooling `EXPERIMENTAL`.

Structural weaknesses: `internal/rl` exists only because of a package rename to
dodge a (later-misdiagnosed) file-loss bug — naming debt; one 23 MB committed
binary; `bin/` in-tree build artifacts; planner/policy files outside the vault
(documented deferral); no plugin runtime loading; zero build-tag separation of
platform-specific code (single-tree cross-platform assumption).

## 6. Feature maturity (ladder)

Level 4 (verified, storage/audit-grade): vault, audit chain, rollback, spine,
workspace crypto, PKI issuance/revocation model.
Level 3 (verified offline): token/PRT parsing, cap evaluation/prediction, exec
providers, validate/export, graph, planner/RL, replay.
Level 2 (functional, limited validation): wstrust/saml/oauth2/msoapx/pivot/IMDS,
watch, plugins, dashboard.
Level 1 (experimental/misrepresented): ztna exec (no command transport), PQC
(detection only), SAML XML-DSig interop.

## 7. Security maturity

Strong, verified-by-code security properties: fail-closed spine (audit
unavailable ⇒ no execution), Ed25519-signed audit chain with fsync per append,
per-record AES-256-GCM + Argon2id workspace sealing, random per-workspace
tamper-evident salt, path-traversal/UNC/reserved-name validation, mTLS with
`RequireAndVerifyClientCert`, cert-derived identity (client-asserted operator
field removed from wire), URI-SAN operator binding, capability files defaulting
read-only, dashboard loopback + mandatory token, empty-passphrase rejection.

Findings (severity / remediation state):

| ID | Finding | Severity | State |
|---|---|---|---|
| S-R1 | `ztna exec` reports success for a command that was never executed | HIGH (misrepresentation in an ops tool) | UNRESOLVED (code) |
| S-R2 | SAML forged signatures not verifiable by real XML-DSig verifiers | HIGH (interop claim) | MITIGATED (verify helper exists; no real C14N) |
| S-R3 | PQC naming overstates a substring detector | MEDIUM | MITIGATED (code framed as detection; naming risk) |
| S-R4 | No native fuzz targets; parser safety un-fuzzed | HIGH (parser risk) | CONFIRMED_MISSING |
| S-R5 | No request idempotency; retried mutations double-execute | MEDIUM | DOCUMENTED/DEFERRED |
| S-R6 | Revocation file-based, no OCSP/CRL; effective for new connections only | MEDIUM | DEFERRED (documented) |
| S-R7 | Event stream transport-authenticated only (no per-event signatures) | LOW | ACCEPTED (audit chain is anchor) |
| S-R8 | Audit-chain trust root (Ed25519 key) lifecycle/rotation undefined | MEDIUM | CONFIRMED_MISSING |
| S-R9 | Plugin manifests unsigned; no runtime loading | MEDIUM | DEFERRED |
| S-R10 | Race/lint/govulncheck are CI-authoritative with no locally retrievable run evidence | LOW (process) | ACCEPTED |
| S-R11 | `x/crypto` module-level findings (2, uncalled) | LOW | MITIGATED (govulncheck symbol-level: 0 affecting, verified locally) |
| S-R12 | Workspace/protocol "interop" claims rest on fixtures only | HIGH (maturity claim) | EVIDENCE_GAP |

Parser security is real but narrow: frame caps (8 MiB), protocol version
validation, traversal validator, JWT alg-confusion tests, channel-binding
tests exist and pass — but none are fuzz-exercised by tooling.

## 8. Reliability and resilience maturity

Verified in code/tests: vault crash recovery (`TestEndToEnd_StorageCrashRecovery`),
cross-process exclusive locking, atomic rollback `Pop`, failed-rollback
retention, fsync'd audit appends, schema-version refusal, connection caps,
in-flight caps with backpressure (configurable flake fix `d6fb93e`), RequestID
correlation under multi-operator race. Unproven: SIGKILL subprocess crash matrix
(deferred to Stage 4), disk-full behavior, partial-write at the OS level, DNS/
timeout/retry-storm behavior of real endpoints, long-run endurance, Windows vs
POSIX divergence under lock contention. Observed this review: two `git`
invocations exceeded 300 s (workspace reliability risk; cause not established —
the project's own deferred register retracted OneDrive as the earlier file-loss
root cause, which was a test's `os.RemoveAll` bug, fixed in `7dc8db0`/`990426b`).

## 9. Interoperability maturity

Every "interoperability" result in the tree is **fixture or httptest
self-compatibility** (CONTROLLED_SYNTHETIC at best). No CONTROLLED_LIVE_VERIFIED
claim is supportable from this repository: no live tenant, no real KDC, no real
IdP, no real IMDS endpoint, no two-host teamserver deployment, no external
XML-DSig verifier. The prompt's claimed "7/7 Tier-0/1 targets" and "32/32
conformance fixtures" have **no artifacts in this tree** (`CONTRADICTED`/
`UNVERIFIED` for this repository). The three interop claims most likely to fail
first contact with a real implementation: SAML signature verification (§7,
S-R2), Kerberos AS-REQ against a real KKDCP (structurally reworked but
unproven), and MS-OAPX broker grant/proof (explicitly deferred).

## 10. Testing maturity

Real strengths: ~389 test functions across 69 files; 27/27 packages green
(locally re-run, exit 0); integration tier (`//go:build integration`) populated
(8+ e2e tests); multi-operator race tests; negative-path depth in
identity/traversal/version/alg-confusion suites; confidence caps tested
(`0% (UNKNOWN)` at zero observations, 66% prediction cap); flake fixes committed
with root-cause comments. Genuine blind spots: **zero native fuzz targets**
(S-R4); race/lint only CI-authoritative (config verified, run results not
retrievable locally); no endurance/fault-injection harness artifacts; no
coverage measurement in CI; test data realism limited (fixture JWTs/DER built
in-test rather than captured from real implementations). The test pyramid is
bottom-heavy in a good way but has **no middle-top**: nothing above the
fixture tier exists.

## 11. Operability maturity

Good: cobra CLI with help/man/completions, passphrase-gated workspaces,
`--i-know-what-im-doing` gating, dashboard loopback default, SECURITY.md with
disclosure SLAs, honest "Known limitations" section in README. Gaps: no
structured logging of events (zap is a dependency; adoption uneven), no
metrics/health endpoints beyond dashboard, no upgrade/migration tooling between
binary versions beyond the legacy workspace importer, no operational runbooks,
no troubleshooting guide, first-run experience assumes expert knowledge.
Installation today = build from source or an in-repo 23 MB binary — no release
distribution exists.

## 12. Release and supply-chain maturity

**This is the weakest dimension.** Facts: zero tags in the entire repository;
VERSION truth consistent (`3.4.0-stage3` in VERSION, CHANGELOG, committed
binary) but ldflags-only (`go run` reports `dev`); no SBOM, no checksums, no
signed artifacts, no release workflow (`ci.yml` has no release job; Makefile
has cosign/syft *targets* requiring external tools not evidenced as used); a
23 MB binary committed to git (23 MB of git history per rebuild); CI third-party
actions pinned only by major tag (`@v4`, `@v5`, `@v6` — SHA-pinning absent);
govulncheck CI-present and locally verified (0 affecting, 2 uncalled module
findings); Makefile `sign`/`sbom` targets are DOCUMENTED_ONLY. A compromised
CI job could alter artifacts undetectably today. `RELEASE-BLOCKING` for any
public distribution.

## 13. Open-source project maturity

Present: LICENSE, SECURITY.md with response targets, README with honest
limitations, CHANGELOG discipline per stage, threat model per stage. Missing:
contribution guidance, issue/PR templates, CI status communication, versioned
releases (tags), installable distribution, verification instructions for users,
deprecation policy, API stability statement. `TECHNICAL_MATURITY` ≫
`PROJECT_MATURITY` ≈ `ADOPTION_MATURITY`.

## 14. Current-cycle execution review

The Stage 3 cycle (and the concurrent Stage 6 documentation cycle) shows high
evidence discipline where it applies: truthful README limitations, retracted
misdiagnoses in writing (OneDrive → test-cleanup root cause), commit-level
traceability, honest deferred registers. But it also shows: evidence claims
slightly ahead of reality (the fuzz claim in `stage6-baseline.md` is
`CONTRADICTED`; stage-report "fuzz smoke PASS" language has no fuzz-target
basis), a persistent pattern of stage reports written before artifacts exist,
and no mechanism that prevents a report from outrunning the repository — the
exact failure mode this review's `BASELINE_ERROR` exemplifies.

## 15. Achieved / partially achieved / not achieved

**Achieved (evidence-backed):** fail-closed mutation governance; tamper-evident
audit; encrypted workspace with safe lifecycle; crash-safe single-writer vault;
mTLS teamserver with cert-bound identity and capability authz; protocol v2
multiplexing/resume; broad offline protocol tooling; reporting/export; CI with
race on two OSes; dependency hygiene (0 affecting vulns).

**Partially achieved:** live-protocol correctness (structural, not
interoperable); plugins (no runtime); watch (no live); planner determinism;
revocation (connection-time); crash matrix (partial coverage, no artifact).

**Not achieved:** any live-lab or real-implementation interoperability evidence;
native fuzzing; request idempotency; release engineering (tags/checksums/SBOM/
signing/distribution); external validation of any kind; two-host deployment
evidence; ztna execution semantics; true PQC.

**Honest answers:** most difficult accomplishments — the spine/vault/audit
governance plane and the Stage 3 PKI teamserver. Most reduced risk — silent
data loss and unattributed mutations. Overestimated — protocol readiness.
Underestimated — release engineering and live validation cost. Largest maturity
gap — zero evidence above the fixture tier. Largest security gap — unfuzzed
parsers + no artifact trust chain. Largest operational gap — runbooks/
distribution. Largest interop gap — everything below the self-test ceiling.
Largest release-trust gap — no tags, no signatures, no reproducible release.
External expert challenge #1 — "show me one capture from a real tenant."

## 16. Root causes of remaining gaps

| Gap | Root cause | Min closure | Priority | Release-blocking? |
|---|---|---|---|---|
| No live validation | no lab tenants/harness ever built | authorized lab tenant + capture harness | P0 (for RC) | public: yes |
| No native fuzz | fuzzing conflated with `simulate` engine | add `Fuzz*` targets for parsers | P0 | public: yes |
| No release chain | never reached a release stage | goreleaser + checksums + tag + provenance | P0 (public) | public: yes |
| ztna no-op | stubbed capability, never finished | implement transport or relabel/deny | P1 | claim-truth fix |
| SAML interop | no C14N engine | real C14N/SignedInfo, or relabel | P1 | claim-scope fix |
| Idempotency | persistence design deferred | RequestID→ActionID ledger | P1 | no (documented) |
| OCSP/CRL | operational scope | CRL distribution | P2 | enterprise: partial |
| Bus factor | project reality | contribution docs | P3 | no |

## 17. Maturity scorecard (0–7, evidence-justified)

| Domain | Score | Basis |
|---|---|---|
| Architecture | 5 | clean layering, canonical contracts, fail-closed spine |
| Protocol engineering | 3 | broad and structured, but self-validated only |
| Identity/cloud correctness | 2 | epistemic honesty good; no live/real-impl proof |
| Security engineering | 4 | strong controls; unfuzzed parsers, no artifact trust chain |
| Reliability | 3 | crash/lock tests pass; OS-level and endurance unproven |
| Interoperability | 1 | fixture-only; three known misrepresentations |
| Testing | 4 | 27/27 green locally, deep negative paths; no fuzz/coverage |
| Cross-platform | 3 | 3-OS CI build+race; no platform-specific validation |
| Operability | 2 | expert-usable; no runbooks/distribution/observability |
| Observability | 2 | dashboard + audit; no structured event log/metrics |
| Release engineering | 1 | VERSION truth yes; zero tags/artifacts/signing |
| Supply-chain | 2 | govulncheck green (verified); unpinned actions; no SBOM |
| Documentation | 4 | honest, detailed, stage-linked; one doc-drift item |
| Maintainability | 3 | clean deps, good comments; single-maintainer, naming debt |
| Open-source maturity | 2 | SECURITY/LICENSE/CHANGELOG yes; no releases/templates |
| External validation | 0 | none exists |

Three strongest: architecture, testing depth, documentation honesty.
Three weakest: external validation (0), release engineering (1),
interoperability (1). Three highest-impact improvements: (1) native fuzz
targets for every parser; (2) one authorized live-lab harness producing real
captures; (3) goreleaser + tag + checksums + provenance.

## 18. Context-specific release decision

| Context | Status | Blocking gaps |
|---|---|---|
| Expert personal lab | **FUNCTIONAL / INTERNAL LAB READY** | none if used knowingly |
| Authorized internal team | **STAGED RELEASE CANDIDATE (conditional)** | idempotency, revocation, runbooks; written ops rules |
| Controlled enterprise use | **EXPERIMENTAL** | OCSP/CRL, idempotency, observability, support |
| Public open-source release | **NOT READY** | no tags/artifacts/signing/SBOM; ztna+SAML+PQC truth; zero external validation |
| Production infrastructure | **NOT READY** | all of the above; no deployment evidence |

## 19. Next-cycle prioritization

P0: (1) native `FuzzXxx` targets for protocol/DER/JWT parsers + fix the fuzz
claim in `stage6-baseline.md`; (2) authorized live-lab harness (Kerberos KKDCP,
one IdP flow, IMDS) producing sanitized captures; (3) release engineering
(goreleaser, tag, checksums, SBOM, provenance; stop tracking `bin/aether.exe`).
P1: ztna truth-fix, SAML C14N or relabel, RequestID→ActionID idempotency
ledger, CRL distribution. P2: structured logging, coverage in CI, SHA-pinned
actions, runbooks. P3: plugin runtime loading, watch live tenants. P4: PQ
research (remain detection-only until real PQ primitives exist).

## 20. Residual risks

All protocol interop unproven beyond fixtures; parsers unfuzzed; race/lint
CI-authoritative without locally retrievable run evidence; audit trust-root
lifecycle undefined; single-maintainer project; slow synced repo path; 23 MB
committed binary bloats history; three capability-truth findings (ztna, SAML
interop, PQC naming) remain user-facing.

## 21. Evidence index (principal claims)

| ID | Claim | Evidence | Verified? |
|---|---|---|---|
| E1 | Build/vet/unit/integration/govulncheck green | local runs this review, exit 0 (see baseline doc table) | SOURCE_VERIFIED |
| E2 | Zero native fuzz targets | search `f.Fuzz(`/`testing.F` → 0 hits; only `engine/validate/fuzz.go:32` | SOURCE_VERIFIED |
| E3 | Fuzz-PASS claims without fuzz targets | `stage6-baseline.md:40`; also propagated to the committed `stage6-final-report.md:55,68` ("fuzz PASS" / "Integration/fuzz PASS") | CONTRADICTED |
| E4 | ztna no-op | `internal/engine/exec/ztna.go:171-191` | SOURCE_VERIFIED |
| E5 | SAML self-consistent signing | `internal/protocol/saml/signature.go:42-61` | SOURCE_VERIFIED |
| E6 | PQC substring-only | `internal/protocol/oauth2/pqc.go:37-48` | SOURCE_VERIFIED |
| E7 | Kerberos rework + placeholder key | `cloudkerberos.go:186-396`; README limitation | SOURCE_VERIFIED |
| E8 | Ed25519 chain, fsync, two backends | `internal/store/audit.go:1-80` | SOURCE_VERIFIED |
| E9 | Spine 8-stage fail-closed | `internal/engine/spine/spine.go:1-130` | SOURCE_VERIFIED |
| E10 | Version truth (and ldflags-only) | `VERSION`; `bin/aether.exe --version`; `go run` → `dev` | SOURCE_VERIFIED |
| E11 | No artifacts/tags/stage4-5 | fs listing; `git tag -l` empty; `git log --all` = 36 | SOURCE_VERIFIED |
| E12 | CI configuration | `.github/workflows/ci.yml` | SOURCE_VERIFIED (config only; run results not retrievable) |
| E13 | govulncheck 0 affecting | local `govulncheck ./...` | TEST_VERIFIED |
| E14 | Prompt's Stage 5 evidence set | absent from tree (`artifacts/` absent) | CONTRADICTED/UNVERIFIED |
| E15 | File-loss root cause retraction | `docs/stage3-deferred.md` item 13; commits `990426b`,`7dc8db0` | SOURCE_VERIFIED |

## 22. Final conclusion

Aether 3.4.0-stage3 is a **credible, honestly-documented, architecturally sound
M3 verified-engineering system with an M1–M2 protocol layer and an M1 release
process**. It is not a Stage 5 release candidate and must not be represented as
one. The single largest blocker to the next maturity level is the total absence
of evidence above the fixture tier: no fuzzing, no live captures, no release
artifacts, no external validation. The correct next action is not more
implementation volume — it is the P0 validation-and-release track (§19),
followed by honest capability-truth fixes (ztna, SAML, PQC naming).

Percentages (defined denominators): vision implementation coverage ≈ 70% of the
offline-capability vision exists in some form; verification coverage ≈ 60% of
implemented surface has meaningful tests, ≈ 5% has real-world validation;
release readiness ≈ 25% (truth + CI only); operational maturity ≈ 30%.
These measure *evidence-backed capability*, not effort.
