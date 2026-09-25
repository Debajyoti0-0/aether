# Stage 45b — Samba4 AD Lab Harness

## Overview

This document describes the reproducible Samba4 AD DC deployment used for Stage 45b live Kerberos qualification.

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
| SMB Port | `445` (mapped to host `4445`) |

## Deployment

### Docker Compose

```yaml
version: '3.8'
services:
  aether-ad-lab:
    image: aether-samba-ad-dc:latest
    hostname: dc01
    domainname: aether.test
    container_name: aether-ad-lab
    privileged: true
    environment:
      - SAMBA_DOMAIN=AETHER
      - SAMBA_REALM=AETHER.TEST
      - ADMIN_PASSWORD=Passw0rd123!
      - KERBEROS_ENCRYPTION_TYPES=aes256-cts-hmac-sha1-96,aes128-cts-hmac-sha1-96,rc4-hmac
      - DNS_FORWARDER=1.1.1.1
    ports:
      - "53:53/tcp"
      - "53:53/udp"
      - "88:88/tcp"
      - "88:88/udp"
      - "135:135/tcp"
      - "139:139/tcp"
      - "389:389/tcp"
      - "389:389/udp"
      - "4445:445/tcp"
      - "464:464/tcp"
      - "464:464/udp"
      - "636:636/tcp"
      - "3268:3268/tcp"
      - "3269:3269/tcp"
    volumes:
      - samba_data:/var/lib/samba
      - samba_etc:/etc/samba
    networks:
      - aether-ad-net
    healthcheck:
      test: ["CMD", "samba-tool", "domain", "info", "127.0.0.1"]
      interval: 30s
      timeout: 10s
      retries: 5
      start_period: 60s
```

### Startup Scripts

- `scripts/ad-lab/start.sh` — Brings up container, waits for healthcheck, verifies ports
- `scripts/ad-lab/stop.sh` — Clean teardown (container, volumes, network)
- `scripts/ad-lab/seed.sh` — Creates deterministic test identities

## Test Identities

| User | Password | Purpose | Special Properties |
|------|----------|---------|-------------------|
| `user1` | `Passw0rd123!` | Normal user | Kerberos auth, TGT |
| `user2` | `Passw0rd123!` | AS-REP roastable | `DONT_REQ_PREAUTH` (userAccountControl=4194304) |
| `user3` | `Passw0rd123!` | Domain Admin | Member of `Domain Admins` via `Tier1-Admins` |
| `user4` | `Passw0rd123!` | ACL target | `user1` has `GenericAll` |
| `svc_sql` | `SvcPass123!` | Kerberoast target | SPN: `MSSQLSvc/sql01.aether.test:1433` |
| `svc_web` | `WebPass123!` | Kerberoast target | SPN: `HTTP/web01.aether.test` |

### Groups

| Group | Members | Purpose |
|-------|---------|---------|
| `Domain Admins` | `Administrator`, `Tier1-Admins` | Built-in |
| `Tier1-Admins` | `user3` | Nested in Domain Admins |
| `Tier2-Admins` | `user2` | ACL path analysis |

### ACLs

| Principal | Right | Target | Purpose |
|-----------|-------|--------|---------|
| `user1` | `GenericAll` | `user4` | ACL path analysis |
| `user2` | `WriteDACL` | `user1` | ACL path analysis |

### SPNs

| Service Account | SPN |
|-----------------|-----|
| `svc_sql` | `MSSQLSvc/sql01.aether.test:1433` |
| `svc_web` | `HTTP/web01.aether.test` |

## Verification

```bash
# Start lab
./scripts/ad-lab/start.sh

# Seed identities
./scripts/ad-lab/seed.sh

# Verify KDC connectivity
docker exec aether-ad-lab kinit -k -t /tmp/test.keytab user1@AETHER.TEST

# Verify LDAP
docker exec aether-ad-lab ldapsearch -x -H ldap://localhost:389 -D "CN=Administrator,CN=Users,DC=aether,DC=test" -w "Passw0rd123!" -b "DC=aether,DC=test" -s base

# List users
docker exec aether-ad-lab samba-tool user list

# List SPNs
docker exec aether-ad-lab samba-tool spn list svc_sql
docker exec aether-ad-lab samba-tool spn list svc_web
```

## Known Limitations

1. **Kerberos Encryption Types**: The `samba-tool domain setencryptiontypes` command is not available in Samba 4.15.13 (Ubuntu 22.04). RC4-HMAC is marked as DEPRECATED in keytabs. AES256/AES128 are functional.

2. **Password Authentication**: Custom ASN.1 encoding in the protocol library causes KDC to reject password-based AS-REQ with EOF. Keytab/ccache-based authentication works correctly.

3. **IPv6**: On Windows hosts, `localhost` resolves to `::1`. Scripts and code must use `127.0.0.1` explicitly.

## Artifacts

- `artifacts/stage45b/user1.ccache` — Valid TGT for user1 (obtained via keytab)
- `artifacts/stage45b/*.json` — Test execution artifacts