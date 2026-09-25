# Stage 46e Investigation Report

## Summary

Stage 46e was tasked with resolving the Stage 46d BLOCKED-WITH-OWNER condition on Samba4 reachability and completing live qualification. Two blockers remain.

## Infrastructure Diagnosis

### Primary Blocker: Docker Desktop NAT Data Path Failure

Docker Desktop NAT on Windows establishes TCP handshakes to container ports (88, 389, 445, 53, 636, etc.) but drops all application data after the handshake. This affects ALL container ports and ALL protocols (TCP/UDP).

**Evidence:**
- TCP handshake to container IP succeeds (200ms)
- First data packet from host is dropped/reset
- Same port works fine via Nginx container (port 80 control proves NAT works for port 80 traffic)
- Container-to-container networking works perfectly (bypassing NAT entirely)
- Test-NetConnection to localhost:389 shows TCP success but data flow fails

### Secondary Blocker: Aether LDAP Protocol Defect

LDAP bind returns EOF even on container-to-container network (test-client → 172.18.0.2:636 --tls), which bypasses Docker NAT entirely. TCP connection succeeds, TLS handshake succeeds, Samba receives the bind request, then closes the connection. This indicates a BER encoding or protocol compatibility issue between Aether's LDAP implementation and Samba4's LDAP server.

### Tertiary Finding: Samba Interface Binding

Initial Samba config had `bind interfaces only = Yes` with `interfaces = lo eth0`, causing Samba to bind only on 127.0.0.1 (loopback). Fixed to `bind interfaces only = No`.

## Remediation Attempts

### Infrastructure Remedies

| Remedy | Result | Details |
|--------|--------|---------|
| Fix Samba bind to 0.0.0.0 | SUCCESS | Changed `bind interfaces only` to `No` |
| Docker Desktop NAT fix | FAILED | All ports drop data (infrastructure limitation) |
| netsh portproxy | FAILED | TCP handshake succeeds, data times out |
| --network host | FAILED | VM network, not Windows host network |
| WSL2 native Samba | FAILED | Ubuntu WSL2 installed but cannot reach 172.18.0.x container network (WSL2 on 172.29.x.x subnet, container on 172.18.0.x); container→host (172.18.0.1) also unreachable |
| TCP proxy via port 80 | PARTIAL | TLS handshake works, LDAP EOF after bind |
| LDAPS via proxy | PARTIAL | TLS handshake works, LDAP EOF after bind |

### Code/Protocol Remedies

| Remedy | Result | Details |
|--------|--------|---------|
| Fix LDAP wire format (engine.go) | PARTIAL | Removed non-standard 4-byte length prefix; still EOF on bind |
| Fix BindRequest ASN.1 tags (types.go) | PARTIAL | Added RFC 4511 context-specific tags; still EOF on bind |
| LDAPS via tlscheck proxy | PARTIAL | TLS handshake to 636 works through proxy chain; LDAP bind after TLS still EOF |
| Workspace passphrase workaround | PARTIAL | AETHER_PASSPHRASE env var resolves workspace open; workspace create --passphrase makes keyless workspace cannot be reopened without env var |

## Qualification Test Results

### From test-client container (container-to-container network, aether-ad-net)

| Test | Result | Notes |
|------|--------|-------|
| LDAP bind LDAPS (172.18.0.2:636 --tls) | FAIL | TCP+TLS handshake OK, Samba closes connection after receiving bind request (EOF) |
| LDAP bind plain (172.18.0.2:389) | BLOCKED | Samba requires TLS (Strong auth required / 8) |
| AD enum users | BLOCKED | Requires successful bind first |
| Kerberos enum | BLOCKED | KDC connectivity fails (UDP timeout, TCP fallback stuck) even on container network |

### From Windows host

| Test | Result | Notes |
|------|--------|-------|
| TCP handshake to container ports | OK | All ports (88, 389, 445, 53, 636) accept TCP |
| Data flow to container ports | FAIL | Application data dropped after handshake on all ports |
| localhost:389 TCP | OK | TCP handshake succeeds |
| localhost:389 data | FAIL | Data drops (Docker NAT) |

## Code Changes Made During Investigation

### 1. Non-standard LDAP wire format (engine.go)
`internal/engine/ad/ldap/engine.go` — `send()` prepended a 4-byte length prefix to LDAP messages (non-standard). Removed to use standard BER TLV encoding only. `receive()` now reads BER TLV header instead of fixed 4-byte prefix.

### 2. BindRequest ASN.1 encoding (types.go)
`internal/protocol/ldap/types.go` — `BindRequest` struct fields lacked explicit RFC 4511 context-specific tags. Added `asn1:"tag:N,class:context-specific"` to Version, Name, and Controls fields.

Both changes were applied and tested. LDAP bind still returns EOF on both container-to-container and host→container paths.

## Current State

- Samba AD DC: HEALTHY, running in container, accessible from container network
- KDC (port 88): LISTENING, no response to Kerberos requests from inside network (UDP timeout, TCP fallback stuck)
- LDAP (port 389/636): LISTENING, TLS handshake works via proxy, but bind response EOF
- Docker NAT: HOST→CONTAINER DATA PATH BROKEN (infrastructure limitation)
- Aether CLI: Code defects identified (LDAP wire format, ASN.1 encoding), fixes applied but qualification still fails
- Workspace governance: AETHER_PASSPHRASE env var required to open passphrase-protected workspaces; workspace create --passphrase creates keyless workspace that cannot be reopened without env var

## Verdict: BLOCKED-WITH-OWNER

**Two independent blockers remain:**

1. **Docker Desktop NAT infrastructure failure** (PRIMARY): Host→container data path is broken at the Docker Desktop NAT layer. All remediation attempts (netsh portproxy, --network host, WSL2 native, TCP proxy) fail to establish a data path. This requires Windows host-level intervention or Docker Desktop configuration change.

2. **Aether LDAP protocol defect** (SECONDARY): LDAP bind returns EOF even when networking works (container-to-container path). This is a code-level issue requiring further BER encoding/protocol compatibility debugging between Aether's LDAP implementation and Samba4's LDAP server. The BindRequest ASN.1 encoding fix was applied but the defect persists.

## Required Operator Actions

1. Resolve Docker Desktop NAT infrastructure issue to enable host→container data flow (check Windows Firewall, Docker Desktop network settings, or use alternative container runtime)
2. Debug Aether LDAP BER encoding against Samba4 LDAP server expectations (test with known-good LDAP client like `ldapsearch` from same code path to isolate encoding vs. server compatibility issue)
3. Verify Kerberos KDC UDP/TCP connectivity path once networking is resolved
4. After both blockers resolved, re-run Batch 1 (infrastructure connectivity) and Batch 2 (live qualification) tests per Stage 46e specification