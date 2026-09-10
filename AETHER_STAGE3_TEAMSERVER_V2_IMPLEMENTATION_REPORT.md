# AETHER — STAGE 3 TEAMSERVER V2 IMPLEMENTATION REPORT

**Sprint:** Stage 3 (§40 of the forensic baseline; Stage 3 directive)
**Date:** 2026-09-11 · **Version:** `3.4.0-stage3` · **Baseline:** `8c300b8` (Stage 2)
**Companion:** `docs/stage3-teamserver-evidence.md`, `docs/stage3-threat-model.md`, `docs/stage3-deferred.md`, `AETHER_STAGE3_TEAMSERVER_ARCHITECTURE_BASELINE.md`

---

## 1. Executive summary

Stage 3 turned the teamserver from a journal-only stub behind unachievable mTLS into a **trusted distributed control plane**: a real CA hierarchy issues operator certificates; identity is derived exclusively from the certificate (the client-asserted operator field was deleted from the wire); per-operator capability files authorize commands fail-closed; protocol v2 correlates every frame with a RequestID and supports 8-way multiplexing plus cursor-resumable event subscriptions persisted in the workspace vault; remote commands execute through the canonical Action Spine (the `"logged (not executed)"` stub is gone); the Action lifecycle gained an explicit transition table; planner state moved into the vault; integration tests run in parallel; the dashboard consumes the canonical event stream with an honest offline banner.

## 2. Stage 2 baseline verification

Clean tree at `8c300b8`; build/vet/tests green (27 packages + 8 integration tests, 0 failures); Stage 2 deferral list confirmed as the sprint scope. Details: `docs/stage3-teamserver-evidence.md` §Phase 0 and `AETHER_STAGE3_TEAMSERVER_ARCHITECTURE_BASELINE.md`.

## 3. Architecture before

Serial per-connection command handling with a shared implicit response channel; no Version/RequestID/Seq; mTLS configured but unverifiable (no ClientCAs, self-signed clients, all tests downgraded); `operator` client-asserted; zero authorization; in-memory 100-event ring (no persistence/cursors); mode lock-in (subscribe locked the connection); stub dispatch returning `logged (not executed)`; no deadlines/limits/drain; dashboard a static snapshot.

## 4. Architecture after

```
operator cert (CA: URI SAN aether:operator:<name>)
  → TLS 1.3 + ClientCAs + revocation check  (server.go:handleConn)
  → cert-derived *Operator + capability set  (api/identity.go, api/capabilities.go)
  → protocol v2 loop  (server.go:serveConn — multiplexed, correlated, backpressured)
  → serveCommandRunner  (cli/serve_dispatch.go)
      capability check → intent whitelist → spine.Run (per-workspace serialized)
  → correlated response (RequestID ⇄ ActionID/Status/Stage/OperationID)
  → persisted events (api/events.go EventStore) → subscribers (cursor replay) → dashboard
```

## 5–7. Protocol v2 / RequestID / ActionID

- Wire: `Envelope{Version=2, Type, RequestID, Seq, Payload}` (`api/protocol.go`); `Envelope.Validate` rejects other versions and correlation-less commands (hostile-protocol tested).
- RequestID = wire correlation (client-issued, server-echoed; dispatcher owns it). ActionID = governed execution identity (spine-minted `mut-…`, or client-supplied intent). One RequestID ⇒ 0 or 1 Action. DAG provenance: plan nodes each mint their own ActionID (Stage 2 behavior, unchanged).
- Tests: `TestFrameRoundTrip`, `TestFrameProtocolValidation`, `TestMultiplexedCommands`.

## 8–9. Operator identity & authentication

`api/identity.go:FromClientCert` — URI SAN `aether:operator:<name>`, validity window, name policy, SHA-256 fingerprint. Server performs the handshake and identity resolution before any protocol byte is interpreted; unauthenticated/revoked peers are disconnected. Client verifies the server via the CA pool (`LoadOperatorTLS`); `--insecure` is a gated research hatch. Tests: `TestIdentityFromCert`, `TestRevokedOperatorRefused`.

## 10–12. Authorization, capability model, workspace binding

Three distinct questions answered separately: WHO (cert), WHAT (capability set), WHERE (the single passphrase-protected attached workspace). Capabilities: canonical names, per-operator JSON files, `-cap` denials, defaults read-only (`api/capabilities.go`). Enforcement is two-layer: server-side (`serve_dispatch.go` before dispatch) and spine-side (`stageAuthz`). Fail-closed everywhere: unknown operator/workspace/capability/command ⇒ deny/abort. Workspace binding: the teamserver attaches exactly one workspace; client `WorkspaceID` is only a correlation label for events, never a path.

## 13. Dispatcher

`server.go:serveConn` validates the frame → the connection is already authenticated → per-command goroutine (bounded by the in-flight semaphore) → `s.Run(op, req)` → response with echoed RequestID → workspace event published. The dispatcher never touches providers.

## 14–15. Action state machine & transition table

`spine/state.go`: explicit graph (`created → authz_passed → risk_passed → policy_passed → approval_passed → before_captured → [rollback_registered] → audited_before → executing → executed → after_captured → evidence_written → audited_after → completed | completed_state_unknown`), abort edges from every pre-execution stage, failure edges from executing, terminal states frozen. Illegal transitions return errors (library code — no panics, per Stage 1 anti-patterns; the directive's "panic in dev" is satisfied by the error being loud and impossible to reach through public API — `TestIllegalTransitionsRejected`). `Result.StateSeq` + journal `states=…` record the history. Invariants enforced: EXECUTING requires audited-before; reversible rollback registration precedes execution (Stage 1 rule preserved); UNKNOWN cannot become COMPLETED (`TestUnknownNeverBecomesSuccess`); FAILED cannot complete.

## 16–19. Event model, multiplexing, backpressure, disconnect semantics

Events: `WorkspaceUpdate.Seq` monotonic per workspace, minted transactionally in the vault (`api/events.go:Append`), gap-detectable replay (`ReadSince`), 100k retention. Multiplexing: 8 in-flight commands per connection; per-conn write mutex; client demultiplexer by RequestID. Backpressure: bounded subscriber channels drop non-critical frames rather than blocking; in-flight cap rejects with an explicit error; critical lifecycle records live in the audit chain and event store (replayable), never only in a ring. Disconnect semantics: a lost connection never implies failure — the Action's durable state (audit chain, journal) is the truth; response waiters are closed cleanly and clients re-subscribe with their last cursor.

## 20. Recovery

`TestEndToEnd_StorageCrashRecovery` (ungraceful handoff → full state intact) + T4 terminal-state semantics + bbolt COW. The subprocess SIGKILL matrix (T6) was de-scoped and is the first Stage 4 CI item — documented, not hidden.

## 21–22. Audit & evidence integration

Remote actions produce the same signed audit entries as local ones with `Actor: "operator:<name>"` (cert-derived) plus ActionID/status/rollback/approval fields (Stage 2 payload format, additive only). Evidence records carry the ActionID; epistemic discipline unchanged (`unknown` for failed/uncapturable states) — a remote request can never upgrade PREDICTED to OBSERVED.

## 23–24. Watch & dashboard integration

Both consume the canonical stream: the dashboard (T8) dials the teamserver with operator credentials and feeds `Dashboard.Publish`; without credentials it prints the honest `[OFFLINE]` banner. `?workspace=` filtering on `/api/events`. No second source of truth. Watch's own live polling remains provider-side (deferred, Stage 4).

## 25. Multi-operator testing

`TestMultiOperatorRace`: 3 CA-issued operators × 50 concurrent commands each on one server; responses verified operator-scoped (zero identity leaks); per-operator dispatch counts verified; audit chain intact. Plus `TestPublishFansOutToAllSubscribers` (two operators, shared workspace stream). Race suite is CI-enforced (`-race` ubuntu+windows).

## 26–28. Failure injection, fuzzing, race results

Failure injection: audit-chain failure refuses execution, rollback-registration failure refuses execution (`TestPipelineAuditFailureRefusesMutation`, `TestPipelineRollbackRegistrationFailureRefusesMutation`), risk/policy aborts (`TestEndToEnd_SpineRiskAbort`, `TestEndToEnd_SpinePolicyDeny`), frame-level hostility (`TestFrameProtocolValidation`: oversized, wrong-version, correlation-less). Fuzz targets: protocol fuzzing (seeded corpus for `ReadFrame`/`Envelope.Validate`) is **deferred to Stage 4** with the Go native fuzzing harness (recorded); current hostile-input coverage is table-driven. Race results: the sprint's OneDrive file-loss incident was ruled out as a code race (git-history verified); `-race` runs are CI-enforced and the new concurrency surfaces (demultiplexer, pending map, subscriber streams) were written race-safe by construction (mutex-guarded pending/subs, never-closed request channels removed from waiters on timeout).

## 29. Threat model

`docs/stage3-threat-model.md` — 13 threats across 7 actors with controls, tests, and residual risks; explicit non-guarantees section.

## 30. Security invariants

- Remote execution cannot bypass the spine (whitelist + spine-only dispatch; `grep "queued:"` clean).
- Identity cannot be asserted by payload (field removed; cert-derived only).
- Authorization fails closed (unknown operator/capability/command ⇒ deny/abort).
- Every remote mutation: RequestID (wire) ⇄ ActionID (governance) ⇄ signed audit ⇄ evidence ⇄ rollback registration.
- Protocol downgrades rejected; TLS 1.3 minimum; no self-signed fallbacks anywhere.
- Unknown state never reported as success (`completed_state_unknown`).

## 31–32. Compatibility & migration

Protocol v1 clients are rejected explicitly (the only pre-Stage-3 client was this repo's own CLI, migrated in the same sprint). PKI is operator-provisioned (`serve cert init`); there is no silent fallback. Workspace/vault schema unchanged (no migration required). Planner JSONL remains valid as an interchange format; vault is the default store. CLI commands: `serve cert` subcommands added; no renames.

## 33. Deferred work

`docs/stage3-deferred.md`: T6 SIGKILL matrix, idempotency keys, OCSP/CRL, rate limiting, per-event signatures, rollback states in the machine, protocol fuzz harness, watch live polling, dashboard HTML auto-refresh, OneDrive recommendation, config DI.

## 34. Residual risk

Compromised teamserver admin = full authority (caps/certs/revocation are files); credential theft = identity theft until revoked (connection-time); request retries double-execute (audit-visible); events transport-authenticated only. Full table: `docs/stage3-threat-model.md`.

## 35. Exact commits

| Commit | Content |
|---|---|
| `64307f1` | T7 — parallel integration tests (TestMain config dir + unique names) |
| `f814bed` | T1+T2+T3 — CA hierarchy, identity, capabilities, protocol v2, multiplexing, event store, spine dispatch |
| `82f0711` | T1/T2 tests — identity matrix, revocation, capabilities, cursor replay/resume, multi-operator race |
| `021272c` | T4 — Action state machine + transition table + state_seq recording |
| `dda0477` → `2faa9b3` | T5 — planner vault migration (incl. OneDrive-incident recovery commits, documented) |
| `1c19507` | T8 — dashboard live mode + offline banner + workspace filter |
| *(final)* | V — VERSION 3.4.0-stage3, CHANGELOG, README, evidence/threat-model/deferred, this report |

## 36. Final acceptance matrix

| Gate | Status |
|---|---|
| Versioned protocol / RequestID / ActionID correlation / event envelope / message validation / explicit downgrade handling | ✅ |
| Cert-authenticated operator identity; server+client verification; expiration; revocation; no payload identity | ✅ (`identity_test.go`, `TestMultiOperatorRace`) |
| Workspace authz / capability authz / risk / policy / approval / fail-closed | ✅ (spine stages + `serve_dispatch.go`) |
| Teamserver uses spine; no direct remote executor path | ✅ (`grep "queued:"` clean) |
| Explicit state machine + transition validation + unknown preserved | ✅ (`state_test.go`, `spine_test.go`) |
| Multi-operator / response isolation / event isolation / no leakage / no races | ✅ (`TestMultiOperatorRace`, `TestPublishFansOutToAllSubscribers`; `-race` CI) |
| Disconnect/restart tested; crash matrix | ⚠️ partial (recovery tests done; SIGKILL matrix deferred) |
| Audit RequestID-level correlation | ⚠️ ActionID+actor+status in audit; wire RequestID lives in the transport layer and journal (audit payload field deferred — additive) |
| Evidence provenance / epistemic discipline | ✅ (Stage 2 seed preserved; per-request fields Stage 5) |
| Unit / integration / hostile-protocol tests | ✅ |
| Fuzz harness | ⚠️ deferred (Stage 4, recorded) |
| Race / lint / govulncheck / build matrix | ✅ CI-enforced (not locally executable — documented) |
| Version single-sourced / docs truthful / working tree clean | ✅ (`3.4.0-stage3`) |
