# Stage 4 Backfill — Lineage Reconciliation (B4-G02 / WS0)

Status: **PASS** — determination: **RE-EXECUTED** (reconstruction not possible)

## 1. Claimed lineage vs verified reality

| Claim | Verified result | Classification |
|---|---|---|
| Stage 5 report exists at `d250d52` | `git cat-file -t d250d52` → `fatal: Not a valid object name` in **both** repositories (`C:\dev\aether`, OneDrive workspace) | **ABSENT** — unreachable; no artifacts to merge |
| Stage 5 report's claimed numbers (protocol conformance, interop, evidence counts) | No commit, tag, or object backing them exists | **ABSENT** — cannot be inherited; per evidence rules they are not evidence until re-executed |
| Stage 4 artifacts ever landed | `git log --all --oneline \| Select-String 'stage ?4|backfill'` → empty; no tag `v3.5.0-stage4*` existed | **ABSENT** |
| Baseline `990426b` (3.4.0-stage3) | valid commit, ancestor of HEAD | VERIFIED |
| `b3ed72f` (Stage 6 Phase 8) | valid commit, present in authoritative history | VERIFIED |
| "Stage 9 = `66b3600`" | `66b3600` is **not a valid object**; the live `v4.0.0-rc1` tag points at `58e6432` (later re-pointed to `28bcb99` by concurrent work) | **CONTRADICTED** — directive citation incorrect; actual current truth is the live tag |
| Archived external work (WS0) | no `d250d52` object in any local/ref position | **ABSENT** — WS0 resolves to **re-execute from scratch** |

## 2. Determination

WS0 resolution: **RE-EXECUTED**. Because the archived Stage 5 lineage is
unreachable and no Stage 4 artifacts exist, this backfill executed the
Stage 4 evidence work directly against the real code (`internal/store`,
`internal/api` PKI, `internal/engine/spine`) with new executable tests
(`test/integration/storage_safety_test.go`, `pki_lifecycle_test.go`,
`endurance_test.go`).

## 3. Concurrent-work note

During the backfill session, a parallel work stream committed
`28bcb99` (Azure Key Vault key provider) and re-pointed `v4.0.0-rc1`.
This backfill's changes are strictly additive to that state and were
rebased-in-order commit `8e956a2` (provider repair) + the backfill
commit. No history was rewritten.
