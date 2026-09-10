# Aether — Deferred to Stage 3+ (from Stage 2)

Findings and scope items discovered or deliberately excluded during Stage 2. Per the sprint's non-goals, none of these were implemented.

## Deferred to Stage 3 (Teamserver v2, §40 of the forensic report)

1. **Teamserver request correlation & multiplexing** — `internal/api/protocol.go` still lacks RequestID; responses rely on connection-serial processing; `connect --exec` remains journal-only (`"logged (not executed)"`). The spine exists; wiring the teamserver through it needs the protocol v2 work first.
2. **Operator identity & capabilities over the wire** — `spine.Chips`-style capability checks are in place (`spine.Capabilities`), but the teamserver still authenticates via client-cert possession without binding cert → operator → capability set.
3. **mTLS CA hierarchy** — self-signed client certs vs `RequireAndVerifyClientCert` remain unachievable out of the box.
4. **Config-path dependency injection** — workspace locations come from process env (`AETHER_CONFIG_DIR`); tests that isolate via env cannot run `t.Parallel`. Injecting the config dir would unlock parallel integration tests.

## Deferred to Stage 3 (planner/graph, per §36 ordering)

5. **Planner determinism (F10)** — `LoadPolicy` still re-seeds RNG from wall clock; epsilon retained in generation.
6. **Graph provenance & cross-provider edges (F15)** — `Weight` still unread; correlate still vacuous on real ingests.
7. **Replay environment capture (F16)** — Action records now carry actor/approval/action-id, but version/commit/graph-hash capture is not persisted yet.

## Deferred to Stage 4

8. **Full evidence model** — provenance chaining, temporal validity (`ExpiresAt` is plumbed but unset by the spine), graph attachment, negative evidence.
9. **Plugin supply-chain signing (F6 completion)** — manifests still SHA-256-optional; installed plugins still not loaded by a runtime.
10. **Audit key escrow/rotation** — the Ed25519 seed lives in the vault meta bucket (plaintext, inside the workspace as before beside the JSONL); a hardware/external key story is Stage 4.
11. **Release engineering (F21)** — goreleaser, checksums, SBOM, provenance; committed `bin/aether.exe` still present.

## Deferred (storage follow-ups)

12. **Planner episode/policy JSONL files** — still file-based outside the vault (they are user-path CLI artifacts, not workspace state); migration touches CLI UX and Stage 3 planner determinism.
13. **True SIGKILL crash matrix** — integration tests simulate ungraceful handoff via lock-drop; bbolt's COW guarantees cover torn pages, but a subprocess kill harness (CI Linux) would make the crash matrix exhaustive.
14. **`export audit` from legacy JSONL workspaces pre-migration** — export reads from the vault; opening the workspace performs migration first, so this is a non-issue in practice, but a "read-only legacy export" tool could avoid forcing migration.

## Newly discovered during Stage 2 (P3)

15. **`orchestrator.Node.Fallback` inline-execution semantics** — unchanged (Stage 1 finding); the DAG now reaches the spine but fallback/retry accounting quirk remains (`dag.go:210-220`).
16. **bbolt lock timeout UX** — a 2s timeout surfaces as "locked by another process"; a `--force-lock` escape hatch or longer configurable timeout may be wanted for shared-operator workflows (Stage 3 multi-operator will decide).
