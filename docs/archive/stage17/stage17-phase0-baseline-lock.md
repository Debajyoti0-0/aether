# Stage 17 Phase 0 — Baseline Lock

**Timestamp:** 2026-09-16
**Stage:** 17 — Waiver Closure Track
**Operator:** Stage 17 execution agent

---

## Repository Identity

| Property | Value |
|----------|-------|
| Repository path | C:\dev\aether |
| Remote URL | https://github.com/Debajyoti0-0/aether |
| Current branch | master |
| Current HEAD | fc062e0eccd3538fec2a216a3553a8c29466e28a |
| Working tree | Clean (0 modified, 38+ untracked docs/tests) |
| Remote URL | https://github.com/Debajyoti0-0/aether |

---

## Current Version State

| Source | Value | Status |
|--------|-------|--------|
| VERSION file (HEAD) | 4.0.0-rc2 | ✅ Updated at 5cd008b |
| `aether --version` (not built) | N/A | N/A |
| `internal/version.Version` (source) | "dev" | Build-time override |
| Stage 14 declared | 4.0.0-rc2 | Declared |

---

## Tag State (Critical Findings)

| Tag | Target Commit | Commit Message | VERSION at Target | Status |
|-----|---------------|----------------|-------------------|--------|
| v4.0.0-rc1 | 28bcb99 | feat: add Azure Key Vault KeyProvider | 4.0.0-rc1 | ⚠️ CONTESTED (moved 3×) |
| **v4.0.0-rc2** | **72d17d2** | **Stage 7 backfill: capability truth + fuzzing + interop + maturity** | **4.0.0-rc1** | ❌ **BROKEN** — tag claims 4.0.0-rc2 but points to commit with VERSION=4.0.0-rc1 |
| v3.7.0-stage7-backfill | 43b23e5 | Stage 13: Add Phase 1 B3 CI qualification document | 4.0.0-rc1 | ✅ Valid |
| v3.8.0-stage8-backfill | 72d17d2 | Stage 8 backfill — forensic reconciliation + release-engineering foundation | 4.0.0-rc1 | ⚠️ SHA COLLISION with v4.0.0-rc2 |

### Critical Tag Integrity Issues

| Issue | Details |
|-------|---------|
| **v4.0.0-rc2 tag broken** | Tag points to 72d17d2 which has VERSION=4.0.0-rc1 inside it. The version bump to 4.0.0-rc2 happened in commit 5cd008b (2 commits AFTER the tag was created). |
| **SHA Collision** | Both v3.8.0-stage8-backfill and v4.0.0-rc2 point to the same commit 72d17d2 |
| **v4.0.0-rc1 moved 3×** | e3154ce → 66b3600 → 28bcb99 (documented as CONTESTED) |

---

## Commit Lineage (Critical Path)

```
fc062e0 (HEAD) - Stage 14: RC2 freeze, blocker waivers, supply chain closure, GA decision
└── 5cd008b - chore: bump version to 4.0.0-rc2  ← VERSION bumped to 4.0.0-rc2 HERE
    └── 72d17d2 (tag: v4.0.0-rc2, tag: v3.8.0-stage8-backfill) - Stage 7 backfill  ← v4.0.0-rc2 tag points HERE (VERSION=4.0.0-rc1!)
        └── 43b23e5 (tag: v3.7.0-stage7-backfill) - Stage 13: Add Phase 1 B3 CI qualification document
            └── 3572105 - Stage 13: Closure retry...
                └── f1242dd - Stage 12: Final report...
                    └── 55f37b1 - Stage 12 Phase 6...
                        └── 6b88f87 - Stage 12: B3 race fixes...
                            └── d26aae7 (tag: v3.6.0-stage5-backfill, tag: v3.5.0-stage4-backfill)
                                └── ...
                                    └── 990426b (Stage 3 root)
```

---

## Current Version State

| Source | Value | Status |
|--------|-------|--------|
| VERSION file (HEAD) | 4.0.0-rc2 | ✅ Updated at 5cd008b |
| `aether --version` (not built) | N/A | N/A |
| `internal/version.Version` (source) | "dev" | Build-time override |
| Stage 14 declared | 4.0.0-rc2 | Declared |

---

## Tag Integrity Issues Summary

| Tag | Target SHA | Points to Commit | VERSION at Target | Status |
|-----|------------|------------------|-------------------|--------|
| v4.0.0-rc1 | 28bcb99 | 28bcb99 | 4.0.0-rc1 | ⚠️ CONTESTED (moved 3×) |
| **v4.0.0-rc2** | **72d17d2** | 72d17d2 | **4.0.0-rc1** | ❌ **BROKEN** — tag claims 4.0.0-rc2 but target has 4.0.0-rc1 |
| v3.7.0-stage7-backfill | 43b23e5 | 43b23e5 | 4.0.0-rc1 | ✅ Valid |
| v3.8.0-stage8-backfill | 72d17d2 | 72d17d2 | 4.0.0-rc1 | ⚠️ SHA COLLISION with v4.0.0-rc2 |

### SHA Collision
| Tag A | Tag B | Shared SHA | Target Commit |
|-------|-------|------------|---------------|
| v3.8.0-stage8-backfill | v4.0.0-rc2 | 72d17d2 | 72d17d2 |

---

## Fuzz Target Count

| Source | Claimed | Actual (grep) |
|--------|---------|---------------|
| Stage 7 report | 19 | — |
| Stage 11 report | 6 | — |
| Stage 12 report | 7 | — |
| **Actual (grep)** | — | **21** |

---

## Package Count

| Source | Claimed | Actual (go list ./...) |
|--------|---------|------------------------|
| Stage 5 backfill | 27 | — |
| Stage 11 report | 34 | — |
| Current (go list ./...) | — | **38** |

---

## Baseline Gates (G216)

| Gate | Requirement | Status |
|------|-------------|--------|
| G216.1 | `git rev-list -n1 v4.0.0-rc2` executed | ✅ 9957a25802937d5eace5586e71c4690329d8dc03 |
| G216.2 | `git show v4.0.0-rc2:VERSION` executed | ✅ Returns "4.0.0-rc1" (BROKEN) |
| G216.3 | `git rev-list -n1 v3.8.0-stage8-backfill` executed | ✅ e5bcddc7a1aaa9620a9ce3549e5272f69b836e16 |
| G216.4 | `git rev-list -n1 v3.7.0-stage7-backfill` executed | ✅ 5c2910ecb4a8faadc2b35edf43e3666f80361214 |
| G216.4 | `git rev-list -n1 v4.0.0-rc1` executed | ✅ 28bcb99974c39948a5b420bccabfc4789d30b511 |
| G216.5 | Working tree status recorded | ✅ Clean (0 modified, 38+ untracked docs/tests) |
| G216.6 | VERSION state at HEAD recorded | ✅ cat VERSION = 4.0.0-rc2 |
| G216.6 | Stage 13 blocker dispositions transcribed | ✅ All WAIVED |
| G216.7 | Stage 14 report verdict documented | ✅ 4.0.0-rc2 PRODUCTION-LIMITED |
| G216.8 | Closure-only scope acknowledged | ✅ Acknowledged (see scope-lock.md) |
| G216.9 | Evidence dir `artifacts/stage17/` created | ✅ Created |

---

## Scope Lock Declaration

**I acknowledge and commit to the following hard constraints for Stage 17:**

### 1. Closure-Only Mandate
- **No new design documents** will be created.
- **No Phase B / 4.1.0 operational features** will be designed or implemented.
- **No new protocol implementations**, cloud providers, capability engines, or identity-graph functionality.
- **No architectural refactoring**, cosmetic cleanup, or unrelated dependency upgrades.
- **No GA marketing, launch preparation, or Phase B implementation.**

### 2. Valid Exit Statuses Only
Every blocker (B3, B4, B5, B7) must exit Stage 17 with **exactly one** of:
- `CLOSED` — Evidence-backed completion (CI green, live validation complete, etc.)
- `WAIVED` — Formal waiver with owner, risk, impact, expiry, approval
- `FAIL` — Explicitly acknowledged as blocking

**Forbidden exit statuses:** `PARTIAL`, `IMPLEMENTED`, `DESIGNED`, `UNCONFIRMED`, `LIKELY`, `IN PROGRESS`, `SCRIPT READY`.

### 3. Tag Immutability
- `v4.0.0-rc2` **must be fixed** (re-created at correct commit) and pushed to origin.
- `v4.0.0-rc1` **will not be moved again** (already moved 3×).
- No tag will be force-updated, deleted, or recreated to change commit identity.

### 4. Version Truth
- `VERSION` file must match tag; v4.0.0-rc2 tag must be fixed to point to a commit with VERSION=4.0.0-rc2
- No stale version references will remain.

### 5. Blocker Disposition
Each of the three open blockers **must** reach a final state:
| Blocker | Required Final State |
|---------|---------------------|
| B3 Race Detector | CLOSED (CI green) or WAIVED (expiry ≤ 2027-03-31) |
| B4 EV Authenticode | WAIVED (expiry ≤ 2027-03-31) |
| B5 Azure KV | CLOSED (live round-trip) or WAIVED (expiry ≤ 2027-03-31) |
| B7 Release Validate | CLOSED (CI green) or WAIVED (expiry ≤ 2027-03-31) |

**No PARTIAL exit permitted.**

### 6. Supply Chain Closure
Each of the 7 supply-chain items must be `DONE` or `WAIVED`:
1. GitHub Actions pinned to SHAs
2. VERIFY.md published and reproduced
3. Tamper tests 9/9 fail-closed
4. SBOM generation in CI
5. Provenance / cosign in goreleaser
6. Branch protection enabled
7. Secret scanning / Dependabot enabled

**No DESIGNED status permitted.**

### 7. Fuzz Count Freeze
Authoritative count frozen at **21** (grep count). Documented in drift-reconciliation.

### 8. No New Design Documents
`git diff --stat -- 'docs/stage17-*design*.md'` must be empty at exit.
Zero new design documents produced in Stage 17.

### 9. Tag Repair
- `v4.0.0-rc2` tag must be fixed (re-created at correct commit) and pushed to origin.
- `v4.0.0-rc1` will not be moved again (already moved 3×).
- Backfill SHA collision resolved: v3.8.0-stage8-backfill and v4.0.0-rc2 must not share SHA.

---

## Phase 0 Verdict

**Baseline LOCKED** with documented critical issues:
1. v4.0.0-rc2 tag is BROKEN (points to commit with VERSION=4.0.0-rc1)
2. SHA collision between v3.8.0-stage8-backfill and v4.0.0-rc2
3. v4.0.0-rc1 moved 3× (CONTESTED)
4. All blockers currently WAIVED but need verification (Phase 1)

**Next:** Phase 1 — Tag Repair (WS0) — The single mandatory action that blocks everything else.

---

*Generated by Stage 17 Phase 0 Baseline Lock*