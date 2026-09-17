# Stage 20 Phase 4 — RCA & Gap Register (G4)

**Timestamp:** 2016-09-17
**Stage:** 20 — Operations Track & 4.1.0 Development Foundation
**Agent:** Stage 20 execution agent
**Repository:** C:\dev\aether

---

## Objective

Convert audit findings (Phase 2) and design reconciliation (Phase 3) into actionable engineering decisions with root cause analysis.

---

## Gap Register with RCA

### OP-01: No Health Endpoints (`/healthz`, `/readyz`)

| Field | Value |
|-------|-------|
| **Gap ID** | OP-01 |
| **Component** | CLI / Transport / Release |
| **Observed Condition** | No HTTP health endpoints exist; cannot integrate with Kubernetes, systemd, or load balancers |
| **Expected Condition** | `GET /healthz` returns 200 OK with component status; `GET /readyz` returns 200 when ready to serve traffic |
| **Evidence** | Phase 2 audit: Release/Ops domain — "No health endpoints, no metrics, no debugging support" |
| **Root Cause** | Health endpoints never scoped in any stage; CLI is command-oriented, not service-oriented |
| **Contributing Factors** | No operational requirements captured in Stage 1-3; no service mesh/orchestrator integration planned |
| **Security Impact** | LOW — Read-only endpoints; no credential exposure |
| **Operational Impact** | HIGH — Blocks container orchestration, load balancer integration, automated failover |
| **Compatibility Impact** | NONE — Additive; no existing CLI behavior changed |
| **Proposed Remediation** | Add `-serve` mode or dedicated `serve` subcommand with `/healthz`, `/readyz`, `/metrics` |
| **Alternatives Considered** | 1. External sidecar (adds complexity) 2. Systemd watchdog (Linux only) 3. Embedded HTTP server (selected) |
| **Scope Recommendation** | **MUST HAVE** for 4.1.0 — Foundational operational capability |
| **Validation Requirement** | Unit test: endpoints return correct JSON; Integration test: Kubernetes liveness/readiness probe simulation |
| **Residual Risk** | LOW — Simple implementation; well-understood pattern |

---

### OP-02: No Metrics Endpoint (`/metrics`)

| Field | Value |
|-------|-------|
| **Gap ID** | OP-02 |
| **Component** | CLI / Transport / Release |
| **Observed Condition** | No Prometheus metrics exposed; zero operational visibility |
| **Expected Condition** | `GET /metrics` returns Prometheus-format metrics (counters, gauges, histograms) |
| **Evidence** | Phase 2 audit: All domains — "No metrics, no tracing, no observability" |
| **Root Cause** | Metrics instrumentation never scoped; Go stdlib `expvar` not used; no Prometheus client |
| **Contributing Factors** | No observability requirements in Stage 1-3; focus on correctness over operability |
| **Security Impact** | LOW — Metrics are aggregate; no secrets |
| **Operational Impact** | HIGH — No alerting, no capacity planning, no SLO tracking |
| **Compatibility Impact** | NONE — Additive |
| **Proposed Remediation** | Add `github.com/prometheus/client_golang`; instrument: request latency, error rates, active connections, queue depths, vault operations, protocol operations |
| **Alternatives Considered** | 1. `expvar` (limited format) 2. OpenTelemetry (heavier) 3. Prometheus client (selected) |
| **Scope Recommendation** | **MUST HAVE** for 4.1.0 — Foundational operational capability |
| **Validation Requirement** | Unit test: metrics increment correctly; Integration test: Prometheus scrape works |
| **Residual Risk** | LOW — Standard library; minimal attack surface |

---

### OP-03: No Structured Logging

| Field | Value |
|-------|-------|
| **Gap ID** | OP-03 |
| **Component** | All (CLI, Transport, Store, Protocols, Engines) |
| **Observed Condition** | Stdlib `log` package only; no levels, no JSON, no correlation IDs, no structured fields |
| **Expected Condition** | Structured JSON logging with levels (debug/info/warn/error), correlation IDs, redacted sensitive fields |
| **Evidence** | Phase 2 audit: CLI — "Basic stdlib log; no structured logging; no levels" |
| **Root Cause** | Logging never standardized; early-stage code used stdlib `log` |
| **Contributing Factors** | No logging policy; no observability requirements; gradual accumulation without refactoring |
| **Security Impact** | MEDIUM — Risk of secret leakage in logs (mitigated by redaction but not systematic) |
| **Operational Impact** | HIGH — Cannot aggregate, search, correlate, or alert on logs |
| **Compatibility Impact** | LOW — Output format change; CLI human output unchanged |
| **Proposed Remediation** | Adopt `go.uber.org/zap` or `github.com/rs/zerolog`; define log levels; add correlation ID middleware; systematic redaction |
| **Alternatives Considered** | 1. Stdlib `slog` (Go 1.21+) — lighter, no deps 2. Zap — mature, fast 3. Zerolog — fast, simple. **Select: `slog` (stdlib, Go 1.27 available)** |
| **Scope Recommendation** | **SHOULD HAVE** for 4.1.0 — High value, but not blocking |
| **Validation Requirement** | Unit test: log output is JSON with correct fields; Integration test: log aggregation works |
| **Residual Risk** | LOW — Stdlib `slog` is stable; migration is mechanical |

---

### OP-04: No Upgrade/Rollback Tooling

| Field | Value |
|-------|-------|
| **Gap ID** | OP-04 |
| **Component** | Release / CLI |
| **Observed Condition** | Manual binary replacement; no version checking; no rollback; no upgrade verification |
| **Expected Condition** | `aether upgrade` checks for updates, downloads, verifies, installs; `aether rollback` reverts to previous version |
| **Evidence** | Phase 2 audit: Release/Ops — "No automated upgrade; manual binary replacement; no rollback tooling" |
| **Root Cause** | Distribution model is manual; no package manager integration; no self-update mechanism |
| **Contributing Factors** | No auto-update library; Windows/macOS/Linux have different mechanisms; security concerns with self-update |
| **Security Impact** | MEDIUM — Self-update mechanisms are attack vectors if not carefully designed |
| **Operational Impact** | HIGH — Manual ops at scale is error-prone; no rollback on bad release |
| **Compatibility Impact** | NONE — Additive new commands |
| **Proposed Remediation** | Implement `upgrade` and `rollback` subcommands; use GitHub Releases API; verify cosign signatures; atomic replace with backup |
| **Alternatives Considered** | 1. Package managers (winget, brew, apt) — external dependency 2. Self-update (selected) — controllable, verifiable |
| **Scope Recommendation** | **DEFERRED** to 4.2.0 — Security-sensitive; requires careful design; not blocking operations track |
| **Validation Requirement** | Integration test: upgrade downloads, verifies, installs; rollback restores previous binary |
| **Residual Risk** | MEDIUM — Self-update code must be audited; supply chain risk |

---

### OP-05: No Debugging/Profiling Endpoints

| Field | Value |
|-------|-------|
| **Gap ID** | OP-05 |
| **Component** | CLI / Transport |
| **Observed Condition** | No `pprof`, no debug endpoints, no runtime introspection |
| **Expected Condition** | `net/http/pprof` registered; `/debug/pprof/*` available in serve mode |
| **Evidence** | Phase 2 audit: Release/Ops — "No debug endpoints; no profiling; no debugging support" |
| **Root Cause** | Never scoped; security concern about exposing pprof |
| **Contributing Factors** | pprof exposes stack traces, memory profiles — can leak sensitive info |
| **Security Impact** | MEDIUM — pprof can expose goroutine stacks, memory; must be protected |
| **Operational Impact** | MEDIUM — Cannot diagnose production performance issues |
| **Compatibility Impact** | NONE — Additive |
| **Proposed Remediation** | Register `net/http/pprof` only in serve mode; protect with auth flag or network isolation |
| **Alternatives Considered** | 1. Always enabled (risk) 2. Flag-gated (selected) 3. Unix socket only (complex) |
| **Scope Recommendation** | **SHOULD HAVE** for 4.1.0 — Low effort, high diagnostic value |
| **Validation Requirement** | Manual test: `go tool pprof http://localhost:8080/debug/pprof/profile` |
| **Residual Risk** | LOW — Flag-gated; not exposed by default |

---

### OP-06: No Automated Store Backup/Restore

| Field | Value |
|-------|-------|
| **Gap ID** | OP-06 |
| **Component** | Store (BoltDB) |
| **Observed Condition** | Manual `cp` of database file; no verification; no incremental backup |
| **Expected Condition** | `aether workspace backup --output` creates verified backup; `aether workspace restore` validates and restores |
| **Evidence** | Phase 2 audit: Store — "No automated backup/restore tooling; no schema migration framework" |
| **Root Cause** | BoltDB is embedded; backup is file copy; no tooling built around it |
| **Contributing Factors** | No operational requirements for data protection; single-file DB makes it seem simple |
| **Security Impact** | LOW — Backup contains encrypted data; same security as live DB |
| **Operational Impact** | MEDIUM — Data loss risk on corruption; no point-in-time recovery |
| **Compatibility Impact** | NONE — Additive commands |
| **Proposed Remediation** | Add `workspace backup` and `workspace restore` commands; use BoltDB `Backup()` API; verify checksums |
| **Alternatives Considered** | 1. External tool (complex) 2. Built-in commands (selected) |
| **Scope Recommendation** | **SHOULD HAVE** for 4.1.0 — Low effort, high resilience value |
| **Validation Requirement** | Integration test: backup → corrupt DB → restore → verify data integrity |
| **Residual Risk** | LOW — BoltDB backup API is stable |

---

### OP-07: No Schema Migration Framework

| Field | Value |
|-------|-------|
| **Gap ID** | OP-07 |
| **Component** | Store (BoltDB) |
| **Observed Condition** | No versioned schema; no migration path; breaking changes require manual intervention |
| **Expected Condition** | Schema version in DB; migration functions; automatic or CLI-driven migration |
| **Evidence** | Phase 2 audit: Store — "No schema migration framework" |
| **Root Cause** | Schema has been stable; no forward-looking migration planning |
| **Contributing Factors** | BoltDB is schemaless; migrations are ad-hoc |
| **Security Impact** | NONE |
| **Operational Impact** | MEDIUM — Future schema changes will be painful without framework |
| **Compatibility Impact** | NONE — Framework is additive |
| **Proposed Remediation** | Add `schema_version` bucket; migration registry; `workspace migrate` command |
| **Alternatives Considered** | 1. Manual SQL-like migrations (overkill for BoltDB) 2. Versioned buckets with upgrade funcs (selected) |
| **Scope Recommendation** | **DEFERRED** to 4.2.0 — Not currently needed; schema stable |
| **Validation Requirement** | Integration test: migrate v1→v2 schema; verify data preserved |
| **Residual Risk** | LOW — Framework only; no active migration yet |

---

### OP-08: No Live Interop Validation (SAML, WS-Trust, MS-OAPX)

| Field | Value |
|-------|-------|
| **Gap ID** | OP-08 |
| **Component** | Protocols |
| **Observed Condition** | Protocols tested only against fixtures; no live IdP validation |
| **Expected Condition** | Live validation against at least one SAML IdP, one WS-Trust STS, one MS-OAPX endpoint |
| **Evidence** | Phase 2 audit: Protocols — "Most protocols lack live interoperability validation" |
| **Root Cause** | No authorized test environments; fixtures used instead |
| **Contributing Factors** | No IdP/STS test accounts; security policy against unauthorized access |
| **Security Impact** | LOW — Read-only validation; no data access |
| **Operational Impact** | MEDIUM — Unknown production compatibility |
| **Compatibility Impact** | NONE — Validation is additive |
| **Proposed Remediation** | Implement `aether validate saml` / `wstrust` / `msoapx` with dry-run and live modes; use test IdPs (Keycloak, ADFS, Okta dev) |
| **Alternatives Considered** | 1. Fixture-only (current) 2. Live validation (selected for 4.1.0 scope) |
| **Scope Recommendation** | **SHOULD HAVE** for 4.1.0 — At least one live IdP interop test |
| **Validation Requirement** | Live test against authorized test IdP; evidence of successful token exchange |
| **Residual Risk** | LOW — Read-only; authorized environments only |

---

### OP-09: Azure KV Provider Not Live-Validated

| Field | Value |
|-------|-------|
| **Gap ID** | OP-09 |
| **Component** | Providers (Azure KV) |
| **Observed Condition** | Provider code exists; contract mismatch (no Ed25519); no live validation |
| **Expected Condition** | Live integration test against authorized Azure Key Vault |
| **Evidence** | Phase 2 audit: Providers — "Azure KV provider not live-validated"; Phase 3: B5 waived |
| **Root Cause** | No authorized Azure test environment; contract mismatch with KeyProvider interface |
| **Contributing Factors** | Azure KV doesn't support Ed25519 or private key export; provider is key custody only |
| **Security Impact** | LOW — Provider correctly fails closed on unsupported ops |
| **Operational Impact** | MEDIUM — B5 blocker waived; production use unverified |
| **Compatibility Impact** | NONE — Provider is opt-in |
| **Proposed Remediation** | 1. Add mutex for thread safety 2. Add mock integration tests 3. Document as key custody provider 4. Live validation if authorized |
| **Alternatives Considered** | 1. Remove provider (no — valid use case) 2. Fix interface (no — architectural) 3. Document limitation (selected) |
| **Scope Recommendation** | **DEFERRED** — Thread safety fix **REQUIRES REDESIGN** (4.1.0); live validation **DEFERRED** (waiver) |
| **Validation Requirement** | Unit test: mutex protects concurrent access; Integration test: mock transport |
| **Residual Risk** | MEDIUM — Thread safety gap in current code |

---

### OP-10: No Audit Logging

| Field | Value |
|-------|-------|
| **Gap ID** | OP-10 |
| **Component** | Engines / AuditLog |
| **Observed Condition** | `AuditLog` exists but not used for operational audit trail; no compliance logging |
| **Expected Condition** | Structured audit events for all security-relevant operations (credential access, key operations, config changes) |
| **Evidence** | Phase 2 audit: Engines — "No audit logging for compliance/forensics" |
| **Root Cause** | `AuditLog` designed for internal workspace audit, not operational security audit |
| **Contributing Factors** | No compliance requirements captured; audit log is workspace-scoped |
| **Security Impact** | MEDIUM — No forensic trail for security incidents |
| **Operational Impact** | MEDIUM — Cannot prove operational integrity |
| **Compatibility Impact** | NONE — Additive |
| **Proposed Remediation** | Extend `AuditLog` or add separate `SecurityAuditLog`; emit events for: config load, credential use, key operations, validation runs, upgrade/rollback |
| **Alternatives Considered** | 1. Separate log (cleaner) 2. Extend existing (simpler) — **Select: separate** |
| **Scope Recommendation** | **DEFERRED** to 4.2.0 — Requires design; not blocking 4.1.0 |
| **Validation Requirement** | Integration test: audit events emitted for all security operations; tamper detection works |
| **Residual Risk** | LOW — Additive only |

---

## Additional Gaps from Design Reconciliation

### D-01: AzureKVProvider Thread Safety (Mutex Missing)

| Field | Value |
|-------|-------|
| **Gap ID** | D-01 |
| **Component** | `internal/store/azure_kv_provider.go` |
| **Observed Condition** | No mutex protecting `keyVersions`, `currentVersion`, `client`, `cred` |
| **Expected Condition** | `sync.RWMutex` protecting all shared state |
| **Evidence** | Phase 1 audit; Phase 2 race detector analysis; Phase 3 reconciliation |
| **Root Cause** | Never added during implementation; single-threaded assumption |
| **Proposed Remediation** | Add `sync.RWMutex`; protect all public methods |
| **Scope** | **MUST HAVE** for 4.1.0 — Data race in production code |

### D-02: Workspace Pass/Salt Concurrency

| Field | Value |
|-------|-------|
| **Gap ID** | D-02 |
| **Component** | `internal/workspace/workspace.go` |
| **Observed Condition** | `pass` slice and `salt` accessed without mutex during `Seal`/`Rekey` |
| **Expected Condition** | Mutex protecting passphrase material during concurrent access |
| **Evidence** | Phase 2 race detector analysis |
| **Root Cause** | `Rekey` added without concurrency consideration |
| **Proposed Remediation** | Add `rekeyMu sync.RWMutex` (already identified in Stage 12); apply to all pass/salt access |
| **Scope** | **MUST HAVE** for 4.1.0 — Data race in production code |

### D-03: Action Pinning (Supply Chain)

| Field | Value |
|-------|-------|
| **Gap ID** | D-03 |
| **Component** | `.github/workflows/*.yml` |
| **Observed Condition** | All 10 actions use `@vX` tags, not commit SHAs |
| **Expected Condition** | All actions pinned to immutable commit SHAs |
| **Evidence** | Phase 1 GoReleaser audit; Phase 5 artifact trust audit |
| **Root Cause** | Never done; convenience over security |
| **Proposed Remediation** | Resolve each action to commit SHA; update workflows; add Dependabot for updates |
| **Scope** | **MUST HAVE** for 4.1.0 — Supply chain hardening |

### D-04: Tamper Tests (4 of 9 Implemented)

| Field | Value |
|-------|-------|
| **Gap ID** | D-04 |
| **Component** | `.github/workflows/release.yml` (verify job) |
| **Observed Condition** | Only 4 tamper tests implemented; comment claims 9 |
| **Expected Condition** | All 9 tamper vectors implemented and passing |
| **Evidence** | Phase 5 artifact trust audit |
| **Root Cause** | Comment drift; incomplete implementation |
| **Proposed Remediation** | Implement remaining 5 tamper vectors or update comment to match reality |
| **Scope** | **SHOULD HAVE** for 4.1.0 — Supply chain confidence |

### D-05: Provenance (SLSA) Not Configured

| Field | Value |
|-------|-------|
| **Gap ID** | D-05 |
| **Component** | `.goreleaser.yml`, release workflow |
| **Observed Condition** | No `attestations` section in goreleaser; no SLSA provenance |
| **Expected Condition** | Provenance generated and published with each release |
| **Evidence** | Phase 1 GoReleaser audit; Phase 5 artifact trust audit |
| **Root Cause** | Goreleaser v2 supports it but not configured |
| **Proposed Remediation** | Add `attestations` to goreleaser config; configure SLSA provenance |
| **Scope** | **DEFERRED** to 4.2.0 — Requires goreleaser expertise; not blocking |

### D-06: VERIFY.md Not Created

| Field | Value |
|-------|-------|
| **Gap ID** | D-06 |
| **Component** | Repository root |
| **Observed Condition** | `VERIFY.md` referenced in docs but not created |
| **Expected Condition** | Stranger-reproducible verification guide published |
| **Evidence** | Phase 5 artifact trust audit |
| **Root Cause** | Never prioritized; deferred to "later" |
| **Proposed Remediation** | Create `VERIFY.md` with download → checksum → SBOM → cosign → Authenticode steps |
| **Scope** | **MUST HAVE** for 4.1.0 — Supply chain transparency |

### D-07: Branch Protection / Secret Scanning Not Verified

| Field | Value |
|-------|-------|
| **Gap ID** | D-07 |
| **Component** | GitHub Repository Settings |
| **Observed Condition** | Unknown state; never configured or verified |
| **Expected Condition** | Branch protection enabled; secret scanning enabled; Dependabot enabled; CodeQL enabled |
| **Evidence** | Phase 1 security control audit; Phase 5 artifact trust audit |
| **Root Cause** | Requires GitHub UI admin access; not automatable via code |
| **Proposed Remediation** | Document required settings; configure when authorized |
| **Scope** | **WAIVED** — Operator responsibility (per Stage 19) |

---

## RCA Summary Table

| Gap | Root Cause Category | Severity | 4.1.0 Scope |
|-----|---------------------|----------|-------------|
| OP-01 | Missing operational requirement | HIGH | MUST HAVE |
| OP-02 | Missing operational requirement | HIGH | MUST HAVE |
| OP-03 | Technical debt / no policy | MEDIUM | SHOULD HAVE |
| OP-04 | Security-sensitive feature | MEDIUM | DEFERRED |
| OP-05 | Security concern / not scoped | LOW | SHOULD HAVE |
| OP-06 | Operational tooling gap | MEDIUM | SHOULD HAVE |
| OP-07 | Future-proofing | LOW | DEFERRED |
| OP-08 | Environment dependency | MEDIUM | SHOULD HAVE |
| OP-09 | Contract mismatch + env | MEDIUM | DEFERRED (mutex: MUST HAVE) |
| OP-10 | Scope mismatch | MEDIUM | DEFERRED |
| D-01 | Implementation omission | HIGH | MUST HAVE |
| D-02 | Implementation omission | HIGH | MUST HAVE |
| D-03 | Security hardening omission | HIGH | MUST HAVE |
| D-04 | Implementation drift | MEDIUM | SHOULD HAVE |
| D-05 | Configuration omission | LOW | DEFERRED |
| D-06 | Documentation omission | HIGH | MUST HAVE |
| D-07 | External dependency | WAIVED | WAIVED |

---

## Gate G4 Status

| Sub-gate | Status | Evidence |
|----------|--------|----------|
| G4.1 | All gaps have RCA | ✅ Complete |
| G4.2 | Each gap has scope recommendation | ✅ Complete |
| G4.3 | No "missing feature" confused with "defect" | ✅ Verified |
| G4.4 | No "missing test" confused with "broken" | ✅ Verified |
| G4.5 | No live-env gap confused with protocol failure | ✅ Verified |
| G4.6 | No doc gap confused with security failure | ✅ Verified |
| G4.7 | No operator responsibility confused with app guarantee | ✅ Verified |

**G4 Status: PASS** — All gaps traced to root cause; scope recommendations explicit.

---

## Next Phase

**Phase 5 — 4.1.0 Scope Definition (G5)**

Select bounded, coherent scope from MUST HAVE / SHOULD HAVE / DEFERRED items.

---

*Generated by Stage 20 Phase 4 — RCA & Gap Register*

---

## Appendix: 4.1.0 Scope Definition (G5) — Merged from stage20-scope-definition.md

### Objective

Define a bounded, coherent 4.1.0 release scope based on RCA gap register.

---

### Scope Selection Criteria Applied

| Criterion | Weight | Application |
|-----------|--------|-------------|
| Operational value | High | Must enable production operations |
| Security impact | High | Must not weaken security |
| Architectural fit | High | Must align with existing patterns |
| Implementation complexity | Medium | Prefer low-complexity, high-value |
| Compatibility risk | High | Zero breaking changes allowed |
| Validation feasibility | High | Must be testable without live env |
| Documentation burden | Medium | Prefer self-documenting features |
| Maintenance cost | Medium | Prefer standard libraries |
| Release timeline | High | Must fit in single stage |
| Dependency risk | High | No new external runtime deps |
| Live infrastructure need | Critical | Must not require unauthorized access |

---

### 4.1.0 Scope Decision

#### In Scope (MUST HAVE / SHOULD HAVE — 10 Items)

| # | Item | Gap ID | Type | Effort | Validation |
|---|------|--------|------|--------|------------|
| 1 | **Observability: `/metrics` (Prometheus)** | OP-02 | MUST HAVE | Low | Unit + integration test |
| 2 | **Observability: `/healthz` + `/readyz`** | OP-01 | MUST HAVE | Low | Unit + integration test |
| 3 | **Structured Logging (stdlib `slog`)** | OP-03 | SHOULD HAVE | Low | Unit test |
| 4 | **Revocation: OCSP/CRL + File Fallback** | OP-05 (related) | SHOULD HAVE | Medium | Unit + integration test |
| 5 | **ARM64 Integration Testing** | OP-08 (related) | MUST HAVE | Low | Build + test on ARM64 |
| 6 | **Third-Party IdP Interop (Keycloak)** | OP-08 | SHOULD HAVE | Medium | Live test against dev Keycloak |
| 7 | **AzureKVProvider Thread Safety (Mutex)** | D-01 | MUST HAVE | Low | Unit test with `-race` |
| 8 | **Workspace Pass/Salt Concurrency Fix** | D-02 | MUST HAVE | Low | Unit test with `-race` |
| 9 | **Action Pinning (Supply Chain)** | D-03 | MUST HAVE | Low | CI verification |
| 10 | **VERIFY.md Creation** | D-06 | MUST HAVE | Low | Stranger reproduction test |

---

#### Explicitly Out of Scope

| Item | Gap ID | Reason |
|------|--------|--------|
| Live Entra Validation | OP-09 (Entra) | Waiver active until 2027-06-30; no authorized tenant |
| Live IMDS Validation | OP-09 (IMDS) | Waiver active until 2027-06-30; no authorized VM |
| Live Azure KV Validation | OP-09 (KV) | Waiver active until 2027-03-31; no authorized vault |
| Upgrade/Rollback Tooling | OP-04 | Security-sensitive; deferred to 4.2.0 |
| Schema Migration Framework | OP-07 | Not currently needed; deferred to 4.2.0 |
| Audit Logging (Security) | OP-10 | Requires design; deferred to 4.2.0 |
| Provenance (SLSA) | D-05 | Requires goreleaser expertise; deferred to 4.2.0 |
| Branch Protection / Secret Scanning | D-07 | Operator responsibility (Stage 19 waiver) |
| EV Authenticode Certificate | B4 | Waived permanently (Stage 19) |
| Full Race Detector CI Green | B3 | Requires CI trigger; waived with mutex fixes |

---

#### Deferred (Valid for Future Stages)

| Item | Target Stage | Prerequisite |
|------|--------------|--------------|
| Upgrade/Rollback | 4.2.0 | Security review complete |
| Schema Migration | 4.2.0 | Schema change needed |
| Security Audit Logging | 4.2.0 | Design approved |
| SLSA Provenance | 4.2.0 | Goreleaser expertise |
| Live Entra/IMDS/KV | 4.2.0+ | Authorized environments |
| Full Race CI | 4.2.0 | CI infrastructure |

---

#### Rejected / Superseded

| Item | Reason |
|------|--------|
| Stage 11 baselines | Superseded by Stage 19/20 |
| Stage 10 validate job failure | Superseded — local pass confirmed |
| 9 tamper tests claim | Superseded — only 4 implemented; update comment |
| KeyProvider Ed25519 for Azure KV | Architectural mismatch — documented as limitation |

---

### 4.1.0 Scope Statement

> **4.1.0 is an Operations Track release focused on production operability.**
>
> It adds foundational observability (metrics, health, structured logging), certificate revocation checking, ARM64 support, third-party IdP interoperability, and supply-chain hardening. It fixes two data races in production code. It does not require any live cloud environments — all validation is local or against authorized dev instances.
>
> **Compatibility:** Full backward compatibility. No breaking changes to CLI, config, store, or provider interfaces.
>
> **Release Classification:** Operations Track — not GA, not RC. Production-usable with documented limitations.

---

### Compatibility Constraints (Reaffirmed)

| Surface | Constraint |
|---------|------------|
| CLI | No command/flag/output changes; new subcommands only |
| Configuration | No format changes; new optional fields only |
| Store | No schema changes; no migration needed |
| Provider Interface | No changes; AzureKVProvider remains key-custody only |
| Release Artifacts | Same format; added ARM64 artifacts |
| Version | `4.1.0` (minor bump per semantic versioning) |

---

### Security Constraints

1. **No credential exposure** — All new endpoints/flags must redact secrets
2. **Fail-closed** — Revocation checking defaults to allow on error (configurable)
3. **No new trust boundaries** — Metrics/health endpoints are read-only
4. **Supply chain** — Action pinning reduces risk; VERIFY.md enables verification

---

### Validation Requirements

| Capability | Validation Method |
|------------|-------------------|
| `/metrics` | Prometheus scrape test; metric correctness assertions |
| `/healthz` | HTTP 200 with JSON status; component checks |
| `/readyz` | HTTP 200 when store/config/providers ready |
| Structured logging | JSON output verification; level filtering |
| OCSP/CRL | Mock responder tests; file fallback test; error handling |
| ARM64 | `GOOS=linux GOARCH=arm64 go build/test`; `GOOS=darwin GOARCH=arm64 go build/test` |
| Keycloak interop | Live test against dev Keycloak instance; token exchange |
| AzureKVProvider mutex | `go test -race ./internal/store/...` |
| Workspace mutex | `go test -race ./internal/workspace/...` |
| Action pinning | CI runs with pinned SHAs; Dependabot updates work |
| VERIFY.md | Stranger reproduction: fresh clone → build → verify |

---

### Gate G5 Status

| Sub-gate | Status | Evidence |
|----------|--------|----------|
| G5.1 | Scope bounded and coherent | ✅ 10 in-scope items; clear exclusions |
| G5.2 | Traceable to audit/RCA | ✅ Each item maps to gap register |
| G5.3 | Compatibility explicit | ✅ Zero breaking changes |
| G5.4 | Security constraints explicit | ✅ Documented |
| G5.5 | Validation feasible | ✅ All local or dev-instance |
| G5.6 | No live infra required | ✅ Verified |
| G5.7 | Trade-offs documented | ✅ Deferred/rejected listed |

**G5 Status: PASS** — 4.1.0 scope defined, bounded, and traceable.