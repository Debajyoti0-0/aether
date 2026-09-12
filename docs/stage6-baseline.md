# Aether — Stage 6 Baseline Document

**Generated:** 2026-09-12
**Repository:** `C:\Users\Debajyoti0-0\OneDrive\Documents\aether`
**Actual Stage:** Stage 3 (3.4.0-stage3) — NOT Stage 5 as originally prompted
**HEAD Commit:** `d6fb93e` (fix(test): TestMultiplexedCommands flaky backpressure)
**Baseline Commit (pre-Stage 6):** `990426b` (fix(test): TestPlannerLegacyMigration)
**Branch:** `master`
**Go Version:** go1.27.1 windows/amd64

---

## 1. Entry Gate Verification (G14 — Adapted)

| Gate | Requirement | Command | Result | Verdict |
|------|-------------|---------|--------|---------|
| G14.1 | `git fsck --full` clean | `git fsck --full` | `dangling tree d760958…`, `dangling tree eeaad29…` (no corruption) | **PASS** (dangling trees are normal GC artifacts) |
| G14.2 | `git status --porcelain` empty | `git status --porcelain` | Clean (after commit `d6fb93e`) | **PASS** |
| G14.3 | Baseline commit verified | `git rev-parse HEAD` | `d6fb93e` (Stage 6 baseline) | **PASS** |
| G14.4 | `aether --version` matches VERSION | `bin/aether.exe --version` | `3.4.0-stage3` = `VERSION` file | **PASS** |
| G14.5 | CI green on baseline | `.github/workflows/ci.yml` | Local: build/vet/test/integration PASS | **PASS** (local verification) |
| G14.6 | Stage 5 artifacts present | `artifacts/stage5/checksums.txt` | **NOT APPLICABLE** — Repository is at Stage 3 | **N/A** |

**Note:** The original Stage 6 prompt assumed a Stage 5 baseline (`d250d52` / `3.6.0-stage5`) that does not exist. This document reflects the ACTUAL repository state at Stage 3 (3.4.0-stage3). Stage 6 work will be adapted to advance from Stage 3 toward release-candidate readiness.

---

## 2. Current Verified State (Stage 3 Complete)

### Completed Workstreams (Stage 1–3)
- **Stage 1:** Core planner, graph, vault, evidence, audit chain, CLI skeleton
- **Stage 2:** Workspace vault (bbolt), mutation spine, risk gate, policy, rollback stack, journal, evidence records, integration tier
- **Stage 3:** Teamserver v2 with mTLS (CA hierarchy, client cert auth, revocation), Protocol v2 (RequestID, multiplexing, event store with cursor replay), Spine dispatch with capability authz

### Verified Gates (Local)
- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test -count=1 ./...` — PASS (all packages)
- `go test -tags=integration ./test/integration/...` — PASS
- `go test -fuzz=Fuzz -fuzztime=10s ./internal/engine/validate/` — PASS
- `govulncheck ./...` — PASS (0 affecting vulnerabilities)

### Known Issues (Pre-Stage 6)
1. **OneDrive sync risk:** Repository in OneDrive-synced path; commit history shows repeated `internal/planner` (now `internal/rl`) deletions by cloud tombstones (commits `c8c6467`, `2faa9b3`, `9e0e62b`, `b363097`, `990426b`).
2. **Race detector:** Requires CGO (`CGO_ENABLED=1`); not available in default Windows toolchain. CI-authoritative per `.github/workflows/ci.yml`.
3. **Lint:** `golangci-lint` not installed locally; CI-authoritative.
4. **Committed binary:** `bin/aether.exe` tracked in repo (legacy; slated for goreleaser in Stage 7).
5. **No Stage 4/5 artifacts:** No protocol-conformance fixtures, interop matrix, SBOM, crash matrix, deferred register for Stage 4/5.

---

## 3. Scope of Stage 6 (Adapted)

Given the actual baseline (Stage 3), Stage 6 objectives are re-scoped:

### Primary Objectives
1. **Deferred-risk reassessment** of Stage 2/3 deferred items (16 items from `stage2-deferred.md` + 16 from `stage3-deferred.md` = 32 total)
2. **Live-lab validation architecture** design (safety-bounded, authorized targets only)
3. **PRT validation scope** documentation (epistemic separation: offline vs live claims)
4. **Artifact signing & trust chain** design (cosign/Sigstore, SLSA, Authenticode decision)
4. **CI/cross-platform/supply-chain** revalidation
5. **Adversarial regression** hardening
6. **Version/release surface** audit
7. **Release-candidate certification** with explicit residual risk

### Non-Goals (Deferred to Stage 7+)
- Actual live-lab captures (requires authorized tenant infrastructure)
- Live PRT validation against real Entra ID (requires authorized lab tenant)
- Full SBOM/checksum generation (requires goreleaser/syft pipeline)
- Binary signing (requires key custody setup)

---

## 4. Deferred Items Register (Consolidated)

### From Stage 2 (stage2-deferred.md) — 16 items
| ID | Item | Status | Notes |
|----|------|--------|-------|
| S2-1 | Teamserver request correlation & multiplexing | **DONE** (Stage 3) | Protocol v2 with RequestID, per-conn multiplexing cap 8 |
| S2-2 | Operator identity & capabilities over wire | **DONE** (Stage 3) | Cert-derived operator identity, capability files |
| S2-3 | mTLS CA hierarchy | **DONE** (Stage 3) | `serve cert init/issue/revoke`, `RequireAndVerifyClientCert` |
| S2-4 | Config-path dependency injection | **PARTIAL** | TestMain pattern works; library-level DI pending |
| S2-5 | Planner determinism (F10) | **DEFERRED** | `LoadPolicy` wall-clock seed; epsilon in generation |
| S2-6 | Graph provenance & cross-provider edges (F15) | **DEFERRED** | Weight unread; correlate vacuous |
| S2-7 | Replay environment capture (F16) | **DEFERRED** | Version/commit/graph-hash not persisted |
| S2-8 | Full evidence model | **DEFERRED** | Provenance chaining, temporal validity, negative evidence |
| S2-9 | Plugin supply-chain signing (F6) | **DEFERRED** | Manifests SHA-256 optional; runtime loading missing |
| S2-10 | Audit key escrow/rotation | **DEFERRED** | Ed25519 seed in vault meta (plaintext) |
| S2-11 | Release engineering (F21) | **DEFERRED** | Goreleaser, checksums, SBOM; `bin/aether.exe` committed |
| S2-12 | Planner episode/policy JSONL files | **DEFERRED** | File-based outside vault; CLI UX migration needed |
| S2-13 | True SIGKILL crash matrix | **DEFERRED** | Subprocess kill harness needed (CI Linux) |
| S2-14 | Export audit from legacy JSONL | **DEFERRED** | Non-issue in practice (migration on open) |
| S2-15 | Orchestrator fallback semantics | **DEFERRED** | `dag.go:210-220` retry accounting quirk |
| S2-16 | bbolt lock timeout UX | **DEFERRED** | 2s timeout; `--force-lock` escape hatch wanted |

### From Stage 3 (stage3-deferred.md) — 16 items
| ID | Item | Status | Notes |
|----|------|--------|-------|
| S3-1 | T6 SIGKILL subprocess crash matrix | **DEFERRED** | `cmd/crashtester` + 8-stage kill/reopen suite |
| S3-2 | Kerberos AS-REQ/AS-REP conformance | **DEFERRED** | Protocol truth (Stage 4) |
| S3-3 | XML-DSig C14N/SignedInfo | **DEFERRED** | Protocol truth (Stage 4) |
| S3-4 | PRT broker grant/proof | **DEFERRED** | Protocol truth (Stage 4) |
| S3-5 | IMDSv2 per-cloud split | **DEFERRED** | Transport proxy/uTLS fix |
| S3-6 | Request idempotency keys | **DEFERRED** | ActionID ledger in vault needed |
| S3-7 | OCSP/CRL for operator revocation | **DEFERRED** | revoked.txt is file-based |
| S3-8 | Per-IP rate limiting + idle reaping | **DEFERRED** | Adaptive limits missing |
| S3-9 | Per-event signatures on event stream | **DEFERRED** | Events transport-auth only; audit chain is tamper-evident |
| S3-10 | Rollback lifecycle states in action machine | **DEFERRED** | Unify into ActionState |
| S3-11 | RequestID/OperatorID on EvidenceRecord | **DEFERRED** | Spine stamps ActionID+actor; evidence has ActionID only |
| S3-12 | Full provenance/confidence/temporal model | **DEFERRED** | Unchanged from S2-8 |
| S3-13 | Planner determinism (F10) | **DEFERRED** | Duplicate of S2-5 |
| S3-14 | Replay environment capture (F16) | **DEFERRED** | Duplicate of S2-7 |
| S3-15 | OneDrive sync race | **RESOLVED** | Root cause: `TestPlannerLegacyMigration` `RemoveAll` bug (fixed in `990426b`) |
| S3-16 | Config-path DI (full) | **DEFERRED** | Beyond TestMain pattern |
| S3-17 | `bin/aether.exe` committed | **DEFERRED** | Goreleaser/checksums/SBOM (Stage 7) |
| S3-18 | Dashboard HTML/JS live-polling | **DEFERRED** | Static HTML; `/api/events` feed done |

---

## 5. Claim-to-Code-to-Test Matrix (Phase 0)

| Claim | Implementation | Test/Evidence | Evidence Type | Confidence | Gap |
|-------|----------------|---------------|---------------|------------|-----|
| mTLS mutual auth | `internal/api/server.go`, `client.go` | `TestTeamserverCommandRoundTrip`, `TestFrameRoundTrip` | Unit/Integration | High | Cross-platform cert validation untested |
| Protocol v2 multiplexing | `serveConn`, `readLoop` | `TestMultiplexedCommands` (fixed), `TestFrameProtocolValidation` | Unit | High | Race detector CI-only |
| WS-Trust/SAML relay | `internal/engine/relay/wstrust.go`, `protocol/wstrust` | Unit tests only | Unit | Medium | No live STS interop |
| PRT→OAuth exchange | `internal/engine/token/prt.go`, `protocol/msoapx` | Unit tests with mock server | Unit | Medium | No live Entra ID validation |
| IMDS identity hijack | `internal/engine/exec/imds.go` | Unit tests only | Unit | Low | Requires Azure VM context |
| Evidence integrity | `internal/workspace`, `engine/spine` | `TestEndToEnd_EvidenceClasses`, `TestEndToEnd_SpineAuditChain` | Integration | High | Provenance chaining missing |
| Single-writer vault | `internal/workspace` (flock) | `TestEndToEnd_StorageConcurrency`, `TestEndToEnd_StorageCrashRecovery` | Integration | High | True SIGKILL untested |
| PKI lifecycle | `internal/api/ca.go`, `identity.go` | Unit tests | Unit | High | OCSP/CRL missing |
| uTLS browser fingerprinting | `internal/transport/tls.go`, `stealth.go` | Unit tests | Unit | Medium | JA3/JA4 rotation untested live |

---

## 6. Baseline Artifacts

| Artifact | Path | Status |
|----------|------|--------|
| VERSION | `VERSION` | `3.4.0-stage3` |
| CHANGELOG | `CHANGELOG.md` | Head: `v3.4.0-stage3` |
| CI Workflow | `.github/workflows/ci.yml` | 8 jobs (governance, build×3, vet, test×2, lint, vuln, integration) |
| Go Module | `go.mod` | go 1.26.0, 9 direct deps |
| Stage 2 Deferred | `docs/stage2-deferred.md` | 16 items |
| Stage 3 Deferred | `docs/stage3-deferred.md` | 16 items (+1 resolved) |
| Stage 3 Evidence | `docs/stage3-teamserver-evidence.md` | Teamserver v2 validation |
| Threat Model | `docs/stage3-threat-model.md` | STRIDE for teamserver |

---

## 7. Stage 6 Work Plan (Adapted)

### Phase 0: Baseline Lock & Repository Audit ✓ (This Document)
### Phase 1: Deferred-Risk Forensic Reassessment (32 items)
### Phase 2: Live-Lab Safety & Evidence Plan
### Phase 3: PRT Validation Scope Documentation
### Phase 4: Interoperability Validation Design (WS-Trust, SAML, PRT, IMDS)
### Phase 5: Artifact Signing & Trust Chain Architecture
### Phase 6: CI/Cross-Platform/Supply-Chain Review
### Phase 7: Adversarial Regression (Fuzz, Malformed, Negative Paths)
### Phase 8: Version/Release Surface Audit
### Phase 9: Release-Candidate Certification

---

## 8. Deviations from Original Prompt

| Original Prompt Assumption | Actual State | Adaptation |
|---------------------------|--------------|------------|
| Stage 5 COMPLETE (`3.6.0-stage5`) | Stage 3 COMPLETE (`3.4.0-stage3`) | Stage 6 re-scoped to advance from Stage 3 |
| Baseline commit `d250d52` | HEAD `990426b` / Stage 6 baseline `d6fb93e` | New baseline established |
| 15 Stage 5 deferred items | 32 Stage 2/3 deferred items | Consolidated register; forensic reassessment |
| Live-lab captures pending | No live-lab infrastructure | Architecture design only; captures deferred |
| Live PRT validation pending | No authorized tenant | Scope documentation only; validation deferred |
| SBOM/checksums exist | No artifacts directory | Design signing pipeline; generation in Stage 7 |
| Release signing pending | No key custody | Architecture design; custody setup in Stage 7 |

---

## 9. Sign-Off

**Baseline Established By:** Automated Stage 6 Entry Verification
**Date:** 2026-09-12
**Repository State:** Clean working tree at `d6fb93e`
**Next Action:** Phase 1 — Deferred-Risk Forensic Reassessment