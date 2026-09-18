# Stage 38 Completion - Campaign 5: Integration

Baseline: master @ `5cc7756`; tag build `bb56cf3`; sandboxed
`AETHER_CONFIG_DIR`. Workspace `engage`.

## C5a - Full engagement lifecycle

| Step | Raw result |
| --- | --- |
| `workspace create engage` | created |
| `audit record --cmd stage-one --result ok` | seq 1 recorded |
| `rollback push` x2 (entra_consent_grant/add, sp_secret/remove_password) | stack depth 2 |
| `rollback list` | both entries with undo mappings |
| `export audit` / `report` / `executive` | JSONL + MD emitted (4 artifacts) |
| `audit verify` (master) | `VERIFIED (1 entries, 0 tampered)` |

## C5b - Rollback determinism

`rollback list` executed twice (tag build then master build): byte-identical
output (`ROLLBACK_LIST_DETERMINISTIC=True`). The stack is a deterministic
function of the recorded state, not of the reader.

## C5c - Rollback undo (deterministic cleanup)

`rollback undo` on both entries: cloud-side reversal is not faked - each
op reports `[FAIL] ... requires manual reversal (recorded in report)
[retained]`. Honest fail-safe: entries retained with the report pointer,
chain VERIFIED after undo, post-undo list deterministic
(`POST_UNDO_DETERMINISTIC=True`).

## C5d - Replay idempotency

Same runbook (validate + simulate + comment line), dry-run default, three
executions across both builds: identical output every time
(`REPLAY_DRY_IDEMPOTENT=True`, `REPLAY_DRY_REPEATABLE=True`), exit 0, no
journal pollution (`audit verify` unchanged after replay dry-run).

## Gate-quality re-verification (same tree)

| Gate | Raw result |
| --- | --- |
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `go test -count=1 ./...` | PASS (29 packages ok) |
| `go test -tags=integration -count=1 ./test/integration/...` | `ok ... 12.995s` |
| golangci-lint | `0 issues.` |
| govulncheck | `Your code is affected by 0 vulnerabilities.` |

Campaign 5 verdict: PASS - integration state consistent; lifecycle,
determinism, and idempotency all hold with raw evidence.
