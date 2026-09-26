# AETHER — STAGE 2 ARCHITECTURE BASELINE (Phase 0)

**Date:** 2026-09-10 · **Baseline commit:** `f5a90ef` (Stage 1 complete, 10 commits)

## 1. Verified Stage 1 state

- `git status` clean; `go build ./...` OK; `go vet ./...` clean; full test suite 0 failures; `go test -tags=integration ./test/integration/...` ok (placeholder).
- CI (`.github/workflows/ci.yml`) defines: governance, build matrix (3 OS), vet, `-race` (ubuntu + windows), golangci-lint, govulncheck, integration. Local host cannot execute `-race`/golangci-lint/govulncheck (no C toolchain / tools absent) — verified honestly in Stage 1 evidence.

## 2. Current execution graph (every mutating path)

| Command | Handler | Executor | Workspace | Journal | Audit | Rollback | Storage system used |
|---|---|---|---|---|---|---|---|
| exec azure/aws/github | cli/exec.go | exec/* | required (F5) | yes | 2 signed entries | irreversible record | workspace records (files) + audit JSONL + rollback JSONL |
| exec gcp | cli/v3c.go | plugin | required | yes | 2 | irreversible | files + JSONL |
| exec parallel | cli/v3.go | per-target pipeline | required | yes | 2×N | irreversible | files + JSONL |
| providers exec | cli/v3.go | plugin | required | yes | 2 | irreversible | files + JSONL |
| simulate stream | cli/v3b.go | validate.HEC | required | yes | 2 | irreversible | files + JSONL |
| plugins install | cli/v3c.go | remote registry | required | yes | 2 | registered | files + JSONL |
| prt import | cli/pivot.go | workspace records | required | yes | 2 | registered | files + JSONL |
| pivot cloud-to-onprem | cli/pivot.go | KKDCP | required | yes | 2 | registered | files + JSONL |
| run plan (DAG) | cli/v3.go | **CLI re-entry** | via child cmd | via child | via child | via child | files + JSONL |
| rollback undo | cli/v3b.go | **CLI re-entry** | required | no | no | n/a | rollback JSONL |
| run (kill chain) | cli/run.go | orchestrate | required | yes | **no** (only risk gate; phases don't mutate) | no | files |

## 3. Current storage implementations (four uncoordinated systems)

1. **Workspace record files** — `internal/workspace/workspace.go`: AES-256-GCM per-file under `<root>/db/<bucket>/<key>`; rewrite-on-append journal blob at `db/events/journal` (O(N²), unlocked, non-atomic, silently self-destructs on corrupt read at `LogEvent → events, _ := loadEvents()`); no fsync; no atomic rename; no locking; schema-less.
2. **Audit JSONL** — `internal/store/audit.go`: Ed25519-signed hash chain at `db/audit.jsonl`, key beside log; no fsync; torn line bricks `Verify`; `New()` resumes from unverified tail.
3. **Rollback JSONL** — `internal/engine/rollback/stack.go`: LIFO at `db/rollback.jsonl`; `Pop` rewrites before validating; failed reversals dropped; per-instance mutex only.
4. **Planner JSONL** — `internal/planner/episode.go` + policy files: unlocked, unencrypted, outside the workspace (user-path CLI artifacts).

Plus dead weight: `internal/store/bolt.go` (bbolt, imported only by its own test), `workspace.DBPath()/vault.aedb` phantom path, `pkg/providers` (zero impls/consumers), dead CLI flags/vars, duplicate `audit audit` registration, `debug_test.go` (no assertions).

## 4. Known races / durability weaknesses carried into Stage 2

- Journal lost-update + self-destruct (fixed by T1/T2 work in this sprint).
- No cross-process locking anywhere (bbolt flock will own this).
- Audit/rollback torn-write windows; audit-after failure leaves unresolved durable state (failure matrix defined in Stage 2 report).
- CLI re-entry execution (DAG, rollback undo) — removed by T2.

## 5. Stage 2 acceptance criteria (from the directive)

One `Store` (bbolt, schema-versioned, locked, crash-safe); one `Spine` (AuthZ → Risk → Policy → Approval → Execute → Evidence → Audit → Rollback); DAG + rollback undo through the spine (no CLI re-entry); evidence records with epistemic classes (CAP predictor never emits 1.0); dead layers removed; populated integration tier; `docs/stage2-evidence.md` + `docs/stage2-deferred.md`; CHANGELOG 3.3.0-stage2; README updated.

## 6. Scope decisions recorded up front

- **bbolt** is the storage engine (already a dependency; no new deps).
- **Planner episode/policy JSONL files are CLI user-path artifacts**, not workspace state; they stay file-based in this sprint and are recorded in `docs/stage2-deferred.md` (their migration touches CLI UX and Stage 3 planner determinism).
- **Audit format frozen** (Entry struct + Ed25519 chain, Stage 1); storage moves from JSONL into `vault.db` with one-time idempotent import of existing `audit.jsonl`.
- `--auto` on `run` is replaced by explicit `--approval auto` semantics in the spine (audited), per the directive's `--auto` rule.
