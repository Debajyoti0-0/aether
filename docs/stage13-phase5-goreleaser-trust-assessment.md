# Stage 13 Phase 5 — GoReleaser / Provenance / Cosign Assessment

**Timestamp:** 2026-09-16
**Scope:** Evaluate GoReleaser upgrade for provenance/cosign integration

---

## Current State

| Component | Version | Status |
|-----------|---------|--------|
| GoReleaser | v2.18.1 | CURRENT |
| Go | 1.27.1 | Compatible |
| SBOM | cyclonedx-json via syft | WORKING |
| Checksums | SHA-256 | WORKING |
| Cosign signing | CI workflow only (not in goreleaser) | PARTIAL |
| Provenance (SLSA) | Not configured | MISSING |
| Artifact signing | CI workflow only | PARTIAL |

---

## GoReleaser Version Analysis

### Current: v2.18.1 (Released ~2024)
**Limitations:**
- No native `signs` section support for cosign keyless signing
- No native `attestations` section for SLSA provenance
- SBOM generation works but limited to syft formats
- Cosign signing must be done in CI workflow (not integrated)

### Target: v2.19+ (Released ~2024)
**New Features:**
- Native `signs` section with cosign support
- Native `attestations` section for SLSA provenance
- Improved SBOM handling
- Better artifact signing integration
- Enhanced verification workflows

### Breaking Changes in v2.x (v2.19+)
- Some configuration fields deprecated
- Archive format changes possible
- Signing workflow changes
- May require config migration

---

## Upgrade Assessment

### Required Changes for v2.19+
```yaml
# Current .goreleaser.yml (v2.18.1 compatible)
sboms:
  - artifacts: archive
    formats:
      - cyclonedx-json

# v2.19+ would support:
signs:
  - artifacts: all
    cmd: cosign
    args:
      - sign-blob
      - --yes
      - --output-signature=${signature}
      - --output-certificate=${certificate}
      - ${artifact}

attestations:
  - name: slsa-provenance
    cmd: goreleaser
    args:
      - attest
      - --format=slsa-provenance
      - --output=${artifact}
```

### Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Config breaking changes | MEDIUM | HIGH | Test snapshot build first |
| Artifact naming changes | LOW | MEDIUM | Compare manifests |
| SBOM format changes | LOW | LOW | Verify CycloneDX output |
| CI workflow compatibility | LOW | MEDIUM | Test in CI before tag |
| Permission changes | LOW | HIGH | Verify OIDC/attestations permissions |

---

## Upgrade Process (If Approved)

### Phase 1: Isolated Test
```bash
# 1. Create test branch
git checkout -b test-goreleaser-v219

# 2. Update goreleaser version in release.yml
# GORELEASER_VERSION: 'v2.19.0' (or latest v2.x)

# 3. Update .goreleaser.yml with signs/attestations
# 4. Test snapshot build
goreleaser release --snapshot --clean

# 5. Compare artifacts with v2.18.1 baseline
# - Check artifact names, sizes, checksums
# - Verify SBOM format
# - Verify signing (if cosign available)
# 6. Run full test suite
go test ./...
```

### Phase 2: CI Validation
```bash
# Push test branch
git push origin test-goreleaser-v219

# Monitor CI runs
# - Verify CI passes
# - Verify Release workflow (if test tag pushed)
# - Verify artifact integrity
```

### Phase 3: Decision
| Outcome | Action |
|---------|--------|
| All tests pass, artifacts stable | Merge upgrade, proceed to RC2 |
| Breaking changes found | Fix config, re-test |
| Critical regression | Revert, defer upgrade |

---

## Current Decision: **DEFERRED — JUSTIFIED**

### Rationale
1. **Current goreleaser (v2.18.1) works correctly** for core release artifacts
2. **SBOM generation works** (cyclonedx-json via syft)
3. **Checksums work** (SHA-256)
4. **Cosign signing works** in CI workflow (not in goreleaser, but functional)
5. **Provenance is a nice-to-have** for GA, not a blocker for RC2
6. **Upgrade risk** — breaking changes could delay RC2 qualification
7. **RC2 is Production-Limited** — provenance not required for limited release

### When to Upgrade
- Before GA (`v4.0.0`) release
- When goreleaser v2.19+ is stable and tested
- When provenance becomes a release requirement
- When time permits proper testing window

---

## Provenance Alternative (Current)

### Current: CI-Based Cosign Signing
The release workflow (`release.yml`) already implements:
- Cosign keyless signing via `sigstore/cosign-installer`
- SBOM generation via `anchore/syft-action`
- Artifact verification with tamper tests
- Checksum verification

### Missing for Full SLSA
- Build provenance attestation (SLSA predicate)
- Reproducible build verification
- Hermetic build environment

### Acceptable for RC2
For **Production-Limited RC2**, the current CI-based approach is sufficient:
- ✅ Artifacts signed (cosign keyless for Linux/macOS)
- ✅ Checksums verified
- ✅ SBOMs generated
- ✅ Tamper tests pass
- ⚠️ Windows Authenticode waived (B4)
- ⚠️ Provenance not generated (deferred)

---

## Gate G25 Status

| Sub-gate | Status |
|----------|--------|
| G25.1 Current behavior recorded | ✅ Documented |
| G25.2 Upgrade changes identified | ✅ Documented |
| G25.3 Snapshot test performed | ⏳ NOT RUN (deferred) |
| G25.4 Artifact comparison done | ⏳ NOT RUN (deferred) |
| G25.5 Decision recorded | ✅ **DEFERRED — JUSTIFIED** |

**G25 Verdict: DEFERRED — JUSTIFIED** — Upgrade not required for RC2; will be evaluated for GA.

---

## Recommendation

**Defer GoReleaser upgrade to v2.19+ until GA preparation phase.**

Current release pipeline is functional for RC2:
- All artifacts built and verified
- SBOMs generated
- Checksums verified
- Cosign signing in CI
- Tamper tests passing
- Only provenance/attestation in goreleaser missing (acceptable for RC2)

---

*Generated by Stage 13 Phase 5 — GoReleaser/Provenance/Cosign Assessment*