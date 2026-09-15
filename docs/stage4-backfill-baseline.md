# Stage 4 Backfill — Baseline (B4-G01)

Status: **PASS** (with one documented deviation, see §2)

## 1. Verified baseline facts

| Item | Value | Verified via |
|---|---|---|
| Audit start commit | `58e6432` (v4.0.0-rc1 tree) | `git rev-parse HEAD` at session start |
| Baseline `990426b` (3.4.0-stage3) | present, **ancestor of HEAD** | `git cat-file -t`, `git merge-base --is-ancestor` |
| Current VERSION | `4.0.0-rc1` — **untouched by backfill** | `cat VERSION`; `./bin/aether.exe --version` → `aether version 4.0.0-rc1` |
| Version-monotonicity rule | ACKNOWLEDGED: backfill produces a historical tag only (`v3.5.0-stage4-backfill`); `VERSION` and the live `v4.0.0-rc1` tag do not move backwards. | this document |
| Vault backend | **bbolt** `go.etcd.io/bbolt v1.3.8`, one file per workspace (`vault.db`), `SchemaVersion uint64 = 1`, exclusive flock with 2 s timeout, buckets: `meta`, `records`, `journal`, `audit`, `rollback`, `rollback_failed` | `internal/store/vault.go` |
| Audit chain | SHA-256 hash chain + Ed25519 signatures, genesis `000…0`, fsync per append, `Log.Verify()` tamper report | `internal/store/audit.go` |

## 2. Working-tree deviation (B4-G01.2)

Go source and test files were clean before and after the backfill work.
The tree additionally contains **pre-existing untracked documentation**
from parallel work streams (stage6-corrections, stage7-*, stage8-*,
stage10-*, `Dockerfile.race`) which is not owned by this backfill and
was left untouched. This is recorded as the sole B4-G01.2 deviation;
no build/test input files are untracked or modified outside this
stage's own changes.

## 3. Pre-backfill storage test inventory (B4-G01.5)

| File | Lines | Scope |
|---|---:|---|
| `internal/store/vault_test.go` | 231 | vault open/schema/records/rollback |
| `internal/store/audit_test.go` | 128 | chain append/verify/export |
| `internal/store/store_test.go` | 54 | config store |
| `internal/store/keyprovider_test.go` | 306 | key provider lifecycle |
| `test/integration/spine_storage_test.go` | 366 | e2e spine, storage concurrency + crash recovery |
| `test/integration/idempotency_*.go` | 446 | replay/idempotency incl. crash recovery |

## 4. Evidence directory

`artifacts/stage4-backfill/` created; contains raw run logs and tool
outputs cited by the gate matrix. SHA-256 checksums in
`artifacts/stage4-backfill/checksums.txt`.
