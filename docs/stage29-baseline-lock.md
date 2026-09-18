# Stage 29 — Baseline Lock, Blocker Reconciliation, and Race-Gate Closure

Stage 29 opens the Production-Limited observation window for `4.1.0-rc2`,
attempts closure of the one `BLOCKED` quality gate (race), and reconciles
every Stage 28 limitation. Decision and handoff: `docs/stage29-decision.md`.
Observation record: `docs/stage29-observation-log.md`.

## 1. Immutable RC freeze (2026-09-17T16:55:20Z)

| Item | Value | Verified |
| --- | --- | --- |
| Tag object `v4.1.0-rc2` | `e8416b5849c26b37a53493782aacfc9bb41f236c` | local == remote (`git ls-remote`) |
| Tagged commit | `bc674af1ab9ac49c8ec154fda103e9e558457301` | unchanged |
| Tag `VERSION` | `4.1.0-rc2` | `git show v4.1.0-rc2:VERSION` |
| Working tree (`C:\dev\aether`, master @ `56b4b4c`) | clean (0 entries) | `git status --porcelain` |
| Baseline `v4.0.0-rc2` | `5cd008be1e3e20b039214a911626a6a4426c7838` | unchanged |
| Withdrawn `v4.1.0` | absent (local + remote) | not recreated |
| Clean room | `/c/dev/clean-room-aether` @ `bc674af1` (from Stage 28; `git status` clean) | reused for all Stage 29 execution |
| Tagged binary | `bin/aether-rc2` built fresh from the tag: `aether version 4.1.0-rc2` | G463 |

Scoping note: Stage 28 already executed build/vet/unit/integration/fuzz
(21 targets)/lint/govulncheck against this same immutable tag; Stage 29
does not re-run those (per charter). Stage 29 adds: the race suite
(previously impossible), a top-level + second-level CLI/argument matrix,
and an exit-code contract regression on a fresh tagged binary.

## 2. Stage 28 blocker reconciliation

| Item | Stage 28 state | Stage 29 action | Actual result | Status |
| --- | --- | --- | --- | --- |
| Race suite | BLOCKED (no C toolchain, Docker down) | Closure attempts A/B/C (§3) | **Suite executed and green** | **CLOSED** — L-28-1 resolved |
| Live Entra | NOT VERIFIED | No authorized live access available | no access | NOT VERIFIED (unchanged, not downgraded) |
| Live IMDS | NOT VERIFIED | No authorized live environment | no access | NOT VERIFIED (unchanged) |
| Live Okta | MOCK ONLY | No live tenant | mocks only | MOCK SUCCESS-PATH VERIFIED (unchanged) |
| Live GitLab | MOCK ONLY | No live instance | mocks only | MOCK SUCCESS-PATH VERIFIED (unchanged) |
| Live Kubernetes | MOCK ONLY | No live cluster | mocks only | MOCK SUCCESS-PATH VERIFIED (unchanged) |

## 3. Race-gate closure — evidence

Required record:

```text
Environment:
OS:            Windows 10.0.26200 (x86_64)
Architecture:  amd64
Go version:    go1.27.1 windows/amd64
Compiler:      gcc.exe (MinGW-W64 x86_64-ucrt-posix-seh, built by Brecht Sanders, r1) 16.2.0
CGO_ENABLED:   1
Command:       CGO_ENABLED=1 go test -race -count=1 ./...
Packages:      all (29 packages with tests reported ok; remainder no test files)
Duration:      full-suite background run, 2026-09-17
Race reports:  0 (no DATA RACE lines)
Exit code:     0
Status:        PASS — race gate CLOSED (green)
Executed at:   /c/dev/clean-room-aether @ bc674af1 (tag v4.1.0-rc2)
```

Attempt ledger (preferred order per charter):

| Option | Attempt | Result |
| --- | --- | --- |
| A — GitHub Actions (`ci.yml` has `-race` jobs on ubuntu + windows; `race-isolation.yml` workflow_dispatch) | `gh` installed (2.98.0) but unauthenticated; `origin/master` is stale at `fc062e0` (Stage 14) so CI would not test the candidate; no `gh auth login` available non-interactively | N/A — not executable from this host |
| B — Docker (`Dockerfile.race`) | Docker CLI present (29.7.2), daemon not running | N/A — daemon down |
| C — local C toolchain | `choco install mingw` → denied (`C:\ProgramData\chocolatey\lib-bad` access denied, requires elevation); **`scoop install mingw-winlibs` → success (user scope, no admin)** | **SUCCESS** — race suite executed, green |

Release-package spot check passed first
(`./internal/revocation/ ./internal/cli/` under `-race`), then the full
suite. The race gate was **never waived**; it was executed and passed.

## 4. CLI discovery and argument matrix (tagged binary `bin/aether-rc2`)

Inventory (from `--help` of the tagged binary):

```text
Top-level commands:        26 (audit, cap, completion, connect, dashboard,
                           doctor, exec, export, graph, pivot, plan, plugins,
                           providers, prt, relay, replay, rollback, run, serve,
                           simulate, token, tunnel, validate, watch, workspace, ztna)
export subcommands:        8 (+ help): attck, audit, executive, graph, pdf,
                           prioritize, report, sarif, verify-evidence
providers subcommands:     4: exec, list, users, validate
verify-evidence flags:     14 (--cache-ttl --cert --crl-file --crl-url
                           --fail-closed --issuer --json --responder
                           --revocation --revocation-file --skip-sig-verify
                           --timeout, plus inherited --config --log-level)
```

Matrix executed (exit codes from `bin/aether-rc2`):

| Case | Result |
| --- | --- |
| `<cmd> --help` for all 26 top-level commands | exit 0, help rendered — PASS |
| `<cmd>` bare invocation | required-argument commands (connect, dashboard, doctor, replay, run, serve, simulate, tunnel, watch) correctly exit 1; parent commands show help (exit 0) — PASS |
| `<cmd> --definitely-not-a-flag` for all 26 | exit 1, unknown flag rejected — PASS |
| `<sub> --help` for 18 second-level subcommands | exit 0 — PASS |
| `<sub> no-such-arg-x` for 18 second-level subcommands | 11 commands reject (exit 1); **3 leaf commands execute while silently ignoring the argument** (see D-29-1); 4 show help (exit 0, cobra parent/help pattern — D-29-2) |

## 5. Exit-code contract regression (fresh tagged binary)

| Case | Expected | Actual |
| --- | --- | --- |
| OCSP GOOD | 0 | 0 |
| OCSP REVOKED | 2 | 2 |
| OCSP unavailable responder | non-zero (1) | 1 |
| CRL REVOKED | 2 | 2 |

Contract unchanged from Stage 28; verified from a freshly built
`bin/aether-rc2` (G463), not a stale binary.

## 6. Defects discovered (full workflow records in observation log)

| ID | Severity | Finding |
| --- | --- | --- |
| D-29-1 | P2 | Leaf commands that take no positional args accept and silently ignore unexpected arguments while still executing: `export verify-evidence`, `providers list`, `workspace list`. Cobra `Args` not restricted. Disposition: add `cobra.NoArgs` in the next candidate (4.2.0 or an rc3 if a GA candidate is cut first). Not release-blocking: behavior/output otherwise correct. |
| D-29-2 | P3 | Parent commands display help with exit 0 for unknown subcommand/argument (audit, cap, exec, export, graph, pivot, plan, plugins, prt, relay, rollback, token, validate, workspace, ztna, cap). Accepted cobra pattern; documented here; no change required. |

No panics, no secret leakage, no state corruption observed in any matrix run.
> Corrected 23 → 21 in Stage 40; the count discrepancy is documented in docs/stage40-corrections-register.md.