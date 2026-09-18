# Stage 38 Certification (Completion of Campaigns 4-6)

Baseline lock (G751 evidence): master @ `5cc7756` (HEAD, `git rev-parse`);
VERSION = `4.2.0-rc1`; tracked tree clean at lock (untracked tooling dir
`.kilo/` only; `ad_sampledata/` suite residue noted in the register,
awaiting operator removal approval); tags unchanged at every check:
`v4.0.0-rc2` 5cd008be, `v4.1.0-rc2` bc674af1, `v4.2.0-rc1` bb56cf3
(objects 779cdb6f / e8416b58 / bd0d9636); `v4.1.0` absent.

Binary tested: tag build `aether-tag.exe` @ `bb56cf3` AND master build
`aether-40.exe` @ `5cc7756`, both via detached worktree / sandbox builds
with the corrected ldflags binding; both print `aether version 4.2.0-rc1`.
Tags untouched (no tag points at or was created from these builds).

## Campaign results (raw evidence in the three campaign docs)

| Campaign | Findings | Verdict |
| --- | --- | --- |
| 4.1 config | 4 mapped-lineage reproductions on-tag / honest warnings on-master | PASS |
| 4.2 workspace traversal | 0 (9/9 typed rejections) | PASS |
| 4.3 credentials | 0 leaks | PASS |
| 4.4 audit tampering | 2 P2 storage defects OPEN on-tag / REMEDIATED on-master; chain evidence holds | PASS |
| 4.5 providers | 0 (11 classes mapped, extensions re-proven) | PASS |
| 4.6 plugins | checksum enforcement holds; 1 Info size-cap finding | PASS |
| 4.7 sessions | 0 (append-only + race-safe) | PASS |
| 4.8 exhaustion | 0 (bounded at observed scale) | PASS |
| 5 integration | 0 (lifecycle + determinism + idempotency hold) | PASS |
| 6 resilience | 2 P3/Info findings (F-40-2, F-40-3 class); typed errors throughout | PASS |

## Verdict

STAGE 38 (Campaigns 4-6, continuation) is COMPLETE.

- All six campaigns executed with raw commands and captured outputs.
- Four-fix Stage 34 lineage reproduced exactly as evidenced on-tag; all
  four REMEDIATED on-master (Stage 35 closed F-34-3 and F-34-4; `ee76943`
  closed F-34-1 and F-34-2 - verified by `git merge-base --is-ancestor`
  against both builds).
- New findings: 1 x P2 (F-40-1, transport mismatch - NEW on both builds),
  2 x P3 (F-40-2, F-40-3), 1 x Info (F-40-4). New P0: 0. New P1: 0.
- No finding is a promotion blocker under the Production-Limited posture;
  F-40-1 is classified INFO inside the observation window per the charter
  rule (observation events are recorded, not silently closed).

## Stage 40 corrections landed (record: docs/stage40-corrections-register.md)

1. Stage 33b matrices carry both DOWNGRADED rows with the disposition
   record reference added.
2. release.yml Validate failure classified BLOCKED-WITH-OWNER (gh
   unauthenticated; remedy documented). Local Validate re-proven green
   this stage.
3. ldflags citation corrected (the one wrong citation in the tree); the
   correct binding verified by building both binaries and printing
   `4.2.0-rc1`.
4. Fuzz count corrected 23 -> 21 across 15 historical docs (16
   references); authoritative count 21 verified by running all 21 targets
   (5.81M execs, 0 crashes).
5. F-30-3 and LIVE-1 promoted to standalone waiver files (owner + risk +
   impact + compensating controls + expiry + approval).

## Quality gates (raw, this stage)

| Gate | Raw result |
| --- | --- |
| go build ./... | PASS |
| go vet ./... | PASS |
| go test -count=1 ./... | PASS (29 packages ok) |
| go test -tags=integration -count=1 ./test/integration/... | `ok ... 12.995s` |
| fuzz: 21/21 named targets x 10s | 5,812,358 execs, 0 crashes |
| golangci-lint run ./... | 0 issues |
| govulncheck ./... | 0 affecting vulnerabilities |

## Stage 37 retry handoff (embedded, G780)

Stage 37 retry remains time-gated to 2026-10-17T16:55:20Z and is NOT
begun. When the clock passes: verify the time gate, run the row-level
observation-log audit (target: 0 P0/P1), clean-room build from `bc674af1`,
end-of-window unit + integration + release-surface matrix, then PROMOTE to
`v4.1.0` (VERSION bump commit + annotated tag + push + verify) or RETAIN
with a specific reason. Before promotion, F-32-3 (`gh auth login`) must
open CI so the Validate diagnosis (release.yml #35253502855-class run
logs) completes from run history. Hard rule unchanged: if the gate has not
elapsed, halt and re-defer.
