# Stage 45 WS4 — Spine Integration

## Capabilities Registered

| Capability | Default | Risk Score | Reversible | Category |
|------------|---------|------------|------------|----------|
| `ad.enum.read` | deny | 10 | yes | enumeration |
| `ad.kerberos.roast` | deny | 20 | yes | credential-access |
| `ad.kerberos.tgt` | deny | 15 | yes | credential-access |

Registered in `internal/engine/cap/registry.go` via `cap.MustRegister()`.

## Engagement File Format

```json
{
  "engagement_id": "eng-001",
  "authorized_domains": ["EXAMPLE.COM", "CORP.LOCAL"],
  "authorized_dcs": ["dc01.example.com", "dc02.example.com"],
  "authorized_capabilities": ["ad.enum.read", "ad.kerberos.roast", "ad.kerberos.tgt"],
  "time_window": {
    "start": "2026-01-01T00:00:00Z",
    "end": "2026-12-31T23:59:59Z"
  },
  "operator": "alice",
  "rate_limits": {
    "requests_per_minute": 60,
    "burst": 10
  }
}
```

Loaded via `--engagement <file>` flag on all `ad` commands.

## Audit Chain

Every `ad` command produces a signed audit entry:

```json
{
  "action_id": "ad-kerberoast-EXAMPLE.COM-1726934567",
  "kind": "ad.kerberos.kerberoast",
  "target": "dc01.example.com/EXAMPLE.COM",
  "actor": "alice",
  "workspace": "ws-eng-001",
  "timestamp": "2026-09-21T12:34:56Z",
  "risk_score": 20,
  "status": "completed",
  "evidence_ids": ["ev-kerberoast-HTTP/web-1726934567"],
  "prev_hash": "sha256:abc123...",
  "signature": "ed25519:xyz789..."
}
```

## Evidence Registration

Each mutation implements `GenerateEvidence() []types.EvidenceRecord`:

```go
func (m *KerberoastMutation) GenerateEvidence() []types.EvidenceRecord {
    // Returns evidence with:
    // - EpistemicClass: ClassObserved (confidence 1.0)
    // - Kind: "ad.kerberos.kerberoast"
    // - Payload: JSON with SPN, account, etype, hash_sha256
}
```

Evidence stored in workspace's evidence bucket, indexed by action_id.

## Engagement Boundary Enforcement

Every command verifies before execution:

```go
func verifyEngagement(eng *Engagement, domain, dc string, caps []string) error {
    // 1. Domain in authorized_domains
    // 2. DC in authorized_dcs (or domain matches)
    // 3. All required caps in authorized_capabilities
    // 4. Now() within time_window
    // 5. Rate limit not exceeded
}
```

Fail-closed: any check fails → command exits with error, audit entry recorded.

## Rate Limiting

Engagement file specifies:
```json
"rate_limits": {
    "requests_per_minute": 60,
    "burst": 10
}
```

Enforced per workspace. Exceeding → `ErrRateLimited`.

## Capability AuthZ Flow

```
CLI Command
    ↓
mutation.Run()
    ↓
spine.New(ws).Run()
    ↓
stageAuthz(a) → checks RequiredCaps against ws.Caps
    ↓
    allow → stageRisk → stagePolicy → stageApproval → Execute
    deny  → abort with StatusAborted
```

## Verification Commands

```bash
# Verify audit chain integrity
aether audit verify --workspace ws-eng-001

# Verify evidence integrity
aether export verify-evidence --workspace ws-eng-001

# List capabilities
aether cap list --workspace ws-eng-001

# Grant capability
aether cap grant --workspace ws-eng-001 --cap ad.kerberos.roast --to alice
```

## Gate Verification

| Gate | Verification |
|------|--------------|
| G2033 | `ad.enum.read` default deny, grant works |
| G2034 | `ad.kerberos.roast` default deny, grant works |
| G2035 | `ad.kerberos.tgt` default deny, grant works |
| G2036 | Risk scores in audit: 10/20/15 |
| G2037 | Audit chain `aether audit verify` → VERIFIED |
| G2038 | Evidence registered per operation |
| G2039 | Outside engagement boundary → exit 1 |
| G2040 | Outside time window → exit 1 |

## Dependencies

- `internal/engine/cap/` — capability registry
- `internal/engine/spine/` — Action, stages, audit
- `internal/types/` — EvidenceRecord, EpistemicClass
- `internal/workspace/` — Workspace, evidence storage
- `internal/engine/mutation/` — Mutation.Run adapter