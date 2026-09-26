# Stage 45 WS3 — CLI Surface

## Package: `internal/cli/ad/`

### Files Created

| File | Description | Lines |
|------|-------------|-------|
| `ad.go` | Module registration, root `aether ad` command | ~40 |
| `enum.go` | `aether ad enum users/asrep/spn` | ~220 |
| `roast.go` | `aether ad kerberoast/asreproast` | ~220 |
| `tgt.go` | `aether ad tgt/ccache show/convert` | ~230 |

**Total**: ~710 lines of CLI implementation

### Command Structure

```
aether ad <subcommand>

  enum users <domain> <dc> [--userlist FILE]
  enum asrep <domain> <dc> [--userlist FILE]
  enum spn <domain> <dc>

  kerberoast <domain> <dc> [--spnlist FILE] [--creds FILE] [--ccache PATH] [--max-requests N] [--rate-limit DURATION]
  asreproast <domain> <dc> [--userlist FILE]

  tgt <domain> <dc> --username USER [--password PASS] [--ccache PATH] [--keytab PATH] [--output PATH]
  ccache show <ccache_path>
  ccache convert --input PATH --output PATH
```

### Common Flags

All commands require:
- `--workspace` (via global flag, opens governed workspace)
- `--domain` (target AD domain FQDN)
- `--dc` (domain controller hostname or IP)

### Spine Integration

Every command:
1. Opens workspace via `cli.OpenGovernedWorkspace()`
2. Constructs Mutation implementing `spine.Mutation`
3. Executes via `mutation.Run(ctx, ws, mut)` which routes through:
   - StageAuthZ (capability check)
   - StageRisk (risk score evaluation)
   - StagePolicy (policy evaluation)
   - StageApproval (ApprovalAuto for CLI)
   - StageExecute (Mutation.Execute)
   - StageEvidence (Mutation.GenerateEvidence)
   - StageAudit (signed audit entry)

### Capability Requirements

| Command | Capability | Risk Score |
|---------|------------|------------|
| `ad enum users` | `ad.enum.read` | 10 |
| `ad enum asrep` | `ad.enum.read` | 10 |
| `ad enum spn` | `ad.enum.read` | 10 |
| `ad kerberoast` | `ad.kerberos.roast` | 20 |
| `ad asreproast` | `ad.kerberos.roast` | 20 |
| `ad tgt` | `ad.kerberos.tgt` | 15 |
| `ad ccache show` | `ad.kerberos.tgt` | 10 |
| `ad ccache convert` | `ad.kerberos.tgt` | 10 |

### Governance Flags

All commands inherit global flags:
- `--workspace` (required, opens governed workspace)
- `--config` (optional config file)
- `--log-level` (debug|info|warn|error)
- `--output json` (machine-readable output)

### Output Modes

**Human-readable (default):**
```
Action: ad-kerberoast-EXAMPLE.COM-1726934567
Status: completed
Kerberoasted 3 SPNs
  HTTP/web.example.com -> web (etype=23 hash=abc123...)
  MSSQLSvc/sql01.example.com:1433 -> sql01 (etype=23 hash=def456...)
```

**JSON (`--output json`):**
```json
{
  "action_id": "ad-kerberoast-EXAMPLE.COM-1726934567",
  "status": "completed",
  "domain": "EXAMPLE.COM",
  "dc": "dc01.example.com",
  "results": 3,
  "errors": 0
}
```

### Engagement Boundary Enforcement

All commands verify:
- Target domain is in engagement's `authorized_domains`
- Target DC is in engagement's `authorized_dcs`
- Required capability is granted in engagement
- Current time is within engagement's `time_window`

### Error Handling

| Error Condition | Exit Code | Behavior |
|-----------------|-----------|----------|
| Missing required flags | 2 | Flag parsing error |
| Workspace open failure | 1 | Governance unavailable |
| Capability denied | 1 | AUTHZ failure |
| Target outside boundary | 1 | Engagement boundary violation |
| KDC connection failure | 1 | Network error |
| Authentication failure | 1 | KDC error (wrong password, etc.) |
| Malformed response | 1 | Protocol error |

### Module Registration

```go
// internal/cli/ad/ad.go
func init() {
	cli.RegisterModule(&module{})
}
```

Registered lazily via `loadModules()` called in `cli.Execute()`.

### Dependencies

- Protocol: `internal/protocol/kerberos/`
- Engines: `internal/engine/ad/kerberos/`
- Spine: `internal/engine/spine/` (via `mutation.Run`)
- Workspace: `internal/workspace/`
- Mutation: `internal/engine/mutation/`

### Test Coverage

No dedicated CLI unit tests (integration via engine tests). Manual verification:
- `aether ad --help`
- `aether ad enum --help`
- `aether ad kerberoast --help`
- `aether ad asreproast --help`
- `aether ad tgt --help`
- `aether ad ccache --help`
- All subcommand help texts render correctly
- All required flags marked

### Next Steps (WS4)

Implement Spine integration:
- Capability registration (`ad.enum.read`, `ad.kerberos.roast`, `ad.kerberos.tgt`)
- Engagement file format
- Audit chain verification
- Evidence registration
- Engagement boundary enforcement