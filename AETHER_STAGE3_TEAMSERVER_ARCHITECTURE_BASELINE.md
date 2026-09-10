# AETHER — STAGE 3 TEAMSERVER ARCHITECTURE BASELINE (Phase 0)

**Date:** 2026-09-10 · **Baseline:** `8c300b8` (Stage 2 complete, version 3.3.0-stage2)

## 1. Current protocol (`internal/api/protocol.go`)

- Framing: 4-byte BE length + JSON `Envelope{Type, Payload}`; 8 MiB cap enforced at read (`protocol.go:94-96`).
- Messages: `command_request{workspace_id, command_line, operator?}`, `command_response{ok, output, error, took_ms}`, `workspace_sub`, `workspace_update`, `error`.
- **No Version field, no RequestID, no Seq, no ActionID.** `Operator` is a client-asserted payload field (`protocol.go:31`) — spoofable.
- `GenerateClientCert` issues **self-signed** client certs (`protocol.go:121-143`); `GenerateServerCert` is ephemeral self-signed.

## 2. Connection lifecycle (`internal/api/server.go`)

- `Serve()` accepts infinitely, one goroutine per conn; no read/write deadlines, no connection cap, no graceful drain.
- `handleConn` is serial: commands execute inline on the read loop (`server.go:112`); after `workspace_sub` the connection is **locked into streaming** (`server.go:154-170`) and can never issue another command.
- Subscriber lifecycle (Stage 1 fix): single teardown owner via `done` channel; never-closed event channels; done-guarded publish — race-free.

## 3. Authentication / authorization

- TLS 1.3 with `RequireAndVerifyClientCert` (`server.go:50`) but **no `ClientCAs` pool** → verification can only succeed against system roots; self-signed client certs cannot verify. All existing tests downgrade to `RequireAnyClientCert`.
- Zero authorization: any verified client → any command → any workspace. No capability model. `PeerCertificates` are never inspected (`grep` confirms zero call sites).

## 4. Request routing / dispatch

- `CommandRunner` func injected at construction (`server.go:15`); the CLI wires it to the Stage 1/2 stub (`connect.go:134-143`): journals into the attached workspace and returns `"logged (not executed): ..."` — honest but not spine-routed.
- No dispatcher, no correlation: responses are assumed to belong to the single in-flight request; concurrent clients cross-deliver silently (`client.go:78` vs `:120-130`).

## 5. Event routing

- Per-workspace in-memory ring of 100 (`server.go:197-219`) + buffered fan-out to `subscriber.ch`. No persistence, no sequence numbers, no cursor/resume. Events are NOT the vault journal (two event systems exist: teamserver ring vs workspace journal).

## 6. Workspace binding

- `serve --workspace <name>` attaches exactly one workspace (passphrase mandatory, Stage 1); the handler journals into it. Client-supplied `WorkspaceID` is used unchecked for the event ring.

## 7. Action integration

- None: the handler does not call `spine.Run`. The spine, intent whitelist, evidence, and rollback are untouched by remote commands.

## 8. Concurrency model

- `sync.Mutex` guards events/subs/operators; per-subscriber `done` ownership; bbolt write lock protects vault state; spine serializes audit per workspace (Stage 2 fix).

## 9. Shutdown model

- `Close()` closes the listener; SIGTERM handler closes the server. In-flight commands are not drained; subscribers are not notified.

## 10. Failure modes / known gaps (driving Stage 3)

1. Protocol v1 has no correlation → multi-operator is unsafe.
2. mTLS unachievable (no ClientCAs, self-signed clients).
3. No authz/capabilities; client-asserted operator.
4. No event persistence/cursors → no reconnect resume.
5. Stub dispatch (no spine, no audit, no evidence, no rollback for remote commands).
6. No deadlines/limits/graceful drain (DoS surface).
7. Event ring vs journal duplication.
8. Dashboard consumes a startup snapshot, not the stream.

## 11. Caller/callee matrix (mutating paths touching the teamserver)

| Caller | Callee | Governance |
|---|---|---|
| `cli/connect.go runServe` handler | `workspace.LogEvent` only | none (stub) |
| `cli/connect.go apiDial` | `api.GenerateClientCert` (self-signed) | none |
| `api/server.go handleConn` | `s.Run` (stub closure) | none |
| `api/server.go handleConn` | `Publish` (ring+fanout) | n/a |

## 12. Stage 3 acceptance criteria

Per directive §41: versioned protocol + RequestID + ActionID correlation + event envelope + cursors; cert-bound identity with CA hierarchy + revocation; capability authz fail-closed; spine-routed dispatch with cert-derived actor in audit; explicit state machine with transition table; deadlines/limits/graceful drain; dashboard live mode; parallel integration tests; multi-operator race tests; hostile-protocol tests.
