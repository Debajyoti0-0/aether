# Stage 45 Forensic Baseline — Phase 0

## Repository State (2026-09-21)

| Property | Value |
|----------|-------|
| Commit SHA | a887a0ed9c9fcdbf0400a5d59ccad162cfcac202 |
| Branch | master |
| Version (VERSION file) | 3.4.0-stage3 |
| Go Version | go1.27.1 windows/amd64 |
| Git Status | Clean (only new artifact directories) |
| Tags at HEAD | None |

## Package Inventory Summary

- **Total packages**: 37
- **Protocol packages (existing)**: msoapx, oauth2, saml, wstrust (4)
- **Protocol packages (missing for Stage 45)**: kerberos (1)
- **Engine packages (existing)**: 13 (cap, exec, graph, mutation, orchestrate, orchestrator, pivot, relay, rollback, spine, token, validate, watch)
- **Engine packages (missing for Stage 45)**: ad, ad/kerberos (2)
- **CLI packages (existing)**: 29 command modules
- **CLI packages (missing for Stage 45)**: ad, ad_kerberos

## Architecture Inventory

### Existing Reusable Abstractions

| Abstraction | Package | Reuse Strategy |
|-------------|---------|----------------|
| Transport (TCP/TLS) | internal/transport/ | USE for KDC connections |
| TLS Config | internal/transport/tls.go | USE for KDC TLS |
| Credential Types | internal/types/credential.go | EXTEND for Kerberos creds |
| Secret/Vault Storage | internal/store/ | USE for ticket/key storage |
| Audit Chain | internal/engine/spine/ | USE for every operation |
| Evidence Registration | internal/types/evidence.go | USE for result registration |
| Capability Registry | internal/engine/cap/ | REGISTER ad.* capabilities |
| Authorization | internal/engine/spine/ | USE existing AuthZ flow |
| Risk Scoring | internal/types/risk.go | USE existing risk engine |
| Policy Engine | internal/types/policy.go | USE existing policy |
| Approval Workflow | internal/engine/spine/ | USE existing approval |
| Workspace | internal/workspace/ | USE for engagement isolation |
| CLI Framework | internal/cli/ (Cobra) | USE existing pattern |
| JSON/Human Output | internal/cli/ | USE existing output |
| Error Types | internal/types/ | EXTEND with Kerberos errors |
| Logging | go.uber.org/zap | USE existing logger |

### New Packages Required

1. `internal/protocol/kerberos/` - Kerberos protocol implementation
2. `internal/engine/ad/kerberos/` - AD Kerberos engines
3. `internal/cli/ad_*.go` - CLI command modules

## Capability Inventory

### Existing Capabilities (25)
cap.read, cap.write, exec.azure, exec.gcp, exec.kubernetes, graph.read, graph.write, identity.read, identity.write, pivot.cross, pivot.lateral, prt.read, prt.write, relay.read, relay.write, token.read, token.write, workspace.create, workspace.delete, workspace.modify

### Missing Capabilities for Stage 45 (3)

| Capability | Risk | Reversible | Category |
|------------|------|------------|----------|
| ad.enum.read | 10 | yes | enumeration |
| ad.kerberos.roast | 20 | yes | credential-access |
| ad.kerberos.tgt | 15 | yes | credential-access |

## Dependency Inventory

- **Direct dependencies**: 8 (jwt, gofpdf, utls, cobra, pflag, viper, bbolt, zap, x/crypto)
- **Crypto available**: golang.org/x/crypto v0.57.0
- **Additional crypto needed**: RC4, AES-CTS, HMAC-SHA1, PBKDF2, ASN.1 (all in stdlib/x/crypto)
- **External Kerberos deps**: NONE (implement from spec using stdlib)
- **Shell-out policy**: FORBIDDEN in protocol packages

## Test Baseline

```
go test ./... → ALL PASS (31 packages)
go vet ./... → NO ISSUES
go test -race ./... → ALL PASS
```

## Gate TB45-G00 Verdict

**PASS** — Baseline verified at `3.4.0-stage3` (commit a887a0ed9c9fcdbf0400a5d59ccad162cfcac202). All existing tests pass. No Stage 45 infrastructure exists. Ready to proceed with implementation.

---

**Baseline artifacts created in**: `artifacts/stage45/baseline/`