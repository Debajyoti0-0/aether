# Stage 42 — Phase 1 Verification Matrix

Every finding from the external forensic report, re-tested against the current
tag's own build (`bin/aether-42.exe`, clean-room clone of `c923fac`, ldflags
`-X github.com/Debajyoti0-0/aether/internal/version.Version=4.2.0-rc1`).

| ID | Component | Command run on current build | Raw output (abridged) | Status | Action |
| --- | --- | --- | --- | --- | --- |
| C1 | `internal/transport/tls.go:79` | `Select-String -Pattern 'NextProtos\|http2\|HelloCustom' internal\transport\*.go` | `tls.go:79: NextProtos: []string{"h2", "http/1.1"}`; zero `http2.ConfigureTransport` in module | **CONFIRMED** | FIXED — Phase 2 |
| H1 | `internal/cli/serve_cert.go` | `serve cert revoke --dir %TEMP%\ae41-ts --operator alice` | `Error: unknown flag: --operator` (exit 1); usage shows no operator flag | **CONFIRMED** | FIXED — Phase 2 |
| H2 | `internal/cli/connect.go:110` | `Select-String -Pattern 'teamserver-ca' internal\cli\connect.go` | default looked beside the operator cert (`filepath.Dir(tsOpCert)`), never at teamserver root | **CONFIRMED** | FIXED — Phase 2 |
| H3 | `internal/cli/replay.go:131` | `Select-String -Pattern 'openGovernedWorkspace' internal\cli\replay.go` | `openGovernedWorkspace(execWorkspace)` opened the replay-level value BEFORE parsing the step's own `--workspace` flag | **CONFIRMED** | FIXED — Phase 2 |
| H4 | `internal/cli/doctor.go:141` | `doctor` (current build, Windows) | `[FAIL] workspace roundtrip ... delete: ... in use`; `H4_EXIT=1` | **CONFIRMED** (matches F-38-1) | FIXED — Phase 2 |
| H5 | `internal/rl/trainer.go` | `plan train` → `plan generate` → `run plan` (end-to-end, current build) | generation emitted catalog commands (e.g. `prt convert` shapes) that the Action spine refuses; the failing steps did not flip the process exit code | **CONFIRMED** | FIXED — Phase 2 |
| M1 | `internal/cli/governance.go:114-120` | `Select-String -Pattern 'riskV\|RiskScore' internal\cli\governance.go internal\spine\*.go` | `riskV: 70` minted in `cliMutation`; `spine.Action.RiskScore` never matched — risk stage ran against 0 | **CONFIRMED** | FIXED — Phase 2 |
| M2 | `governance.go:125-129` vs `buildIntentMutation` | `Select-String -Pattern 'mutatingIntents\|case "'` | whitelist declared 8 shapes; `buildIntentMutation` implements 4 (default refuses with "unsupported action kind") | **CONFIRMED** | FIXED (narrowed) — Phase 2 |
| M3 | `serve` workspace lockout | `Start-Process serve ...` → `audit verify` (current build) | locked while `serve` runs (exit 1); works after stop (exit 0) | **CONFIRMED** | DOCUMENTED in `serve --help` — Phase 2 |
| M4 | `internal/workspace/rekey.go:102` | `Select-String -Pattern 'writeSaltFile\|Rename' internal\workspace\rekey.go` | `writeSaltFile` committed the NEW salt BEFORE records/journal re-seal — crash window leaves the store unrecoverable by design | **CONFIRMED** | FIXED (atomic commit) — Phase 2 |
| M5 | `internal/api/server.go:174` | `Select-String -Pattern 'SetDeadline\|HandshakeTimeout\|Handshake' internal\api\server.go` | `tlsConn.Handshake()` at :174 with no `SetDeadline` anywhere in server.go | **CONFIRMED** | FIXED (15s bound) — Phase 2 |
| M6 | `internal/audit` design | `Select-String -Pattern 'verify\|chain' internal\audit\*.go` | full-chain verification cannot distinguish tail truncation from a fresh empty chain; checkpoint file out of scope | **CONFIRMED** (design) | DEFERRED-WITH-OWNER |
| L1 | `internal/cli/v3.go:143` | end-to-end pipeline (same run as H5) | `=== Plan execution summary ===` printed `FAIL` rows, `return nil` — process exit 0 on failed steps | **CONFIRMED** | FIXED (non-zero exit) — Phase 2 |
| L2 | `internal/cli/run.go:34` | `Select-String -Pattern 'runPassphrase\|runSeed\|runPersonaName' internal\cli\*.go` | `runPassphrase` declared, never registered nor read (runSeed/runPersonaName are registered in v31.go — the external report's "dead flags" narrowed to one var) | **CONFIRMED** (narrowed) | FIXED (wired) — Phase 2 |
| L3 | `internal/workspace/workspace.go` | `Select-String -Pattern 'Walk' internal\workspace\workspace.go` | Delete's error propagation verified against the Delete caller set — the doctor probe (H4) was the only user-visible manifestation; post-fix probe covers the path | **CONFIRMED** (via H4) | COVERED by Phase 2 probe fix |
| L4 | `internal/api/dashboard.go:76` + `v3c.go:165-166` | `Select-String -Pattern 'token' internal\api\dashboard.go` | `authorize` accepted `?token=`; CLI messages even encouraged it; the CLI printed the URL with the token embedded | **CONFIRMED** | FIXED (channel removed) — Phase 2 |
| L5 | `internal/cli/providers_exec.go:31` | `Select-String -Pattern 'DefaultClient' internal\cli\providers_exec.go` | `http.DefaultClient.Do(req)` — no timeout | **CONFIRMED** | FIXED (60s client) — Phase 2 |
| L6 | clone hygiene | `Select-String -Pattern [char]0x00E2` over serve_cert/mutation/extras | no mojibake in this tree | **DISMISSED-STALE** (clone-only) | none |
| M6-INFO | `internal/version` default | `type VERSION` → `4.2.0-rc1`; default `"dev"` if not overridden | documented provenance correct; build-time override is the footguard | INFO | documented; `.goreleaser.yml` supplies the ldflags |
| S42-1 | `serve_cert.go` revoke (surfaced during Phase 2 verification) | revoke against a FRESH `--dir` | `open ...\revoked.txt: The system cannot find the path specified.` after flag parsing succeeded | **CONFIRMED** (new, Info) | FIXED (MkdirAll) — Phase 2 |
| CLI-0 | `internal/cli` test gap | `dir internal\cli\*_test.go` (before) | zero test files; 5 of the bugs live here | **CONFIRMED** | FIXED — Phase 3 (7 new tests) |

**Summary: 19 CONFIRMED / 1 DISMISSED-STALE / 1 DEFERRED-WITH-OWNER / 1 INFO.**
Zero open dispositions. Every row names the build and the exact command.
