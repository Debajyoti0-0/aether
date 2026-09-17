# Stage 20 Phase 8 — Implementation Plan (G8)

**Timestamp:** 2016-09-17
**Stage:** 20 — Operations Track & 4.1.0 Development Foundation
**Agent:** Stage 20 execution agent
**Repository:** C:\dev\aether

---

## Objective

Create the execution plan for 4.1.0 in-scope items, with dependency-aware sequencing and explicit gates.

---

## Execution Sequence (Dependency-Aware)

```
Phase A: Foundation (No Deps)
  ├── WI-01: Action Pinning (Supply Chain)
  ├── WI-02: AzureKVProvider Mutex Fix
  ├── WI-03: Workspace Pass/Salt Mutex Fix
  └── WI-04: VERIFY.md Creation

Phase B: Observability Core (Depends on Phase A)
  ├── WI-05: Structured Logging (slog)
  ├── WI-06: Metrics Server (/metrics)
  ├── WI-07: Health Server (/healthz, /readyz)

Phase C: Operational Features (Depends on Phase B)
  ├── WI-08: Revocation Checking (OCSP/CRL + File)
  ├── WI-09: ARM64 CI Integration
  ├── WI-10: Third-Party IdP (Keycloak)

Phase D: Release Preparation
  ├── WI-11: Goreleaser ARM64 Config
  ├── WI-12: CI Matrix Updates
  ├── WI-13: Documentation Updates
  ├── WI-14: Full Quality Gate Run
  ├── WI-15: VERSION -> 4.1.0
  ├── WI-16: Tag v4.1.0
```

---

## Work Item Details

### WI-01: Action Pinning (Supply Chain) — Phase A

| Field | Value |
|-------|-------|
| **Objective** | Pin all GitHub Actions to immutable commit SHAs |
| **Files Modified** | `.github/workflows/ci.yml`, `.github/workflows/release.yml` |
| **Interface Changes** | None (CI only) |
| **Data/Schema Changes** | None |
| **Security Implications** | Reduces supply chain risk; no runtime impact |
| **Compatibility** | None |
| **Implementation Steps** | 1. Resolve each action to commit SHA via GitHub API<br>2. Replace `@vX` with `@<sha>`<br>3. Add Dependabot config for action updates<br>4. Test CI runs with pinned actions |
| **Tests Required** | CI runs pass with pinned actions |
| **Integration Tests** | N/A |
| **Fuzzing** | N/A |
| **Documentation** | Update CONTRIBUTING.md with pinning procedure |
| **Rollback** | Revert workflow files |
| **Exit Criteria** | All workflows use commit SHAs; CI passes |
| **Evidence** | Workflow YAML diffs; CI run URLs |
| **Dependencies** | None |
| **Non-Goals** | Pinning transitive action deps (not feasible) |

---

### WI-02: AzureKVProvider Mutex Fix — Phase A

| Field | Value |
|-------|-------|
| **Objective** | Add `sync.RWMutex` to protect all shared state in AzureKVProvider |
| **Files Modified** | `internal/store/azure_kv_provider.go` |
| **Interface Changes** | None (internal only) |
| **Data/Schema Changes** | None |
| **Security Implications** | Fixes data race; no new attack surface |
| **Compatibility** | None |
| **Implementation Steps** | 1. Add `mu sync.RWMutex` field<br>2. Wrap `GetKeyVersion`, `ListKeyVersions` with `RLock`/`RUnlock`<br>3. Wrap `RotateKey`, `ensureKey`, `Close` with `Lock`/`Unlock`<br>4. Ensure `Close` acquires write lock |
| **Tests Required** | `go test -race -count=1 ./internal/store/...` passes |
| **Integration Tests** | Mock transport test for concurrent access |
| **Fuzzing** | N/A |
| **Documentation** | Code comments on mutex usage |
| **Rollback** | Revert file |
| **Exit Criteria** | Race detector passes for store package |
| **Evidence** | Race test output; diff |
| **Dependencies** | None |
| **Non-Goals** | Live Azure validation (waived) |

---

### WI-03: Workspace Pass/Salt Mutex Fix — Phase A

| Field | Value |
|-------|-------|
| **Objective** | Add `sync.RWMutex` to protect `pass` and `salt` in Workspace |
| **Files Modified** | `internal/workspace/workspace.go` |
| **Interface Changes** | None (internal only) |
| **Data/Schema Changes** | None |
| **Security Implications** | Fixes data race; prevents passphrase corruption |
| **Compatibility** | None |
| **Implementation Steps** | 1. Add `rekeyMu sync.RWMutex` field<br>2. Protect `pass` and `salt` access in `Rekey`, `Seal`, `Open`, `LoadRecord`<br>3. Use `RLock` for reads, `Lock` for writes<br>4. Ensure no lock held during vault I/O |
| **Tests Required** | `go test -race -count=1 ./internal/workspace/...` passes |
| **Integration Tests** | Concurrent Rekey/Seal test |
| **Fuzzing** | N/A |
| **Documentation** | Code comments |
| **Rollback** | Revert file |
| **Exit Criteria** | Race detector passes for workspace package |
| **Evidence** | Race test output; diff |
| **Dependencies** | None |
| **Non-Goals** | Schema migration (deferred) |

---

### WI-04: VERIFY.md Creation — Phase A

| Field | Value |
|-------|-------|
| **Objective** | Create stranger-reproducible verification guide |
| **Files Modified** | `VERIFY.md` (new at repo root) |
| **Interface Changes** | None |
| **Data/Schema Changes** | None |
| **Security Implications** | Enables supply chain verification; no risk |
| **Compatibility** | None |
| **Implementation Steps** | 1. Document download from GitHub Releases<br>2. Checksum verification (`sha256sum -c`)<br>3. SBOM verification (`syft`, `jq`)<br>4. Cosign verification (Linux/macOS)<br>5. Authenticode verification (Windows)<br>6. Manifest verification<br>7. Tamper test procedures<br>8. Tool versions and install commands |
| **Tests Required** | Stranger reproduction: fresh clone -> follow guide -> all pass |
| **Integration Tests** | N/A |
| **Fuzzing** | N/A |
| **Documentation** | This IS the documentation |
| **Rollback** | Delete file |
| **Exit Criteria** | Stranger reproduction succeeds |
| **Evidence** | VERIFY.md content; reproduction log |
| **Dependencies** | None |
| **Non-Goals** | Automated verification script (manual guide only) |

---

### WI-05: Structured Logging (slog) — Phase B

| Field | Value |
|-------|-------|
| **Objective** | Replace stdlib `log` with `slog` + JSON handler + redactor |
| **Files Modified** | `internal/observability/logging.go` (new)<br>`internal/store/*.go`<br>`internal/engine/*.go`<br>`internal/transport/*.go`<br>`internal/protocol/*.go`<br>`internal/workspace/*.go` |
| **Interface Changes** | New `observability` package; log output format (JSON) |
| **Data/Schema Changes** | None |
| **Security Implications** | Systematic redaction reduces secret leakage risk |
| **Compatibility** | Human CLI output unchanged (uses `fmt`, not `slog`) |
| **Implementation Steps** | 1. Create `internal/observability/logging.go` with `Logger` wrapper<br>2. Add `Redactor` for sensitive fields (tokens, keys, secrets, passphrases)<br>3. Add `NewLogger(level, format)` -- text (default) or JSON<br>4. Replace `log.Printf` calls across codebase<br>5. Add config: `logging.level`, `logging.format` |
| **Tests Required** | Unit: log output is valid JSON; redaction works; levels filter |
| **Integration Tests** | Run full test suite; verify no log regressions |
| **Fuzzing** | Fuzz redactor with secret-like strings |
| **Documentation** | Config reference; logging guide |
| **Rollback** | Revert logging changes; restore `log.Printf` |
| **Exit Criteria** | All packages use `slog`; JSON output valid; redaction verified |
| **Evidence** | Diff; test output; log sample |
| **Dependencies** | WI-01..04 complete |
| **Non-Goals** | Log aggregation/export (external concern) |

---

### WI-06: Metrics Server (/metrics) — Phase B

| Field | Value |
|-------|-------|
| **Objective** | Implement Prometheus `/metrics` endpoint in serve mode |
| **Files Modified** | `internal/observability/metrics.go` (new)<br>`internal/observability/server.go` (new)<br>`cmd/aether/serve.go` (new)<br>`cmd/aether/main.go` (add `-serve` flag) |
| **Interface Changes** | New `-serve` flag; HTTP server on configurable port |
| **Data/Schema Changes** | None |
| **Security Implications** | Read-only endpoint; bind to localhost default; no secrets in metrics |
| **Compatibility** | Opt-in via `-serve`; default CLI behavior unchanged |
| **Implementation Steps** | 1. Create `internal/observability/metrics.go` with Prometheus collectors<br>2. Instrument: HTTP requests (latency, errors), vault ops, protocol ops, engine ops<br>3. Create `internal/observability/server.go` with HTTP mux<br>4. Register `/metrics`, `/healthz`, `/readyz`<br>5. Create `cmd/aether/serve.go` with server config<br>6. Add `-serve` flag to `main.go` |
| **Tests Required** | Unit: metrics increment correctly; HTTP server starts/stops<br>Integration: Prometheus scrape test |
| **Fuzzing** | N/A |
| **Documentation** | Metrics reference; serve mode guide |
| **Rollback** | Revert observability package and serve command |
| **Exit Criteria** | `aether serve` starts; `curl localhost:8080/metrics` returns Prometheus format |
| **Evidence** | Server logs; curl output; test results |
| **Dependencies** | WI-05 (logging for server) |
| **Non-Goals** | Push gateway; alerting rules; Grafana dashboards |

---

### WI-07: Health Server (/healthz, /readyz) — Phase B

| Field | Value |
|-------|-------|
| **Objective** | Implement Kubernetes-style health/readiness endpoints |
| **Files Modified** | `internal/observability/health.go` (new)<br>`internal/observability/server.go` (extend) |
| **Interface Changes** | Part of `-serve` mode; new endpoints |
| **Data/Schema Changes** | None |
| **Security Implications** | Read-only; localhost default; no secrets |
| **Compatibility** | Opt-in via `-serve` |
| **Implementation Steps** | 1. Create `internal/observability/health.go` with `HealthChecker` interface<br>2. Implement checks: config loaded, vault accessible, providers healthy<br>3. `/healthz` -- liveness (always 200 if server running)<br>4. `/readyz` -- readiness (200 if all checks pass)<br>5. Register in `server.go` |
| **Tests Required** | Unit: health checks return correct status<br>Integration: Kubernetes probe simulation |
| **Fuzzing** | N/A |
| **Documentation** | Health endpoint guide; Kubernetes integration |
| **Rollback** | Revert health endpoints |
| **Exit Criteria** | `curl localhost:8080/healthz` -> 200 OK; `curl localhost:8080/readyz` -> 200 when ready |
| **Evidence** | curl output; test results |
| **Dependencies** | WI-06 (same server) |
| **Non-Goals** | Deep dependency checks (DB, external APIs) |

---

### WI-08: Revocation Checking (OCSP/CRL + File) — Phase C

| Field | Value |
|-------|-------|
| **Objective** | Implement certificate revocation checking with file fallback |
| **Files Modified** | `internal/revocation/checker.go` (new)<br>`cmd/aether/validate_revocation.go` (new)<br>`internal/transport/tls.go` (integrate) |
| **Interface Changes** | New `validate revocation` command; config options |
| **Data/Schema Changes** | None |
| **Security Implications** | Fail-open default (availability); fail-closed option; no network by default |
| **Compatibility** | Opt-in via config; default behavior unchanged |
| **Implementation Steps** | 1. Create `internal/revocation/checker.go` with `RevocationChecker` interface<br>2. Implement: OCSP (stdlib `crypto/ocsp`), CRL (stdlib `x509`), File (local PEM list)<br>3. Add caching (5 min TTL default)<br>4. Config: `revocation.mode` (ocsp|crl|file|none), `revocation.cache_ttl`, `revocation.fail_closed`<br>5. Create `cmd/aether/validate_revocation.go` for manual checking<br>6. Integrate in `internal/transport/tls.go` for TLS handshake |
| **Tests Required** | Unit: OCSP/CRL/file modes; caching; fail-open/closed<br>Integration: Mock OCSP responder; file fallback test |
| **Fuzzing** | Fuzz OCSP/CRL response parsing |
| **Documentation** | Revocation guide; config reference |
| **Rollback** | Revert revocation package and command |
| **Exit Criteria** | `aether validate revocation --cert <file>` works; TLS handshake uses revocation when enabled |
| **Evidence** | Command output; test results |
| **Dependencies** | WI-05, WI-06 (logging/metrics for revocation events) |
| **Non-Goals** | OCSP stapling; CRL distribution points auto-discovery |

---

### WI-09: ARM64 CI Integration — Phase C

| Field | Value |
|-------|-------|
| **Objective** | Add ARM64 to CI build/test matrix |
| **Files Modified** | `.github/workflows/ci.yml`<br>`.github/workflows/release.yml`<br>`.goreleaser.yml` |
| **Interface Changes** | None (CI/config only) |
| **Data/Schema Changes** | None |
| **Security Implications** | None |
| **Compatibility** | Additive platforms |
| **Implementation Steps** | 1. Update `.goreleaser.yml`: enable `linux/arm64`, `darwin/arm64` (remove ignore)<br>2. Update `ci.yml`: add `ubuntu-latest` ARM64 runner (or use `ubuntu-latest` with `GOARCH=arm64`)<br>3. Update `release.yml`: build all 6 platforms<br>4. Test ARM64 builds locally with `GOARCH=arm64` |
| **Tests Required** | `GOOS=linux GOARCH=arm64 go build ./...`<br>`GOOS=darwin GOARCH=arm64 go build ./...`<br>CI: ARM64 jobs pass |
| **Integration Tests** | Run unit tests on ARM64 runner |
| **Fuzzing** | N/A (ARM64 runners may not support race) |
| **Documentation** | Supported platforms list |
| **Rollback** | Revert CI/goreleaser configs |
| **Exit Criteria** | 6 artifacts built in release; ARM64 CI jobs pass |
| **Evidence** | CI run URLs; artifact list |
| **Dependencies** | WI-01 (pinned actions) |
| **Non-Goals** | Windows ARM64 (not supported by goreleaser) |

---

### WI-10: Third-Party IdP Interop (Keycloak) — Phase C

| Field | Value |
|-------|-------|
| **Objective** | Implement Keycloak validation (dry-run + live opt-in) |
| **Files Modified** | `cmd/aether/validate_keycloak.go` (new)<br>`internal/protocol/oauth2/keycloak.go` (new) |
| **Interface Changes** | New `validate keycloak` subcommand |
| **Data/Schema Changes** | None |
| **Security Implications** | Dry-run default; explicit `--live` flag; redaction; no credential persistence |
| **Compatibility** | Additive command; uses existing OAuth2/SAML |
| **Implementation Steps** | 1. Create Keycloak validation profile (OIDC discovery, token exchange)<br>2. Implement dry-run: OIDC config fetch, JWKS fetch, no token exchange<br>3. Implement live: device code flow or client credentials (opt-in)<br>3. Redact all tokens in output<br>4. Config: `keycloak.url`, `keycloak.realm`, `keycloak.client_id` |
| **Tests Required** | Unit: dry-run works without auth<br>Integration: Live test against dev Keycloak (if authorized) |
| **Fuzzing** | N/A |
| **Documentation** | Keycloak validation guide |
| **Rollback** | Revert command |
| **Exit Criteria** | `aether validate keycloak --dry-run` works; live test passes (if env available) |
| **Evidence** | Command output; test results |
| **Dependencies** | WI-05, WI-06 (logging/metrics) |
| **Non-Goals** | Okta/Auth0 (future); full IdP protocol validation |

---

### WI-11: Goreleaser ARM64 Config — Phase D

| Field | Value |
|-------|-------|
| **Objective** | Enable ARM64 artifacts in goreleaser release |
| **Files Modified** | `.goreleaser.yml` |
| **Implementation** | Remove `ignore` for `linux/arm64` and `darwin/arm64` |
| **Exit Criteria** | `goreleaser release --snapshot` produces 6 artifacts |

---

### WI-12: CI Matrix Updates — Phase D

| Field | Value |
|-------|-------|
| **Objective** | Update CI to build/test on ARM64 |
| **Files Modified** | `.github/workflows/ci.yml` |
| **Implementation** | Add ARM64 build job; use `ubuntu-latest` with `GOARCH=arm64` |
| **Exit Criteria** | CI matrix includes ARM64; jobs pass |

---

### WI-13: Documentation Updates — Phase D

| Field | Value |
|-------|-------|
| **Objective** | Update docs for 4.1.0 features |
| **Files Modified** | `README.md`, `CHANGELOG.md`, `docs/configuration-reference.md`, `docs/operational-limitations.md` |
| **Implementation** | Add 4.1.0 features; update limitations |
| **Exit Criteria** | Docs reflect 4.1.0 capabilities and limitations |

---

### WI-14: Full Quality Gate Run — Phase D

| Field | Value |
|-------|-------|
| **Objective** | Run all quality gates before release |
| **Commands** | `go build ./...`<br>`go vet ./...`<br>`go test -count=1 ./...`<br>`go test -tags=integration ./test/integration/...`<br>`go test -race ./internal/store/... ./internal/workspace/...`<br>`go test -fuzz=. -fuzztime=60s`<br>`golangci-lint run ./...`<br>`govulncheck ./...` |
| **Exit Criteria** | All PASS |

---

### WI-15: VERSION Bump — Phase D

| Field | Value |
|-------|-------|
| **Objective** | Update VERSION to 4.1.0 |
| **Files Modified** | `VERSION` |
| **Exit Criteria** | `cat VERSION` -> `4.1.0` |

---

### WI-16: Tag v4.1.0 — Phase D

| Field | Value |
|-------|-------|
| **Objective** | Create and push v4.1.0 tag |
| **Command** | `git tag -a v4.1.0 -m "Stage 20 -- 4.1.0 operations release" && git push origin v4.1.0` |
| **Exit Criteria** | `git rev-parse v4.1.0` returns tag SHA |

---

## Quality Gates per Phase

| Phase | Gates |
|-------|-------|
| A (Foundation) | `go test -race ./internal/store/... ./internal/workspace/...` passes; CI with pinned actions passes; VERIFY.md stranger test passes |
| B (Observability) | `go build ./...`; `go test ./internal/observability/...`; serve mode starts; endpoints respond |
| C (Features) | Revocation unit tests pass; ARM64 builds pass; Keycloak dry-run works |
| D (Release) | All quality gates pass; 6 artifacts generated; tag created |

---

## Gate G8 Status

| Sub-gate | Status | Evidence |
|----------|--------|----------|
| G8.1 | All work items defined | 16 work items |
| G8.2 | Dependency sequencing explicit | Phase A->B->C->D |
| G8.3 | Each item has exit criteria | Per work item |
| G8.4 | Evidence artifacts defined | Per work item |
| G8.5 | Rollback strategies defined | Per work item |
| G8.6 | Non-goals explicit | Per work item |
| G8.7 | Implementation order feasible | No circular deps |

**G8 Status: PASS** -- Implementation plan complete and executable.

---

## Appendix: Release & Evidence Strategy (G9) — Merged from stage20-release-evidence-strategy.md

### Objective

Define how 4.1.0 will be proven before implementation begins. Every in-scope capability must have a validation and evidence path.

---

### Evidence Categories

#### 1. Build Evidence

| Requirement | Tool / Method | Release Gate |
|-------------|---------------|--------------|
| Supported platforms | `goreleaser release --snapshot` | 6 artifacts (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64) |
| Reproducible build | Two builds from same commit; compare outputs | Identical checksums |
| Version integrity | `./aether --version` matches `VERSION` file | Exact match |
| Commit integrity | `./aether --version` includes git commit | Embedded via ldflags |
| Artifact checksums | `sha256sum -c checksums.txt` | All PASS |
| Binary flags | `file`, `go tool objdump` | Stripped, static, `-trimpath` |

---

#### 2. Quality Evidence

| Requirement | Tool / Method | Release Gate |
|-------------|---------------|--------------|
| Unit tests | `go test -count=1 ./...` | All 38+ packages PASS |
| Integration tests | `go test -tags=integration ./test/integration/...` | All PASS |
| Race detection (store, workspace) | `go test -race ./internal/store/... ./internal/workspace/...` | Zero races |
| Fuzzing | `go test -fuzz=. -fuzztime=60s` | 21 targets; zero crashes |
| Static analysis | `golangci-lint run ./...` | Zero issues |
| Vulnerability scan | `govulncheck ./...` | Zero affecting |
| Code coverage | `go test -cover ./...` | Report generated |

---

#### 3. Security Evidence

| Requirement | Tool / Method | Release Gate |
|-------------|---------------|--------------|
| Credential handling | Redaction test (fuzz + manual) | No secrets in logs/output |
| Secret redaction | `slog` redactor unit tests | All sensitive patterns redacted |
| Provider validation | AzureKVProvider mock tests | Contract compliance |
| Supply chain | Action pinning verification | All workflows use commit SHAs |
| Signing (Linux/macOS) | Cosign keyless in CI | Signatures verify |
| Signing (Windows) | Authenticode (if cert) | `signtool verify` PASS |
| SBOM | `syft` per artifact | CycloneDX/SPDX valid |
| Provenance | Goreleaser attestations (if configured) | SLSA provenance |
| VERIFY.md | Stranger reproduction test | Fresh clone -> all steps PASS |

---

#### 4. Compatibility Evidence

| Requirement | Tool / Method | Release Gate |
|-------------|---------------|--------------|
| CLI compatibility | Run all 4.0.0-rc2 commands | Identical output/behavior |
| Config compatibility | Load 4.0.0-rc2 config | No errors; new fields optional |
| Store compatibility | Open 4.0.0-rc2 workspace | No migration needed |
| Provider compatibility | Use 4.0.0-rc2 providers | No interface changes |
| Upgrade test | Replace binary 4.0.0-rc2 -> 4.1.0 | Works without config change |
| Rollback test | Replace binary 4.1.0 -> 4.0.0-rc2 | Works without data loss |

---

#### 5. Operations Evidence

| Requirement | Tool / Method | Release Gate |
|-------------|---------------|--------------|
| `/metrics` endpoint | `curl localhost:8080/metrics` | Prometheus format; key metrics present |
| `/healthz` endpoint | `curl localhost:8080/healthz` | 200 OK; JSON status |
| `/readyz` endpoint | `curl localhost:8080/readyz` | 200 when ready; 503 when not |
| Structured logging | `slog` JSON output | Valid JSON; levels; redaction |
| Revocation checking | `aether validate revocation --cert` | File/OCSP/CRL modes work |
| ARM64 artifacts | `goreleaser release` | 6 platforms built |
| Keycloak validation | `aether validate keycloak --dry-run` | OIDC discovery works |
| Keycloak live | `aether validate keycloak --live` | Token exchange (if authorized) |

---

### Evidence Artifacts (Per Release)

| Artifact | Location | Verification |
|----------|----------|--------------|
| `checksums.txt` | `artifacts/stage20/release/` | SHA-256 per file |
| `release-manifest.json` | `artifacts/stage20/release/` | Artifact metadata |
| `sbom-cyclonedx.json` | `artifacts/stage20/release/` | Per artifact SBOM |
| `*.sig` / `*.pem` | `artifacts/stage20/release/` | Cosign signatures |
| `VERIFY.md` | Repo root | Stranger reproduction log |
| Test results | `artifacts/stage20/test/` | JUnit XML / raw output |
| Race detector output | `artifacts/stage20/race/` | Zero races |
| Fuzz results | `artifacts/stage20/fuzz/` | 21 targets; zero crashes |
| Lint output | `artifacts/stage20/lint/` | Zero issues |
| Vuln scan | `artifacts/stage20/govulncheck.txt` | Zero affecting |
| Coverage report | `artifacts/stage20/coverage/` | Per-package coverage |

---

### Stranger Reproduction Protocol (VERIFY.md)

```bash
# 1. Fresh environment
git clone https://github.com/Debajyoti0-0/aether.git
cd aether

# 2. Checkout release tag
git checkout v4.1.0

# 3. Build
go build ./...

# 4. Test
go test -count=1 ./...

# 5. Verify artifacts (if release published)
# Download from GitHub Releases
sha256sum -c checksums.txt

# 6. Cosign verify (Linux/macOS)
cosign verify-blob --signature aether_...sig --certificate aether_...pem aether_...

# 7. Authenticode verify (Windows)
signtool verify /pa /v aether.exe

# 7. SBOM verify
jq -e '.specVersion' sbom-cyclonedx.json

# 8. Manifest verify
jq -r '.artifacts[] | "\(.sha256)  \(.name)"' release-manifest.json | sha256sum -c
```

**Success Criteria:** All steps complete without error.

---

### Release Gate Checklist (Pre-Tag)

| Gate | Command | Expected | Status |
|------|---------|----------|--------|
| G253 | `go build ./...` | PASS | ⏳ |
| G253 | `go vet ./...` | PASS | ⏳ |
| G253 | `go test -count=1 ./...` | PASS | ⏳ |
| G254 | `go test -tags=integration ./test/integration/...` | PASS | ⏳ |
| G254 | `go test -race ./internal/store/...` | PASS | ⏳ |
| G254 | `go test -race ./internal/workspace/...` | PASS | ⏳ |
| G254 | `go test -fuzz=. -fuzztime=60s` | 21 targets, 0 crashes | ⏳ |
| G255 | `golangci-lint run ./...` | 0 issues | ⏳ |
| G256 | `govulncheck ./...` | 0 affecting | ⏳ |
| G257 | `aether serve` + `curl /metrics` | Prometheus format | ⏳ |
| G257 | `curl /healthz` | 200 OK | ⏳ |
| G257 | `curl /readyz` | 200 OK (when ready) | ⏳ |
| G258 | `aether validate revocation --cert` | PASS | ⏳ |
| G259 | `GOARCH=arm64 go build ./...` | PASS | ⏳ |
| G260 | `aether validate keycloak --dry-run` | PASS | ⏳ |
| G262 | `cat VERSION` | `4.1.0` | ⏳ |
| G263 | `goreleaser release --snapshot` | 6 artifacts | ⏳ |
| G264 | `git tag -a v4.1.0` | Created | ⏳ |
| G265 | `(Get-ChildItem docs/stage20-*).Count` | ≤ 10 | ⏳ |
| G266 | `git diff --stat docs/stage20-*design*.md` | Empty | ⏳ |
| G267 | `git show v4.0.0-rc2:VERSION` | `4.0.0-rc2` | ⏳ |

---

### Post-Release Verification

1. **GitHub Release** — Artifacts attached, prerelease=false
2. **VERIFY.md** — Stranger reproduction by independent party
3. **Cosign Verification** — All signatures valid
4. **SBOM Integrity** — All components catalogued
5. **Tamper Tests** — CI verify job passes all 4+ tests
6. **Monitoring** — Metrics/health endpoints accessible

---

### Gate G9 Status

| Sub-gate | Status | Evidence |
|----------|--------|----------|
| G9.1 | Build evidence defined | ✅ Complete |
| G9.2 | Quality evidence defined | ✅ Complete |
| G9.3 | Security evidence defined | ✅ Complete |
| G9.4 | Compatibility evidence defined | ✅ Complete |
| G9.5 | Operations evidence defined | ✅ Complete |
| G9.6 | Evidence artifacts listed | ✅ Complete |
| G9.7 | Stranger reproduction protocol | ✅ VERIFY.md |
| G9.8 | Release gate checklist | ✅ Complete |

**G9 Status: PASS** — Release & evidence strategy complete.