# Stage 45 WS2 — Batch 1 Engines

## Package: `internal/engine/ad/kerberos/`

### Files Created

| File | Description | Lines |
|------|-------------|-------|
| `enum.go` | User enumeration, AS-REP enumeration, SPN enumeration | ~390 |
| `roast.go` | Kerberoasting, AS-REP roasting | ~370 |
| `tgt.go` | TGT acquisition, CCache management | ~360 |
| `engine_test.go` | Unit tests for all mutations | ~230 |

**Total**: ~1,350 lines of engine implementation + tests

### Mutation Architecture

All engines implement the `spine.Mutation` interface:

```go
type Mutation interface {
    Kind() string
    Target() string
    BeforeState() ([]byte, error)
    Execute(ctx context.Context) (string, error)
    AfterState() ([]byte, error)
    UndoRecipe() *spine.UndoSpec
    // Engine-specific methods:
    Name() string
    Capability() string
    RiskScore() int
    Reversible() bool
    GenerateEvidence() []types.EvidenceRecord
}
```

### Mutations Implemented

| Mutation | Kind | Capability | Risk | Reversible | Description |
|----------|------|------------|------|------------|-------------|
| `EnumUsersMutation` | `ad.kerberos.enum.users` | `ad.enum.read` | 10 | ✅ | Username validation via AS-REQ |
| `EnumASREPMutation` | `ad.kerberos.enum.asrep` | `ad.enum.read` | 10 | ✅ | Detect accounts without pre-auth |
| `EnumSPNMutation` | `ad.kerberos.enum.spn` | `ad.enum.read` | 10 | ✅ | Enumerate SPN-registered accounts |
| `KerberoastMutation` | `ad.kerberos.kerberoast` | `ad.kerberos.roast` | 20 | ✅ | Request TGS per SPN → extract crackable blob |
| `ASREPRoastMutation` | `ad.kerberos.asreproast` | `ad.kerberos.roast` | 20 | ✅ | Request AS-REP for no-preauth accounts |
| `TGTMutation` | `ad.kerberos.tgt` | `ad.kerberos.tgt` | 15 | ✅ | Get TGT for valid credential → ccache |
| `CCacheMutation` | `ad.kerberos.ccache` | `ad.kerberos.tgt` | 10 | ✅ | Read/write/inspect/convert ccache files |

### Kerberos Transport

```go
type kerberosTransport struct {
    conn   net.Conn
    dc     string
    domain string
}

func newKerberosTransport(dc, domain string) (*kerberosTransport, error)
func (t *kerberosTransport) Send(data []byte) error
func (t *kerberosTransport) Recv() ([]byte, error)
func (t *kerberosTransport) Close() error
```

TCP transport to KDC port 88 with 4-byte length prefix framing.

### Evidence Generation

Every mutation implements `GenerateEvidence() []types.EvidenceRecord` producing `types.EvidenceRecord` with:

- `EpistemicClass: types.ClassObserved` (confidence 1.0)
- `Kind`: operation type (e.g., `ad.kerberos.kerberoast`)
- `Method`: protocol method (e.g., `kerberos-tgsreq`)
- `Payload`: JSON with operation details (no secrets, only hashes)

### Test Coverage

All 10 unit tests pass:
- Mutation interface compliance (7 mutations)
- Input validation (4 test suites)
- Risk scores (7 mutations)
- Reversibility (7 mutations)
- TCP framing
- Time truncation

### Dependencies

- Protocol: `internal/protocol/kerberos/`
- Spine: `internal/engine/spine/` (Mutation interface)
- Types: `internal/types/` (EvidenceRecord, ClassObserved)
- Transport: `net` (stdlib)
- **No external dependencies**

### Integration Points

| Mutation | Spine Stage | Evidence | Reversible |
|----------|-------------|----------|------------|
| EnumUsers | Execute | ClassObserved | ✅ |
| EnumASREP | Execute | ClassObserved | ✅ |
| EnumSPN | Execute | ClassObserved | ✅ |
| Kerberoast | Execute | ClassObserved | ✅ |
| ASREPRoast | Execute | ClassObserved | ✅ |
| TGT | Execute | ClassObserved | ✅ |
| CCache (show) | Execute | ClassObserved | ✅ |
| CCache (convert) | Execute | ClassObserved | ✅ (UndoRecipe provided) |

### Next Steps (WS3)

Implement CLI surface in `internal/cli/ad/`:
- `aether ad enum users/asrep/spn`
- `aether ad kerberoast`
- `aether ad asreproast`
- `aether ad tgt`
- `aether ad ccache show/convert`