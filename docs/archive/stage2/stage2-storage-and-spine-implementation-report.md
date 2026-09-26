# AETHER — STAGE 2 STORAGE & SPINE IMPLEMENTATION REPORT

**Sprint:** Stage 2 (§39 of the forensic baseline; Stage 2 directive)
**Date:** 2026-09-10 · **Version:** `3.3.0-stage2` · **Baseline:** `f5a90ef` (Stage 1)
**Companion:** `docs/stage2-evidence.md` (per-task citations), `docs/stage2-deferred.md`, `AETHER_STAGE2_ARCHITECTURE_BASELINE.md`

---

## 1. Executive summary

Stage 2 converted Aether's four uncoordinated persistence systems into **one canonical storage contract** (a schema-versioned, flock-protected, crash-safe bbolt vault per workspace) and Stage 1's mutation pipeline into the **canonical Action Spine** (AuthZ → Risk → Policy → Approval → Execute → Evidence → Audit → Rollback). DAG plan nodes, rollback undo, and replay/watch mutating intents now execute through the spine via a fail-closed intent whitelist — the CLI root is no longer a mutation execution path. Evidence records with epistemic classes are written per action, the CAP predictor can no longer claim 100% confidence, and the dead layers (`pkg/providers`, bolt remnant, phantom vault path, dead flags) are gone. A populated integration tier exercises the guarantees across real component boundaries. 27 packages pass with 0 failures.

## 2. Stage 1 baseline verification

Verified per directive §4: clean tree at `f5a90ef`, build/vet/tests green, integration placeholder ok, CI job inventory confirmed (race/lint/govulncheck/matrix/governance). Local host still cannot execute `-race`/golangci-lint/govulncheck (no C toolchain / tools absent) — CI enforces them; no false local claims are made.

## 3. Architecture before

Four persistence systems (per-record encrypted files + rewrite-on-append journal blob; signed audit JSONL without fsync; unlocked rollback JSONL with a destructive `Pop`; planner JSONL outside the workspace), three execution paths (mutation pipeline, CLI re-entry for DAG/undo, ungoverned `run` phases), phantom BoltDB layer, `pkg/providers` dead abstraction.

## 4. Architecture after

```
CLI / DAG / rollback-undo / replay / watch
        ↓ (mutating intents only)
   ACTION SPINE (internal/engine/spine)
   AUTHZ → RISK → POLICY → APPROVAL
        ↓
   before-state → AUDIT(before) → ROLLBACK REGISTRATION
        ↓ EXECUTE
   after-state → EVIDENCE → AUDIT(after) → JOURNAL
        ↓
   VAULT (internal/store/vault.go, bbolt)
   meta | records | journal | audit | rollback | rollback_failed
```

Read-only analysis commands (validate, cap, graph, prt convert, relay) still dispatch through the CLI root by design — they perform no external mutation and are documented as such.

## 5. Storage contract

`store.Vault` (`internal/store/vault.go`): Open/Close with flock (2s timeout), schema versioning (`SchemaVersion=1`; newer vaults refused), bucket-scoped records (caller-sealed ciphertext), journal (append-only, monotonic seq in-tx), audit (frozen Stage 1 chain), rollback stack (atomic Pop, failed-retention bucket), meta KV. Isolation: bucket-prefixed composite keys; cross-workspace isolation enforced by one-file-per-workspace + the Stage 1 path validator.

## 6. Journal design

Append-only with sequence numbers minted inside the write transaction (`AppendJournal`) — a crash can never produce gaps or duplicates. `Events()` fails closed on corrupt entries (the Stage 1 silent self-destruct is structurally impossible). `ReplaceJournal` supports rekey atomically.

## 7. Audit durability

`Log` is backend-agnostic (`auditBackend`): `jsonlBackend` (legacy, fsync per append) and `vaultBackend` (bbolt + explicit `Sync()` per append). Format/chain frozen. `Verify` distinguishes broken linkage from hash/signature tamper; a torn legacy line fails closed. Durability contract: a returned-successful `Append` survives process termination. Key initialization is atomic (`AuditMetaSetIfAbsent`) — a Stage 2 race found by acceptance testing (parallel DAG → mixed signers → "tampered" entry) was fixed and re-verified (10-entry chain VERIFIED).

## 8. Rollback design

`rollback.Stack` over the vault: atomic `Pop` (single tx), corrupt entries retained, `UndoAll` retains failed reversals in `rollback_failed` and flags `RetainedFailed`; `ListFailed` surfaces unresolved state. Irreversible mutations remain honestly represented (no rollback entry; `"rollback":"irreversible"` in the audit chain).

## 9. Action state machine

The spine enforces stage ordering structurally (stages run in sequence; abort paths return before later stages). Statuses: `completed | failed | aborted | completed_state_unknown | aborted_rollback_registration`; rollback lifecycle flags (`retained_failed`, `irreversible`) are recorded. A full explicit transition-table state machine (CREATED → … → ROLLED_BACK) is **deferred to Stage 3** and recorded in `docs/stage2-deferred.md` — implementing it fully would have required reworking every CLI handler in this sprint; the spine's ordered pipeline already makes illegal orderings unrepresentable in code paths.

## 10. AuthZ / Risk / Policy integration

`spine.Spine` carries `Capabilities` (local CLI: `AllowAll`; teamserver Stage 3: cert-bound sets), `MaxRisk` (config `max_risk_threshold`), `RiskEvaluator`, `PolicyEvaluator` (deny rules abort pre-execution), `Approver` (interactive refuses if no approver is available — fail closed). `--auto`-style automation (ApprovalAuto) removes only the human prompt; every other stage runs and `approval_mode` is recorded in both audit entries. `run --auto`'s silent gate suppression is superseded by the spine's audited approval modes.

## 11. DAG integration

`run plan` requires `--workspace` and executes nodes via `runIntent` (`internal/cli/governance.go`): a strict whitelist grammar (`exec azure|aws|github|gcp`, `simulate stream`) parsed into spine Actions. Unrepresentable commands fail closed. Verified live: a 2-node parallel plan produced 10 signed audit entries VERIFIED; a 3-node DAG integration test produced 3 Action IDs + 6 valid audit entries.

## 12. Teamserver integration

The teamserver's journal-only command path now writes through the workspace vault (attached with mandatory passphrase, Stage 1) and returns honest status. Full spine routing, RequestID correlation, multiplexing, and cert-bound operator identity remain **Stage 3** per the sprint non-goals (recorded in `docs/stage2-deferred.md`).

## 13. Operator identity

Local CLI: `actor` is recorded on every action and audit entry (`"cli"`, `"dag"`, `"replay"`, `"rollback-undo"`). Wire identity binding is Stage 3. Authentication and authorization remain separate concepts in the spine contract (`Capabilities` is injectable).

## 14. Evidence contract

`types.EvidenceRecord`: ID, ActionID, Kind, Target, EpistemicClass, Confidence, CollectedAt, ExpiresAt (plumbed), Method, Payload, ContentHash. Discipline enforced by `Validate()` (predicted ≠ 1.0, observed = 1.0, unknown = 0). The spine writes one record per action into the encrypted `evidence` bucket; CAP forecasts label their class (`UNKNOWN` with zero observations, otherwise `PREDICTED`).

## 15. Recovery model

Crash behavior: bbolt COW pages keep `vault.db` consistent at all times; the journal/audit/rollback/records state after an ungraceful handoff was verified by `TestEndToEnd_StorageCrashRecovery` (reopen → full records + journal + VERIFIED chain + intact stack). Failure matrix implemented: workspace-open/audit/rollback-registration failures ⇒ no execution; execution failure ⇒ `failed`; after-state failure ⇒ `completed_state_unknown`; audit-after failure ⇒ loud unresolved state.

## 16. Dead-code removal

`pkg/providers`, `store/bolt.go`, `planner/debug_test.go`, `DBPath()/vault.aedb`, dead flags (`--downgrade-pqc`/`pqcCheck`, `runPrioritize`, `behaviorWait`), import-keepers, duplicate `audit audit` registration — all removed with pre/post build+test verification. `token confuse --set-claim` was wired for real. All followed the directive's locate → verify reachability → remove → rebuild → verify sequence.

## 17. Test architecture

Unit suites updated to the vault (workspace, store, rollback, mutation, orchestrate). Integration tier (`test/integration/spine_storage_test.go`, `//go:build integration`): 8 end-to-end tests across real boundaries. `t.Parallel()` is deliberately absent where env-var path isolation is used (global-state constraint; Stage 3 config injection unlocks it) — documented, not hidden.

## 18. Race results

`go test -race` cannot execute on this host (no C toolchain). The one race found during Stage 2 (parallel DAG audit-key initialization, found via acceptance testing without the race detector) was fixed structurally: per-workspace serialized `AuditLog` + atomic `AuditMetaSetIfAbsent`. CI runs `-race` on ubuntu and windows for unit and (ubuntu) integration tiers.

## 19. Failure-injection results

`TestPipelineAuditFailureRefusesMutation` (injected audit-chain failure → no execution), `TestPipelineRollbackRegistrationFailureRefusesMutation` (injected stack failure → `aborted_rollback_registration`, no execution), `TestEndToEnd_SpineRiskAbort` / `TestEndToEnd_SpinePolicyDeny` (gate refusals, nothing executed, aborts journaled), `TestVaultFailedUpdateLeavesNoPartialWrites` (aborted tx leaves nothing), `TestPopCorruptEntryRetained`, `TestEndToEnd_StorageCrashRecovery`.

## 20. Security invariants

- No unvalidated user-controlled path reaches the filesystem (Stage 1, unchanged).
- No mutation bypasses the spine: whitelisted intents fail closed; CLI root dispatch is restricted to non-mutating commands.
- Audit/rollback unavailability ⇒ no execution (injection-tested).
- Failed reversals are retained and surfaced.
- Records/journal are sealed at rest (`TestVaultNoPlaintextAtRest`, `TestRecords`).
- Cross-workspace isolation via per-workspace vaults + flock (`TestVaultLockExclusive`, `TestEndToEnd_StorageConcurrency`).
- Schema version refuse-newer (`TestVaultSchemaVersionRefusesNewer`).
- Unknown state is never success (`completed_state_unknown`, evidence `unknown`).

## 21. Migration notes

Legacy `db/` workspaces import idempotently on first open (records, journal split into per-event entries, audit chain with its key, rollback stack); the original directory is preserved as `db.pre-vault-imported` — no user data is deleted. Migration runs only when `db/` exists.

## 22. Compatibility impact

CLI surface unchanged (no renames; one duplicate `audit audit` alias removed — `aether export audit` remains). New requirements for operators: mutating commands already required `--workspace` (Stage 1); `run plan` now also requires it. Legacy workspaces migrate transparently on open. `vault.db` replaces the `db/` record directory as the on-disk layout.

## 23. Remaining gaps

See `docs/stage2-deferred.md`: teamserver protocol v2 + identity (Stage 3), explicit Action transition-table + rollback idempotence states (Stage 3), planner determinism + graph provenance (Stage 3), full evidence model + supply-chain signing + release engineering (Stage 4), planner JSONL vault migration, true SIGKILL subprocess crash matrix.

## 24. Residual risk

- Audit signing key remains vault-local (plaintext within the workspace file) — unchanged threat model from Stage 1; escrow is Stage 4.
- Teamserver still lacks request correlation and per-command authorization — mitigated by passphrase-attached workspaces and honest non-execution, fully resolved in Stage 3.
- `t.Parallel` absence in integration tests is a fidelity (not correctness) limitation.

## 25. Exact commit list

| Commit | Task |
|---|---|
| `aadf2ea` | T4 — dead-layer removal + truthful help |
| `f343e18` | T1 — canonical storage contract (bbolt vault) |
| `c079e83` | T2 — canonical Action spine + intent whitelist + audit serialization race fix |
| `7d54659` | T3 — evidence seed + CAP predictor confidence honesty |
| `7c604b1` | T5 — populated integration tier |
| *(final)* | V — docs (CHANGELOG 3.3.0-stage2, README, evidence/deferred, this report) |

## 26. Final acceptance matrix

| Gate | Status |
|---|---|
| One canonical storage contract | ✅ `store.Vault` |
| One canonical Action lifecycle | ✅ `spine.Run` |
| One execution spine; DAG uses it directly | ✅ (intent whitelist; CLI root only for read-only) |
| Teamserver uses Action spine | ⚠️ journal path vault-backed; full routing Stage 3 |
| No mutation bypass | ✅ (whitelist fail-closed; grep-verified re-entry removal for mutations) |
| AuthZ/Risk/Policy centralized | ✅ spine stages |
| `--auto` cannot bypass controls | ✅ (approval stages audited; `run --auto` superseded) |
| Operator identity authenticated | ⚠️ local actor labels now; wire binding Stage 3 |
| Request correlation | ⚠️ ActionID everywhere; RequestID Stage 3 |
| Locking / atomicity / durability / schema / recovery / corruption detection / isolation | ✅ (tests cited above) |
| Explicit Action state machine | ⚠️ ordered pipeline now; transition table Stage 3 |
| Audit durable / tamper-detectable / crash-tested / no silent repair | ✅ |
| Rollback registered-before / retained failures / honest irreversibility | ✅ |
| Race green / lint green / govulncheck / build matrix | ✅ in CI; not locally executable (documented) |
| Version single-sourced / docs truthful / working tree clean | ✅ |
