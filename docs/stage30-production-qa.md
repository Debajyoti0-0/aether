# Stage 30 — Production QA Evidence (4.2.0 Qualification)

All results from the post-fix master tree (`36be959`, the D-29-1 CLI
contract fix). Companion: `stage30-baseline-lock.md` (premises, audit),
`stage30-certification.md` (gate, verdict).

## 1. CLI contract

### D-29-1 fix (the one permitted code change)

`export verify-evidence`, `providers list`, and `workspace list` accepted
and silently ignored unexpected positional arguments (Stage 29 finding).
Fix: `Args: cobra.NoArgs` + regression test
(`TestVerifyEvidenceRejectsUnknownPositionalArg`). Verified from the
rebuilt binary:

```text
aether export verify-evidence no-such-arg  → Error: unknown command …  exit 1
aether providers list no-such-arg          → exit 1
aether workspace list no-such-arg          → exit 1
```

### Command/argument matrix (26 top-level + 18 second-level commands)

| Case | Result |
| --- | --- |
| `<cmd> --help` (all 26) | exit 0, help rendered — PASS |
| bare `<cmd>` | required-argument commands exit 1; parents show help — PASS |
| unknown flag (all 26) | exit 1 — PASS |
| unknown positional arg (18 subcommands) | 14 reject (exit 1) after D-29-1 fix; 4 parent/help patterns documented (P3) — PASS |
| silent-ignore cases | **0 remaining** (was 3, fixed) — PASS |

Depth note (honest classification): the per-flag × per-command boundary
matrix (every flag's zero/negative/max/long/special-value cases across all
~26 commands) is **PARTIALLY VERIFIED** — executed fully for the release
surface (verify-evidence: 13-scenario matrix incl. exit codes and JSON;
providers validate: 3 providers × success/401/403/malformed/unreachable)
and at command/argument level for the full surface. Full per-flag boundary
sweep across every command remains qualification debt (tracked).

## 2. Exit-code contract (re-verified post-fix)

| Case | Expected | Actual |
| --- | --- | --- |
| OCSP GOOD | 0 | 0 |
| OCSP REVOKED | 2 | 2 |
| CRL GOOD | 0 | 0 |
| CRL REVOKED | 2 | 2 |
| unavailable responder / config error | 1 | 1 |
| unknown positional arg (new) | 1 | 1 |

## 3. Output contract

| Item | Result |
| --- | --- |
| `--json` on verify-evidence | valid JSON (parsed independently), fields stable, no secret material (subject + SHA-256 only) — PASS |
| stderr/stdout separation | status/diagnostics on stderr (`Revocation check mode`, `Using config file`, `EVIDENCE ACCEPTED/REJECTED`); JSON/human results on stdout — PASS |
| human output | PASS |

## 4. Configuration precedence (verified from implementation + execution)

Implementation: viper — `--config <file>` explicit path, else
`aether.json` in `paths.ConfigSearchPaths()`; `viper.AutomaticEnv()`;
flags bound via `BindPFlag`. Observed precedence (viper standard,
confirmed by code inspection): **CLI flag > environment > config file >
default**.

```text
--config /tmp/aether-test.json doctor   → "Using config file: …/aether-test.json"  VERIFIED
%APPDATA%\aether\aether.json            → auto-discovered and loaded          VERIFIED
wrong location ($HOME/aether.json)      → correctly NOT loaded (not a search path)
```

## 5. Release artifact forensics (goreleaser snapshot run, no publish)

`goreleaser release --snapshot --clean` succeeded (2m36s):

| Check | Result |
| --- | --- |
| Archives | 4: linux/amd64, linux/arm64, darwin/amd64 (tar.gz), windows/amd64 (zip) |
| Checksums | `sha256sum -c checksums.txt` — all match — PASS |
| Embedded version | windows artifact binary reports `v4.1.0-rc2-next` (snapshot version template) — ldflags injection VERIFIED |
| SBOM | syft SPDX-2.3 per archive; parsed independently (48 components) — VALID |
| Signing | no signatures produced (no `signs:` config) — BLOCKED, see F-30-2 |
| Provenance | not generated — NOT VERIFIED (F-30-3) |
| darwin/arm64 | no artifact (matrix excludes it) — F-30-1 |

## 6. Cross-platform

| Platform | Compile | Runtime |
| --- | --- | --- |
| windows/amd64 | PASS | VERIFIED (this host: full CLI matrix, install test, journeys) |
| linux/amd64 | PASS | BLOCKED — no Linux host/CI run in this stage |
| linux/arm64 | PASS | BLOCKED — no ARM host |
| darwin/amd64 | PASS | BLOCKED — no macOS host |

Runtime verification from compilation alone is NOT claimed for any
non-Windows platform.

## 7. Clean-room installation test (operator journey)

Isolated directory, isolated `%APPDATA%`, artifact-only (no dev checkout,
no local config, no cached state):

```text
aether --version                → "aether version dev" (plain go build;
                                  ldflags injection is a release-pipeline
                                  behavior — verified separately, §5)
aether --help                   → exit 0
workspace list (empty state)    → "No workspaces." — PASS (no leakage of
                                  the developer's real workspace list)
workspace create opws --passphrase … → created under isolated APPDATA — PASS
workspace list                  → opws — PASS
export verify-evidence (file)   → correct serial + status — PASS
workspace create without --passphrase → fail-closed error (keyless mode
                                  refused) — PASS
```

## 8. Security

| Check | Result |
| --- | --- |
| Secret scan (patterns: client_secret, tokens, passwords, private keys; non-test code) | 0 hardcoded secrets — PASS |
| Output leakage | tokens appear only in request construction; redaction set present in `store/log.go` — PASS |
| Failure injection (release surface) | connection refused → exit 1 + diagnostic; HTTP 401/403 → exit 1 + mapped error; malformed response → exit 1; missing/unreadable/malformed cert & CRL → exit 1; no panic, no silent success in any case — PASS |
| Panics across full suite | 0 (unit + integration + race + fuzz runs) — PASS |

## 9. Quality gates (post-fix master, `36be959`)

| Gate | Result | Detail |
| --- | --- | --- |
| build | PASS | |
| vet | PASS | |
| unit (`go test -count=1 ./...`) | PASS | 29 packages |
| integration (`-tags=integration`) | PASS | 32.6s |
| race (`CGO_ENABLED=1 go test -race ./...`, mingw-winlibs GCC 16.2.0) | PASS | 29 packages ok, 0 DATA RACE, exit 0 |
| fuzz (21 targets × 60s) | see certification doc (final counts) | |
| lint (golangci-lint) | PASS | 0 issues |
| govulncheck | PASS | 0 affecting (1 unreachable, in required modules) |

## 10. Historical reconciliation (truth table, stages 19–30)

| Stage | Claimed | Actual | Status |
| --- | --- | --- | --- |
| 19 | 4.0.0 line closed | `v4.0.0-rc2` terminal, unchanged at `5cd008be` | ✅ CONFIRMED (re-verified this stage) |
| 20 | gates ✅ | false gates | ❌ preserved as false |
| 21 | real QA | real | ✅ |
| 22 | FIX-1/2/3 PASS | pass | ✅ |
| 23 | CERTIFIED 4.1.0 | version truth broken; no tag | ❌ preserved as false |
| 24 | CERTIFIED 4.1.0 | tag created without implementation | ❌ superseded |
| 25 | PUBLISHED 4.1.0 | error-path-only evidence | ❌ preserved as false |
| 26 | 4.1.0 STABLE | every success path ERROR-PATH ONLY | ❌ false cert (as the charter's own conflict report states) |
| 27 | REAL QA | `v4.1.0` withdrawn; root cause: code never committed | ✅ |
| 28 | CERTIFIED 4.1.0-rc2 | committed, fixed, tagged, verified from tag; race BLOCKED (honest) | ✅ |
| 29 | RETAIN rc2 | race gate CLOSED (green); window opened 2026-09-17 | ✅ |
| 30 | 4.2.0 qualification | this stage; verdict in certification doc | IN PROGRESS → see verdict |

No historical record was rewritten; contradictions remain preserved and labeled.
> Corrected 23 → 21 in Stage 40; the count discrepancy is documented in docs/stage40-corrections-register.md.