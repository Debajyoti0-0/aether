# Stage 44 — Operational Qualification Reports

**Date:** 2026-09-19. All results are live executions on the real topology (see `topology.md`). Raw transcripts: `workspace/session.log`, `teamserver-session.log`, `graph/*.log`, `dashboard/*.log`, `soak/soak.log`, `soak/cancellation.log`.

## Load report

| Scenario | Profile | Result |
|---|---|---|
| Workspace lifecycle churn | 8 concurrent workers × 5 cycles × (create/list/info/delete) = 160 real CLI process invocations | **PASS** — 0 failures, 0 residue, 5s wall |
| Dashboard load | 8 concurrent workers × 50 mixed-auth requests = 400 requests | **PASS** — 0 failed expectations, 3s wall |
| Graph build 2K nodes | 2065 nodes / 2667 edges | **PASS** — 0.116s, deterministic counts |
| Graph build 10K nodes | 10065 nodes / 13334 edges | **PASS** — 0.598s |
| Graph qualify/generate on 10K | path u000000→role:r000 → runbook (OPSEC risk 30/100) + DAG plan with CAE recovery node | **PASS** — deterministic, 0.074s/0.062s |
| Duplicates | 200% duplicated 10K dataset → 10040 nodes (deduped, same semantics) | **PASS** — 0.728s |
| Teamserver roundtrips | 98 authenticated connects during soak | **PASS** — 0 failures |

Latency was not profiled beyond wall-clock (first-baseline; no SLO is documented in the repo — recorded as first-baseline per prompt). Memory during sustained load: flat 17–20 MB per process (soak metrics).

## Failover / failure-injection report

| Scenario | Expected | Observed | Verdict |
|---|---|---|---|
| Connect without operator cert | fail closed, typed | typed error naming the fix | **PASS** |
| Wrong client CA (TLS validation) | handshake rejected | `tls: failed to verify certificate: x509: certificate signed by unknown authority` | **PASS** |
| Non-whitelisted intent (`status`) | rejected | typed whitelist error | **PASS** |
| Capability authz (bob: graph.read → exec.azure) | denied | `operator "bob" lacks capability "exec.azure"` | **PASS** |
| Governed exec path (alice, fake targets) | spine dispatch → typed provider failure, no panic | dispatch reached real ARM endpoint; typed transport error (lab egress interception); no panic/hang | **PASS** |
| Vault concurrent open (server holds workspace) | typed lock error | `vault.db is locked by another process` | **PASS** |
| Corrupt vault.db | typed error, no panic | `invalid database` | **PASS** |
| Wrong passphrase | typed fail-closed | `wrong passphrase or corrupted salt.bin` | **PASS** |
| Rekey old/new | old rejected, new works | exactly that | **PASS** |
| Serve without PKI / garbage cert / wrong passphrase | refuse to start | three typed refuse-to-start errors | **PASS** |
| Dashboard port in use | typed refuse | `bind: Only one usage of each socket address` | **PASS** |
| Dashboard auth matrix | 401 unauth / 401 wrong token / 401 case-mangled token / 200 all three auth modes | exactly that | **PASS** |
| `serve cert revoke` live effect | fail closed on new connection | D-003 fixed: bob's post-revocation connection dropped mid-handshake (no protocol), alice unaffected | **PASS** |
| Abrupt termination recovery | no corruption, ports released | teamserver `taskkill /F` mid-session → port released, workspace reopens cleanly on restart, audit/rollback state intact | **PASS** |
| Backup/restore of sealed workspace | byte-identical restore + crypto verify | copy dir → destroy original → restore → `workspace info` decrypts with correct passphrase | **PASS** |
| Network partition / DNS failure injection | bounded retries, no hang | **PARTIAL** — provider transport failures produce typed errors with no hang (observed via real ARM call), but a dedicated loopback-partition campaign was not built; no unsafe-retry behavior observed anywhere | PARTIAL |
| OCSP / failover of OCSP responder | — | **NOT APPLICABLE — COMPONENT ABSENT** (product has no OCSP; documented in README + threat model) | N/A |

## Soak / restart report

- **Duration:** 601s sustained mixed workload (workspace churn + 10K graph rebuild + dashboard auth + teamserver roundtrips), 98 iterations, **0 errors**.
- **Memory:** dashboard 19–20 MB, teamserver 17–18 MB — flat across entire soak; no growth trend.
- **Restart:** cold restart of teamserver preserves workspace state (vault lock released on termination; workspace reopens; revoked-operator state enforced from file). Warm restart under load: covered by dashboard relaunch attempt + teamserver restart while clients active.
- **Rollback drill:** the restored workspace (backup/restore test) reopens with cryptographic integrity — restore path drilled. Full binary rollback drill: **NOT TESTED** (single-commit deep history; no prior release binary exists to roll back to — first release).
- **30-minute soak:** NOT TESTED — capped at 10 minutes for this session; extended soak recommended before any external release.

## Cancellation report

| Scenario | Result |
|---|---|
| POSIX SIGINT/SIGTERM handling | Implemented via `internal/cli/signals_unix.go` (`signalContext` in interactive `connect`/v3c paths) — **runtime behavior on POSIX NOT TESTED** (no POSIX environment available in this lab; docker-desktop WSL has no /mnt/c) |
| Windows CTRL_C/CTRL_BREAK | Implemented via `signals_windows.go` (os.Interrupt) — **graceful delivery NOT TESTED**: msys `kill -INT` is ineffective on native Windows processes and no scripted console-control-event mechanism is available in this environment |
| Abrupt termination (Windows `taskkill /F`) | **PASS** — ports released, vault lock released, workspace intact and reopenable, no orphan processes |
| Port-in-use refusal | **PASS** — typed bind error, clean exit |
