# Stage 11 Handoff — Operations & External Validation

**Prerequisite:** Stage 10 final report completed
**Target Release:** `4.1.0` (Operations track) or `4.0.0` GA (if B3/B4 resolved)
**Waiver Expiry:** 2027-06-30 (Entra/IMDS)

---

## Stage 11 Scope

### Primary Tracks

| Track | Description | Priority |
|-------|-------------|----------|
| **Observability** | Metrics (/metrics), health endpoints (/health), structured logging | P0 |
| **Revocation Operations** | OCSP responder, CRL distribution, automated rotation alerts | P0 |
| **Multi-Host Deployment** | Docker Compose, Kubernetes Helm, systemd units | P1 |
| **ARM64 Runtime** | Native ARM64 builds, CI validation, cross-compile verification | P1 |
| **Third-Party IdP Interop** | Okta, PingFederate, AD FS, Keycloak validation | P1 |
| **Live Entra/IMDS** | **MANDATORY before 2027-06-30** — Real Azure AD + IMDS validation | P0 (waiver expiry) |

---

## Detailed Work Items

### 1. Observability Stack (P0)

| Component | Specification |
|-----------|---------------|
| **Metrics** | Prometheus `/metrics` endpoint with: request latency, error rates, active connections, queue depths, cache hit ratios |
| **Health** | `/health` (liveness) and `/ready` (readiness) endpoints with dependency checks (vault, keyprovider, teamserver) |
| **Logging** | Structured JSON logging with correlation IDs, severity levels, structured fields for audit events |
| **Tracing** | OpenTelemetry integration (W3C TraceContext), span attributes for audit operations |
| **Alerting** | PrometheusRule definitions for: high error rate, latency p99, vault lock contention, key rotation due |

**Deliverables:**
- `internal/observability/` package
- `cmd/aether/metrics.go` — metrics server
- `docs/operations/observability.md`
- Grafana dashboard JSON

### 2. Revocation Operations (P0)

| Component | Specification |
|-----------|---------------|
| **OCSP Responder** | RFC 6960 compliant, serves revocation status for audit signing certs |
| **CRL Distribution** | Periodic CRL generation (daily), HTTP/HTTPS distribution points |
| **Rotation Alerts** | Automated alerts 30/7/1 days before key expiry; Slack/Email/PagerDuty |
| **Emergency Revocation** | CLI command `aether keyprovider revoke --version <v> --reason <r>` with audit trail |

**Deliverables:**
- `internal/revocation/ocsp.go` — OCSP responder
- `internal/revocation/crl.go` — CRL generator
- `cmd/aether/keyprovider_revoke.go`
- `docs/operations/revocation.md`

### 3. Multi-Host Deployment (P1)

| Target | Specification |
|--------|---------------|
| **Docker** | Multi-stage Dockerfile (builder + distroless), `docker-compose.yml` for dev |
| **Kubernetes** | Helm chart with: Deployment, Service, ConfigMap, Secret, NetworkPolicy, PodDisruptionBudget |
| **Systemd** | `aether.service` with: Type=notify, Restart=on-failure, LimitNOFILE=65536 |
| **Config** | Environment-first config (12-factor), `.env` support, Vault agent integration |

**Deliverables:**
- `Dockerfile` (multi-stage)
- `deploy/helm/aether/` — Helm chart
- `deploy/systemd/aether.service`
- `docs/operations/deployment.md`

### 4. ARM64 Runtime (P1)

| Item | Specification |
|------|---------------|
| **Build** | Goreleaser `goarch: [amd64, arm64]` for all OS |
| **CI** | GitHub Actions `arm64` runners (self-hosted or macOS) |
| **Testing** | `go test -race` on ARM64, cross-compile verification |
| **Release** | `aether_<version>_linux_arm64.tar.gz`, `aether_<version>_darwin_arm64.tar.gz` |

**Note:** Goreleaser config already has arm64 targets; need ARM64 runners.

### 5. Third-Party IdP Interop (P1)

| IdP | Test Scenarios |
|-----|----------------|
| **Okta** | SAML 2.0, WS-Federation, OIDC, MFA, adaptive auth |
| **PingFederate** | SAML 2.0, WS-Trust, token exchange |
| **AD FS** | WS-Federation, WS-Trust 1.3, SAML 2.0 |
| **Keycloak** | OIDC, SAML 2.0, token exchange, mTLS |

**Test Matrix:**
- PRT acquisition → Entra token exchange
- SAML assertion validation → Graph token
- IMDS token acquisition (Azure)
- Cross-IdP token exchange chains

**Deliverables:**
- `internal/interop/` — test suite per IdP
- `docs/interop/` — validation reports
- `aether interop validate --idp <name>`

### 6. Live Entra/IMDS Validation (P0 — MANDATORY)

**Waiver Context:** Stage 9 waived B1 (Live Entra) and B2 (IMDS) with expiry **2027-06-30** and classification `OFFLINE_VALIDATED_ONLY`.

**Mandatory Before Expiry:**

| Validation | Specification |
|------------|---------------|
| **Live Entra ID** | Acquire PRT → exchange for Graph token → call Graph API → validate roles/groups |
| **Live IMDS** | Acquire IMDS identity token → validate signature → extract metadata (subscription, resource group, VM size) |
| **Token Exchange** | PRT → Graph token → IMDS token → full chain validation |
| **Error Handling** | Network partition, token expiry, revocation, conditional access |

**Test Environment:**
- Dedicated Azure subscription (isolated)
- Test VM with managed identity
- Entra ID tenant with conditional access policies
- IMDS endpoint access

**Success Criteria:**
- All Stage 6 live-lab validation scenarios pass against live endpoints
- No `OFFLINE` banners in dashboard/CLI
- Token lifecycles match documented behavior

**Deliverables:**
- `test/live/` — live validation suite
- `docs/interop/live-validation-report.md`
- CI pipeline for scheduled live validation (weekly)

---

## Dependencies

| Dependency | Version | Notes |
|------------|---------|-------|
| Prometheus client | v1.x | Go client |
| OpenTelemetry | v1.x | Go SDK |
| OCSP | RFC 6960 | Custom implementation or `ocsp` package |
| Helm | v3.12+ | Chart development |
| Docker | 24+ | Buildx for multi-arch |

---

## Timeline

| Sprint | Focus | Deliverable |
|--------|-------|-------------|
| 1 | Observability + Revocation | Metrics, health, OCSP, CRL |
| 2 | Multi-Host + ARM64 | Helm, Docker, ARM64 CI |
| 3 | IdP Interop | Okta, PingFederate, AD FS, Keycloak |
| 4 | **Live Entra/IMDS** | **Full live validation suite** |
| 5 | Integration + Hardening | End-to-end, chaos testing |
| 6 | **GA Release** | **4.1.0 or 4.0.0 GA** |

---

## Risk Register

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Live Entra/IMDS validation delayed | Medium | High (waiver expiry) | Start Sprint 1, dedicated Azure subscription |
| ARM64 runner availability | Low | Medium | Use macOS ARM64 runners + cross-compile |
| IdP interop complexity | High | Medium | Start with Okta (best documented), parallelize |
| OCSP/CRL operational burden | Medium | Medium | Automate generation, monitor expiry |

---

## Success Criteria for Stage 11

| Criterion | Measure |
|-----------|---------|
| Observability | `/metrics` exposes 20+ metrics; `/health` + `/ready` respond <100ms |
| Revocation | OCSP responder <50ms p99; CRL regenerated daily |
| Deployment | Helm chart installs in <5min; systemd service starts <10s |
| ARM64 | All tests pass on ARM64; releases include ARM64 artifacts |
| IdP Interop | 4/4 IdPs validated; test reports published |
| **Live Entra/IMDS** | **All Stage 6 scenarios pass live; waiver lifted** |

---

## Stage 11 → Stage 12 Handoff

If Stage 11 completes with live Entra/IMDS validation successful:

**Stage 12 = `4.2.0` Enterprise Hardening**
- Advanced threat detection
- Compliance reporting (SOC2, FedRAMP)
- Multi-region deployment
- Disaster recovery automation

---

**Handoff Author:** Stage 10 Final Report
**Date:** 2026-09-15
**Next Review:** Sprint 1 planning