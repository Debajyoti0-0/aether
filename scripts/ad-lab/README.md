# Stage 45b / Stage 46 — Samba4 AD Lab

Disposable Active Directory lab for live Kerberos and LDAP qualification.

## Quick Start

```bash
# Start the lab
./scripts/ad-lab/start.sh

# Seed test identities
./scripts/ad-lab/seed.sh

# Run integration tests (Stage 45b)
go test -tags=integration -v ./test/integration/ad/...

# Run Stage 46 integration tests (when available)
go test -tags=integration -v ./test/integration/ad/...

# Stop and clean up
./scripts/ad-lab/stop.sh
```

## Lab Configuration

| Property | Value |
|----------|-------|
| Realm | `AETHER.TEST` |
| Domain | `AETHER` |
| Base DN | `DC=aether,DC=test` |
| DC Hostname | `dc01.aether.test` (resolves to `localhost`) |
| Administrator | `Administrator` / `Passw0rd123!` |
| KDC Port | `88` (TCP/UDP) |
| LDAP Port | `389` (LDAP), `636` (LDAPS) |
| SMB Port | `445` |

## Test Identities

| User | Password | Purpose | Special Properties |
|------|----------|---------|-------------------|
| `user1` | `Passw0rd123!` | Normal user | Kerberos auth, TGT |
| `user2` | `Passw0rd123!` | AS-REP roastable | `DONT_REQ_PREAUTH` set |
| `user3` | `Passw0rd123!` | Domain Admin | Member of Domain Admins |
| `user4` | `Passw0rd123!` | ACL target | `user1` has `GenericAll` |
| `svc_sql` | `SvcPass123!` | Kerberoast target | SPN: `MSSQLSvc/sql01.aether.test:1433` |
| `svc_web` | `WebPass123!` | Kerberoast target | SPN: `HTTP/web01.aether.test` |

## Groups

| Group | Members | Purpose |
|-------|---------|---------|
| `Domain Admins` | `Administrator`, `Tier1-Admins` | Built-in |
| `Tier1-Admins` | `user3` | Nested in Domain Admins |
| `Tier2-Admins` | `user2` | ACL path analysis |

## ACLs

| Principal | Right | Target | Purpose |
|-----------|-------|--------|---------|
| `user1` | `GenericAll` | `user4` | ACL path analysis |
| `user2` | `WriteDACL` | `user1` | ACL path analysis |

## SPNs

| Service Account | SPN |
|-----------------|-----|
| `svc_sql` | `MSSQLSvc/sql01.aether.test:1433` |
| `svc_web` | `HTTP/web01.aether.test` |

## Quick Verification

```bash
# Test KDC connectivity
kinit user1@AETHER.TEST

# Test LDAP
ldapsearch -x -H ldap://localhost:389 -D "CN=Administrator,CN=Users,DC=aether,DC=test" -w "Passw0rd123!" -b "DC=aether,DC=test" -s base

# List users
samba-tool user list -H ldap://localhost:389 -U "Administrator%Passw0rd123!"

# List SPNs
samba-tool spn list svc_sql -H ldap://localhost:389 -U "Administrator%Passw0rd123!"
```

## Cleanup

```bash
./scripts/ad-lab/stop.sh
```

This removes the container, volumes, and network. The lab is fully disposable.