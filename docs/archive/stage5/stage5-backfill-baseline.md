# Stage 5 Backfill — Baseline (B5-G01)

**Date:** 2025-09-15  
**Operator:** Stage 5 backfill execution  
**Baseline commit:** `d26aae7` (Stage 4 backfill final report — COMPLETE WITH EXPLICIT LIMITATIONS)  
**Baseline tag:** `v3.5.0-stage4-backfill` (historical, does not modify VERSION)  
**Current VERSION:** `4.0.0-rc1` (untouched per version-monotonicity rule)  
**Repository:** `C:\dev\aether` (off OneDrive, confirmed)  
**Working tree:** clean at baseline (`git status --porcelain` empty)

---

## Entry Criteria Verification

| Gate | Requirement | Verification | Status |
|------|-------------|--------------|--------|
| B5-G01.1 | Stage 4 backfill accepted | `docs/stage4-backfill-final-report.md` exists, reads **STAGE 4 COMPLETE WITH EXPLICIT LIMITATIONS** | ✅ PASS |
| B5-G01.2 | Working tree clean | `git status --porcelain` = empty at baseline | ✅ PASS |
| B5-G01.3 | Crash matrix 8/8 available | `artifacts/stage4-backfill/crash-matrix.json` (verified in Stage 4 evidence) | ✅ PASS |
| B5-G01.4 | `d250d52` reachable | `git cat-file -t d250d52` → `fatal: Not a valid object name` in **both** local repos | ❌ ABSENT (WS0 prerequisite fails → re-execute) |
| B5-G01.5 | Version-monotonicity acknowledged | Documented here; VERSION `4.0.0-rc1` will NOT be modified | ✅ ACKNOWLEDGED |
| B5-G01.6 | Protocol fixture inventory | `test/integration/protocol_conformance_test.go` = 32 fixtures across 6 families | ✅ INVENTORIED |
| B5-G01.7 | Evidence dir created | `artifacts/stage5-backfill/` created | ✅ CREATED |

---

## Baseline State Summary

- **Stage 4 backfill**: COMPLETE WITH EXPLICIT LIMITATIONS (all 16 gates PASS or CI-AUTHORITATIVE)
- **Archived Stage 5 (`d250d52`)**: ABSENT — not reachable in any local repo; WS0 → re-execute from scratch
- **Protocol conformance tests**: 32 fixtures implemented in `test/integration/protocol_conformance_test.go`
- **Interop matrix tests**: 7/7 targets implemented in `test/integration/interop_matrix_test.go`
- **Evidence verification tests**: 3/3 records + tamper detection in `test/integration/evidence_verification_test.go`
- **Replay/idempotency tests**: 10 scenarios in `test/integration/replay_idempotency_test.go`
- **All tests passing**: `go test -tags=integration -count=1 ./test/integration/...` → PASS

---

## Version Discipline

- **Historical tag only**: `v3.6.0-stage5-backfill` will be created at completion
- **VERSION file**: `4.0.0-rc1` — **WILL NOT BE MODIFIED** (monotonicity rule)
- **Current live tag**: `v4.0.0-rc1` points to `28bcb99` (Azure KV provider + concurrent work)

---

## Evidence Directory

`artifacts/stage5-backfill/` created for output artifacts:
- `protocol-conformance.json`
- `interoperability-matrix.json`
- `evidence-verification.json`
- `replay/` (idempotency evidence)
- `sbom-cyclonedx.json`
- `checksums.txt`
- `release-manifest.json`

---

**Baseline ACCEPTED** — proceeding to WS0–WS8 execution.