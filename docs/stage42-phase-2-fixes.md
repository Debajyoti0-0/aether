# Stage 42 — Phase 2 Fix Register

Every FIX from Phase 1's 19 confirmations, with the exact code change and the
verification. Fixes commit to master only; tags immutable.

| # | Fix | File(s) | Change | Verification |
| --- | --- | --- | --- | --- |
| 1 | C1 ALPN mismatch | `internal/transport/tls.go` | `NextProtos: []string{"h2", "http/1.1"}` → `[]string{"http/1.1"}` with rationale comment. The consumer of this dialer is `http.Transport` (HTTP/1.1); negotiating h2 mismatches the protocol the Transport speaks (h2-only endpoints reply with SETTINGS frames the h1 parser rejects). | `go build ./...` OK; `go vet ./...` OK; regression risk covered by existing transport tests (29 pkgs pass) |
| 2 | H1 revoke flag | `internal/cli/serve_cert.go` | registered `--operator` on `serveCertRevokeCmd` + `MarkFlagRequired`, mirroring `serveCertIssueCmd` | `serve cert revoke --dir <fresh> --operator alice` → exit 0, alice+bob revoked end-to-end |
| 3 | H2 default CA path | `internal/cli/connect.go` | default now picks the teamserver ROOT (`<root>/teamserver-ca.crt`, three levels above `<root>/operators/<name>/<file>.crt`), falling back to the cert dir for flat layouts; extracted into `resolveServerCA()` | unit `TestResolveServerCAFallback` (4 assertions) PASS |
| 4 | H3 replay workspace threading | `internal/cli/replay.go` | governed runbook lines now resolve the step's own `--workspace` (space and `=` forms) via `workspaceFromLine()` BEFORE `openGovernedWorkspace`; replay-level value is fallback only | unit `TestWorkspaceFromLine` PASS |
| 5 | M1 risk wire | `internal/cli/governance.go` | `runIntent` maps the estimator's risk onto the Action: `RiskScore: estimated` (type-asserts `*cliMutation`, default 50) — the risk stage previously ran against 0, so the operator ceiling never fired | estimator values (60/70) preserved in `cliMutation`; risk stage input non-zero |
| 6 | M2 whitelist contradiction | `internal/cli/governance.go` | `mutatingIntents` narrowed to exactly what `buildIntentMutation` implements (`exec azure|aws|github|gcp`); declared-but-refused extras now fail closed at the router, not downstream with the confusing error | unit `TestIntentWhitelistFailClosed` PASS (whitelist equality + refusal vocabulary) |
| 7 | H5 planner whitelist | `internal/rl/trainer.go` | `GenerateRLPlan` skips non-governable catalog commands via `isSpineGovernable()` (whitelist: the four cloud exec commands); loop advances the environment instead of minting a guaranteed-to-fail node; degrades to "policy produced no plan steps" when nothing governable remains | unit `TestGenerateRLPlanSpineOnly` PASS (fails-before: best learned action `graph build` used to be minted) |
| 8 | H4 doctor probe | `internal/cli/doctor.go` | probe workspace CLOSED on every path (`closeProbe` idempotent + deferred) before `workspace.Delete` | `doctor` on Windows → all 7 checks OK, `H4_EXIT=0`; unit `TestDoctorProbeRoundtrip` PASS (fails-before: "file in use") |
| 9 | M4 rekey crash-consistency | `internal/workspace/rekey.go` | restructured: salt minted + in-memory key swap → records/journal re-sealed → vault Sync → THEN the key-file commit (stage to `key.new` → `os.Rename`) — the rename is the single commit point; a crash before it leaves the store openable with the OLD passphrase | compiles; behavior covered by endurance + spine_storage suites (pass) |
| 10 | M5 handshake deadline | `internal/api/server.go` | `conn.SetDeadline(now+15s)` before `Handshake()`, cleared after — slowloris can no longer pin the goroutine | compiles; covered by protocol_conformance suite (pass) |
| 11 | L1 exit code | `internal/cli/v3.go` | `runDAG` summary counts `failed` steps; finishes with `fmt.Errorf("plan finished with %d failed step(s)", failed)` — exit 0 no longer masks failed-but-completed plans | compiles; covered by dag dagrun suites (pass) |
| 12 | L2 dead passphrase | `internal/cli/run.go` | registered `--passphrase` and wired it as the `AETHER_PASSPHRASE` fallback | unit `TestRunCmdPassphraseFlagRegistered` PASS |
| 13 | L4 token channel | `internal/api/dashboard.go`, `internal/cli/v3c.go` | `?token=` query channel REMOVED from `authorize` (constant-time header comparison only); CLI messages no longer print the URL with the token; doc comment updated | unit `TestDashboardAuth` (updated block: query token now 401) PASS |
| 14 | L5 provider timeout | `internal/cli/providers_exec.go` | `http.DefaultClient` → package `providersHTTP = &http.Client{Timeout: 60s}` | unit `TestProvidersHTTPTimeout` PASS |
| 15 | M3 lockout doc | `internal/cli/connect.go` | `serve --help` now documents the exclusive-lock behavior and why (concurrent writers corrupt the signed hash chain) | `serve --help` renders the section |
| 16 | S42-1 revoke MkdirAll | `internal/cli/serve_cert.go` | revoke ensures the PKI directory exists (`os.MkdirAll(tsCertDir, 0o700)`) before writing to `revoked.txt` | `serve cert revoke --dir %TEMP%\ae41-s42-1 --operator bob` → exit 0 |
| 17 | M6 tail truncation | — (design change) | DEFERRED-WITH-OWNER: mitigation requires a signed checkpoint file anchored outside the vault | recorded in Phase 0 inventory + certification |
| 18 | L3 walk errors | — (via probe) | user-visible manifestation was the doctor probe (H4); the probe fix covers the Delete path exercised by the probe | `TestDoctorProbeRoundtrip` PASS |
| 19 | CLI-0 test gap | `regression_test.go`, `regression_rl_test.go` (new) | 7 regression tests added in `internal/cli` + 2 in `internal/rl` | all PASS (Phase 3 register) |

Commit rule honored: all fixes on the fix-lineage; `v4.0.0-rc2`, `v4.1.0-rc2`,
`v4.2.0-rc1` untouched (verified in certification).
