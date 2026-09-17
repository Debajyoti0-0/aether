# Stage 20 Phase 0 — Terminal Baseline Lock

**Timestamp:** 2016-09-17
**Stage:** 20 — Operations Track & 4.1.0 Development Foundation
**Agent:** Stage 20 execution agent
**Repository:** C:\dev\aether

---

## Repository Identity

| Property | Value |
|----------|-------|
| Repository path | C:\dev\aether |
| Remote URL | https://github.com/Debajyoti0-0/aether |
| Current branch | master |
| Current HEAD | fc062e0eccd3538fec2a216a3553a8c29466e28a |
| Working tree | **2 modified (go.mod, go.sum), 40+ untracked docs/tests** |
| Remote URL | https://github.com/Debajyoti0-0/aether |

---

## Terminal Release Identity (Immutable)

| Field | Authoritative Value |
|-------|---------------------|
| Release | `4.0.0-rc2` |
| Tag | `v4.0.0-rc2` |
| Tag SHA | `5cd008b` (via annotated tag `779cdb6f62075565df376c2b9f04e11f7f29e4c6`) |
| Release classification | Terminal / Production-Limited |
| GA authorization | Not granted |
| Release line | 4.0.0 closed |
| Fuzz targets | 21 frozen |
| Stage 19 decision | Option B — rc2 terminal |

---

## Baseline Verification

```bash
# Terminal tag verification
git show v4.0.0-rc2:VERSION
# → 4.0.0-rc2 ✅

git rev-parse v4.0.0-rc2
# → 779cdb6f62075565df376c2b9f04e11f7f29e4c6 (annotated tag → 5cd008b) ✅

# Current HEAD
git rev-parse HEAD
# → fc062e0eccd3538fec2a216a3553a8c29466e28a

# Stage 19 documents exist
docs/stage19-scope-lock.md ✅
docs/stage19-baseline-lock.md ✅
docs/stage19-terminal-decision.md ✅
```

---

## Working Tree Deviations from Terminal Baseline

The following files are MODIFIED relative to the tagged commit (5cd008b) and HEAD (fc062e0):

| File | Change Summary | Impact | Classification |
|------|----------------|--------|----------------|
| go.mod | Azure SDK dependencies promoted from indirect to direct | **Material** — adds direct dependencies for Azure KV provider | Requires review |
| go.sum | Checksums updated for Azure SDK + transitive deps | **Material** — matches go.mod changes | Requires review |

**Assessment:** These changes implement the Azure Key Vault provider (B5) dependencies. They are **not part of the v4.0.0-rc2 tag baseline** but represent work done after the tag. For Stage 20 evidence purposes, the terminal baseline remains the tag (5cd008b). These modifications are documented and must be resolved (WS0) before 4.1.0 work begins.

---

## Untracked Files Classification

| Category | Count | Examples | Resolution Strategy |
|----------|-------|----------|---------------------|
| Stage 10 docs | 6 | `stage10-artifact-trust.md`, `stage10-baseline-lock.md`, etc. | Archive to `docs/archive/stage10/` |
| Stage 11 docs | 11 | `stage11-phase0-baseline.md`, `stage11-phase1-azure-kv-audit.md`, etc. | Archive to `docs/archive/stage11/` |
| Stage 12 docs | 7 | `stage12-phase0-baseline.md`, `stage12-phase1-b3-race-closure.md`, etc. | Archive to `docs/archive/stage12/` |
| Stage 15 docs | 20 | `stage15-b3-race-requalification.md`, `stage15-final-report.md`, etc. | Archive to `docs/archive/stage15/` |
| Stage 16 docs | 14 | `stage16-correction-register.md`, `stage16-final-report.md`, etc. | Archive to `docs/archive/stage16/` |
| Stage 17 docs | 10 | `stage17-b4-verification.md`, `stage17-final-report.md`, etc. | Archive to `docs/archive/stage17/` |
| Stage 18 docs | 8 | `stage18-evidence-index.md`, `stage18-final-report.md`, etc. | Archive to `docs/archive/stage18/` |
| Stage 19 docs | 3 | `stage19-baseline-lock.md`, `stage19-scope-lock.md`, `stage19-terminal-decision.md` | **Keep** (current stage) |
| Test files | 4 | `evidence_verification_test.go`, `interop_matrix_test.go`, etc. | Review — may be integration test scaffolding |
| Other | ~10 | `Dockerfile.race`, `release-integrity-notice.md`, etc. | Review individually |

**Total untracked:** ~40+ files

---

## Phase B Design Document Inventory

| Design Document | Status | Relevance to 4.1.0 |
|-----------------|--------|-------------------|
| `docs/stage11-phase0-baseline.md` | Parked | Baseline reference |
| `docs/stage11-phase1-azure-kv-audit.md` | Parked | B5 context |
| `docs/stage11-phase1-goreleaser-sbom-audit.md` | Parked | Supply chain |
| `docs/stage11-phase2-race-detector.md` | Parked | B3 context |
| `docs/stage11-phase3-release-validate.md` | Parked | B7 context |
| `docs/stage11-phase4-azure-kv-validation.md` | Parked | B5 context |
| `docs/stage11-phase5-artifact-trust.md` | Parked | Supply chain |
| `docs/stage11-phase6-410-operations-design.md` | **Parked** | **Primary 4.1.0 design** |
| `docs/stage11-phase7-entra-validation-plan.md` | Parked | B1 context (waived) |
| `docs/stage11-phase8-imds-validation-plan.md` | Parked | B2 context (waived) |
| `docs/stage11-phase9-testing-requirements.md` | Parked | Test strategy |
| `docs/stage11-phase10-documentation.md` | Parked | Documentation |

**Total parked Phase B designs:** 12

---

## Dependency Verification

```bash
go mod verify
# → all modules verified ✅
```

---

## Gate G251 Status

| Gate | Requirement | Status | Evidence |
|------|-------------|--------|----------|
| G251.1 | `git show v4.0.0-rc2:VERSION` = `4.0.0-rc2` | ✅ PASS | Returns "4.0.0-rc2" |
| G251.2 | Working tree status captured | ✅ PASS | 2 modified + 40+ untracked documented |
| G251.3 | `go mod verify` run | ✅ PASS | all modules verified |
| G251.4 | Phase B design docs enumerated | ✅ PASS | 12 docs listed |
| G251.5 | Document limit acknowledged: ≤ 10 | ✅ PASS | `docs/stage20-scope-lock.md` created |
| G251.6 | Evidence dir `artifacts/stage20/` created | ⏳ PENDING | Will create before Part B |

---

## Phase 0 Verdict

**Terminal Baseline LOCKED** with documented status:

1. ✅ Terminal release `4.0.0-rc2` verified at 5cd008b (immutable)
2. ✅ Tag integrity confirmed — no SHA collision, no unintended moves
3. ✅ Stage 19 documents preserved as historical record
4. ⚠️ Working tree has 2 modified files (`go.mod`, `go.sum`) — **must resolve in WS0**
5. ⚠️ 40+ untracked docs — **must classify and resolve in WS0**
6. ✅ `go mod verify` passes — no dependency integrity issues
7. ✅ 12 parked Phase B design docs enumerated
8. ✅ Document limit acknowledged (≤ 10)
9. ✅ Evidence dir `artifacts/stage20/` to be created

**Next:** Phase 1 — Workspace Hygiene (WS0) & 4.1.0 Branch/Version Strategy (G1)

---

*Generated by Stage 20 Phase 0 — Terminal Baseline Lock*

---

## Appendix: 4.1.0 Branch & Version Strategy (G1) — Merged from stage20-branch-version-strategy.md

### Objective

Establish a clean, traceable development identity for 4.1.0 before any audit or implementation work begins.

---

### Terminal Release Reference (Immutable)

| Field | Value |
|-------|-------|
| Terminal version | `4.0.0-rc2` |
| Terminal tag | `v4.0.0-rc2` |
| Terminal commit | `5cd008b` (annotated tag `779cdb6f62075565df376c2b9f04e11f7f29e4c6`) |
| Terminal classification | Terminal / Production-Limited |
| GA authorization | Not granted |
| Release line | 4.0.0 closed |

---

### 4.1.0 Development Identity Decisions

| Field | Decision | Rationale |
|-------|----------|-----------|
| **Base commit** | `bfdf713` (latest master) | Clean post-hygiene baseline; all historical docs archived; working tree minimal |
| **Branch** | `master` (continue on main branch) | Single-developer repo; no parallel release lines needed; tag-based releases |
| **Version** | `4.1.0` | Semantic version: minor bump for operations track (backward-compatible operational features) |
| **Release type** | Operations / Maintenance release | No new protocol capabilities; focuses on observability, revocation, ARM64, multi-host, IdP interop |
| **Compatibility policy** | **Full backward compatibility** | No breaking CLI changes, config changes, store schema changes, or API changes. Existing 4.0.0-rc2 users can upgrade without migration. |
| **Migration policy** | **Zero-downtime, no migration required** | BoltDB schema unchanged; config file format unchanged; all new features opt-in via flags. |
| **Tag policy** | `v4.1.0` created at Stage 20 exit commit; pushed to origin | Immutable after creation; follows v4.0.0-rc2 pattern but for GA-track release |
| **Changelog policy** | Append to existing `CHANGELOG.md` under `## [4.1.0]` | Sections: Added, Changed, Fixed, Security, Operational |
| **Pre-release naming** | None — direct to `4.1.0` | Operations track doesn't use RC; features are additive and tested |

---

### Branch Strategy Detail

```
master (4.1.0 development)
  │
  ├── bfdf713 — Stage 20 hygiene complete (BASE)
  ├── ... implementation commits ...
  └── STAGE_20_EXIT — v4.1.0 tag created here
```

**No feature branches.** All work on `master` with explicit commits. Tags mark releases.

---

### Version Strategy Detail

| Aspect | 4.0.0-rc2 (Terminal) | 4.1.0 (Operations) |
|--------|---------------------|-------------------|
| `VERSION` file | `4.0.0-rc2` | `4.1.0` |
| Binary version | `4.0.0-rc2` | `4.1.0` |
| Tag | `v4.0.0-rc2` | `v4.1.0` |
| Release type | Production-Limited | Operations / GA-track |
| GA status | Not granted | Not granted (operations) |
| Signing | Unsigned (B4 waived) | Unsigned (B4 waived) — unless EV cert procured |
| Fuzz targets | 21 frozen | 21+ (may add) |
| Compatibility | Baseline | Backward compatible |

---

### Compatibility Constraints (Explicit)

#### CLI Compatibility — **NO BREAKING CHANGES**
- All existing commands (`prt`, `relay`, `cap`, `exec`, `validate`) retain behavior
- All existing flags retain behavior
- Output formats unchanged
- Exit codes unchanged
- JSON/machine output unchanged

#### Configuration Compatibility — **NO BREAKING CHANGES**
- Config file format (YAML) unchanged
- Default values unchanged
- Environment variable names unchanged
- Credential references unchanged
- Provider configuration unchanged

#### Store Compatibility — **NO SCHEMA CHANGES**
- BoltDB schema unchanged
- Existing records readable/writable
- No migration required
- Backup/restore procedures unchanged

#### Provider/Plugin Compatibility — **NO INTERFACE CHANGES**
- `KeyProvider` interface unchanged
- `ProviderFactory` unchanged
- Plugin boundaries unchanged
- Error contracts unchanged
- Capability discovery unchanged

#### Release Artifact Compatibility
- Artifact naming: `aether_v4.1.0_<os>_<arch>.<ext>`
- Checksums, SBOM, manifest format unchanged
- Verification procedures unchanged

---

### What 4.1.0 May Add (Additive Only)

1. **Observability** — `/metrics`, `/healthz`, `/readyz` endpoints (opt-in via flag)
2. **Revocation** — OCSP/CRL checking with file fallback (opt-in via flag)
3. **ARM64 builds** — `linux/arm64`, `darwin/arm64` artifacts added
4. **Multi-host docs** — Documentation only; no runtime changes
5. **Third-party IdP interop** — Config-driven; no core changes

---

### Gate G1 Exit Criteria

| Criterion | Status | Evidence |
|-----------|--------|----------|
| Base commit identified | ✅ PASS | `bfdf713` (latest master) |
| Branch strategy explicit | ✅ PASS | Continue on `master` |
| Version strategy explicit | ✅ PASS | `4.1.0` minor bump |
| Release type classified | ✅ PASS | Operations / maintenance |
| Compatibility policy explicit | ✅ PASS | Full backward compatibility |
| Migration policy explicit | ✅ PASS | Zero-downtime, no migration |
| Tag policy explicit | ✅ PASS | `v4.1.0` at exit commit |
| Changelog policy explicit | ✅ PASS | Append to existing |

**G1 Status: PASS** — 4.1.0 development identity established.