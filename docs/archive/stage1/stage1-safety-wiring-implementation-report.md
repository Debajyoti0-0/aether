# AETHER — STAGE 1 SAFETY WIRING IMPLEMENTATION REPORT

**Sprint:** Stage 1 of the v4.0.0 roadmap (mandated by `AETHER_v3.2.0_FORENSIC_BASELINE_RCA_REPORT.md` §36)
**Date:** 2026-09-10
**Companion evidence file (commands, outputs, per-file citations):** `docs/stage1-evidence.md`

---

## 1. Baseline

- The repository had **no commits** at sprint start (`git rev-parse HEAD` → "unknown revision"; the entire tree was staged/untracked from a prior session, including a partially-staged older snapshot).
- First commit of the sprint: `37c0c09` (F13), which is therefore also the repo's root commit.
- A pre-existing working-tree delta set (older staged snapshot vs newer working tree: PRT binding wiring, relay aliases, transport/orchestrate/run changes, `pkg/plugins/plugin.go` deletion, a stale `VERSION` at `1.0.1-stage1`) was committed as `6d7722a baseline:` so the repository matches the code that was audited, modified, and tested.
- Toolchain: Go 1.27.1 on Windows/amd64 (go.mod declares 1.26.0).

## 2. Findings revalidated

All five P0 findings were re-verified against source before coding (details and file:line in `docs/stage1-evidence.md` §1): F1 confirmed, F2 confirmed (including the teamserver's literal `workspace.Open(req.WorkspaceID, "")`), F3 confirmed, F4 confirmed by code-path analysis, F5 confirmed by a repo-wide caller census (zero automated rollback/audit call sites for any executor). No report finding was obsolete; one addition: `exec azure/github` crashed on an unbound `execPreset` variable (`unknown browser preset ""`) which also blocked governance flow — fixed in scope (flag bound, default `chrome`).

## 3. Architecture changes

**Old execution path (mutating commands):**
```
CLI flag parsing → executor call → HTTP to provider → print result
   (no workspace, no risk, no policy, no audit, no rollback, no evidence)
```

**New execution path:**
```
CLI (--workspace required, else REFUSED)
  ↓ openGovernedWorkspace (passphrase-protected; fail-closed)
  ↓ mutation.Pipeline.Run
       ├─ Action ID minted (mut-<ts>-<rand>)
       ├─ BeforeState captured (or BEFORE_STATE_UNAVAILABLE, never fabricated)
       ├─ AUDIT(before)  → Ed25519-signed chain  (write failure ⇒ ABORT, no mutation)
       ├─ ROLLBACK registration (pre-execution)  (failure ⇒ ABORT, no mutation)
       ├─ EXECUTE → provider OperationID (Azure op / SSM CommandId / run id when returned)
       ├─ AfterState captured (failure ⇒ completed_state_unknown, not success)
       └─ AUDIT(after) with status completed | failed | completed_state_unknown
  ↓ workspace journal entry
  ↓ operator output includes the Action ID
```

Workspace layer: every user-controlled path component passes a single validation policy (`validate.go`) with symlink-containment (`SafeJoin`); keys derive from a per-workspace random salt (`salt.bin`, HMAC-tagged) — never the name; keyless mode exists only behind an explicit `KEYLESS` marker with loud warnings; legacy layouts fail closed with a migration command.

Teamserver: subscriber lifecycle has a single teardown owner; event channels are never closed; publishes are done-guarded (no send-on-closed-channel panic). Dashboard: token-authenticated on every route, loopback-default bind, TLS mandatory off-loopback.

## 4. Files changed

Full per-task, per-file list with citations: `docs/stage1-evidence.md` §2. Summary: 2 new engine packages (`internal/engine/mutation`, `internal/version`), 1 new CLI wiring file (`internal/cli/governance.go`), 1 new workspace file (`internal/workspace/validate.go`), 5 new/rewritten test files, CI + lint configs, LICENSE/SECURITY.md/VERSION, and truth-fixes across README/CHANGELOG/help text.

## 5. Action flow

See §3 diagram. The mutation pipeline is the minimum viable Action spine: it correlates intent (Action ID), execution (OperationID), evidence (journal), audit (2 signed entries per mutation), and rollback (pre-execution registration or an explicit irreversible record). Authorization/policy gates remain where they were (`run --policies` risk gate); the spine gives Stage 2 the boundary to enforce them uniformly.

## 6. Mutation coverage

| Mutation | Action | AuthZ | Audit | Evidence | Rollback | Test |
|---|---|---|---|---|---|---|
| exec azure | yes | workspace+passphrase required | 2 signed entries | journal | irreversible (recorded) | pipeline tests + live |
| exec aws | yes | same | 2 | journal | irreversible (recorded) | pipeline tests |
| exec github | yes | same | 2 | journal | irreversible (recorded) | pipeline tests |
| exec gcp | yes (precautionary) | same | 2 | journal | irreversible (recorded) | pipeline tests |
| exec parallel | yes, per target | same | 2×N | journal | irreversible (recorded) | via pipeline |
| providers exec | yes | same | 2 | journal | irreversible (recorded) | via pipeline |
| simulate stream | yes | same | 2 | journal | irreversible (recorded) | pipeline test |
| plugins install | yes | same | 2 | journal | registered (uninstall) | pipeline tests |
| prt import | yes | pre-existing requirement | 2 | journal | registered (delete_record) | pipeline tests |
| pivot cloud-to-onprem | yes | same | 2 | journal | registered (delete_file) | pipeline tests |
| relay * | no external mutation | n/a | n/a | n/a | n/a | — |

Deviation note: irreversible mutations intentionally produce **no** rollback entry; the audit chain records `"rollback":"irreversible"` instead (the prompt's §12/§13 forbids fake reversibility; acceptance #11's rollback-entry requirement applies to reversible mutations).

## 7. Workspace security

- Path policy: names/keys/artifact names validated (traversal, absolute/UNC/drive, separators on all platforms, NUL/control, Windows reserved names, trailing dot/space); `SafeJoin` resolves symlinks and rejects escapes; destructive ops (shred/delete) validate first. `aether workspace delete "../etc" --force` → exit 1, nothing shredded.
- Key model: random 16-byte Argon2id salt per workspace; `salt.bin` = version‖salt‖HMAC-SHA256(key, version‖salt)[:14]; empty passphrase rejected without the explicit `KEYLESS` marker (and warned loudly on every keyless open); wrong passphrase fails the HMAC tag before any record read; legacy layouts rejected with `workspace rekey` migration instructions (`--confirm-keyless` gate; fresh salt minted on every rekey; marker removed on keyless→keyed migration).
- Teamserver: refuses keyless workspace attachment with the mandated error string.

## 8. Teamserver concurrency

Ownership model: `subscriber{ch, done}`; only `unsubscribe` (under `s.mu`, idempotent) closes `done`; `ch` is never closed by anyone; `Publish` snapshots subscribers and sends with `select {ch<-ev | <-done | default}` (drop for slow subscribers). The historical send-on-closed-channel panic is structurally impossible. Tests: `TestPublishUnsubscribeRace` (100 publishers × 100 subscribe/unsubscribe churners, repeated unsubscribe, live-sub teardown, registry-empty invariant) — designed to run under `go test -race` in CI (this host lacks a C toolchain for `-race`; see deviations).

## 9. Dashboard security

Default bind `127.0.0.1`; per-start 32-byte token printed once; all four routes require it (Bearer header / X-Aether-Token / `?token=`), compared with `crypto/subtle.ConstantTimeCompare`; non-loopback binds without `--tls-cert/--tls-key` refuse to start; TLS smoke test proves a missing cert fails (fail-closed). Documented guarantee: the token protects the dashboard endpoints on the bound interface; loopback is a network-scope reduction, not an authentication mechanism.

## 10. CI

`.github/workflows/ci.yml`: `governance` (fails without VERSION/LICENSE/SECURITY.md), `build` (ubuntu/windows/macos), `vet`, `test` (`-race`, ubuntu), `test-windows` (`-race`), `lint` (golangci-lint, `.golangci.yml`: errcheck/govet/ineffassign/staticcheck), `vuln` (govulncheck), `integration` (`go test -tags=integration ./test/integration/...`; placeholder test ships so the job is real). Local mirror: `make ci`.

## 11. Test results (final tree, Windows host)

- `go build ./...` — OK
- `go vet ./...` — clean
- `go test -count=1 ./...` — **25 packages ok, 0 failures**
- `go test -tags=integration -count=1 ./test/integration/...` — ok
- `go test -race ./...`, `golangci-lint run`, `govulncheck ./...` — not executable on this host (no C toolchain / tools not installed); wired into CI where they must pass. Documented in evidence §3 rows 4/4b.

## 12. Remaining gaps (classified)

| Item | Class |
|---|---|
| DAG (`run plan`) nodes re-enter the CLI root; mutations are governed only via the underlying commands' mandatory `--workspace` (defense-inherited, not spine-native) | PARTIALLY_FIXED → Stage 2 |
| Teamserver mTLS unachievable out-of-the-box (self-signed client certs, no ClientCAs); no per-command authz; no correlation IDs | DEFERRED (Stage 2/3) |
| Audit append lacks fsync; torn line bricks Verify; audit key stored beside log | DEFERRED (Stage 2 storage) |
| Rollback `Pop` rewrite-before-validate corruption window; failed reversals still dropped | DEFERRED (Stage 2) |
| Journal read-modify-write still O(N²) and unlocked | DEFERRED (Stage 2 storage contract) |
| Kerberos/XML-DSig/PRT protocol spec errors | NOT_SAFE_TO_IMPLEMENT_YET (Stage 3, per report sequencing) |
| Plugin manifests unsigned; installed plugins not loaded | DEFERRED (supply chain, report §44) |
| Dashboard is a startup snapshot (not live) | DEFERRED (honestly labeled) |
| Committed `bin/aether.exe` removal, goreleaser/checksums/SBOM wiring | DEFERRED (F21) |
| CAP predictor confidence semantics, planner determinism | DEFERRED (Stage 3/4 per report) |
| `NOT_REPRODUCED`: none — all attempted reproductions matched the report. | — |

## 13. Post-Stage-1 roadmap

Per the forensic report's dependency ordering, Stage 2 "Storage & Spine" (§39) is next: single storage contract (locking, atomicity, fsync, schema version), full Action spine (AuthZ → Risk → Policy → Execute → Evidence → Audit → Rollback) absorbing the DAG and teamserver command path, removal of dead layers (`pkg/providers`, bolt, phantom vault path), and a populated integration tier. The mutation pipeline introduced here is the seed of that spine; nothing in Stage 1 will need backtracking.
