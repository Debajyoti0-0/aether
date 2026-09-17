# Stage 32a — Retrospective CLI Truth Table

Binary under test: `bin/aether-32a`, built in a clean room from the pushed
tag `v4.2.0-rc1` (commit `bb56cf3`); `--version` = `aether version
4.2.0-rc1`. Historical stages are reconciled by **claim → command →
actual behavior**, never by re-executing failed charters.

## 1. Claim register (extracted from stages 10–18, 20–26)

| Stage | Claim | Command/flag claimed | Claim-era reality |
| --- | --- | --- | --- |
| 10 | Release infrastructure, signing, SBOM, CI | release pipeline, cosign, Authenticode | partially built; signing never wired (see truth rows) |
| 11–18 | audits, tag governance, waivers | (no CLI surface claims) | reconciled in stage 28/30/31 history |
| 20 | Observability endpoints | `serve --metrics-addr`, `/metrics`, `/healthz`, `/readyz` | implemented, never live-tested then |
| 20 | Revocation checking | `export verify-evidence --revocation=*` | implemented, success paths unproven then |
| 20 | ARM64 support | linux/arm64, darwin/arm64 builds | compile claims only |
| 20 | IdP validation | `providers validate okta/gitlab/kubernetes` | error-path only then |
| 23–24 | FIX-4/FIX-5 certified; `--cert` flag | verify-evidence, providers validate | **false** — code was never committed (Stage 27 RCA) |
| 25–26 | published/stable 4.1.0 | — | **false** — withdrawn Stage 27 |

## 2. Truth table (all rows from the 32a clean-room binary)

| Stage | Claim | Command tested | Expected | Actual | Status | Evidence |
| --- | --- | --- | --- | --- | --- | --- |
| 20 | `/metrics` served | `serve --metrics-addr 127.0.0.1:9090` + curl | 200 + Prometheus exposition | HTTP 200; `# HELP aether_goroutines …` valid exposition | PASS | raw curl (Stage 32 run) |
| 20 | `/healthz` served | curl | 200 | HTTP 200 | PASS | raw curl |
| 20 | `/readyz` served | curl | 200 or documented not-ready | HTTP **503** with no workspace registered — fail-closed readiness (semantics verified in `internal/observability/health.go`: vault accessibility is a ready check) | PASS (fail-closed correct) | raw curl + source |
| 20 | Revocation OCSP | `export verify-evidence --revocation=ocsp --cert … --responder …` | GOOD | exit 0, GOOD, evidence accepted | PASS | raw CLI |
| 20 | Revocation CRL | `--revocation=crl --crl-file crl.pem` | not revoked | exit 0, GOOD | PASS | raw CLI |
| 20 | Revocation CRL revoked | `--crl-file crl-revoked.pem` | revoked | exit 2, REVOKED, rejected | PASS | raw CLI |
| 20 | IdP validation okta/gitlab/kubernetes | `providers validate <p> --domain … --token test` (mocks on 8091–8093; original 8081–8083 held by an unrelated stale process) | 200 + validation OK | exit 0 ×3, identity/validation echoed | PASS (MOCK) | raw CLI |
| 20 | ARM64 builds | `GOOS=linux GOARCH=arm64 go build`, `GOOS=darwin GOARCH=arm64 go build` | compile | both BUILD OK; darwin/arm64 also in goreleaser matrix since Stage 31 | PASS (compile; runtime NOT claimed) | raw build |
| 23–24 | FIX-4/FIX-5 "certified"; `--cert` | `--cert` flag present and drives the evaluated certificate | flag works | PASS since Stage 28 (real serial `635766…` used; hardcoded 12345 removed) | PASS (now; was FALSE historically — preserved) | Stage 28 evidence + 32a re-run |
| 25 | Signed artifacts | `.sig` files | signatures | **NEVER PRODUCED** (F-30-2/F-32-2; Stage 31 removed the false instructions; CI signing step now added, unexecuted) | NEVER IMPLEMENTED | release.yml + dist/ |
| 26 | 4.1.0 STABLE | — | stability | **WITHDRAWN** (Stage 27); superseded by 4.1.0-rc2 + observation window | WITHDRAWN (historical) | stage27–29 records |

Statuses used: PASS / WITHDRAWN / NEVER IMPLEMENTED — no FAIL, no
REGRESSED, no NOT IMPLEMENTED rows against the current binary. No
capability regressed versus the Stage 28/30/31 verified set (the 32a
re-runs reproduce every prior PASS).

## 3. VERIFY.md consistency (G534)

* Header still reads `Version: 4.0.0-rc2` — stale version reference
  (F-32-4, LOW: the guide's artifact table is for the 4.0.0-rc2 release;
  it is not the 4.2.0 release notes). Disposition: OPEN — regenerate per
  release at the next published release.
* Signing claims: corrected in Stage 31 (banner + NOT CURRENTLY PRODUCED
  labels). Commands advertised vs binary: every command in VERIFY.md
  exists; no instruction references a non-existent command.

## 4. Production readiness assessment (G535)

**READY FOR CONTROLLED PUBLICATION — with the standing qualification
debt unchanged and no new blockers.** Rationale: every CLI claim ever
made by the lineage now maps to a verified behavior or a truthful
NEVER-PRODUCED label; no instruction tells an operator to run anything
that does not exist; no regression exists against any prior verified
capability. Publication remains bounded by the pre-existing register:
signing (CI step added, unexecuted — F-32-2/F-32-3), provenance (F-30-3),
live integrations (LIVE-1), non-Windows runtime, and the observation
window. Those are distribution-trust items, not binary-correctness items;
"controlled publication" (expert lab / authorized internal team, per the
Stage 29–31 posture) is supportable; public GA is not.
