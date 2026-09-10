# Aether Stage 3 — Threat Model (Teamserver V2)

Scope: the distributed control plane after Stage 3 — mTLS teamserver, cert-bound identity, capability authz, protocol v2, spine-routed remote execution. Format per directive §38: asset → threat → control → test → residual risk.

## Actors

| Actor | Definition |
|---|---|
| Unauthenticated network client | Any host that can reach the listen port without a CA-issued certificate |
| Authenticated low-privilege operator | Holds a valid cert; default caps are read-only |
| Compromised operator credential | Attacker with alice.key on disk |
| Malicious operator | Authenticated insider attempting escalation/recon |
| Replayed client | Captured traffic replayed against the server |
| Network attacker | On-path actor (no keys) |
| Compromised workspace | Malicious workspace name/content on the server filesystem |

## Threats

| Threat | Asset | Control | Test | Residual risk |
|---|---|---|---|---|
| Identity spoofing (client-asserted operator) | Operator identity, audit attribution | Operator field removed from the wire; identity derived from cert URI SAN only (`api/identity.go:FromClientCert`); actor stamped `operator:<name>` | `TestTeamserverCommandRoundTrip`, `TestMultiOperatorRace` (no identity leaks) | Key theft = identity theft (see compromised credential) |
| Unauthenticated access | Command surface | TLS 1.3 + `RequireAndVerifyClientCert` + ClientCAs; handshake before any protocol; unauthenticated peers get no protocol at all | `TestTeamserverCommandRoundTrip` (cert-bound PKI); refusal without PKI files (serve refuse-to-start) | Network reachability of the port itself; bind-address discipline is operator duty |
| Expired/revoked identity | Session validity | `FromClientCert` NotBefore/NotAfter checks; `revoked.txt` checked per connection | `TestIdentityFromCert` (expired/not-yet-valid), `TestRevokedOperatorRefused` | Revocation is file-based (no OCSP/CRL distribution — Stage 4); revocation takes effect for new connections only |
| Capability escalation | Execution surface | Capability files server-side (`api/capabilities.go`); fail-closed `Has`; spine AuthZ double-checks `RequiredCaps` | `TestOperatorCapabilities`; spine `stageAuthz` | File-level admin compromise = full grant power (documented trust anchor) |
| Workspace escape | Other workspaces' vaults | Teamserver attaches exactly one workspace (passphrase mandatory, Stage 1); commands execute against that workspace only; path validator blocks traversal; per-workspace flock | `TestVaultLockExclusive`, F1 traversal suite | A malicious operator with execute caps can still mutate the ATTACHED workspace — that is the delegated authority |
| Response confusion (multi-operator) | Response/event isolation | Protocol v2 RequestID correlation end-to-end; per-conn write mutex; client demultiplexer by rid | `TestMultiplexedCommands`, `TestMultiOperatorRace` | None known beyond Go runtime guarantees (race suite in CI) |
| Event injection/forgery | Event stream | Events written only by the server (`Publish` → `EventStore.Append`); clients cannot inject; seq minted transactionally per workspace | `TestEventStoreSequencing`, `TestSubscribeWithCursor` | Event payloads are not signed per-event (the audit chain is the tamper-evident record; events are transport-level) |
| Event replay | Consumers | Cursor model: consumers request `> lastSeq`; replay is deterministic from the store; gaps detected and refused | `TestSubscribeWithCursor` | Replay of OLD events to a fresh subscriber is by design (history), not an attack surface |
| Request replay / duplicate execution | Provider state | RequestID correlates but does not deduplicate: retried commands CAN execute twice (documented semantics — the spine Action identity + audit trail make duplicates visible) | — | Idempotency keys are Stage 4 (recorded in deferred); operators must not blindly retry mutating commands |
| Audit tampering | Audit chain | Ed25519-signed hash chain in the vault (Stage 1, frozen); `audit verify` | `store/audit_test.go` suite | Local-key trust model unchanged (Stage 4 escrow/rotation) |
| Downgrade | Protocol/TLS | `ProtocolVersion=2` only — `Envelope.Validate` rejects others; TLS 1.3 MinVersion; no cipher flexibility | `TestFrameProtocolValidation` | `--insecure` (client-side server-verification skip) remains as a gated research hatch (`--i-know-what-im-doing` + warning) |
| DoS | Availability | Frame size cap (8 MiB), per-conn in-flight cap (8), max connections (32, configurable), per-conn write deadlines via bounded writes, read loop never blocks on slow consumers (drop policy) | `TestFrameProtocolValidation`, `TestMultiplexedCommands` | No per-IP rate limiting yet (deferred); idle connections are not reaped until the listener closes (deferred) |
| Malicious workspace | Server filesystem | Stage 1 path validation + symlink containment + shred-containment; vault schema versioning | F1 test suite | Unchanged from Stage 1/2 |
| Cross-workspace leakage (events) | Event stream | Subscriptions are workspace-scoped; fan-out matches by workspace; vault is per-workspace | `TestTeamserverWorkspaceStream`, `TestPublishFansOutToAllSubscribers` | Event payloads contain no secrets (operator redaction discipline); graph/event content is engagement-sensitive — dashboard auth (Stage 1 token) governs that surface |

## Explicit non-guarantees (truthfulness)

- The teamserver does not claim protection against a **compromised teamserver admin** (filesystem = full authority over caps, certs, revocation).
- Revocation is connection-time, not mid-session.
- Request idempotency is NOT provided — retried mutating commands may double-execute (audit-visible).
- Event payloads are authenticated by transport only; the signed audit chain is the integrity anchor.
- `-race`/lint/govulncheck verification is CI-enforced, not locally executed (no C toolchain on the dev host).
