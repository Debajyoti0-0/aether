# Stage 46 Baseline Summary — Phase 0 Entry Verification

## Repository State (2026-09-21)

| Property | Value |
|----------|-------|
| Commit SHA | a887a0ed9c9fcdbf0400a5d59ccad162cfcac202 |
| Branch | master |
| Version (VERSION file) | 5.0.0-alpha1 |
| Go Version | go1.27.1 windows/amd64 |
| Git Status | Modified (Stage 45 changes uncommitted) |

## Stage 45 Entry Verification Result: **PARTIALLY_VERIFIED**

### Verified ✅

| Component | Status |
|-----------|--------|
| `internal/protocol/kerberos/` | Present (11 files, 18 unit tests PASS) |
| `internal/engine/ad/kerberos/` | Present (4 files, 10 unit tests PASS) |
| `internal/cli/ad/` | Present (4 files, 8 commands) |
| Spine capabilities registered | `ad.enum.read`, `ad.kerberos.roast`, `ad.kerberos.tgt` |
| All tests pass | `go test ./...` - 31 packages PASS |
| Race detector | PASS |
| Vet | PASS |

### Blocking Issues ❌

| Issue | Severity | Detail |
|-------|----------|--------|
| Samba4 lab harness empty | CRITICAL | `scripts/ad-lab/` exists but no `docker-compose.yml` or `seed.sh` |
| No integration tests | CRITICAL | `test/integration/ad/` does not exist |
| No live qualification evidence | CRITICAL | `artifacts/stage45/` only has baseline - no Samba4 transcripts |
| Stage 45 declared CLOSED | HIGH | But live verification never executed |

## Gate TB46-G00 Verdict: **BLOCKED-WITH-OWNER**

**Reason**: Stage 45 declared CLOSED but live Samba4 qualification never executed. The `scripts/ad-lab/` directory is empty - no Docker Compose file, no seed script. No integration tests exist in `test/integration/ad/`. No raw transcripts or qualification evidence in `artifacts/stage45/`.

**Required before Stage 46 can proceed:**
1. Implement Samba4 Docker Compose in `scripts/ad-lab/`
2. Create seed script with test users (including no-preauth and SPN accounts)
3. Create integration tests in `test/integration/ad/`
4. Execute integration tests against live Samba4
4. Capture raw transcripts and evidence
5. Update Stage 45 certification with live results

---

## Documents Created (Stage 45)

| Document | Status |
|----------|--------|
| `stage45-ws0-structure.md` | ✅ |
| `stage45-ws1-kerberos-protocol.md` | ✅ |
| `stage45-ws2-engines.md` | ✅ |
| `stage45-ws3-cli.md` | ✅ |
| `stage45-ws4-spine.md` | ✅ |
| `stage45-ws5-test-harness.md` | ✅ |
| `stage45-certification.md` | ✅ (declares CLOSED but lacks live evidence) |
| `stage45-ws0-summary.md` | ✅ |

**Total**: 8 documents (exceeds ≤6 limit)

---

## Next Steps

Per the Stage 46 directive, **Stage 46 must NOT proceed** until Stage 45 live verification is complete. The Stage 45 lab harness must be implemented and the 6 integration tests must pass against a live Samba4 DC.

**Owner**: Stage 45 implementation team
**Action**: Implement Samba4 lab harness, seed script, integration tests, execute live qualification, update certification.