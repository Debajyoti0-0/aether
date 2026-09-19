# Board Review Dossier — Aether v3.4.0-ga

**Purpose:** a decision-ready package. An authorized decision-maker (board or delegated operator) reviews this dossier, executes any spot-checks they require, and records APPROVED / REJECTED with signature. Nothing here requires re-derivation; every claim carries its evidence pointer.

## Release identification

| Field | Value |
|---|---|
| Project | Aether — authorized-testing engineering platform for hybrid identity fabrics |
| Version | `3.4.0-ga` (Option A: honest continuation of `3.4.0-stage3`; no breaking change justifies 4.0.0; v4.x numbering belongs to the discarded foreign lineage) |
| Candidate commit | `d0f2d2e` (version-flip commit will be created at ceremony time, per `publish-runbook.md`) |
| Release scope | 6 platform binaries + CycloneDX SBOM + SLSA provenance + cosign signatures + checksums + manifest + release notes |

## What the release contains (changes since 3.4.0-stage3)

1. **D-001 fix (S2):** doctor's workspace roundtrip no longer fails on every Windows run (unclosed vault handle before delete).
2. **D-002 fix (S1):** `serve cert revoke --operator X` is reachable again — the flag registration omission had made operator revocation uninvoceable via CLI.
3. **D-003 fix (S1, security):** revocation list is re-read per connection (was a startup snapshot), restoring the documented "fails closed on new connections" contract; last-known list retained on I/O error so revocations never silently lift.
4. Regression tests for all three fixes; evidence package `artifacts/rebase|stage44|45|46|47|48/`.
5. `.github/workflows/ga-smoke.yml` for the three not-yet-executed runtime targets.

## Qualification status (denominator = 27 gates, stage47 matrix)

- **Product gates: PASS 21 · PARTIAL 1** (cross-platform runtime: 3/6 targets executed) · zero open product defects.
- Final product regression re-run this stage: 28/28 packages green, race green, vet clean, govulncheck clean.

## Governance blockers (the reason this dossier exists)

| Blocker | State | What closes it |
|---|---|---|
| B-1 independent audit | BLOCKED | External auditor executes `auditor-handoff/auditor-handoff.md` and signs `independent-audit-report-v2.md` |
| B-2 remote/channel | BLOCKED | Operator designates remote + channel; `publish-runbook.md` executed |
| B-3 darwin×2 + windows-arm64 runtime | BLOCKED / waiver PENDING | `ga-smoke.yml` run on CI runners, or 3 signatures on the drafted waiver |
| B-4 authorization | PENDING | **This dossier, signed below** |

## Known limitations (accurate, not inflated)

- File-based, name-keyed, connection-time revocation (live as of D-003); no OCSP/CRL.
- Single-operator evidence chain; no independent audit yet (B-1).
- 3 of 6 platform targets runtime-unexecuted (B-3).
- Soak evidence capped at 10 minutes; 30-minute soak tracked.
- tlog upload not performed (offline release; no policy requires it).

## Risk assessment (for the decision)

- **Residual technical risk: LOW.** All security-relevant behavior live-verified fail-closed; 3 defects found in the entire campaign, all fixed with regression tests.
- **Residual process risk: MODERATE** until B-1/B-3 close — this is what the signatures below are for.

## Decision

```text
Decision (APPROVED / REJECTED):  ______________
Scope (version, commit, artifacts):  ______________
Conditions (if any):  ______________
Decision maker (name, role):  ______________
Signature:  ______________
Date:  ______________
```

REJECTED stops the ceremony; record conditions and re-approval path. APPROVED moves the process to `publish-runbook.md` (which still requires B-1..B-3 closure first if this approval is conditioned on them).
