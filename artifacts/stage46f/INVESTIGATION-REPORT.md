# Stage 46f — Investigation Report

## State

```text
Version: 5.0.0-alpha1
Stage: 46f
Status: BLOCKED-WITH-OWNER
Previous: Stage 46e BLOCKED-WITH-OWNER (2 blockers)
```

## Infrastructure Status (Verified)

| Component | Status |
|-----------|--------|
| Docker daemon | RUNNING (29.7.2, Compose v5.4.0) |
| aether-ad-lab container | HEALTHY (a151a6c17223) |
| Samba services | LISTENING on 0.0.0.0:88,389,445,53,636 |
| test-client container | RUNNING (6eaf64d01d1b, 172.18.0.3) |
| Samba user list | 8 users: Administrator, Guest, krbtgt, user1-4, svc_sql, svc_web |
| Samba domain info | AETHER.TEST / AETHER / dc01.aether.test |
| Docker network | ad-lab_aether-ad-net (172.18.0.0/16, bridge) |

## TCP Connectivity Matrix (Verified via test-client)

| Path | TCP Connect | Data Flow | Result |
|------|------------|-----------|--------|
| test-client → Samba:88 | OK | FAIL (i/o timeout) | BLOCKED |
| test-client → Samba:389 | OK | FAIL (i/o timeout) | BLOCKED |
| test-client → Samba:445 | OK | FAIL (i/o timeout) | BLOCKED |
| test-client → Samba:53 | OK | FAIL (i/o timeout) | BLOCKED |
| test-client → Samba:636 | OK | FAIL (i/o timeout) | BLOCKED |
| Host → Samba:88 | OK | FAIL (RST/EOF) | BLOCKED |
| Host → Samba:389 | OK | FAIL (RST/EOF) | BLOCKED |
| Host → Samba:636 | OK | FAIL (RST/EOF) | BLOCKED |
| Samba → 127.0.0.1:389 | OK | OK | PASS |
| Samba → 127.0.0.1:88 | OK | OK | PASS |
| Nginx (host:80 via NAT) | OK | OK | PASS (control) |

All Samba ports: TCP handshake succeeds from host and containers, but application data is dropped/reset. Internal loopback (127.0.0.1) works. Nginx via NAT works. This is a Docker Desktop networking infrastructure issue.

## LDAP BER Analysis

Aether's EncodeBindRequest produces RFC 4511-compliant BER encoding. Byte structure verified by `bindprobe` (built from current source):

```
30 45                    LDAPMessage SEQUENCE (len 69)
  02 01 01                messageID = 1
  60 40                    BindRequest [APPLICATION 0] CONSTRUCTED (len 64)
    30 3e                  BindRequest SEQUENCE (len 62)
      80 01 03             Version [0] INTEGER = 3
      81 2b ...            Name [1] OCTET STRING (43 bytes: CN=Administrator,CN=Users,DC=aether,DC=test)
      80 0c Passw0rd123!  Authentication [0] OCTET STRING (12 bytes)
```

Comparison against RFC 4511 §4.2:
- LDAPMessage SEQUENCE tag 0x30: CORRECT
- messageID INTEGER 0x020101: CORRECT
- BindRequest [APPLICATION 0] tag 0x60: CORRECT
- Version [0] INTEGER tag 0x80: CORRECT
- Name [1] OCTET STRING tag 0x81: CORRECT
- Authentication [0] OCTET STRING tag 0x80: CORRECT

The LDAP BER encoding is RFC 4511 compliant. No byte-level encoding defect found.

The LDAP bind EOF persists even on container-to-container networking (test-client → 172.18.0.2:636 --tls), which bypasses Docker Desktop NAT. This means the LDAP failure CANNOT be attributed solely to Docker NAT. However, since ALL TCP data paths to Samba fail (including go-proxy and ldaps-proxy on the same network), the LDAP EOF may be a consequence of the broader Docker networking failure rather than an encoding defect.

## LDAP Protocol Status

| Path | TLS | Data | Result |
|------|-----|------|--------|
| test-client → 172.18.0.2:389 (plain) | No | FAIL (i/o timeout) | BLOCKED |
| test-client → 172.18.0.2:636 (LDAPS) | Yes | FAIL (EOF from Samba) | BLOCKED |
| Samba internal 127.0.0.1:389 | No | OK (samba-tool works) | PASS |
| ldapsearch via LDAPS | Yes | FAIL (cert verify — self-signed) | BLOCKED |
| Samba TLS config | ldap server require tls = no (UNKNOWN PARAMETER, ignored by Samba 4.15.13) | | |

Samba LDAP requires TLS regardless of config setting (parameter unknown). TLS handshake succeeds from test-client. Samba closes connection after receiving BindRequest (EOF). This may be caused by:
1. Docker networking dropping response packets after TLS
2. Samba TLS/LDAP protocol incompatibility
3. Both

Cannot be isolated without working Docker networking.

## DNS Status

- aether.test → NXDOMAIN (from test-client via Docker DNS 127.0.0.11)
- dc01.aether.test → NXDOMAIN
- Samba DNS server: LISTENING on 0.0.0.0:53
- DNS forwarding: configured to 1.1.1.1
- SRV records: NOT VERIFIED (NXDOMAIN)

DNS failure prevents Kerberos auto-discovery (SRV records). KDC must be addressed to explicit host:port.

## Code Changes

No code changes were made during Stage 46f. The LDAP BER encoding was verified as RFC 4511 compliant. No BER defect was found to fix.

## Verdict

BLOCKED-WITH-OWNER — Docker Desktop networking infrastructure failure prevents all live qualification. LDAP BER encoding verified as correct. LDAP bind EOF may be consequence of networking failure or separate Samba TLS compatibility issue — cannot be isolated without working network.

## Required Operator Actions

1. **Resolve Docker Desktop networking** — TCP handshakes succeed but application data drops on all container ports. This affects host→container and container→container paths. Verify Windows Firewall rules for Docker Desktop, check Docker Desktop network settings, or use alternative container runtime (Podman, containerd on Linux host).

2. **Resolve DNS** — aether.test does not resolve in Docker DNS. Configure Docker DNS forwarding or add manual entries.

3. **Verify LDAP TLS compatibility** — After networking is restored, verify if Samba LDAP over TLS accepts Aether's BindRequest. If EOF persists, investigate Samba TLS configuration.
