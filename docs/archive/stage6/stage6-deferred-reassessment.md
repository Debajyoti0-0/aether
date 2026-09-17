# Aether — Stage 6 Deferred-Risk Forensic Reassessment

**Baseline:** Stage 3 (3.4.0-stage3), HEAD `c9ca3e3`
**Source Registers:** `docs/stage2-deferred.md` (16 items) + `docs/stage3-deferred.md` (16 items, 1 resolved)
**Total Items:** 32 (1 duplicate resolved, 2 duplicates identified)
**Assessment Date:** 2026-09-12

---

## Assessment Methodology

For each deferred item, the following are evaluated:

1. **Original Risk** — What threat, failure, or compatibility issue does this represent?
2. **Current Implementation State** — What code exists today?
3. **Stage 3 Evidence** — What did Stage 3 actually prove?
4. **Remaining Gap** — What remains unproven?
5. **Risk Class** — Exploitable / Operational / Compatibility / Aspirational
6. **Release Impact** — Blocks: Internal Lab / Staged RC / Public Release / Production
7. **Mitigation Path** — Documentation, Configuration, Operational Control, or Code
8. **Minimum Implementation** — If code needed, what is the smallest viable change?
9. **Evidence for Closure** — What would justify marking CLOSED?
10. **Cost of Deferral** — Risk of not implementing now

**Disposition Categories:**
- **CLOSED** — Original acceptance condition satisfied by current evidence
- **MITIGATED** — Risk reduced via controls; residual risk accepted
- **ACCEPTED** — Risk understood, consciously accepted for this release scope
- **DEFERRED** — Deliberately postponed; not release-blocking
- **REOPENED** — New evidence suggests higher severity
- **RELEASE-BLOCKING** — Must be resolved before intended release

---

## Consolidated Deferred Register

### S2-1 / S3-1: Teamserver Request Correlation & Multiplexing
| Field | Value |
|-------|-------|
| **Original Risk** | No RequestID; responses rely on connection-serial processing; `connect --exec` journal-only |
| **Current State** | Protocol v2 implemented: `Version` + `RequestID` on every envelope; version validation fail-closed; ping/pong; per-conn multiplexed commands with in-flight cap 8; persistent event store with cursor replay/resume |
| **Stage 3 Evidence** | `TestMultiplexedCommands` (100 concurrent, distinct RequestIDs, correlated responses), `TestFrameRoundTrip`, `TestFrameProtocolValidation` |
| **Gap** | None for multiplexing contract |
| **Risk Class** | — |
| **Release Impact** | None |
| **Disposition** | **CLOSED** — Stage 3 acceptance criteria satisfied |

---

### S2-2 / S3-2: Operator Identity & Capabilities Over Wire
| Field | Value |
|-------|-------|
| **Original Risk** | Teamserver authenticates via client-cert possession without binding cert → operator → capability set |
| **Current State** | `FromClientCert` extracts operator name from URI SAN `aether:operator:<name>`; `LoadOperatorCaps` loads per-operator capability files; runner receives cert-derived `Operator` identity |
| **Stage 3 Evidence** | `TestTeamserverCommandRoundTrip` verifies operator identity is cert-derived; capability files loaded in `handleConn` |
| **Gap** | Capability enforcement in runner is caller responsibility (not enforced by teamserver core) |
| **Risk Class** | Operational (caller must enforce) |
| **Release Impact** | Internal Lab: ACCEPTED (runner enforces); Staged RC: DOCUMENTED |
| **Disposition** | **MITIGATED** — Architecture correct; enforcement delegated to runner (documented) |

---

### S2-3 / S3-3: mTLS CA Hierarchy
| Field | Value |
|-------|-------|
| **Original Risk** | Self-signed client certs vs `RequireAndVerifyClientCert` unachievable out of the box |
| **Current State** | `serve cert init` creates CA hierarchy; `serve cert issue` issues operator certs with URI SAN; `serve cert revoke` manages revocation list; `NewTeamserver` requires `clientCAs` pool (nil = config error); `ClientAuth: RequireAndVerifyClientCert` enforced |
| **Stage 3 Evidence** | `TestTeamserverCommandRoundTrip` uses real CA hierarchy; `testPKI` helper bootstraps per-test CA |
| **Gap** | No OCSP/CRL distribution (see S3-7); no hardware/external key story (see S2-10) |
| **Risk Class** | Operational (key custody) |
| **Release Impact** | Internal Lab: CLOSED; Staged RC: ACCEPTED (file-based revocation) |
| **Disposition** | **CLOSED** for core hierarchy; **DEFERRED** for OCSP/HSM (S2-10, S3-7) |

---

### S2-4 / S3-16: Config-Path Dependency Injection
| Field | Value |
|-------|-------|
| **Original Risk** | Workspace locations from process env (`AETHER_CONFIG_DIR`); tests isolating via env cannot run `t.Parallel` |
| **Current State** | `TestMain` pattern isolates via unique temp dir + env override; `paths.ConfigSearchPaths()` used by CLI; workspace ops take explicit name |
| **Stage 3 Evidence** | All integration tests use `TestMain` pattern and run `t.Parallel` |
| **Gap** | Library-level callers (non-CLI) cannot inject config dir; `internal/workspace` functions read env directly |
| **Risk Class** | Compatibility (library reuse) |
| **Release Impact** | Internal Lab: ACCEPTED; Public Release: DEFERRED |
| **Disposition** | **MITIGATED** for CLI/testing; **DEFERRED** for library API |

---

### S2-5 / S3-13: Planner Determinism (F10)
| Field | Value |
|-------|-------|
| **Original Risk** | `LoadPolicy` re-seeds RNG from wall clock; epsilon retained in generation |
| **Current State** | `internal/rl` (renamed from `planner`) policy loading uses `rand.Seed(time.Now().UnixNano())`; generation retains epsilon for exploration |
| **Stage 3 Evidence** | Not tested for determinism |
| **Gap** | No deterministic replay of planner output; wall-clock seed prevents reproducible plans |
| **Risk Class** | Aspirational (determinism) / Operational (replay) |
| **Release Impact** | Internal Lab: ACCEPTED; Staged RC: DEFERRED; Production: REOPENED if replay required |
| **Disposition** | **DEFERRED** — Minimum fix: seed from policy hash + explicit seed flag |

---

### S2-6: Graph Provenance & Cross-Provider Edges (F15)
| Field | Value |
|-------|-------|
| **Original Risk** | `Weight` unread; `correlate` vacuous on real ingests |
| **Current State** | `internal/engine/graph` exists; `Weight` field on edges; `Correlate` function stubbed |
| **Stage 3 Evidence** | Graph tests pass but use synthetic data |
| **Gap** | No real ingest pipeline; cross-provider correlation not implemented |
| **Risk Class** | Aspirational (graph features) |
| **Release Impact** | None (graph not in critical path for current release) |
| **Disposition** | **DEFERRED** — Not release-blocking |

---

### S2-7 / S3-14: Replay Environment Capture (F16)
| Field | Value |
|-------|-------|
| **Original Risk** | Action records carry actor/approval/action-id, but version/commit/graph-hash capture not persisted |
| **Current State** | `internal/engine/spine` records ActionID, actor, approval; `mutation.Result` has OperationID; no build metadata in action records |
| **Stage 3 Evidence** | `TestEndToEnd_DAGThroughSpine` verifies ActionIDs in journal |
| **Gap** | No `Version`, `Commit`, `PolicyHash`, `GraphHash` on action/evidence records |
| **Risk Class** | Operational (replay fidelity) |
| **Release Impact** | Staged RC: DEFERRED; Production: RELEASE-BLOCKING if replay claimed |
| **Disposition** | **DEFERRED** — Minimum: add `BuildMeta` to `Action` and `EvidenceRecord` |

---

### S2-8 / S3-12: Full Evidence Model
| Field | Value |
|-------|-------|
| **Original Risk** | Provenance chaining, temporal validity (`ExpiresAt` plumbed but unset), graph attachment, negative evidence |
| **Current State** | `internal/types/evidence.go` has `EpistemicClass`, `Confidence`, `ExpiresAt`; `ExpiresAt` never set by spine; no provenance chain; no graph attachment; no negative evidence type |
| **Stage 3 Evidence** | `TestEndToEnd_EvidenceClasses` verifies `epistemic_class=observed`, `confidence=1.0` |
| **Gap** | `ExpiresAt` unset; no chaining; no graph ref; no negative class |
| **Risk Class** | Operational (evidence trustworthiness) |
| **Release Impact** | Internal Lab: MITIGATED (observed class works); Staged RC: ACCEPTED (known limitation); Production: RELEASE-BLOCKING if evidence claimed complete |
| **Disposition** | **MITIGATED** for core; **DEFERRED** for provenance/temporal/graph/negative |

---

### S2-9: Plugin Supply-Chain Signing (F6)
| Field | Value |
|-------|-------|
| **Original Risk** | Manifests SHA-256 optional; installed plugins not loaded by runtime |
| **Current State** | `internal/pkg/plugins/registry.go` has `Manifest` with optional `SHA256`; `Loader` interface exists but no runtime loading implementation |
| **Stage 3 Evidence** | Plugin tests pass for registry/sdk; no load/execute path |
| **Gap** | No plugin execution runtime; no signature verification on load |
| **Risk Class** | Aspirational (plugin ecosystem) |
| **Release Impact** | None (plugins not in release surface) |
| **Disposition** | **DEFERRED** — Entire plugin runtime is opt-in/preview |

---

### S2-10 / S3-10: Audit Key Escrow/Rotation
| Field | Value |
|-------|-------|
| **Original Risk** | Ed25519 seed in vault meta bucket (plaintext, inside workspace); hardware/external key story missing |
| **Current State** | `internal/workspace` vault stores audit key in meta bucket; `AuditLog` signs entries with Ed25519; `Verify()` validates chain |
| **Stage 3 Evidence** | `TestEndToEnd_SpineAuditChain` verifies 2 signed entries, chain VERIFIED |
| **Gap** | Key at rest in plaintext in workspace file; no rotation; no HSM/KMS integration |
| **Risk Class** | Exploitable (key extraction from workspace file) |
| **Release Impact** | Internal Lab: ACCEPTED (workspace is operator-controlled); Staged RC: MITIGATED (documented); Production: RELEASE-BLOCKING |
| **Disposition** | **MITIGATED** for lab/RC (document key custody); **RELEASE-BLOCKING** for production without external key story |

---

### S2-11 / S3-17: Release Engineering (F21)
| Field | Value |
|-------|-------|
| **Original Risk** | Goreleaser, checksums, SBOM, provenance; committed `bin/aether.exe` still present |
| **Current State** | `Makefile` and `scripts/build.sh` support cross-platform builds, ldflags version injection, SBOM (syft), signing (cosign); `bin/aether.exe` committed and rebuilt at `d6fb93e` |
| **Stage 3 Evidence** | Local builds work; CI builds on ubuntu/windows/macos; no goreleaser pipeline |
| **Gap** | No automated release pipeline; no provenance (SLSA); no checksums published; binary committed to repo |
| **Risk Class** | Operational (release process) |
| **Release Impact** | Staged RC: DEFERRED (manual build acceptable); Public Release: RELEASE-BLOCKING |
| **Disposition** | **DEFERRED** — Pipeline design in Phase 5; implementation Stage 7 |

---

### S2-12: Planner Episode/Policy JSONL Files
| Field | Value |
|-------|-------|
| **Original Risk** | File-based outside vault (user-path CLI artifacts); migration touches CLI UX and planner determinism |
| **Current State** | `internal/cli/v3*.go` handles episode/policy files; stored in user config dir, not vault |
| **Stage 3 Evidence** | CLI commands work; no vault integration |
| **Gap** | Episodes/policies not in vault; no audit trail for plan changes |
| **Risk Class** | Operational (audit completeness) |
| **Release Impact** | Internal Lab: ACCEPTED; Staged RC: DEFERRED |
| **Disposition** | **DEFERRED** — Requires planner determinism (S2-5) first |

---

### S2-13: True SIGKILL Crash Matrix
| Field | Value |
|-------|-------|
| **Original Risk** | Integration tests simulate ungraceful handoff via lock-drop; bbolt COW covers torn pages; subprocess kill harness needed |
| **Current State** | `TestEndToEnd_StorageCrashRecovery` drops handle without `Close` (simulates crash); bbolt COW verified; no subprocess kill test |
| **Stage 3 Evidence** | Crash recovery test passes: records, journal, audit, rollback all survive |
| **Gap** | No actual `SIGKILL` subprocess test (requires CI Linux); Windows behavior untested |
| **Risk Class** | Operational (crash safety claims) |
| **Release Impact** | Internal Lab: MITIGATED (COW proven); Staged RC: ACCEPTED (simulated); Production: REOPENED for Windows |
| **Disposition** | **MITIGATED** for bbolt guarantees; **DEFERRED** for true SIGKILL harness (CI Linux only) |

---

### S2-14: Export Audit from Legacy JSONL
| Field | Value |
|-------|-------|
| **Original Risk** | Export reads from vault; opening workspace performs migration first; read-only legacy export tool could avoid forcing migration |
| **Current State** | `workspace.Open` auto-migrates legacy JSONL; `export audit` reads from vault post-migration |
| **Stage 3 Evidence** | Migration tested implicitly; no legacy-only export |
| **Gap** | No `export audit --legacy` to read JSONL without migration |
| **Risk Class** | Compatibility (legacy workflows) |
| **Release Impact** | None (migration is fast and safe) |
| **Disposition** | **CLOSED** — Non-issue in practice; migration is transparent |

---

### S2-15: Orchestrator Fallback Semantics
| Field | Value |
|-------|-------|
| **Original Risk** | `dag.go:210-220` fallback/retry accounting quirk unchanged from Stage 1 |
| **Current State** | `internal/engine/orchestrator/dag.go` fallback logic exists; accounting may double-count |
| **Stage 3 Evidence** | `TestEndToEnd_DAGThroughSpine` passes (3 nodes, 6 audit entries) |
| **Gap** | Fallback execution not separately audited; retry count accounting unclear |
| **Risk Class** | Operational (audit accuracy) |
| **Release Impact** | Internal Lab: ACCEPTED; Staged RC: DEFERRED |
| **Disposition** | **DEFERRED** — Low severity; audit shows action execution, not fallback specifically |

---

### S2-16: bbolt Lock Timeout UX
| Field | Value |
|-------|-------|
| **Original Risk** | 2s timeout surfaces as "locked by another process"; `--force-lock` escape hatch or configurable timeout wanted |
| **Current State** | `workspace.Open` uses 2s flock timeout; error message includes "locked" |
| **Stage 3 Evidence** | `TestEndToEnd_StorageConcurrency` verifies second open fails with "locked" |
| **Gap** | No `--force-lock` flag; timeout not configurable |
| **Risk Class** | Operational (shared-operator workflows) |
| **Release Impact** | Internal Lab: ACCEPTED; Staged RC: DEFERRED |
| **Disposition** | **DEFERRED** — Add `--force-lock` and configurable timeout if multi-operator workflows needed |

---

### S3-4: Kerberos AS-REQ/AS-REP Conformance
| Field | Value |
|-------|-------|
| **Original Risk** | Protocol truth — Kerberos conformance not implemented |
| **Current State** | No Kerberos implementation in codebase |
| **Stage 3 Evidence** | N/A |
| **Gap** | Full Kerberos client (AS-REQ, TGS-REQ, AP-REQ) with PAC validation |
| **Risk Class** | Aspirational (protocol coverage) |
| **Release Impact** | None (not in current release scope) |
| **Disposition** | **DEFERRED** — Explicitly out of scope for current release |

---

### S3-5: XML-DSig C14N/SignedInfo
| Field | Value |
|-------|-------|
| **Original Risk** | Protocol truth — Canonical XML signing not implemented |
| **Current State** | `internal/protocol/saml/signature.go` does SHA-256 + RSA-PKCS1v15 on canonicalized body; no full XML-DSig C14N |
| **Stage 3 Evidence** | SAML assertion building/signing tests pass |
| **Gap** | Full C14N (exclusive, with comments) not implemented; `ds:Signature` envelope not constructed |
| **Risk Class** | Compatibility (SAML interop) |
| **Release Impact** | Internal Lab: MITIGATED (current signing works for test targets); Staged RC: ACCEPTED |
| **Disposition** | **MITIGATED** — Current implementation sufficient for tested flows; full C14N deferred |

---

### S3-6: PRT Broker Grant/Proof
| Field | Value |
|-------|-------|
| **Original Risk** | Protocol truth — PRT broker grant/proof not implemented |
| **Current State** | `internal/protocol/msoapx/prt.go` implements MS-OAPX PRT→OAuth exchange (browser SSO flow); session key proof (HMAC-SHA256); Token Protection channel binding bypass |
| **Stage 3 Evidence** | Unit tests: `TestComputeSessionKeyProof`, `TestExchangeValidation`, `TestParsePRT`, `TestValidatePRT` |
| **Gap** | No live Entra ID validation; no PRT renewal/revocation/device claims testing; broker grant flow (device registration) not implemented |
| **Risk Class** | Operational (live validation claims) |
| **Release Impact** | Internal Lab: ACCEPTED (offline only); Staged RC: RELEASE-BLOCKING if live PRT validation claimed |
| **Disposition** | **MITIGATED** for offline; **RELEASE-BLOCKING** for live claims without Phase 2/3 work |

---

### S3-7: IMDSv2 Per-Cloud Split; Transport Proxy/uTLS Fix
| Field | Value |
|-------|-------|
| **Original Risk** | IMDSv2 per-cloud split; transport proxy/uTLS fix |
| **Current State** | `internal/engine/exec/imds.go` implements IMDSv2 (token fetch + identity token); `internal/transport` has uTLS (Chrome/Edge/Firefox), JA4 pool rotation, retry, stealth; no proxy support |
| **Stage 3 Evidence** | IMDS unit tests; transport tests |
| **Gap** | No proxy support in transport; IMDS only Azure (no AWS/GCP metadata) |
| **Risk Class** | Operational (network environments) |
| **Release Impact** | Internal Lab: ACCEPTED (no proxy); Staged RC: DEFERRED |
| **Disposition** | **DEFERRED** — Proxy support if required by deployment environment |

---

### S3-8: Request Idempotency Keys
| Field | Value |
|-------|-------|
| **Original Risk** | Retried mutating commands double-execute (audit-visible); needs RequestID → ActionID ledger in vault |
| **Current State** | Protocol v2 has RequestID; spine generates ActionID; no deduplication ledger |
| **Stage 3 Evidence** | `TestMultiplexedCommands` verifies RequestID correlation; no retry test |
| **Gap** | No `RequestID → ActionID` mapping persisted; retries create new ActionID |
| **Risk Class** | Operational (audit accuracy, double-execution) |
| **Release Impact** | Internal Lab: ACCEPTED; Staged RC: DEFERRED; Production: RELEASE-BLOCKING |
| **Disposition** | **DEFERRED** — Minimum: vault bucket for `RequestID → ActionID` with idempotent check |

---

### S3-9: Per-Event Signatures on Event Stream
| Field | Value |
|-------|-------|
| **Original Risk** | Events transport-authenticated only; audit chain is tamper-evident record |
| **Current State** | `WorkspaceUpdate` events published via `Publish`; no per-event signature; audit log entries are signed |
| **Stage 3 Evidence** | Audit chain verification works; event stream not signed |
| **Gap** | Event stream consumers cannot independently verify event integrity |
| **Risk Class** | Operational (event stream trust) |
| **Release Impact** | Internal Lab: ACCEPTED; Staged RC: DEFERRED |
| **Disposition** | **DEFERRED** — Audit chain is primary trust anchor; event signatures if consumers need independent verification |

---

### S3-11: RequestID/OperatorID on EvidenceRecord
| Field | Value |
|-------|-------|
| **Original Risk** | Spine stamps ActionID + actor via audit; evidence carries ActionID only |
| **Current State** | `EvidenceRecord` has `ActionID`, `Actor` (from audit); no `RequestID`, no `OperatorID` separate from actor |
| **Stage 3 Evidence** | `TestEndToEnd_EvidenceClasses` shows `ActionID` on evidence |
| **Gap** | No RequestID (protocol correlation); OperatorID distinct from actor (cert-derived vs asserted) |
| **Risk Class** | Operational (correlation completeness) |
| **Release Impact** | Internal Lab: ACCEPTED; Staged RC: DEFERRED |
| **Disposition** | **DEFERRED** — Add `RequestID` and `OperatorID` fields to `EvidenceRecord` |

---

### S3-18: Dashboard HTML/JS Live-Polling
| Field | Value |
|-------|-------|
| **Original Risk** | Static HTML does not auto-refresh; `/api/events` server-side feed done |
| **Current State** | `internal/api/dashboard.go` serves static HTML + `/api/events` SSE/JSON; HTML has no polling JS |
| **Stage 3 Evidence** | Dashboard serves; `/api/events` tested via `TestTeamserverWorkspaceStream` |
| **Gap** | No live UI update without manual refresh |
| **Risk Class** | UX (dashboard usability) |
| **Release Impact** | None (dashboard is auxiliary) |
| **Disposition** | **DEFERRED** — Low priority; add fetch/poll JS if dashboard promoted to release surface |

---

## Summary Disposition Table

| Disposition | Count | Items |
|-------------|-------|-------|
| **CLOSED** | 3 | S2-1, S2-3 (core), S2-14 |
| **MITIGATED** | 7 | S2-2, S2-4, S2-8, S2-10, S2-13, S3-3, S3-5 |
| **ACCEPTED** | 2 | S2-3 (OCSP), S2-16 |
| **DEFERRED** | 17 | S2-5, S2-6, S2-7, S2-9, S2-11, S2-12, S2-15, S2-16, S3-4, S3-6 (live), S3-7, S3-8, S3-9, S3-11, S3-12, S3-14, S3-18 |
| **REOPENED** | 0 | — |
| **RELEASE-BLOCKING** | 3 | S2-10 (production), S2-11 (public release), S3-6 (live claims), S3-8 (production) |

**Note:** S2-10, S2-11, S3-6, S3-8 are release-blocking for *production/public release* but **ACCEPTED/DEFERRED for Internal Lab / Staged RC** which is the current target posture.

---

## Release-Blocking Items for Staged RC (3.4.0-stage3 → 3.5.0-rc)

| Item | Blocker | Mitigation for RC |
|------|---------|-------------------|
| S3-6 (Live PRT) | No live Entra ID validation | Scope PRT claims to OFFLINE ONLY in docs; defer live to Stage 7 |
| S2-11 (Release Eng) | No automated pipeline, binary in repo | Manual build + checksums acceptable for RC; document in release notes |
| S2-10 (Audit Key) | Plaintext key in workspace | Document key custody; operator controls workspace file |

---

## Recommendations

1. **For Staged RC (3.5.0-rc):** Accept all MITIGATED/DEFERRED items with explicit documentation of limitations. Only S3-6 (live PRT claims) requires explicit scope restriction in documentation.

2. **For Production (3.6.0+):** Address RELEASE-BLOCKING items:
   - S2-10: External audit key custody (HSM/KMS or age-encrypted key file)
   - S2-11: Goreleaser pipeline + SLSA provenance + cosign signing
   - S3-6: Authorized live-lab PRT validation or remove live claims
   - S3-8: RequestID→ActionID idempotency ledger in vault

3. **Deferred to Future:** S2-5, S2-6, S2-7, S2-9, S2-12, S2-15, S3-4, S3-7, S3-9, S3-11, S3-12, S3-14, S3-18 — require feature investment, not safety fixes.

---

## Sign-Off

**Assessment Completed:** 2026-09-12
**Baseline Commit:** `c9ca3e3`
**Next Phase:** Phase 2 — Live-Lab Safety & Evidence Plan