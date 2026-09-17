# Stage 30 — 4.2.0 Operations Qualification: Baseline Lock and Premise Reconciliation

The Stage 30 charter (the "Stage 27 — 4.2.0 Operations Release
Qualification" prompt) was written against a stale premise. Per its own
prime directive — *discover the truth from the implementation* — this
document reconciles the charter's assumptions with the repository's actual
state before any qualification work. Companion documents:
`stage30-production-qa.md`, `stage30-certification.md`.

## 1. Charter premise vs. observed reality (freeze 2026-09-17)

| Charter assumption | Observed reality | Disposition |
| --- | --- | --- |
| Baseline is `v4.1.0` at `38c9abd` | `v4.1.0` **does not exist** (locally or on `origin`); it was withdrawn by the real Stage 27 after the conflict-detection report contained in the charter itself | Charter premise corrected; qualification proceeds from the actual baseline |
| "Stage 26 FALSE CERT / fixtures missing / error-path only" | Superseded: the real Stages 28–29 regenerated all fixtures for the real leaf serial, verified FIX-4/FIX-5 success paths (13/13 + 3/3) **from the tagged build**, and the race gate is CLOSED (green) | Not repeated; evidence in `stage28-certification.md`, `stage29-baseline-lock.md` |
| Previous terminal RC is `v4.0.0-rc2` at `779cdb6f` | Correct and unchanged: commit `5cd008be1e3e20b039214a911626a6a4426c7838` (tag object `779cdb6f`) | Confirmed |
| Target `4.2.0` | Correct target; charter executed as **Stage 30** (stage numbers 27–29 already consumed by the executed recovery stages; `docs/stage27-*.md`–`stage29-*.md` exist and are not overwritten) | Numbering corrected |
| Implied: FIX-4/5 success paths never run | FIX-4 OCSP/CRL/file and FIX-5 okta/gitlab/kubernetes success paths verified from the tagged build; re-verified again this stage after the D-29-1 fix | Complete |

Actual baseline at freeze:

```text
HEAD            : ec8cbf5 (master), tree clean
Version line    : 4.1.0-rc2 certified (tag v4.1.0-rc2 → bc674af1), in
                  30-day observation window opened 2026-09-17
Withdrawn       : v4.1.0 — absent locally and remotely, not recreated
```

## 2. Implementation audit sweep

| Sweep | Result | Disposition |
| --- | --- | --- |
| TODO / FIXME / HACK / NOT-IMPLEMENTED / PLACEHOLDER (non-test Go code) | 2 raw hits | 1 false positive (`fuzz.go` comment about `\uXXXX` escapes); 1 finding below |
| `wstrust/downgrade.go:61` `"PLACEHOLDER"` | Password slot in the downgrade RST builder carries a placeholder; claims are injected downstream and credential injection is never exercised against a live STS | INTENTIONAL-but-untested: live WS-Trust downgrade qualification remains open debt (part of live protocol qualification, BLOCKED — no authorized tenant) |
| Dead code / unreachable branches | `go vet` clean; 0 lint issues (golangci-lint full config) | PASS |
| Hardcoded secrets (password/client_secret/access_token patterns, non-test code) | 0 findings; all matches are redaction lists (`store/log.go`), OAuth form-field construction, or env reads (`AETHER_PASSWORD`) | PASS |
| Version references | `internal/version` single source of truth; `VERSION` file consistent; stale refs: none found | PASS |
| CI workflows | `ci.yml` has `-race` jobs (ubuntu+windows) and `race-isolation.yml` (workflow_dispatch); executed locally this stage via CGO instead (CI runners unreachable: gh unauthenticated, origin/master stale) | Race gate CLOSED locally; CI race run remains recommended |

## 3. Release-surface audit (`.goreleaser.yml`)

| Item | State | Finding |
| --- | --- | --- |
| Build matrix | linux/amd64, linux/arm64, windows/amd64, darwin/amd64; **darwin/arm64 and windows/arm64 explicitly ignored** | F-30-1 (MEDIUM): no Apple Silicon artifact exists; recommend removing the darwin/arm64 ignore in a future release once runtime-tested |
| ldflags | version + commit injected via `internal/version` | PASS (verified: release artifact reports injected version, not `dev`) |
| Checksums | sha256 `checksums.txt` | PASS (verified — see production-qa §5) |
| SBOM | syft per-archive, SPDX-2.3 | PASS (verified in snapshot run) |
| Signing | **No `signs:` section** — yet the release footer instructs operators to verify cosign signatures and Authenticode | F-30-2 (HIGH, documentation drift + missing infrastructure): the pipeline never produces the signatures the footer tells users to verify. EV Authenticode remains BLOCKED (no certificate). Disposition: either add cosign keyless signing (`signs:` with cosign sign-blob) or remove the verification instructions until signing exists |
| SLSA provenance | Not configured | F-30-3 (MEDIUM): no provenance generated; documented, not claimed |
| Docker | Intentionally disabled (`dockers: []`) | Intentional |
