# Stage 13 Phase 0 — Baseline Reconciliation & Candidate Identity

**Timestamp:** 2026-09-16
**Stage:** 13 (Closure Retry — Stage 12 Retry)
**Operator:** Stage 13 execution agent

---

## Repository Identity

| Property | Value |
|----------|-------|
| Repository path | C:\dev\aether |
| Remote URL | https://github.com/Debajyoti0-0/aether |
| Current branch | master |
| Current HEAD | f1242ddf8795addfed1835079f86b62a20e0062e |
| Current branch (remote) | master |
| Working tree status | Clean (0 modified, 38 untracked docs/tests) |
| Remote URL | https://github.com/Debajyoti0-0/aether |

---

## Candidate Commit Identity

| Property | Value |
|----------|-------|
| Candidate commit | f1242ddf8795addfed1835079f86b62a20e0062e |
| Candidate message | "Stage 12: Final report and Stage 13 handoff" |
| Candidate author | Debajyoti Haldar |
| Candidate date | Wed Sep 16 13:16:23 2026 +0530 |
| Parent commit | 55f37b1 (Stage 12 Phase 6: Supply chain trust) |
| Ancestry | f1242dd → 55f37b1 → 6b88f87 → d26aae7 → b807ad2 → 8e956a2 → 28bcb99 → ... |
| Distance from Stage 10 RC1 tag (28bcb99) | 6 commits ahead |
| Distance from Stage 12 exit (f1242dd) | 0 commits (this IS the Stage 12 exit commit) |

---

## Version Identity

| Source | Value |
|--------|-------|
| VERSION file | 4.0.0-rc1 |
| `aether --version` (not built) | N/A |
| `internal/version.Version` (source) | "dev" (build-time override via ldflags) |
| Stage 10 declared | 4.0.0-rc2 |
| Stage 11 declared | 4.0.0-rc2 (Production-Limited) |
| Stage 12 declared | 4.0.0-rc2 (Production-Limited) |
| **Actual VERSION file** | **4.0.0-rc1** (STALE) |

**CRITICAL:** VERSION file still reads `4.0.0-rc1` despite 3 stages declaring `4.0.0-rc2`.

---

## Tag Identity

| Tag | Local Target | Remote Target | Status |
|-----|--------------|---------------|--------|
| v3.5.0-stage4-backfill | e1065b5 | e1065b5 | Stable |
| v3.6.0-stage5-backfill | 1ea4191 | 1ea4191 | Stable |
| **v4.0.0-rc1** | **28bcb99** | **28bcb99** | **CONTESTED** (moved twice) |
| **v4.0.0-rc2** | **NOT EXISTS** | **NOT EXISTS** | **MISSING** |

### Tag Movement History (v4.0.0-rc1)

| Move | From | To | Commit Message |
|------|------|-----|----------------|
| 1 (Stage 9) | e3154ce | 66b3600 | "chore: bump version to 4.0.0-rc1" |
| 2 (Stage 10) | 66b3600 | 28bcb99 | "feat: add Azure Key Vault KeyProvider implementation for HSM/KMS custody (B5)" |

**INTEGRITY VIOLATION:** Published tag `v4.0.0-rc1` moved **twice** after initial publication.

---

## Candidate Commit Ancestry

```
f1242dd (HEAD, Stage 13 baseline)
└── 55f37b1 Stage 12 Phase 6: Supply chain trust
    └── 6b88f87 Stage 12: B3 race fixes (mutex), B5 contract refactor (capabilities), B7 Validate prep
        └── d26aae7 stage4 backfill: final report
            └── b807ad2 stage4 backfill: storage & spine hardening evidence (B4-G01..G16)
                └── 8e956a2 fix(store): complete Azure KV provider — dedupe extractVersionFromKID, fail-closed Ed25519 contract
                    └── 28bcb99 (tag: v4.0.0-rc1) feat: add Azure Key Vault KeyProvider implementation for HSM/KMS custody (B5)
                        └── 58e6432 goreleaser: add SBOM generation (cyclonedx-json)
                        └── 0b20e6d ci: upgrade to golangci-lint-action@v7 for v2 support
                        └── 150962a ci: use specific golangci-lint v2.13.2 version
                        └── cb032e5 ci: use golangci-lint v2 explicitly
                        └── 56fc1e5 fix: resolve golangci-lint failures (ineffassign); disable staticcheck style-only checks
                        └── 2c6e909 chore: update goreleaser config for Stage 10; add build artifacts to gitignore
                        └── 66b3600 chore: bump version to 4.0.0-rc1
                        └── ... (Stage 9 and earlier)
```

**Commits between Stage 10 RC1 tag (28bcb99) and current HEAD (f1242dd): 6 commits**

---

## Existing CI Status

### Workflows at Candidate Commit (f1242dd)

| Workflow | File | Triggers | Key Jobs |
|----------|------|----------|----------|
| CI | .github/workflows/ci.yml | push, PR | governance, build(3), vet, test(-race), test(-race,windows), lint, vuln, integration |
| Release | .github/workflows/release.yml | tag push (v*) | validate, goreleaser, windows-sign, verify, publish |
| Race Isolation | .github/workflows/race-isolation.yml | workflow_dispatch, push to main | race-api, race-workspace, race-store, race-engine, race-protocol, race-transport, race-all |

### CI Run Status (from Stage 12)

| Workflow | Last Known Run | Status | Notes |
|----------|----------------|--------|-------|
| CI (main) | Not verified for f1242dd | Unknown | Need to check |
| Race Isolation | Created in Stage 12 | Not yet run for f1242dd | Triggered on push to main |
| Release Validate | Triggered on tag push only | Not run for f1242dd | Only runs on tag push |

**CRITICAL:** No CI runs verified for candidate commit f1242dd. Race isolation workflow created but not yet executed on this commit.

---

## Existing Security Control Status

| Control | Configured? | Evidence |
|---------|-------------|----------|
| Branch protection | Unknown | Not verified |
| Required status checks | Unknown | Not verified |
| Required PR reviews | Unknown | Not verified |
| Force-push restrictions | Unknown | Not verified |
| Secret scanning | Unknown | Not verified |
| Push protection | Unknown | Not verified |
| Dependabot alerts | Unknown | Not verified |
| Code scanning | Unknown | Not verified |
| Tag protection | Unknown | Not verified |
| Release permissions | Unknown | Not verified |
| Environment protection | Unknown | Not verified |

---

## Existing Artifact Status

| Artifact Type | Status | Location |
|---------------|--------|----------|
| Release binaries | Not built for f1242dd | N/A |
| SBOMs | Not generated for f1242dd | N/A |
| Checksums | Not generated for f1242dd | N/A |
| Provenance | Not configured (goreleaser v2.18.1) | N/A |
| Cosign signatures | Not generated | N/A |
| Authenticode signature | Not available (no EV cert) | N/A |

### Last Successful Snapshot Build (Stage 12)

- Commit: 55f37b1 (1 commit before f1242dd)
- 4 platform archives generated (linux_amd64, linux_arm64, darwin_amd64, windows_amd64)
- 4 SBOMs generated (SPDX via syft)
- Checksums.txt generated
- No provenance, no cosign in goreleaser (v2.18.1 limitation)

---

## Blocker Status (Inherited from Stage 12)

| Blocker | Stage 12 Status | Current Reality |
|---------|-----------------|-----------------|
| B1 Live Entra | WAIVED (2027-06-30) | Waiver documented |
| B2 IMDS | WAIVED (2027-06-30) | Waiver documented |
| **B3 Race Detector** | **PARTIAL** | Mutex fixes applied; CI race isolation workflow created; **CI run not yet observed for f1242dd** |
| **B4 EV Authenticode** | **WAIVED** | Formal waiver filed (2027-03-31 expiry) |
| **B5 Azure KV** | **PARTIAL** | Contract fixed (capability-aware); mutex added; **no live validation** |
| **B7 Release Validate** | **PARTIAL** | Local PASS; CI not yet run for f1242dd |

---

## Supply Chain Status (Inherited from Stage 12)

| Item | Status | Evidence |
|------|--------|----------|
| GitHub Actions pinned to SHAs | ✅ DONE | ci.yml, release.yml updated with SHA pins |
| VERIFY.md published | ✅ DONE | VERIFY.md at repo root |
| Tamper tests (4/4) | ✅ DONE | In release.yml verify job |
| SBOM generation | ✅ DONE | .goreleaser.yml + syft |
| Provenance (SLSA) | ⚠️ DEFERRED | Requires goreleaser ≥ v2.19 |
| Cosign in goreleaser | ⚠️ DEFERRED | Requires goreleaser ≥ v2.19 |
| Branch protection | ⏳ NOT VERIFIED | Not checked |
| Secret scanning | ⏳ NOT VERIFIED | Not checked |
| Dependabot | ⏳ NOT VERIFIED | Not checked |

---

## Drift Reconciliation (Inherited from Stage 12)

| Metric | Stage 5 Claim | Stage 11 Claim | Stage 12 Claim | Actual (this audit) |
|--------|---------------|----------------|----------------|---------------------|
| Package count | 27 | 34 | 38 | **38** (confirmed via `go list ./...`) |
| Fuzz target count | 19 | 6 | 7 | **21** (confirmed via grep) |
| RC1 tag target | e3154ce | 28bcb99 | 28bcb99 | 28bcb99 (CONTESTED) |
| RC2 tag | N/A | "to be created" | "to be created" | **NOT EXISTS** |

**Fuzz targets found (21):**
- internal/api: 4 (FuzzReadFrame, FuzzDecodePayload, FuzzEncodePayload, FuzzEnvelopeValidation)
- internal/engine/exec: 2 (FuzzParseIMDSIdentityToken, FuzzParseInstanceMetadata)
- internal/engine/token: 3 (FuzzParsePRT, FuzzParseOAuthTokens, FuzzValidatePRT)
- internal/protocol/msoapx: 4 (FuzzDecodeKey, FuzzComputeSessionKeyProof, FuzzURLValuesEncoding, FuzzDeriveNonce)
- internal/protocol/saml: 2 (FuzzParseAssertion, FuzzAssertionXMLMarshal)
- internal/protocol/wstrust: 4 (FuzzParseRSTR, FuzzExtractAssertionWS, FuzzBuildRST, FuzzParseMEX)
- internal/workspace: 2 (FuzzLoadRecord, FuzzAuditLogEntry)

---

## Toolchain Versions

| Tool | Version |
|------|---------|
| Go | 1.27.1 windows/amd64 |
| Goreleaser | v2.18.1 |
| Git | 2.x |
| GitHub CLI | Not authenticated |

---

## Phase 0 Gates (S13-G00)

| Gate | Requirement | Status | Evidence |
|------|-------------|--------|----------|
| S13-G00.1 | Baseline captured | ✅ | This document |
| S13-G00.2 | Candidate commit reconciled | ✅ | f1242dd confirmed |
| S13-G00.3 | RC1 lineage documented | ✅ | 3 moves documented |
| S13-G00.4 | RC2 existence verified | ✅ | NOT EXISTS (confirmed) |

---

## Known Limitations Inherited from Stage 12

1. **VERSION file stale** — reads `4.0.0-rc1` despite 3 stages declaring `4.0.0-rc2`
2. **Tag `v4.0.0-rc1` moved twice** — CONTESTED, must not move again
3. **Tag `v4.0.0-rc2` missing** — must be created at qualified commit
4. **B3 Race Detector** — mutex fixes applied, CI evidence pending
5. **B4 EV Authenticode** — waived (2027-03-31 expiry), no cert
6. **B5 Azure KV** — contract fixed, no live validation
7. **B7 Release Validate** — local PASS, CI evidence pending
7. **Provenance/Cosign in goreleaser** — deferred (goreleaser v2.18.1)
8. **Branch protection/secret scanning** — not verified
9. **VERSION file stale** — still reads `4.0.0-rc1`
10. **No RC2 tag created** — must freeze at qualified commit

---

## Phase 0 Verdict

**S13-G00: BASELINE RECONCILED — PASS**

Candidate commit `f1242dd` is confirmed as the Stage 12 exit commit. All lineage, drift, and blocker status documented. No new design documents created in this phase.

**Next:** Phase 1 — CI Race-Isolation Qualification (fetch CI run evidence for race-isolation workflow on f1242dd).

---

*Generated by Stage 13 Phase 0 — Baseline Reconciliation & Candidate Identity*