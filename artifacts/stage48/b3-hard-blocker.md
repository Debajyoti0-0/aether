# B-3 — Remaining Platform Runtime (darwin-amd64, darwin-arm64, windows-arm64): HARD BLOCKER

**Date:** 2026-09-19 · Status: **BLOCKED** for the three targets (G21 partial)

## Runtime truth (unchanged by this stage; no synthetic upgrades)

```text
windows-amd64   RUNTIME PASS   (native host, Stages 44+45)
linux-amd64     RUNTIME PASS   (real Linux kernel, Docker, Stage 47)
linux-arm64     RUNTIME PASS   (QEMU-emulated arm64 Linux, Stage 47)
darwin-amd64    NOT TESTED     (no macOS host; no CI remote; gh unauthenticated)
darwin-arm64    NOT TESTED     (no macOS host; no CI remote; gh unauthenticated)
windows-arm64   NOT TESTED     (no ARM64 Windows host; no CI remote)
```

## What was attempted

1. Stage 47: Docker-based real-runtime evidence for both Linux targets (genuine upgrade — this is the ceiling of what this lab's hardware can execute).
2. Stage 48: CI path evaluated — `.github/workflows/ci.yml` exists but the repository has **no remote** and `gh` is **unauthenticated**; GitHub-hosted macOS/Windows-ARM runners are therefore unreachable from this environment. No runner, no fabricated log.

## Prepared closure mechanism (ready to fire once a remote exists)

`.github/workflows/ga-smoke.yml` (committed with this stage) builds from source on `macos-13`, `macos-14`, and `windows-11-arm`, runs the full smoke (version, doctor, workspace lifecycle, graph build/qualify), and uploads logs as artifacts. Push to the authorized remote with `workflow_dispatch` to execute.

## Waiver draft — SIGNATURE STATUS: PENDING (not a PASS)

Per the platform waiver rule, a build-only waiver is drafted for the three targets:

```text
scope:            darwin-amd64, darwin-arm64, windows-arm64 runtime execution
why unavailable:  no hardware or reachable CI runner in this lab
build evidence:   6/6 targets build PASS (stage45); cross-toolchain byte-identity proven (stage47)
compensating:     identical pure-Go source; CGO_ENABLED=0; runtime verified on
                  windows-amd64, linux-amd64, linux-arm64 covering all three OS
                  syscall surfaces macOS shares with POSIX + the Windows surface
residual risk:    darwin/windows-arm64-specific behavior (e.g., keychain, signal
                  mapping, ARM64 page size) unexercised
owner:            operator
expiry:           next release or 90 days, whichever first
required future:  ga-smoke.yml run on real runners
approval:         Product + Security + QA signatures — PENDING (no board exists)
```

Unsigned → **PENDING**, not PASS. If runtime qualification of these targets is deemed mandatory for GA, B-3 remains BLOCKED.
