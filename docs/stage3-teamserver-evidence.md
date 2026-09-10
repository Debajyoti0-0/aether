# Aether Stage 3 — Teamserver V2 Evidence Report

**Sprint:** Stage 3 "Teamserver V2 & Carryover Hardening" (forensic report §40)
**Date:** 2026-09-11 · **Baseline:** `8c300b8` (Stage 2) · **Version:** `3.4.0-stage3`

---

## Per-task evidence

### Phase 0 — Baseline
`AETHER_STAGE3_TEAMSERVER_ARCHITECTURE_BASELINE.md` — full protocol/lifecycle/authn/authz/dispatch/event/concurrency/shutdown map of the pre-sprint teamserver, caller/callee matrix, 8 enumerated failure modes.

### T7 — `t.Parallel()` enablement (commit `64307f1`)
- `test/integration/spine_storage_test.go:TestMain` owns one config dir per process; tests use unique per-test workspace names (`newWorkspace`), `t.Parallel()` on all 8 tests.
- Why this shape: `t.Setenv` forbids `t.Parallel`; per-process env + unique names gives parallel safety without library changes. Full config-path DI remains deferred (see deferred doc).
- Verified: `go test -tags=integration -count=1 -parallel 4 ./test/integration/...` → ok.

### T1 — mTLS & cert-bound identity (commits `f814bed`, `82f0711`)
- **CA hierarchy** (`internal/api/ca.go`): `InitCA` (CA cert, pathlen=1 + server cert, never overwrites), `IssueOperatorCert` (CN + `URI:aether:operator:<name>`, ClientAuth EKU), `LoadServerTLS`/`LoadOperatorTLS`. ECDSA P-256 chosen over the directive's Ed25519 sketch for non-Go tooling interop (documented deviation — identical trust properties).
- **Identity** (`internal/api/identity.go`): `FromClientCert` — fail-closed on nil cert, missing/malformed URI SAN, expiry, hostile names; SHA-256 cert fingerprint; `RevocationList` (revoked.txt, one name per line).
- **Capabilities** (`internal/api/capabilities.go`): canonical names (exec.*/simulate.stream/plugins.install/prt.import/pivot.cloud-to-onprem/relay/read.*); `LoadOperatorCaps` (operators/<name>/capabilities.json, `-cap` denial, defaults read-only); `WriteOperatorCaps`.
- **Server** (`internal/api/server.go`): `ClientCAs` + `RequireAndVerifyClientCert`; handshake + identity extraction before any protocol; revoked operators refused; max-connections bound; per-conn write mutex. `CommandRequest.Operator` **removed** from the wire; `CommandRunner` now receives the cert-derived `*Operator`.
- **CLI** (`internal/cli/serve_cert.go`, `connect.go`): `serve cert init|issue|revoke`; serve refuses to start without all PKI files (no self-signed fallback); `connect --operator-cert/--operator-key/--server-ca` mandatory (self-signed fallback removed); `--insecure` gated behind `--i-know-what-im-doing` + loud warning.
- **Tests:** `identity_test.go` (`TestIdentityFromCert` valid/expired/not-yet-valid/missing-SAN/hostile-name/nil; `TestRevokedOperatorRefused`; `TestOperatorCapabilities` defaults/denial/traversal). All Stage 1/2 test downgrades (`RequireAnyClientCert`, `genClientCert`) removed — no mTLS weakening remains (`grep -rn "RequireAnyClientCert" internal/` → nothing).

### T2 — Protocol v2 (commits `f814bed`, `82f0711`)
- `internal/api/protocol.go`: `ProtocolVersion=2`; `Envelope{Version, Type, RequestID, Seq, Payload}`; `Envelope.Validate` rejects wrong versions and correlation-less commands (no downgrade, no implicit association); `NewRequestID`.
- `CommandResponse{ActionID, Status, Stage, OperationID, Output, Error, TookMS}`; `WorkspaceRequest{Cursor}`; `WorkspaceUpdate.Seq`.
- **Multiplexing** (`server.go:serveConn`): per-connection in-flight cap (8) with backpressure rejection; per-conn write mutex; commands run on goroutines and echo RequestID.
- **Event store** (`internal/api/events.go`): `EventStore` over the workspace vault (`ts_events` bucket, `meta/ts_events_seq:<ws>`), monotonic per-workspace sequences minted transactionally, gap detection on replay, 100k retention pruning, `ReadSince` cursor replay.
- **Client** (`internal/api/client.go`): read-loop demultiplexer (`pending` map by RequestID; subscription matching by workspace); `ExecuteCommandCtx` correlated; `StreamWorkspaceFrom(ws, cursor)`; `Ping`.
- **Tests:** `TestFrameRoundTrip` (real decode — replacing the Stage 1 tautological test), `TestFrameProtocolValidation` (v1 rejection, missing rid, oversized frame), `TestMultiplexedCommands` (100 concurrent, 8-in-flight, no cross-talk), `TestSubscribeWithCursor` (replay 3-5, live 6, fresh-client resume at 5), `TestEventStoreSequencing`, `TestMultiOperatorRace` (3 operators × 50 concurrent commands, cert-derived actor verified, zero identity leaks).

### T3 — Spine dispatch (commits `f814bed`, `82f0711`)
- `internal/cli/serve_dispatch.go:serveCommandRunner`: cert-derived operator → capability check (`capabilityForKind`) → intent whitelist (`buildIntentMutation`) → `spine.Run` with `s.Caps = op.Caps`, `Actor: "operator:<name>"` → honest `CommandResponse{ActionID, Status, Stage, OperationID, Error}`. No fabricated success (`grep "queued:" internal/api|internal/cli` → nothing).
- Audit/evidence/rollback flow automatically via the spine; audit entries carry `actor=operator:<cert-name>`.
- Cross-operator safety: per-workspace serialized audit chain (Stage 2) + bbolt write lock; verified by `TestMultiOperatorRace`.

### T4 — Action state machine (commit `021272c`)
- `internal/engine/spine/state.go`: `State` graph + `ActionState.Transition` (illegal transitions error; terminal states frozen: `completed | completed_state_unknown | failed | aborted`).
- `spine.Run` records the full transition history into `Result.StateSeq` and the journal entry (`states created>…>completed`). Happy-path sequence pinned by `TestSpineRunsRecordStateSeq`; failure path by `TestSpineFailureStateSeq`; invariants by `TestIllegalTransitionsRejected` (CREATED→EXECUTING and FAILED→COMPLETED impossible) and `TestUnknownNeverBecomesSuccess`.
- Note: rollback lifecycle states (`rollback_pending/rolled_back/rollback_failed`) remain tracked by the rollback stack's retention model rather than the action machine (documented design choice).

### T5 — Planner vault migration (commits `dda0477`, `442b67e`, `0e56ea3`, `a65076e`, `e174078`, `2faa9b3`)
- `internal/planner/episode.go`: `VaultEpisodeStore` (`planner_episodes` bucket, deterministic ID ordering).
- `plan export` defaults to the vault; `--output` retains JSONL as an explicit interchange format; legacy `<ws>-episodes.jsonl` imports idempotently and is preserved as `.pre-vault-imported`.
- Tests: `TestPlannerVaultRoundTrip`, `TestPlannerLegacyMigration`, `TestPlannerNoPlaintextOnDisk`.
- **Environment incident (corrected):** the recurring deletion of `internal/planner` / `internal/rl` working-tree files was initially misattributed to a OneDrive sync race. True root cause: `TestPlannerLegacyMigration` used `defer os.RemoveAll(dir)` with `dir` = the test's working directory — the deferred cleanup deleted the entire package directory after every test run (including every `go test ./...`), which perfectly mimicked a sync race. Fixed in `internal/rl/store_vault_test.go` (cleanup removes only the renamed legacy artifact); the recovery commits `442b67e…2faa9b3` remain the audit trail of the misdiagnosis.

### T8 — Dashboard live mode (commit `1c19507`)
- `internal/cli/v3c.go` dashboard: `--teamserver` + operator credentials dial the teamserver and stream the canonical event feed into the dashboard event store; on failure prints exactly `[OFFLINE] Teamserver unreachable — event stream is static (…)`. Default (no flags) is offline with the banner — no fabricated liveness.
- `internal/api/dashboard.go:/api/events` gains `?workspace=` filtering.
- The dashboard remains a projection of the vault/event stream — no second source of truth.

### T6 — SIGKILL crash matrix: DEFERRED
Not implemented in this sprint (see `docs/stage3-deferred.md`): building the subprocess crashtester was de-scoped after the OneDrive incident consumed the sprint's tail budget; partial coverage exists (`TestEndToEnd_StorageCrashRecovery` ungraceful-handoff + T4 terminal-state semantics + bbolt COW guarantees). The matrix remains the first CI item for Stage 4.

---

## Live verification

| Check | Result |
|---|---|
| `go build ./...` / `go vet ./...` | clean |
| `go test -count=1 ./...` | 0 failures (multiple consecutive full runs) |
| `go test -tags=integration -parallel 4` | ok |
| `TestMultiplexedCommands` (100 cmds, 1 conn) | pass |
| `TestMultiOperatorRace` (3 ops × 50 cmds) | pass, no identity leaks |
| `TestSubscribeWithCursor` (replay + live + resume) | pass |
| `TestIdentityFromCert` (expiry/SAN/hostile) | pass |
| `aether serve` without PKI | refuses with required error |
| `grep "RequireAnyClientCert" internal/` | no matches |
| `grep "queued:" internal/` | no matches |
| `cap predict` empty | `confidence 0%, UNKNOWN` (Stage 2, unchanged) |
| `-race`/lint/govulncheck | CI-enforced (not locally executable; documented) |
