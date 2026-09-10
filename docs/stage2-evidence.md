# Aether Stage 2 Evidence Report

**Sprint:** Stage 2 "Storage & Spine" (forensic report §39)
**Date:** 2026-09-10 · **Baseline:** Stage 1 complete (`f5a90ef`) · **Version:** `3.3.0-stage2`

---

## Per-task citations

### Phase 0 — Baseline
- `AETHER_STAGE2_ARCHITECTURE_BASELINE.md` — verified Stage 1 state, execution-graph table for every mutating path, four-storage-system inventory, Stage 2 acceptance criteria.

### T4 — Dead-layer removal (commit `aadf2ea`)
- Deleted: `pkg/providers/` (zero impls/consumers), `internal/store/bolt.go`, `internal/planner/debug_test.go` (assertion-free).
- Removed: `workspace.DBPath()` + phantom `vault.aedb`; dead flags `--downgrade-pqc`/`--jwks-url` + `pqcCheck` (`v33.go`), `runPrioritize`, `behaviorWait` (`v31.go`); import-keepers (`extras.go`, `providers_exec.go`); duplicate `audit audit` registration (`v3c.go` init).
- Wired for real: `token confuse --set-claim` is now a `StringSlice` bound to `confuseClaimList` (was declared-but-unbound).
- Truthful help: `tunnel` ("single proxy probe — not a tunnel"), `ztna exec` ("reachability probe — does not execute commands"), `relay fido2-downgrade` ("preview only — not sent"), export `--workspace` flags relabeled as engagement labels.
- Verification: `grep -r "pkg/providers\|vault.aedb" .` → no matches; `go build ./...` + full suite green.

### T1 — Storage contract (commit `f343e18`)
- `internal/store/vault.go` — `Vault` over bbolt: buckets `meta/records/journal/audit/rollback/rollback_failed`; `SchemaVersion=1` enforced (newer refuses); cross-process `flock` (2s timeout → "locked by another process"); journal appends carry a monotonic seq minted in the same tx (no gaps/dupes); `ReplaceJournal` for rekey; `RollbackPop` is a single tx; `RollbackRetainFailed` preserves failed reversals.
- `internal/store/audit.go` — `Log` made backend-agnostic (`auditBackend`): `jsonlBackend` (legacy, fsync per append, torn line fails closed) and `vaultBackend` (bbolt + explicit `Sync()` per append — audit is the legal artifact). Entry format and Ed25519 chain frozen (Stage 1). Signing key initialized atomically via `AuditMetaSetIfAbsent` (concurrent openers converge on one key).
- `internal/workspace/workspace.go` — all state flows through the vault: records/journal sealed per-record AES-256-GCM; journal is append-only (`LogEvent` no longer rewrites history — the Stage 1 self-destruct bug is structurally gone); corrupt journal entries fail closed; legacy `db/` layout imported idempotently then preserved as `db.pre-vault-imported` (no user data destroyed); `Close()` releases the flock.
- `internal/engine/rollback/stack.go` — rewritten over the vault; corrupt popped entries retained; `ListFailed`/`UndoOutcome.RetainedFailed` surface unresolved reversals.
- New layout: `workspaces/<name>/{vault.db, salt.bin, KEYLESS?, artifacts/, reports/}`.
- Tests: `internal/store/vault_test.go` (10 tests: isolation, monotonic journal, failed-tx atomicity, audit chain across reopen, schema refusal, lock exclusivity, no-plaintext-at-rest), `internal/workspace/workspace_test.go` (rewritten incl. `TestVaultLockExclusive`), live CLI check: `workspace create` → `vault.db` present, `db/` gone; `audit record` + `audit verify` → `VERIFIED (1 entries)`.

### T2 — Action spine (commit `c079e83`)
- `internal/engine/spine/spine.go` — canonical `Spine.Run`: StageAuthZ (capabilities) → StageRisk (score vs config MaxRisk) → StagePolicy (deny rules) → StageApproval (`auto` never bypasses the other stages; `pre-approved` requires ApprovalRef; `interactive` requires an Approver, refuses without one) → execution pipeline (before-state → AUDIT(before) → pre-execution rollback registration → Execute → after-state → EVIDENCE → AUDIT(after)) → journal. Statuses: `completed | failed | aborted | completed_state_unknown | aborted_rollback_registration`. Stage-1 fail-closed rules preserved unchanged.
- `internal/engine/mutation/mutation.go` — `Mutation`/`UndoSpec` are aliases of the spine's canonical types; `Run` is a spine adapter (all Stage 1 CLI call sites are now spine-native without handler churn).
- CLI re-entry eliminated for mutations: `run plan` nodes, `rollback undo` commands, and replay/watch mutating intents route through `runIntent` (`internal/cli/governance.go`) — a strict intent whitelist (`exec azure|aws|github|gcp`, `simulate stream`) that fails closed on anything unrepresentable; read-only analysis commands still dispatch through the root by design (documented).
- Race fix found by acceptance testing: parallel DAG nodes each instantiated a `NewVaultLog` → Ed25519 key-init race (mixed signers → "tampered" entry 1). Fixed by per-workspace cached `AuditLog()` (serialized appends, `workspace.workspace.go:AuditLog`) + atomic key init (`store.vault.go:AuditMetaSetIfAbsent`). Re-verified: 2-node parallel DAG → 10 entries VERIFIED.
- Tests: `internal/engine/mutation/pipeline_test.go` ported to the spine (incl. failure-injection seams for audit/rollback refusal).

### T3 — Evidence seed (commit `7d54659`)
- `internal/types/evidence.go` — `EvidenceRecord` with `EpistemicClass` (observed/inferred/predicted/unknown) and confidence discipline enforced by `Validate()` (predicted can NEVER carry 1.0; observed must be exactly 1.0; unknown is 0).
- `internal/engine/spine/spine.go:writeEvidence` — one record per action into the `evidence` records bucket; validation failures are journaled loudly.
- `internal/engine/cap/predictor.go` — zero observations → confidence 0 (rendered `UNKNOWN — no observations collected`); 1 day → 0.33; 3+ days → 0.66; class always PREDICTED. The fabricated 100% is gone.
- Tests: `internal/types/evidence_test.go`, updated `predictor_test.go` (`TestPredictorEmpty` asserts 0).

### T5 — Integration tier (commit `7c604b1`)
- `test/integration/spine_storage_test.go` — 8 end-to-end tests: spine audit chain (verify VERIFIED + evidence + journal), risk abort, policy deny, storage lock concurrency, crash recovery (ungraceful handoff + reopen integrity), rollback undo retention + audited undo, DAG-through-spine (3 nodes → 3 Action IDs + 6 valid audit entries), evidence classes.
- Note: `t.Parallel()` is intentionally NOT used on these tests — workspace path isolation relies on process-global env vars; parallelism requires Stage 3 config-path injection (recorded in deferred).

### V
- `docs/stage2-deferred.md`, `CHANGELOG.md` (`3.3.0-stage2`), `README.md` (storage layout + spine), this file, and `AETHER_STAGE2_STORAGE_AND_SPINE_IMPLEMENTATION_REPORT.md`.

## Live verification

| Check | Result |
|---|---|
| `go build ./...` / `go vet ./...` | clean |
| `go test -count=1 ./...` | 27 packages, 0 failures |
| `go test -tags=integration -count=1 ./test/integration/...` | ok (8 tests) |
| `go test -race` / golangci-lint / govulncheck | not executable on this Windows host (no C toolchain / tools absent); CI jobs enforce them (ubuntu + windows) |
| `workspace create` layout | `vault.db`, `salt.bin`, `artifacts/`, `reports/` (no `db/`) |
| two concurrent workspace opens | second fails "locked by another process" |
| `audit record` + `audit verify` | `VERIFIED (1 entries, 0 tampered)` |
| 2-node parallel DAG (fake tokens) | 10 audit entries, `VERIFIED (10 entries, 0 tampered)` |
| `cap predict` with no data | `confidence 0%, class: UNKNOWN` |
| version | single-sourced from `VERSION` → `internal/version` |
