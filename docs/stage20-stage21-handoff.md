# Stage 20 — Stage 21 Handoff (G268)

**Timestamp:** 2016-09-17
**Stage:** 20 — Operations Track & 4.1.0 Development Foundation
**Agent:** Stage 20 execution agent
**Repository:** C:\dev\aether

---

## Summary

Stage 20 completed the foundation for the 4.1.0 Operations Track. The 4.0.0 line is closed (terminal release `v4.0.0-rc2`). 4.1.0 scope is defined, bounded, and traceable to audit findings.

---

## 4.0.0 Terminal Release (Immutable)

| Field | Value |
|-------|-------|
| Release | `4.0.0-rc2` |
| Tag | `v4.0.0-rc2` |
| Commit | `5cd008b` (annotated tag `779cdb6f62075565df376c2b9f04e11f7f29e4c6`) |
| Classification | Terminal / Production-Limited |
| GA Authorization | Not granted |
| Fuzz Targets | 21 frozen |

**All Stage 19 waivers remain active and documented.** Do not modify terminal release evidence.

---

## 4.1.0 Development Identity

| Field | Decision |
|-------|----------|
| Base Commit | `bfdf713` (master HEAD post-hygiene) |
| Branch | `master` (no feature branches) |
| Version | `4.1.0` |
| Release Type | Operations / Maintenance |
| Compatibility | Full backward compatibility |
| Migration Policy | Zero-downtime, no migration required |
| Tag Policy | `v4.1.0` at Stage 20 exit commit |
| Changelog | Append to `CHANGELOG.md` under `## [4.1.0]` |

---

## 4.1.0 Scope (10 In-Scope Items)

| # | Item | Gap ID | Type | Validation |
|---|------|--------|------|------------|
| 1 | `/metrics` (Prometheus) | OP-02 | MUST HAVE | Unit + integration |
| 2 | `/healthz` + `/readyz` | OP-01 | MUST HAVE | Unit + integration |
| 3 | Structured Logging (slog) | OP-03 | SHOULD HAVE | Unit test |
| 4 | Revocation (OCSP/CRL + File) | OP-05 | SHOULD HAVE | Unit + integration |
| 5 | ARM64 Integration Testing | OP-08 | MUST HAVE | Build + test on ARM64 |
| 6 | Third-Party IdP (Keycloak) | OP-08 | SHOULD HAVE | Live test vs dev Keycloak |
| 7 | AzureKVProvider Mutex | D-01 | MUST HAVE | `go test -race ./internal/store/...` |
| 8 | Workspace Pass/Salt Mutex | D-02 | MUST HAVE | `go test -race ./internal/workspace/...` |
| 9 | Action Pinning | D-03 | MUST HAVE | CI verification |
| 10 | VERIFY.md Creation | D-06 | MUST HAVE | Stranger reproduction |

---

## Explicitly Out of Scope (Deferred to 4.2.0+)

- Live Entra/IMDS/KV validation (waivers until 2027-06-30 / 2027-03-31)
- Upgrade/Rollback tooling (security-sensitive)
- Schema migration framework (not needed)
- Security audit logging (requires design)
- SLSA Provenance (goreleaser expertise)
- Full Race CI Green (waived with mutex fixes)
- EV Authenticode (permanent waiver)
- Branch protection / secret scanning (operator responsibility)

---

## Implementation Plan (16 Work Items, 4 Phases)

```
Phase A: Foundation (No Deps)
  WI-01: Action Pinning
  WI-02: AzureKVProvider Mutex
  WI-03: Workspace Mutex
  WI-04: VERIFY.md

Phase B: Observability Core
  WI-05: Structured Logging (slog)
  WI-06: Metrics Server (/metrics)
  WI-07: Health Server (/healthz, /readyz)

Phase C: Operational Features
  WI-08: Revocation (OCSP/CRL + File)
  WI-09: ARM64 CI Integration
  WI-10: Keycloak Interop

Phase D: Release Preparation
  WI-11: Goreleaser ARM64 Config
  WI-12: CI Matrix Updates
  WI-13: Documentation Updates
  WI-14: Full Quality Gate Run
  WI-15: VERSION -> 4.1.0
  WI-16: Tag v4.1.0
```

**Quality Gates per Phase:**
- Phase A: Race tests pass; CI pinned; VERIFY.md stranger test
- Phase B: Observability endpoints work; structured logging verified
- Phase C: Revocation tests pass; ARM64 builds; Keycloak dry-run
- Phase D: All quality gates; 6 artifacts; tag created

---

## Evidence & Release Gates

### Required Evidence (Pre-Tag)

| Gate | Command | Expected |
|------|---------|----------|
| G253 | `go build ./...` | PASS |
| G253 | `go vet ./...` | PASS |
| G253 | `go test -count=1 ./...` | PASS |
| G254 | `go test -race ./internal/store/...` | PASS |
| G254 | `go test -race ./internal/workspace/...` | PASS |
| G254 | `go test -fuzz=. -fuzztime=60s` | 21 targets, 0 crashes |
| G255 | `golangci-lint run ./...` | 0 issues |
| G256 | `govulncheck ./...` | 0 affecting |
| G257 | `aether serve` + `curl /metrics` | Prometheus format |
| G257 | `curl /healthz` | 200 OK |
| G257 | `curl /readyz` | 200 OK (when ready) |
| G258 | `aether validate revocation --cert` | PASS |
| G259 | `GOARCH=arm64 go build ./...` | PASS |
| G260 | `aether validate keycloak --dry-run` | PASS |
| G262 | `cat VERSION` | `4.1.0` |
| G263 | `goreleaser release --snapshot` | 6 artifacts |
| G264 | `git tag -a v4.1.0` | Created |
| G265 | `docs/stage20-*` count | ≤ 10 |
| G266 | `git diff docs/stage20-*design*.md` | Empty |
| G267 | `git show v4.0.0-rc2:VERSION` | `4.0.0-rc2` |

---

## Document Set (10 Total — Within Limit)

1. `stage20-scope-lock.md` — Scope lock declaration
2. `stage20-baseline-lock.md` — Baseline + branch/version strategy (G251, G1)
3. `stage20-workspace-hygiene.md` — Working tree resolution (G252)
4. `stage20-operations-audit.md` — Operations architecture audit (G2)
5. `stage20-design-reconciliation.md` — Parked design reconciliation (G3)
6. `stage20-gap-register.md` — RCA + gaps + scope definition (G4, G5)
7. `stage20-security-review.md` — Security review + compatibility (G7, G6)
8. `stage20-implementation-plan.md` — Implementation plan + evidence strategy (G8, G9)
9. `stage20-certification.md` — This certification (G10)
10. `stage20-stage21-handoff.md` — This handoff (G268)

---

## Pre-Implementation Conditions (Must Resolve Before Stage 21)

1. ✅ Document consolidation complete (10 docs)
2. ⚠️ `golangci-lint run ./...` — **MUST PASS** (G255)
3. ⚠️ `govulncheck ./...` — **MUST PASS** (G256)
4. ⚠️ Evidence dir `artifacts/stage20/` created

---

## Stage 21 Preview

**If conditions met: Stage 21 — 4.1.0 Controlled Implementation**

| Phase | Work Items | Est. Duration |
|-------|------------|---------------|
| A | WI-01..04 (Foundation) | ~1 week |
| B | WI-05..07 (Observability) | ~1 week |
| C | WI-08..10 (Features) | ~1 week |
| D | WI-11..16 (Release) | ~1 week |

**Total: ~4 weeks** for 4.1.0 implementation.

**Stage 21 Output:** `v4.1.0` tagged, pushed, with all quality gates passed, 6 release artifacts, VERIFY.md verified.

---

## Key References

- Terminal release: `docs/stage19-terminal-decision.md`
- Baseline: `docs/stage20-baseline-lock.md`
- Scope: `docs/stage20-gap-register.md` (appendix)
- Implementation: `docs/stage20-implementation-plan.md`
- Evidence strategy: `docs/stage20-implementation-plan.md` (appendix)
- Security: `docs/stage20-security-review.md`
- Audit: `docs/stage20-operations-audit.md`
- Design reconciliation: `docs/stage20-design-reconciliation.md`

---

*Generated by Stage 20 Phase 10 — Stage 20 Certification*

*Stage 20 Status: COMPLETE WITH EXPLICIT LIMITATIONS (pending lint/vuln checks)*

*4.0.0 Line: CLOSED / TERMINAL — v4.0.0-rc2 is the final 4.0.0 release*

*4.1.0 Operations Track: READY FOR IMPLEMENTATION (pending pre-conditions)*