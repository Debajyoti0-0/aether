# Aether — Deferred to Stage 4+ (from Stage 3)

Per directive §40 (no feature creep): everything discovered but not required for the Stage 3 control-plane gates.

## Stage 4 (Protocol Truth, §41)

1. **T6 — SIGKILL subprocess crash matrix** (the one de-scoped Stage 3 task): `cmd/crashtester` + 8-stage kill/reopen integration suite. Partial coverage today: `TestEndToEnd_StorageCrashRecovery` (ungraceful handoff), T4 terminal-state semantics, bbolt COW guarantees. First CI item for Stage 4.
2. Kerberos AS-REQ/AS-REP conformance, XML-DSig C14N/SignedInfo, PRT broker grant/proof (unchanged from the forensic report §41).
3. IMDSv2 per-cloud split; transport proxy/uTLS fix.

## Stage 4/5 (control-plane follow-ups)

4. **Request idempotency keys** — retried mutating commands currently double-execute (audit-visible). Needs persistence semantics design (RequestID → ActionID ledger in the vault).
5. **OCSP/CRL distribution for operator revocation** — revoked.txt is connection-time file-based.
6. **Per-IP rate limiting + idle-connection reaping** — frame caps, in-flight caps, and connection caps exist; adaptive limits do not.
7. **Per-event signatures on the event stream** — events are transport-authenticated only; the audit chain remains the tamper-evident record.
8. **Rollback lifecycle states in the action machine** — `rollback_pending/rolled_back/rollback_failed` remain tracked by the rollback stack retention model; unifying them into `ActionState` needs an undo-execution spine action design.

## Stage 5 (Evidence, §42)

9. RequestID/OperatorID on EvidenceRecord (spine currently stamps ActionID + actor via audit; evidence carries ActionID only).
10. Full provenance/confidence/temporal model (unchanged deferral).

## Stage 6 (Determinism, §43)

11. Planner determinism (F10): `LoadPolicy` still wall-clock seeds; epsilon retained in generation.
12. Replay environment capture (version/commit/graph/policy hashes).

## Operational

13. **Run the repository outside OneDrive-synced paths** (or pause sync during sprints): Stage 3 lost `internal/planner/` working-tree files repeatedly to a OneDrive sync race (recovered from git; commits `442b67e…2faa9b3`). Risk: silent file loss during heavy write bursts.
14. Full config-path dependency injection (beyond T7's TestMain pattern) to unlock library-level parallel isolation.
15. `bin/aether.exe` still committed; goreleaser/checksums/SBOM (F21, Stage 7).
16. Dashboard HTML/JS live-polling of `/api/events` (server-side live feed done in T8; the static HTML does not yet auto-refresh).
