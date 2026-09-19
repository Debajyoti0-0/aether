# Stage 45 — Supply Chain Evidence

**Date:** 2026-09-19 · Commit `0dae3ae` (+3 defect fixes) · Go `go1.27.1`

## Reproducible build — PASS

`go build -trimpath -ldflags="-s -w -X ...version.Version=3.4.0-stage3 -X ...version.Commit=<rev>" ./cmd/aether`

- Build 1 (warm cache): `f5cafd277defdef002c1d290e1dc417fccf77cc1b8fec1f7f8f6435a5885c6dc`
- Build 2 (warm cache): identical
- Build 3 (after `go clean -cache`): identical

Byte-for-byte reproducible on windows/amd64. Cross-output-path determinism confirmed (dist binary hashes equal repro hashes).

## Cross-platform matrix — PASS (build) / PARTIAL (runtime)

| Target | Build | Runtime smoke |
|---|---|---|
| windows/amd64 | PASS (hash above) | **PASS** — `--version` (3.4.0-stage3), `doctor` all-OK, full workspace lifecycle, teamserver + dashboard operations |
| windows/arm64 | PASS | NOT RUN (no arm64 host) |
| linux/amd64 | PASS | NOT RUN — docker-desktop WSL distro has no `/mnt/c`; no other Linux environment |
| linux/arm64 | PASS | NOT RUN (no host) |
| darwin/amd64 | PASS | NOT RUN (no host) |
| darwin/arm64 | PASS | NOT RUN (no host) |

All six built from the same reproducible command line (`Makefile build-*` equivalents, `-trimpath`). Per-target SHA-256 in `checksums.txt`.

## SBOM — PASS

- `sbom/aether-3.4.0-stage3.cdx.json` — CycloneDX 1.6 via `cyclonedx-gomod bin` against the release binary; 26 components; root component `github.com/Debajyoti0-0/aether`.
- No prior SBOM exists (first release) — diff N/A.
- Signed: `supply-chain/aether-3.4.0-stage3.cdx.json.sig`.

## Provenance — PASS

- `provenance.json` — SLSA v1 in-toto statement: 6 subjects (all dist binaries), commit SHA, toolchain, build flags, SBOM digest, signature digest, public-key digest.
- Independent verification: every subject digest recomputed from the artifact bytes and matched — `PROVENANCE_VERIFICATION=PASS`.
- Signed: `supply-chain/provenance.json.sig`.

## Signing — PASS (offline; tlog upload NOT PERFORMED)

- cosign key-based signing; **release private key custody: `~/.aether-release-keys/release-3.4.0-stage3.key` (outside the repository, 0600)**; public key committed at `supply-chain/release.pub`.
- Positive verification: `Verified OK` (binary, checksums, provenance, SBOM).
- Tamper rejection: appended byte → `invalid signature` (rejected).
- Wrong-key rejection: second keypair → `invalid signature` (rejected).
- Transparency log: **NOT PERFORMED** — offline release; `--tlog-upload=false`. Recorded as deviation, not waived as PASS.

## Checksums & manifest — PASS

- `checksums.txt` (SHA-256, all 6 targets), signed.
- `release-manifest.json` lists artifacts, digests, signatures, SBOM, provenance, toolchain, commit.

## Publication — BLOCKED (process, not product)

The repository has **no configured git remote** and no authorized release channel was designated in this session. Remote release = BLOCKED. Offline release package is complete in `artifacts/stage45/dist/`.
