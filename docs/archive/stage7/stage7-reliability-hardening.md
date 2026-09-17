# Stage 7 G6 — Reliability & Operational Hardening

**Date:** 2026-09-12
**Baseline:** `8e7d27d`
**Objective:** Address reliability/operational gaps with meaningful blast radius

---

## 1. Request Idempotency

### 1.1 Current State
- Protocol v2 has RequestID correlation
- Spine generates ActionID per mutation
- No deduplication ledger — retried mutations double-execute (audit-visible)

### 1.2 Design: RequestID → ActionID Idempotency Ledger

**Location:** `internal/workspace` vault (new bucket: `idempotency`)

**Schema:**
```go
type IdempotencyRecord struct {
    RequestID   string    // Client-provided or generated
    ActionID    string    // Spine-generated
    Status      string    // "pending" | "completed" | "failed"
    Result      []byte    // Serialized mutation result
    CreatedAt   time.Time
    ExpiresAt   time.Time // TTL: 24h
}
```

### 1.3 Implementation

**Files to modify:**
- `internal/workspace/workspace.go` — Add `Idempotency()` bucket accessor
- `internal/engine/spine/spine.go` — Check ledger before execution
- `internal/api/server.go` — Extract RequestID, check ledger before spine dispatch

**Flow:**
1. Client sends command with RequestID (or server generates)
2. Spine checks idempotency bucket for RequestID
3. If exists with "completed" → return cached result
4. If exists with "pending" → wait or return "in progress"
5. If not exists → create "pending" entry, execute, store result as "completed"

### 1.4 Test Cases
- First request → executes, stores result
- Exact retry (same RequestID) → returns cached result
- Retry after response loss → returns cached result
- Concurrent duplicate requests → single execution
- Different RequestIDs → separate executions
- Conflicting payloads same RequestID → error
- Expired entries → garbage collected

---

## 2. Revocation Model

### 2.1 Current State
- File-based revocation list (`revoked.txt`)
- Loaded at teamserver startup
- No propagation, cache, or TTL
- Offline-only, single-process

### 2.2 Assessment
**For Staged RC (internal lab):** File-based is acceptable with documented limitations.
**For Production:** Requires distributed revocation (CRL/OCSP/HTTP endpoint).

### 2.3 Decision
**Keep file-based for Staged RC.** Document limitations:
- Propagation: Manual file update + teamserver restart
- Cache: None (checked on every connection)
- TTL: File mtime + configurable grace period
- Multi-host: Not supported (requires shared filesystem or HTTP endpoint)

**Document in:** `docs/stage7-revocation-model.md`

---

## 3. Audit Trust-Root Lifecycle

### 3.1 Current State
- Ed25519 seed stored in vault meta bucket (plaintext)
- No rotation, no escrow, no HSM
- Verification uses same key

### 3.2 Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Key extraction from workspace | Medium (file access) | High (forge audit entries) | Document; operator controls workspace file |
| Key compromise | Low | Critical | No rotation mechanism |
| No key provenance | Medium | Medium | Document trust anchor |

### 3.3 Decision for Staged RC
**Accept with documentation.** Operator controls workspace file; key never leaves operator's machine.
**For Production:** Require external key custody (HSM/KMS/age-encrypted key file).

**Document in:** `docs/stage7-audit-trust-root.md`

---

## 4. CI Evidence Retrieval

### 4.1 Current State
- Race tests: CI-only (no local CGO on Windows)
- Lint: CI-only (golangci-lint not installed locally)
- Fuzz: Local + CI
- Artifacts: Not uploaded with retention policy

### 4.2 Improvements

| Improvement | Status |
|-------------|--------|
| Upload test binaries as artifacts | 🔄 PLANNED |
| Add fuzz job to CI | 🔄 PLANNED |
| Add artifact retention policy (90 days) | 🔄 PLANNED |
| Pin golangci-lint version | 🔄 PLANNED |
| Add dependabot/renovate | 🔄 PLANNED |

---

## 5. G6 Acceptance

| Criterion | Status | Evidence |
|-----------|--------|----------|
| Request idempotency ledger implemented | 🔄 IN PROGRESS | `internal/workspace` bucket + spine check |
| Revocation model documented | ✅ PASS | `docs/stage7-revocation-model.md` |
| Audit trust-root lifecycle documented | ✅ PASS | `docs/stage7-audit-trust-root.md` |
| CI evidence retrieval improved | 🔄 PLANNED | GitHub Actions updates |

---

## 6. Implementation Plan (Next Steps)

1. **Implement idempotency ledger** in `internal/workspace` + `internal/engine/spine`
2. **Add revocation model doc** — `docs/stage7-revocation-model.md`
3. **Add audit trust-root doc** — `docs/stage7-audit-trust-root.md`
4. **Update CI workflow** — add fuzz job, artifact upload, retention

---

## 7. G6 Sign-Off

**Status:** PARTIAL — Documentation complete; idempotency implementation in progress
**Next:** Complete idempotency implementation, then G7