# Stage 11 Phase 5 — Artifact Trust / Supply-Chain Closure Audit

**Timestamp:** 2026-09-16
**Scope:** Build artifacts, SBOMs, checksums, signing readiness, action pinning

## Build Verification (Snapshot)

### Goreleaser Snapshot Run
```bash
$ goreleaser release --snapshot --clean
# SUCCESS after 35s
```

### Artifacts Generated

| Artifact | Format | Size | SHA-256 (verified) |
|----------|--------|------|-------------------|
| linux_amd64 | tar.gz | 6.5 MB | `ca74b2a145953efa443ab953c92ee3d957bad72b329f22a60ec819c41f8d640f` ✅ |
| linux_arm64 | tar.gz | 5.8 MB | `4c9e302238bb0e2bce9fca1270799a5e01269fd47b60950c4176e82f747098d5` ✅ |
| darwin_amd64 | tar.gz | 6.6 MB | `719665e1fd367abce0ed8bcf28a68c8520f823c1fc6ebc045b5db172fe7a2540` ✅ |
| windows_amd64 | zip | 6.6 MB | `8e0f203f99accae27c0b66597870e287033c798cd6c54aa062d5e777b4691aa4` ✅ |

### Checksums Verification
- **checksums.txt** generated with all 8 artifacts (4 archives + 4 SBOMs)
- All SHA-256 hashes verified against actual files ✅

### SBOM Verification
- **Format**: SPDX 2.3 (syft default, not CycloneDX as configured)
- **Packages per SBOM**: 39 (Go module dependencies)
- **Spec Version**: SPDX-2.3 ✅
- **Components**: Each SBOM includes all transitive dependencies with checksums, licenses, CPEs, purls
- **Tool**: syft (anchore/syft-action@v1)

### Archive Contents Verification
Each archive contains:
- `aether` (or `aether.exe` for Windows)
- `LICENSE`
- `README.md`
- `CHANGELOG.md`

No unexpected files, no development artifacts, no secrets detected.

## Binary Inspection

### Build Flags Verification
```yaml
# .goreleaser.yml
ldflags:
  - -s -w          # Strip debug info and symbol table
  - -X ...Version={{.Version}}
  - -X ...Commit={{.Commit}}
flags:
  - -trimpath      # Remove file paths from binary
env:
  - CGO_ENABLED=0  # Static linking, no CGO
```

### Binary Checks (Sample: linux_amd64)
```
$ file dist/aether_linux_amd64_v1/aether
# ELF 64-bit LSB executable, x86-64, version 1 (SYSV), statically linked, stripped
```
- Statically linked ✅
- Stripped (`-s -w`) ✅
- No debug symbols ✅

### Version Embedding
```bash
$ ./dist/aether_linux_amd64_v1/aether --version
# aether version v3.6.0-stage5-backfill-next (commit <hash>)
```
Version and commit correctly embedded via ldflags.

## Signing Readiness Audit

### Current State

| Artifact | Cosign (Keyless) | Authenticode (EV) | Status |
|----------|------------------|-------------------|--------|
| Linux/macOS archives | ✅ CI workflow configured | N/A | Ready (needs CI run) |
| Windows binary | N/A | ❌ No EV cert | BLOCKED |
| SBOMs | ✅ CI workflow configured | N/A | Ready |
| Checksums | ✅ CI workflow configured | N/A | Ready |
| Manifest | ✅ CI workflow configured | N/A | Ready |

### Goreleaser Config Gaps
- **No `signs` section** — Cosign signing not integrated into goreleaser
- **No `attestations` section** — SLSA provenance not generated
- **Cosign only in CI workflow** — Not in goreleaser, so local `goreleaser release` won't sign

### Windows Signing (B4)
- Workflow has conditional step: `if: env.AUTHENTICODE_CERT != ''`
- Requires GitHub Secrets: `AUTHENTICODE_CERT` (base64 PFX), `AUTHENTICODE_PASSWORD`
- **No certificate procured** — BLOCKED

### Signing Evidence Required (for GA)
| Evidence | Source | Status |
|----------|--------|--------|
| `signtool verify /pa /all aether.exe` | Windows-sign job | ❌ Not run |
| `cosign verify-blob --signature ...` | Verify job | ❌ Not run |
| Timestamp verification | DigiCert TS | ❌ Not configured |

## Action Pinning Audit (Supply Chain Hardening - G6)

### Current State: ALL UNPINNED

| Workflow | Action | Current | Required SHA |
|----------|--------|---------|--------------|
| ci.yml | actions/checkout | `@v4` | ❌ |
| ci.yml | actions/setup-go | `@v5` | ❌ |
| ci.yml | golangci/golangci-lint-action | `@v7` | ❌ |
| release.yml | actions/checkout | `@v4` | ❌ |
| release.yml | actions/setup-go | `@v5` | ❌ |
| release.yml | sigstore/cosign-installer | `@v3` | ❌ |
| release.yml | anchore/syft-action | `@v1` | ❌ |
| release.yml | goreleaser/goreleaser-action | `@v6` | ❌ |
| release.yml | actions/download-artifact | `@v4` | ❌ |
| release.yml | actions/upload-artifact | `@v4` | ❌ |
| release.yml | softprops/action-gh-release | `@v1` | ❌ |

### Pinning Procedure
For each action, resolve to commit SHA:
```bash
# Example: actions/checkout@v4
# https://github.com/actions/checkout/tags -> v4.2.2 -> commit SHA
```

### Required Changes
1. Replace all `@vX` with `@<commit-SHA>`
2. Document update procedure in CONTRIBUTING.md
3. Add Dependabot for action updates (pinned SHAs still need updates)

## Branch Protection & Secret Scanning

### Not Verified (Repository Settings)
| Protection | Status |
|------------|--------|
| Require PR reviews | ❓ Unknown |
| Require status checks (CI) | ❓ Unknown |
| Require signed commits | ❓ Unknown |
| Require linear history | ❓ Unknown |
| Admin enforcement | ❓ Unknown |
| Secret scanning | ❓ Unknown |
| Push protection | ❓ Unknown |
| Dependabot alerts | ❓ Unknown |

**Action Required**: Configure via GitHub UI or `gh api` / `gh repo edit`.

## Independent Verification (G7)

### VERIFY.md Status
- **Not created** — Stage 10 report says "NOT TESTED"
- Required: Stranger-reproducible download → checksum → SBOM → cosign verify → Authenticode verify

### Reproducibility Test
```bash
# Fresh clone test (not executed)
git clone https://github.com/Debajyoti0-0/aether
cd aether
go build ./...
go test -count=1 ./...
goreleaser release --snapshot --clean
# Compare artifacts with published release
```

## Tamper Tests (Release Workflow Verify Job)

### Implemented in CI (4 of 9 claimed)
| Test | Description | Status |
|------|-------------|--------|
| 1 | Modified binary fails checksum | ✅ Implemented |
| 2 | Modified manifest fails signature | ✅ Implemented |
| 3 | Modified SBOM fails signature | ✅ Implemented |
| 4 | Modified checksums fail | ✅ Implemented |
| 5-9 | Additional tamper vectors | ❌ Not implemented |

**Discrepancy**: Workflow comment says "All 9 tamper tests PASSED" but only 4 implemented.

## Release Workflow Artifact Attachment

### Publish Job
- Uses `softprops/action-gh-release@v1`
- Attaches all artifacts from `release-artifacts/`
- Sets `prerelease: true` for `-rc`, `-alpha`, `-beta` tags
- **Not tested** — No tag push since Stage 10

## Summary: Supply Chain Gates

| Gate | Requirement | Status | Evidence |
|------|-------------|--------|----------|
| G2 | Artifact Trust + Provenance | 🔄 PARTIAL | SBOM ✅, Checksums ✅, Provenance ❌, Signing ❌ |
| G6 | CI/CD Supply-Chain Hardening | 🔄 PARTIAL | Lint ✅, Action pinning ❌, Branch protection ❓ |
| G7 | Independent Reproduction | ⏳ PENDING | VERIFY.md ❌, Repro test ❌ |
| G8 | Adversarial Audit | ⏳ PENDING | Not executed |

## Critical Blockers for GA

1. **B4**: EV Authenticode certificate not procured
2. **G2**: Provenance generation not configured in goreleaser
3. **G2**: Cosign signing not in goreleaser (CI only)
4. **G6**: All GitHub Actions unpinned
5. **G6**: Branch protection, secret scanning not verified
6. **G7**: VERIFY.md not published
7. **G8**: Adversarial audit not executed

## Recommended Immediate Actions

1. **Pin all GitHub Actions to commit SHAs** (G6)
2. **Add `signs` and `attestations` to `.goreleaser.yml`** (G2)
3. **Configure branch protection via `gh repo edit`** (G6)
4. **Create `VERIFY.md` with reproduction steps** (G7)
5. **Procure EV certificate or file waiver** (B4/G3)

---

*Generated by Stage 11 Phase 5 Artifact Trust Audit*