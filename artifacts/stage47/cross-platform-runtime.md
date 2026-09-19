# Stage 47 — Cross-Platform Runtime Qualification

**Date:** 2026-09-19 · Docker Engine 29.7.2 (WSL2 backend) as real Linux runtime; alpine:latest (linux/amd64) and linux/arm64 (QEMU emulation via binfmt).

## Per-target results

| Target | Build | Runtime | Environment | Evidence |
|---|---|---|---|---|
| linux/amd64 | PASS | **RUNTIME PASS** | alpine container, real Linux kernel | `--version` → `3.4.0-stage3`; `doctor` all checks OK (POSIX signals note); workspace create→list→delete lifecycle OK; 10K-node graph build (10065 nodes/13334 edges) + qualify → runbook, results identical to windows-amd64 run |
| linux/arm64 | PASS | **RUNTIME PASS** | alpine container, `--platform linux/arm64` (QEMU) | `--version` → `3.4.0-stage3`; workspace create→list→delete lifecycle OK (`ARM64_LIFECYCLE_OK`) |
| windows/amd64 | PASS | **RUNTIME PASS** | native host | prior stage: version, doctor, full lifecycle, teamserver, dashboard |
| darwin/amd64 | PASS | **NOT TESTED** | no macOS host available in this lab | hardware limitation |
| darwin/arm64 | PASS | **NOT TESTED** | no macOS host available in this lab | hardware limitation |
| windows/arm64 | PASS | **NOT TESTED** | no ARM64 Windows host available in this lab | hardware limitation |

## Integrity chain

Container-side `sha256sum` of the executed binaries matched `checksums.txt` exactly:
- `1c657152045f08a0da49310147a99bfb537dfc23498f10a45d67e2aa7bd454c0` aether-linux-amd64
- `0e0928286a616a34d48130ddeb88512e0c30ba8b59e7bdf162e4ed5b939df725` aether-linux-arm64

The bytes executed in Linux are bit-identical to the released, signed artifacts.

## Cross-toolchain reproducibility — PROVEN

Native rebuild inside `golang:1.27-alpine` (linux/amd64, Go 1.27.1, CGO_ENABLED=0) vs. cross-compile from the Windows host (same Go 1.27.1, same flags, `-buildvcs=false`):

```text
host cross-compile:  321c96fb6d75397d34f9dda97a8d723049ce4c42ea2c042ab9ef4966e7c3461b
container native:    321c96fb6d75397d34f9dda97a8d723049ce4c42ea2c042ab9ef4966e7c3461b  → IDENTICAL
```

Investigated intermediate mismatches (documented for the record):
1. dist binaries were built from the pre-commit **dirty** tree → VCS stamp `+dirty` in buildinfo explains their difference from clean-tree rebuilds (`go version -m` shows `mod …+dirty`).
2. Host rebuild without explicit `GOOS` produced a windows binary (mousetrap dep present) — operator error, corrected.
3. With identical VCS state (`-buildvcs=false`), Windows→linux cross-compile and native Linux build are **byte-identical across operating systems and toolchain installs**.

**Release-process recommendation:** final GA binaries should be built from the committed clean tree with `-buildvcs=false` (or from an identical clean checkout) so every target is reproducible cross-OS.

## Remaining

darwin/amd64, darwin/arm64, windows/arm64: runtime NOT TESTED — no hardware. Paths to closure: CI runners (GitHub Actions macos-13/macos-14/windows-11-arm) or a formal signed waiver if release policy permits build-only qualification for those targets.
