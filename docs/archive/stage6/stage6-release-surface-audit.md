# Aether — Stage 6 Version / Release Surface Audit

**Baseline:** Stage 3 (3.4.0-stage3), HEAD `350dd91`
**Phase:** 8 of 9
**Goal:** Ensure version truth consistency; no undocumented breaking changes; release surface coherent

---

## 1. Version Truth Consistency

| Source | Value | Consistent? |
|--------|-------|-------------|
| `VERSION` file | `3.4.0-stage3` | ✓ Source of truth |
| `internal/version/version.go` (default) | `dev` | ✓ Overridden at build |
| `go.mod` (module path) | `github.com/Debajyoti0-0/aether` | ✓ No version in module path |
| `bin/aether.exe --version` | `3.4.0-stage3` | ✓ Matches VERSION (rebuilt at `d6fb93e`) |
| `CHANGELOG.md` head | `v3.4.0-stage3 — Teamserver V2 (Stage 3...)` | ✓ Matches |
| Git tags | None | ⚠ No tags (Stage 7 will tag) |
| Makefile `VERSION` variable | `$(shell cat VERSION)` | ✓ Dynamic |
| Build script `VERSION` | `$(cat VERSION)` | ✓ Dynamic |

**Verdict:** **CONSISTENT** — All version references align at `3.4.0-stage3`.

---

## 2. CLI Surface Audit

### 2.1 Command Inventory

| Command | Subcommands | Flags | Stable Since | Breaking Changes |
|---------|-------------|-------|--------------|------------------|
| `aether` | --version, --help, --config, --log-level | Global | Stage 1 | None |
| `aether serve` | `cert init/issue/revoke/list`, `teamserver` | Stage 3 | None |
| `aether connect` | `--exec`, `--workspace` | Stage 1 | `--exec` journal-only (Stage 3) |
| `aether relay` | `mfa`, `devicecode`, `stretch`, `cae-handler`, `fido2-downgrade`, `mex`, `saml-strip` | Stage 2/3 | None |
| `aether prt` | `convert`, `show` | Stage 2 | None |
| `aether validate` | `soc` | Stage 3 | None |
| `aether workspace` | `create`, `open`, `list`, `delete` | Stage 2 | None |
| `aether export` | `audit`, `evidence`, `journal` | Stage 2/3 | None |
| `aether completion` | `bash`, `zsh`, `fish`, `powershell` | Stage 3 | None |

### 2.2 Flag Stability

| Command | Flag | Type | Required | Stable | Notes |
|---------|------|------|----------|--------|-------|
| `prt convert` | `--prt-file` | string | Yes | ✓ | |
| `prt convert` | `--output` | string | No | ✓ | |
| `prt convert` | `--resource` | string | No | ✓ | Default: graph.microsoft.com |
| `prt convert` | `--client-id` | string | No | ✓ | Default: Azure CLI |
| `prt convert` | `--tenant` | string | No | ✓ | Override PRT tenant |
| `prt convert` | `--browser-preset` | string | No | ✓ | chrome/edge/firefox |
| `prt convert` | `--tls-binding` | string | No | ✓ | Token Protection |
| `prt convert` | `--timeout` | int | No | ✓ | Default: 30s |
| `relay mfa` | `--username` | string | Yes | ✓ | |
| `relay mfa` | `--password` | string | No | ✓ | Env: AETHER_PASSWORD |
| `relay mfa` | `--sts-endpoint` | string | Yes | ✓ | |
| `relay mfa` | `--tenant` | string | Yes | ✓ | |
| `relay mfa` | `--client-id` | string | No | ✓ | |
| `relay mfa` | `--applies-to` | string | No | ✓ | Default: MicrosoftOnline |
| `relay mfa` | `--output` | string | No | ✓ | |
| `relay mfa` | `--browser-preset` | string | No | ✓ | |
| `relay mfa` | `--timeout` | int | No | ✓ | |

**No flag additions/removals in Stage 6.** Surface frozen at Stage 3.

---

## 3. Protocol Behavior Stability

| Protocol | Behavior | Stable | Notes |
|----------|----------|--------|-------|
| Teamserver v2 (Protocol v2) | RequestID correlation, multiplexing cap 8, event cursor replay | ✓ | Stage 3 complete |
| MS-OAPX (PRT→OAuth) | `grant_type=urn:ietf:params:oauth:grant-type:prt_sso`, session key proof | ✓ | Stage 2; offline validated |
| WS-Trust 1.3 | UsernameToken, SAML 1.1 Bearer grant | ✓ | Stage 2; mock tested |
| SAML 2.0 | Assertion building, signing, stripping | ✓ | Stage 2/3; mock tested |
| Device Code | RFC 8628 flow | ✓ | Stage 2; mock tested |
| CAE | Claims challenge handling | ✓ | Stage 3; mock tested |
| IMDSv2 | Token + Identity + Metadata | ✓ | Stage 2; mock tested |

**No protocol behavior changes in Stage 6.**

---

## 4. File Format Stability

| Format | Used By | Stable | Versioned |
|--------|---------|--------|-----------|
| Workspace vault (bbolt) | `internal/workspace` | ✓ | Schema in code; migration on open |
| PRT JSON | `aether prt convert/show` | ✓ | `internal/types/token.go` |
| SAML XML | `aether relay saml-strip`, WS-Trust | ✓ | `internal/protocol/saml` |
| Audit log JSONL | `internal/workspace` (legacy) / bbolt (current) | ✓ | Migrated on open |
| Event store | `internal/api/eventstore.go` | ✓ | Seq-based cursor |
| Config JSON | `aether` CLI (`viper`) | ✓ | `aether.json` in config dirs |

**No format changes in Stage 6.**

---

## 5. Build & Release Metadata

| Metadata | Source | In Binary | In Release |
|----------|--------|-----------|------------|
| Version | `VERSION` file → ldflags | ✓ (`internal/version`) | ✓ |
| Commit | `git rev-parse --short HEAD` → ldflags | ✓ | ✓ |
| Go version | `runtime.Version()` | ✓ | ✓ |
| Build timestamp | Not embedded | ✗ | ✗ (reproducible) |
| Builder identity | Not embedded | ✗ | CI metadata |

**Reproducible:** Yes (strip `-s -w`, fixed ldflags, no timestamps).

---

## 6. README / Documentation Consistency

| Document | Version Reference | Consistent? |
|----------|------------------|-------------|
| `README.md` | "Aether v3.4.0-stage3" | ✓ |
| `CHANGELOG.md` | Head: `v3.4.0-stage3` | ✓ |
| `SECURITY.md` | No version | N/A |
| `LICENSE` | No version | N/A |
| `PLATFORM.md` | No version | N/A |
| Stage 1-3 docs | Historical versions | ✓ (archival) |
| Stage 6 docs | Reference `3.4.0-stage3` baseline | ✓ |

---

## 7. Compatibility Guarantees (Staged RC)

### 7.1 Backward Compatibility

| Surface | Guarantee | Scope |
|---------|-----------|-------|
| CLI flags | No removals; new flags additive only | Staged RC |
| CLI output (JSON) | Field additions only; no removals | Staged RC |
| Workspace vault | Auto-migration on open; read backward compat | Staged RC |
| Protocol v2 | Version field; fail-closed on unsupported | Staged RC |
| PRT JSON | Field additions only | Staged RC |
| SAML XML | Builder/Parser backward compatible | Staged RC |

### 7.2 Known Breaking Changes (None in Stage 6)

| Change | Version | Migration |
|--------|---------|-----------|
| — | — | — |

### 7.3 Deprecation Notices (None)

| Feature | Deprecated In | Removal Target | Alternative |
|---------|---------------|----------------|-------------|
| — | — | — | — |

---

## 8. Installation & Verification Instructions

### 8.1 Current (Manual)

```bash
# Build from source
git clone https://github.com/Debajyoti0-0/aether
cd aether
go build -ldflags="-X github.com/Debajyoti0-0/aether/internal/version.Version=$(cat VERSION) -X github.com/Debajyoti0-0/aether/internal/version.Commit=$(git rev-parse --short HEAD)" -o aether ./cmd/aether

# Verify version
./aether --version
# Output: aether version 3.4.0-stage3
```

### 8.2 Planned (Stage 7 - Goreleaser)

```bash
# Download release artifacts
# Verify checksums
sha256sum -c checksums.txt

# Verify signatures (Linux/macOS)
cosign verify-blob --signature aether-linux-amd64.sig --certificate aether-linux-amd64.pem aether-linux-amd64

# Verify signature (Windows)
signtool verify /pa /v aether-windows-amd64.exe
```

---

## 9. Release Notes Template (Staged RC)

```markdown
# Aether 3.5.0-rc1 (Staged Release Candidate)

## Baseline
- Stage 3 complete (3.4.0-stage3)
- 32 deferred items reassessed (3 CLOSED, 7 MITIGATED, 2 ACCEPTED, 17 DEFERRED)

## Changes Since 3.4.0-stage3
- **Fix:** Teamserver in-flight command cap now configurable (`SetMaxInFlight`) — resolves test flakiness
- **Docs:** Stage 6 baseline, deferred reassessment, live-lab architecture, PRT validation scope, interoperability design, artifact signing design, CI/supply-chain review, adversarial regression

## Supported Platforms (Tested)
- Windows/amd64 — Full test coverage
- Linux/amd64 — Full test coverage (CI)
- macOS/amd64 — Build verified only

## Known Limitations
- Live Entra ID validation (PRT, WS-Trust, Device Code, CAE) pending authorized lab tenant
- IMDSv2 live validation pending Azure VM
- No automated release pipeline (manual build)
- Audit key stored in workspace (plaintext) — operator custody
- No OCSP/CRL for operator revocation
- Request idempotency keys not implemented

## Verification
```bash
./aether --version
# aether version 3.5.0-rc1
```

## Upgrade Notes
- No breaking changes from 3.4.0-stage3
- Workspace auto-migration on open
```

---

## 10. Gate B8 — Assessment

| Criterion | Status | Evidence |
|-----------|--------|----------|
| Version truth internally consistent | **PASS** | All sources = `3.4.0-stage3` |
| Reproducible build metadata | **PASS** | `-s -w`, fixed ldflags, no timestamps |
| Release notes drafted | **PASS** | Template in §9 |
| Upgrade notes documented | **PASS** | No breaking changes |
| Installation instructions current | **PASS** | Manual build documented |
| Verification instructions defined | **PASS** | Stage 7 design referenced |
| No undocumented breaking behavior | **PASS** | CLI/protocol/format stable |

**Gate B8: PASS**

---

## Sign-Off

**Audit Completed:** 2026-09-12
**Baseline Commit:** `350dd91`
**Version:** `3.4.0-stage3` (baseline) → `3.5.0-rc1` (target Staged RC)
**Next Phase:** Phase 9 — Final Release-Candidate Certification