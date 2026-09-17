# Stage 7 — Revocation Model Documentation

**Date:** 2026-09-12
**Baseline:** `8e7d27d`
**Scope:** Staged RC (Internal Lab / Authorized Partner Validation)

---

## 1. Current Implementation

### 1.1 Mechanism
- **File-based revocation list:** `revoked.txt` in teamserver config directory
- **Format:** One operator name per line (newline-separated)
- **Loaded:** At teamserver startup and on SIGHUP (manual reload)
- **Checked:** On every new mTLS connection (`handleConn`)

### 1.2 Revocation Entry Format
```
operator-name-1
operator-name-2
compromised-operator
```

### 1.3 Check Logic (`internal/api/server.go:handleConn`)
```go
if s.revoked != nil && s.revoked.IsRevoked(op.Name) {
    return // revoked operator: fail closed
}
```

---

## 2. Operational Characteristics (Staged RC)

| Property | Value | Notes |
|----------|-------|-------|
| **Source of Truth** | Local file (`revoked.txt`) | Operator-controlled |
| **Propagation** | Manual file edit + teamserver restart | No auto-propagation |
| **Cache** | None (checked per connection) | No stale reads |
| **TTL / Grace Period** | None (immediate on reload) | Configurable via restart |
| **Multi-Host** | Not supported | Requires shared FS or HTTP endpoint |
| **Audit Trail** | File mtime + git history (if tracked) | Manual |

---

## 3. Limitations (Staged RC)

| Limitation | Impact | Mitigation |
|------------|--------|------------|
| No auto-propagation | Revocation delayed until restart | Documented SLA: "restart within 5 min of revocation" |
| Single-process | Multi-host deployments unsupported | Documented: "single-host lab only" |
| No grace period | Immediate revocation on reload | Documented: "instant revocation" |
| No audit of revocation events | No automatic log of revocation | Manual: "log revocation in CHANGELOG" |

---

## 4. Production Requirements (Not in Staged RC)

| Requirement | Description |
|-------------|-------------|
| **Distributed revocation** | HTTP endpoint (CRL/OCSP/JSON) with caching |
| **Propagation** | Pub/sub or consensus for multi-host |
| **Cache with TTL** | Configurable staleness tolerance |
| **Grace period** | Configurable delay before enforcement |
| **Revocation audit** | Immutable log of revocation events |
| **Emergency bypass** | Break-glass procedure with audit |

---

## 5. Staged RC Acceptance

**Status:** ACCEPTED WITH LIMITATIONS

**Justification:** For internal lab / authorized partner validation:
- Single-host deployment is the norm
- Operator controls teamserver process
- Restart is acceptable operational burden
- No multi-host requirement

**Documentation Requirement:** All limitations must be explicitly stated in release notes and operator documentation.

---

## 6. Migration Path to Production

| Phase | Change |
|-------|--------|
| 1 | Add HTTP revocation endpoint (JSON) alongside file |
| 2 | Add in-memory cache with configurable TTL |
| 3 | Add grace period configuration |
| 3 | Add revocation event to audit log |
| 4 | Replace file with HTTP endpoint as primary |
| 5 | Add multi-host consensus (Raft/etcd) |

---

## 7. Sign-Off

**Status:** DOCUMENTED — Accepted for Staged RC with explicit limitations
**Documented:** 2026-09-12
**Review:** 2026-12-12 (or before production migration)