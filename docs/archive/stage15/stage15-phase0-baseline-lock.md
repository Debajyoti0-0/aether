# Stage 15 Phase 0 — Baseline Lock

**Timestamp:** 2026-09-16
**Stage:** 15 — Waiver Closure, Repository Security Hardening & GA Requalification
**Operator:** Stage 15 execution agent

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
| v4.0.0-rc2 | 72d17d2 | Stage 7 backfill: capability truth + fuzzing + interop + maturity | **4.0.0-rc1** | ❌ **BROKEN** — tag claims 4.0.0-rc2 but points to commit with VERSION=4.0.0-rc1 |
| v3.7.0-stage7-backfill | 43b23e5 | Stage 13: Add Phase 1 B3 CI qualification document | 4.0.0-rc1 | ✅ Valid |
| v3.8.0-stage8-backfill | 72d17d2 | Stage 8 backfill — forensic reconciliation + release-engineering foundation | 4.0.0-rc1 | ⚠️ SHA COLLISION with v4.0.0-rc2 |
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
| v4.0.0-rc2 | 72d17d2 | 72d17d2 | **4.0.0-rc1** | ❌ **BROKEN** — claims 4.0.0-rc2 but target has 4.0.0-rc1 |
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
| Stage 7 backfill (actual grep) | — | **21** |

---

## Package Count

| Source | Claimed | Actual (go list ./...) |
|--------|---------|------------------------|
| Stage 5 backfill | 27 | — |
| Stage 11 report | 34 | — |
| Current (go list ./...) | — | **38** |

---

## Baseline Gates (G166)

| Gate | Requirement | Status |
|------|-------------|--------|
| G166.1 | Working tree clean | ✅ Clean (0 modified, 38+ untracked docs/tests) |
| G166.2 | `git rev-list -n1 v4.0.0-rc2` executed | ✅ 9957a25802937d5eace5586e71c4690329d8dc03 |
| G166.3 | `git show v4.0.0-rc2:VERSION` executed | ✅ Returns "4.0.0-rc1" (BROKEN) |
| G166.4 | Backfill tag SHAs recorded | ✅ Documented above |
| G166.5 | All waiver files enumerated | ⏳ Pending (Phase 1) |
| G166.6 | Supply-chain controls enumerated | ⏳ Pending (Phase 4) |
| G166.7 | Fuzz count snapshot taken | ⏳ Pending (Phase 5) |
| G166.8 | Closure-only scope acknowledged | ✅ Acknowledged (see scope-lock.md) |
| G166.9 | Evidence dir `artifacts/stage15/` created | ✅ Created |

---

## Scope Lock Declaration

**I acknowledge and commit to the following hard constraints for Stage 15:**

1. **Closure-only mandate** — No new design documents, no Phase B work, no feature development
2. **Valid exit statuses only** — Every blocker must be CLOSED or WAIVED-with-verified-file; no PARTIAL, PENDING, IMPLEMENTED, DESIGNED, UNCONFIRMED, LIKELY
3. **Tag immutability** — v4.0.0-rc2 must be fixed (re-created at correct commit), never moved again; v4.0.0-rc1 documented as CONTESTED
4. **Version truth** — VERSION file must match tag; v4.0.0-rc2 tag must be fixed to point to a commit with VERSION=4.0.0-rc2
5. **Blocker disposition** — B3, B4, B5, B7 must be CLOSED or WAIVED-with-verified-file; no PARTIAL
6. **Supply chain closure** — Each of 10 items must be DONE or WAIVED-with-expiry; no DESIGNED
6. **Fuzz count** — Frozen at 21 (authoritative grep count)
7. **No new design documents** — Zero Stage 15 design docs produced
8. **Tag repair** — v4.0.0-rc2 tag must be fixed (re-created at correct commit); backfill SHA collision resolved

---

## Phase 0 Verdict

**Baseline LOCKED** with documented critical issues:
1. v4.0.0-rc2 tag is BROKEN (points to commit with VERSION=4.0.0-rc1)
2. SHA collision between v3.8.0-stage8-backfill and v4.0.0-rc2
3. v4.0.0-rc1 moved 3× (CONTESTED)
4. All blockers currently WAIVED but need verification (Phase 1)
4. Supply chain controls need manual GitHub config (Phase 2)

**Next:** Phase 1 — Authoritative Waiver Register

---

*Generated by Stage 15 Phase 0 Baseline Lock*