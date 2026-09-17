# Stage 30 — Certification: 4.2.0-rc1

Companion: `stage30-baseline-lock.md` (premises, audit), 
`stage30-production-qa.md` (evidence). Charter: the "4.2.0 Operations
Release Qualification" prompt, executed with premise reconciliation
(the assumed baseline `v4.1.0@38c9abd` does not exist — it was withdrawn
in the executed Stage 27; the live candidate line is `4.1.0-rc2`).

## 1. Release candidate

| Item | Value |
| --- | --- |
| Version | `4.2.0-rc1` |
| Commit | `<set at tag time>` |
| Tag | `v4.2.0-rc1` (annotated, created locally; **not pushed** — the charter does not authorize a push, and the 4.1.0-rc2 observation window remains open until 2026-10-17) |
| Change vs `4.1.0-rc2` | one code change: D-29-1 CLI contract fix (`cobra.NoArgs` on `export verify-evidence`, `providers list`, `workspace list` + regression test, commit `36be959`) |
| Parent baselines | `v4.1.0-rc2` → `bc674af1` unchanged; `v4.0.0-rc2` → `5cd008be` unchanged; `v4.1.0` still absent |

## 2. 4.2.0-rc1 gate results

| Gate | Status | Evidence |
| --- | --- | --- |
| build / vet | PASS | production-qa §9 |
| unit | PASS | 29 packages |
| integration | PASS | 32.6s |
| race | PASS | CGO + mingw-winlibs GCC 16.2.0; 29 ok, 0 DATA RACE |
| fuzz | PASS | 23 targets × 60s; 23/23 exit 0; 0 crashes/panics/timeouts |
| lint | PASS | 0 issues |
| govulncheck | PASS | 0 affecting |
| CLI matrix | PASS at command/argument level; release-surface flag matrix PASS; full per-flag boundary sweep PARTIALLY VERIFIED (tracked debt) | production-qa §1 |
| exit-code / output contract | PASS | production-qa §2–3 |
| config precedence | VERIFIED (CLI > env > config > default; explicit + auto-discovery proven) | production-qa §4 |
| security / secret scan / failure injection | PASS | production-qa §8 |
| FIX-4 (OCSP/CRL/file) success + negative paths | VERIFIED (mock PKI, from this build) | production-qa §2 |
| FIX-5 (okta/gitlab/kubernetes) | MOCK SUCCESS-PATH VERIFIED; live BLOCKED | production-qa |
| Live Entra / IMDS / IdP | BLOCKED — no authorized access (explicitly BLOCKED, never PASS) | carried limitation |
| Cross-platform | compile VERIFIED (4/4 goreleaser targets); runtime VERIFIED only on windows/amd64; others BLOCKED | production-qa §6 |
| Release artifacts | checksums + SBOM (SPDX-2.3) + embedded version VERIFIED via goreleaser snapshot | production-qa §5 |
| Signing (cosign/Authenticode) | BLOCKED — no signing config, no EV certificate; F-30-2 documents the footer/config drift | baseline-lock §3 |
| SLSA provenance | NOT VERIFIED — not configured (F-30-3) | |
| Clean-room install / operator journey | PASS (isolated state, fail-closed keyless refusal) | production-qa §7 |
| Historical reconciliation | COMPLETE — false historical certs preserved as false | production-qa §10 |
| Working tree | CLEAN | |

## 3. Blocker register

| ID | Severity | Finding | Status |
| --- | --- | --- | --- |
| D-29-1 | P2 | positional args silently ignored by 3 commands | RESOLVED (`36be959`, regression test) |
| F-30-2 | HIGH | release footer advertises cosign/Authenticode verification the pipeline cannot produce (no `signs:` config, no EV cert) | OPEN — blocks public distribution claims; fix: add cosign keyless `signs:` or remove instructions |
| F-30-1 | MEDIUM | no darwin/arm64 (Apple Silicon) release artifact | OPEN — matrix decision required |
| F-30-3 | MEDIUM | no SLSA provenance | OPEN |
| DEBT-1 | MEDIUM | full per-flag × per-command boundary matrix | OPEN — release surface fully tested; long-tail commands at command/argument level only |
| DEBT-2 | MEDIUM | non-Windows runtime verification (linux amd64/arm64, darwin) | OPEN — requires hosts or CI runners |
| LIVE-1 | BLOCKED | live Okta/GitLab/Kubernetes/Entra/IMDS | OPEN — requires authorized access |
| L-28-1 | — | race gate blocked | RESOLVED (Stage 29/30: executed, green) |

No BLOCKER/CRITICAL findings. F-30-2 is HIGH with explicit disposition:
it does not affect the binary's behavior, but public release distribution
claims are not permitted while the advertised verification path is
non-functional.

## 4. Verdict

**OPTION B — `4.2.0-rc1` QUALIFIED AS A RELEASE CANDIDATE WITH EXPLICIT
QUALIFICATION DEBT.** Every executable gate passed on evidence; the
remaining items (signing, live integrations, provenance, non-Windows
runtime, boundary-matrix long tail) are explicitly BLOCKED or tracked —
none was converted into a PASS.

**A production `4.2.0` (GA) is BLOCKED** until at minimum: F-30-2
(signing or footer correction), LIVE-1 disposition (live qualification or
an explicitly approved alternative), and the 4.1.0-rc2 observation-window
close (2026-10-17) with its promotion decision.

## 5. Stage 31 handoff

`4.2.0-rc1` (local tag) carries one code change over the certified
`4.1.0-rc2` and passes every executable gate, including race (now a
proven, repeatable local capability). Next: (1) on/after 2026-10-17 close
the 4.1.0-rc2 observation window (end-of-window integration + unit on the
tagged binary) and issue the PROMOTE/RETAIN decision; (2) resolve F-30-2 —
prefer adding cosign keyless signing to `.goreleaser.yml` so the footer's
instructions become true; (3) disposition F-30-1/F-30-3 (darwin/arm64
matrix, SLSA provenance); (4) live Entra/IMDS/IdP qualification when
authorized access exists; (5) only then consider the `4.2.0` GA gate.
Push `v4.2.0-rc1` only with explicit authorization.
