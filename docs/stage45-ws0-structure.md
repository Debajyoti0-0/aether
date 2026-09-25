# Stage 45 WS0 — Repository Structure & Version Line

## Directories Created

```bash
mkdir -p internal/protocol/kerberos
mkdir -p internal/engine/ad/kerberos
mkdir -p internal/cli/ad
mkdir -p testdata/ad
mkdir -p scripts/ad-lab
```

## Version Update

```bash
echo "5.0.0-alpha1" > VERSION
```

**Previous**: `3.4.0-stage3`
**New**: `5.0.0-alpha1`

### Rationale for 5.0.0 (New Major)

Per semantic versioning, a new major version indicates incompatible API changes or a significant new capability domain. The addition of Active Directory (on-prem) support represents:

1. **New capability domain** — `ad.*` capabilities, `aether ad` CLI surface
2. **New protocol packages** — `internal/protocol/kerberos/`, future `ldap/`, `ms-wcce/`
3. **New engine packages** — `internal/engine/ad/kerberos/`, future `ad/ldap/`, `ad/adcs/`
4. **New test infrastructure** — `scripts/ad-lab/` for Samba4 AD DC
5. **New CLI surface** — `aether ad enum`, `aether ad kerberoast`, etc.

This is a fundamental expansion of Aether's scope from cloud identity (Azure, GCP, Okta, etc.) to on-premises Active Directory.

### Version Discipline

- No git tag created (`git tag -l 'v5.*'` returns empty)
- Version string only in `VERSION` file and build `ldflags`
- Monotonic progression: `3.4.0-stage3` → `5.0.0-alpha1` (skipping 4.x which didn't land)

## Package Inventory (Post-WS0)

| Category | Existing | Added in WS0 |
|----------|----------|--------------|
| Protocol | msoapx, oauth2, saml, wstrust (4) | kerberos (1) |
| Engine | 13 | ad, ad/kerberos (2) |
| CLI | 29 command modules | ad, ad_kerberos (2) |
| Test | testdata/ | testdata/ad (1) |
| Scripts | genman, rmdir | ad-lab (1) |

## Architecture Map

```
internal/
├── protocol/
│   ├── kerberos/          ← NEW (WS1)
│   ├── msoapx/
│   ├── oauth2/
│   ├── saml/
│   └── wstrust/
├── engine/
│   ├── ad/
│   │   └── kerberos/      ← NEW (WS2)
│   ├── cap/               ← EXISTING (capability registry)
│   ├── spine/             ← EXISTING (authz, audit, evidence)
│   └── ...
├── cli/
│   ├── ad/                ← NEW (WS3)
│   │   ├── enum.go
│   │   ├── kerberoast.go
│   │   ├── asreproast.go
│   │   ├── tgt.go
│   │   └── ccache.go
│   └── ...
├── store/                 ← EXISTING (vault for ticket storage)
└── types/                 ← EXISTING (evidence, risk, credential)
```

## Integration Boundaries

| Component | Reuses | Extends |
|-----------|--------|---------|
| Transport | `internal/transport/` (TCP/TLS) | KDC connection logic |
| Credential | `internal/types/credential.go` | Kerberos-specific cred types |
| Vault | `internal/store/` | Ticket/key storage |
| Audit | `internal/engine/spine/` | Kerberos operation records |
| Evidence | `internal/types/evidence.go` | Ticket hashes, roast material hashes |
| Capability | `internal/engine/cap/` | `ad.enum.read`, `ad.kerberos.roast`, `ad.kerberos.tgt` |
| Authorization | `internal/engine/spine/` | Engagement boundary, rate limiting |
| CLI | `internal/cli/` (Cobra) | `aether ad` command tree |

## Gate WS0 Verification

```bash
# G2005: Structure created
ls -la internal/protocol/kerberos/ internal/engine/ad/kerberos/ internal/cli/ad/ testdata/ad/ scripts/ad-lab/

# G2003: Clean-room build with version
cd /c/dev/clean-room-45 && git clone C:/dev/aether .
go build -ldflags "-X main.Version=5.0.0-alpha1" -o bin/aether-45.exe ./cmd/aether
./bin/aether-45.exe --version  # → 5.0.0-alpha1
```