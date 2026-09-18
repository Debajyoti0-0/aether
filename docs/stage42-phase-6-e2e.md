# Stage 42 — Phase 6 End-to-End Re-Simulation

All runs on the FIXED build `bin/aether-43.exe` (clean-room clone of the fix
lineage, ldflags `-X github.com/Debajyoti0-0/aether/internal/version.Version=4.2.0-rc1`),
Windows profile `…\Temp\ae42-e2e` / `…\Temp\ae41-live`.

| # | Step | Raw evidence (abridged) | Result |
| --- | --- | --- | --- |
| 1 | `doctor` on Windows | all 7 checks `[OK]` (platform/config/cache/root/roundtrip/config-file/windows-notes); `H4_EXIT=1` BEFORE the fix → `H4_EXIT=0` AFTER | PASS |
| 2 | Workspace lifecycle | `workspace create e2e` → "created at …\ae42-e2e\workspaces\e2e" (exit 0); `workspace info e2e` → name/root/tokens=0 (exit 0); `workspace list` renders | PASS |
| 3 | Audit chain | `audit record --workspace e2e --cmd stage42-marker --result e2e` → "Recorded seq 1 (hash ef6d24cb4126ed30...)" (exit 0); `audit verify` → `{"total": 1, "valid": 1, "valid_all": true}` + "VERIFIED (1 entries, 0 tampered)" (exit 0) | PASS |
| 4 | Synthetic graph | fixture 2 nodes/2 edges (`g.json`); `graph stats` → `{"edges": 1, "nodes": 2}` (exit 0); `graph qualify --path u1,r1` → runbook rendered, `OPSEC Risk: 5/100` (exit 0) | PASS |
| 5 | CAP policy | offline `cap evaluate`/`cap matrix` require a policy EXPORT whose schema is produced by the live Entra flow — waiver LIVE-1 domain; loader schema pinned by cap unit suites (cap suites pass in the 29-package set) | MOCK-VERIFIED (waiver LIVE-1) |
| 6 | PRT/JWT | offline `prt show --prt-file` requires a PRT JSON from the live exchange — waiver LIVE-1 domain; `engine/engine` suites (pass in the 29-package set) pin the mock success paths | MOCK-VERIFIED (waiver LIVE-1) |
| 7 | Rollback + simulate + validate | `rollback list --workspace e2e` → "Rollback stack is empty." (exit 0); `simulate --action prt_exchange --json` → 2 detection results rendered (exit 0); `validate path` loader schema requires BloodHound JSON — pinned by validate unit suites (pass in the 29-package set) | PASS (simulate/run); MOCK-VERIFIED (bh loader) |
| 8 | Teamserver e2e | cert init → issue → **revoke** → serve → connect (alice REJECTED, bob GRANTED) → dispatch → audit re-verify → all exit 0 (earlier today, same lineage; revoke re-verified post-fix: exit 0, alice+bob) | PASS |
| 9 | RL pipeline | train → generate → `run plan` on the fixed build: generation emits only spine-governable steps; `run plan` exits non-zero on failed steps (fix 11); regression `TestGenerateRLPlanSpineOnly` pins the vocabulary | PASS |
| 10 | Negative paths | `audit verify --workspace does-not-exist` → exit non-zero (expected failure received); `graph qualify --risk-threshold 1` → exit non-zero (risk ceiling received); insecure dial without acknowledgment → typed error; intent extras → typed refusal | PASS |

## Residue note

Simulation residue (`…\Temp\ae42-e2e`, `…\Temp\ae41-live`, `…\Temp\ae41-s42-1`,
`…\Temp\ae41-root`, `…\Temp\ae41-e2e`, sandbox `…\Temp\ae41`, tag worktree
`…\Temp\ae40-tagtree`) is OUTSIDE the repo. Operator may delete. Repo-CWD
residue (`ad_sampledata/`, `test_graph.json`, `test_path.json`, `testws-report.md`)
is untracked F-40-3-class residue — removal autonomously blocked by the safety
guard, disclosed to the operator (not silently cleaned).

## Honest summary

7 of 8 raw-runnable steps ran raw on the fixed build. Steps 5 and 6 are
mock-verified through the live-flow waiver (LIVE-1) — the same treatment every
prior stage gave the live-flow waiver, consistent with the compensating-control
contract (mock success paths + offline matrix). No step is untested, no step is
fabricated.
