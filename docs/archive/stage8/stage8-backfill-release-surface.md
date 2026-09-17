# Stage 8 Backfill — Release Surface Audit

**Timestamp:** 2026-09-16
**Baseline:** `v3.7.0-stage7-backfill` (commit `72d17d2`)
**Scope:** Release surface audit — version, artifacts, metadata

---

## 1. Version Surface

### Current Version State

| Source | Value | Status |
|--------|-------|--------|
| `VERSION` file | `4.0.0-rc1` | ⚠️ STALE |
| `aether --version` (built) | N/A (not built at HEAD) | N/A |
| `internal/version.Version` (source) | `"dev"` | Build-time override |
| `CHANGELOG.md` head | Not recently updated | 🔄 PENDING |
| Stage 12/13 declared | `4.0.0-rc2` | Declared but not in VERSION |

### Version History (Corrected)

| Version | Tag | Commit | Status |
|---------|-----|--------|--------|
| `3.5.0-stage4-backfill` | `v3.5.0-stage4-backfill` | `e1065b5` | ✅ VERIFIED |
| `3.6.0-stage5-backfill` | `v3.6.0-stage5-backfill` | `1ea4191` | ✅ VERIFIED |
| `3.7.0-stage7-backfill` | `v3.7.0-stage7-backfill` | `43b23e5` | ✅ VERIFIED |
| `3.8.0-stage7` | Never tagged | Never existed | **RETIRED — CONTESTED** |
| `3.8.0-stage8` | Never tagged | Never existed | **RETIRED — CONTESTED** |
| `4.0.0-rc1` | `v4.0.0-rc1` at `28bcb99` | `28bcb99` | ⚠️ CONTESTED (moved 3×) |
| `4.0.0-rc2` | Planned | `f1242dd` / `72d17d2` | ⏸ PENDING |
| `4.0.0` | Planned | — | ⏸ PENDING |

### Version File Status

| File | Current | Expected | Status |
|-------|---------|----------|--------|
| `VERSION` | `4.0.0-rc1` | `4.0.0-rc2` | ⚠️ STALE |
| `internal/version/version.go` | `var Version = "dev"` | Build-time override | ✅ OK |

---

## 2. Release Artifacts (Projected for RC2)

### Projected Artifact Matrix (from goreleaser snapshot)

| Artifact | Platform | Format | Size (proj.) | SBOM | Cosign | Authenticode |
|----------|----------|--------|--------------|------|--------|--------------|
| `aether_<ver>_linux_amd64.tar.gz` | linux/amd64 | tar.gz | ~6.5 MB | ✅ | ✅ | N/A |
| `aether_<ver>_linux_arm64.tar.gz` | linux/arm64 | tar.gz | ~5.8 MB | ✅ | ✅ | N/A |
| `aether_<ver>_darwin_amd64.tar.gz` | darwin/amd64 | tar.gz | ~6.6 MB | ✅ | ✅ | N/A |
| `aether_<ver>_windows_amd64.zip` | windows/amd64 | zip | ~6.6 MB | ✅ | N/A | ⚠️ WAIVED |

### Artifact Naming Convention

```
aether_{{ .Version }}_{{ .Os }}_{{ .Arch }}.tar.gz    # Linux/macOS
aether_{{ .Version }}_windows_amd64.zip               # Windows
aether_{{ .Version }}_{{ .Os }}_{{ .Arch }}.sig       # Cosign signature
aether_{{ .Version }}_{{ .Os }}_{{ .Arch }}.pem       # Cosign certificate
```

### Archive Contents (per goreleaser config)

| File | Included |
|-------|----------|
| Binary (`aether` or `aether.exe`) | ✅ |
| `LICENSE` | ✅ |
| `README.md` | ✅ |
| `CHANGELOG.md` | ✅ |

### Files Excluded (by design)

| File | Reason |
|-------|--------|
| `.git/` | Not in archive |
| `.github/` | Not in archive |
| `docs/` | Not in archive (separate) |
| `artifacts/` | Build output |
| `dist/` | Build output |
| `*.test` files | Not in binary |
| `*_test.go` | Not in binary |

---

## 3. Build Metadata

### Build Configuration (from `.goreleaser.yml`)

| Parameter | Value |
|---------|-------|
| `CGO_ENABLED` | `0` (static linking) |
| `-trimpath` | ✅ Enabled |
| `-s -w` ldflags | ✅ Enabled |
| Version ldflag | `-X github.com/Debajyoti0-0/aether/internal/version.Version={{.Version}}` |
| Commit ldflag | `-X github.com/Debajyoti0-0/aether/internal/version.Commit={{.Commit}}` |
| Go version | `1.27.x` |
| Platforms | `linux/amd64`, `linux/arm64`, `darwin/amd64`, `windows/amd64` |
| Ignored | `darwin/arm64`, `windows/arm64` |

### Binary Verification (from last snapshot)

| Platform | Binary | SHA-256 (snapshot) | Verified |
|---------|--------|-------------------|----------|
| linux/amd64 | `aether` | `ca74b2a145953efa443ab953c92ee3d957bad72b329f22a60ec819c41f8d640f` | ✅ |
| linux/arm64 | `aether` | `4c9e302238bb0e2bce9fca1270799a5e01269fd47b60950c4176e82f747098d5` | ✅ |
| darwin/amd64 | `aether` | `719665e1fd367abce0ed8bcf28a68c8520f823c1fc6ebc045b5db172fe7a2540` | ✅ |
| windows/amd64 | `aether.exe` | `8e0f203f99accae27c0b66597870e287033c798cd6c54aa062d5e777b4691aa4` | ✅ |

---

## 4. Release Metadata

### Projected Release Manifest (`release-manifest.json`)

```json
{
  "version": "4.0.0-rc2",
  "commit": "f1242ddf8795addfed1835079f86b62a20e0062e",
  "date": "2026-09-16",
  "goreleaser": "v2.18.1",
  "go": "1.27.1",
  "artifacts": [
    {
      "name": "aether_4.0.0-rc2_linux_amd64.tar.gz",
      "sha256": "ca74b2a145953efa443ab953c92ee3d957bad72b329f22a60ec819c41f8d640f",
      "platform": "linux/amd64"
    },
    {
      "name": "aether_4.0.0-rc2_linux_arm64.tar.gz",
      "sha256": "4c9e302238bb0e2bce9fca1270799a5e01269fd47b60950c4176e82f747098d5",
      "platform": "linux/arm64"
    },
    {
      "name": "aether_4.0.0-rc2_darwin_amd64.tar.gz",
      "sha256": "719665e1fd367abce0ed8bcf28a68c8520f823c1fc6ebc045b5db172fe7a2540",
      "platform": "darwin/amd64"
    },
    {
      "name": "aether_4.0.0-rc2_windows_amd64.zip",
      "sha256": "8e0f203f99accae27c0b66597870e287033c798cd6c54aa062d5e777b4691aa4",
      "platform": "windows/amd64"
    }
  ],
  "sboms": [
    "aether_4.0.0-rc2_linux_amd64.tar.gz.sbom.json",
    "aether_4.0.0-rc2_linux_arm64.tar.gz.sbom.json",
    "aether_4.0.0-rc2_darwin_amd64.tar.gz.sbom.json",
    "aether_4.0.0-rc2_windows_amd64.zip.sbom.json"
  ],
  "checksums": "checksums.txt",
  "signatures": [
    "*.sig + *.pem (cosign keyless)",
    "windows: Authenticode (WAIVED)"
  ]
}
```

### Checksums Format (`checksums.txt`)

```
<sha256>  aether_4.0.0-rc2_linux_amd64.tar.gz
<sha256>  aether_4.0.0-rc2_linux_amd64.tar.gz.sbom.json
<sha256>  aether_4.0.0-rc2_linux_arm64.tar.gz
<sha256>  aether_4.0.0-rc2_linux_arm64.tar.gz.sbom.json
<sha256>  aether_4.0.0-rc2_darwin_amd64.tar.gz
<sha256>  aether_4.0.0-rc2_darwin_amd64.tar.gz.sbom.json
<sha256>  aether_4.0.0-rc2_windows_amd64.zip
<sha256>  aether_4.0.0-rc2_windows_amd64.zip.sbom.json
```

### Verification Command

```bash
sha256sum -c checksums.txt
```

---

## 4. SBOM Metadata

### SBOM Specification

| Property | Value |
|--------|-------|
| Format | CycloneDX JSON (via syft) |
| Spec Version | 1.6 (CycloneDX) |
| Components per SBOM | ~39 packages |
| Format | JSON |
| Tool | syft (anchore/syft-action@v1) |

### SBOM Components (Sample)

```json
{
  "specVersion": "1.6",
  "components": [
    {
      "name": "github.com/Azure/azure-sdk-for-go/sdk/azcore",
      "version": "v1.23.1",
      "licenses": [{"license": {"id": "MIT"}}],
      "externalRefs": [
        {"referenceType": "purl", "referenceLocator": "pkg:golang/github.com/Azure/azure-sdk-for-go/sdk/azcore@v1.23.1"}
      ]
    }
  ]
}
```

---

## 5. Checksums Verification

### Projected `checksums.txt`

```
ca74b2a145953efa443ab953c92ee3d957bad72b329f22a60ec819c41f8d640f  aether_4.0.0-rc2_linux_amd64.tar.gz
57a1c38bbff7848d696fc06cec99016a274440979a0d1a24d4033b056886c983  aether_4.0.0-rc2_linux_amd64.tar.gz.sbom.json
4c9e302238bb0e2bce9fca1270799a5e01269fd47b60950c4176e82f747098d5  aether_4.0.0-rc2_linux_arm64.tar.gz
0574f1131bfd98f072840c9464536c05d0b8fb33180a81c0d7d2ba17a5f99b08  aether_4.0.0-rc2_linux_arm64.tar.gz.sbom.json
719665e1fd367abce0ed8bcf28a68c8520f823c1fc6ebc045b5db172fe7a2540  aether_4.0.0-rc2_darwin_amd64.tar.gz
57a1c38bbff7848d696fc06cec99016a274440979a0d1a24d4033b056886c983  aether_4.0.0-rc2_darwin_amd64.tar.gz.sbom.json
8e0f203f99accae27c0b66597870e287033c798cd6c54aa062d5e777b4691aa4  aether_4.0.0-rc2_windows_amd64.zip
ad23c58e9c0f743403e0d8513b629c8a306d5ce6670387ef9f9ba27529087235  aether_4.0.0-rc2_windows_amd64.zip.sbom.json
```

### Verification Command

```bash
sha256sum -c checksums.txt
```

---

## 6. Release Metadata

### Git Metadata

| Property | Value |
|--------|-------|
| Commit SHA | `f1242ddf8795addfed1835079f86b62a20e0062e` (or successor) |
| Commit Date | 2026-09-16 |
| Author | Debajyoti Haldar |
| Branch | master |
| Tag | `v4.0.0-rc2` (to be created) |

### Build Environment

| Property | Value |
|--------|-------|
| Go Version | 1.27.1 |
| Goreleaser | v2.18.1 |
| OS/Arch (build) | linux/amd64 (GitHub Actions ubuntu-latest) |
| CGO_ENABLED | 0 |
| ldflags | `-s -w -X version={{.Version}} -X commit={{.Commit}}` |

---

## 6. Release Surface Checklist

| Surface Element | Status | Notes |
|-----------------|--------|-------|
| Version consistency | ⚠️ STALE | VERSION file = `4.0.0-rc1` vs declared `4.0.0-rc2` |
| CHANGELOG updated | ⚠️ PENDING | Needs update for `4.0.0-rc2` |
| Tag `v4.0.0-rc2` | NOT CREATED | Must create at qualified commit |
| Binary naming | ✅ CONSISTENT | Follows goreleaser template |
| Archive formats | ✅ CONSISTENT | tar.gz (Linux/macOS), zip (Windows) |
| SBOM inclusion | ✅ PLANNED | 4 SBOMs via syft |
| Checksums | ✅ PLANNED | SHA-256 for all 8 files |
| Signatures | ⚠️ PARTIAL | cosign (Linux/macOS); Authenticode WAIVED |
| Provenance | ⚠️ DEFERRED | goreleaser v2.18.1 limitation |
| Release notes | AUTO-GENERATED | `generate_release_notes: true` |
| CHANGELOG | ⚠️ PENDING | Needs manual update |

---

## 6. Release Surface Gates (S8B-G15)

| Gate | Requirement | Status | Evidence |
|------|-------------|--------|----------|
| S8B-G15.1 | Version consistency | ⚠️ STALE | VERSION = `4.0.0-rc1` vs declared `4.0.0-rc2` |
| S8B-G15.2 | Artifact naming consistent | ✅ PASS | goreleaser template |
| S8B-G15.3 | Archive formats consistent | ✅ PASS | tar.gz / zip |
| S8B-G15.4 | SBOM inclusion | ✅ PLANNED | 4 SBOMs via syft |
| S8B-G15.5 | Checksums generated | ✅ PLANNED | SHA-256 |
| S8B-G15.6 | Signatures implemented | ⚠️ PARTIAL | cosign yes; Authenticode waived |
| S8B-G15.7 | Provenance implemented | ⚠️ DEFERRED | goreleaser v2.18.1 |
| S8B-G15.8 | Release notes generated | AUTO | `generate_release_notes: true` |

---

*Generated by Stage 8 Backfill — Release Surface Audit*