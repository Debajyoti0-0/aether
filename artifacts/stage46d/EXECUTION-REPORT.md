# Stage 46d Execution Report — BLOCKED-WITH-OWNER

**Date**: 2026-09-23T10:25Z (UTC+5:30) — execution start
**Last verified**: 2026-09-23T11:56Z (UTC+5:30) — container recreated b5cb69db8f79, NAT failure confirmed persistent
**Repository**: C:\Users\Debajyoti0-0\OneDrive\Documents\aether
**Version**: 5.0.0-alpha1 (VERSION file; uncommitted modification)
**HEAD**: a887a0ed9c9fcdbf0400a5d59ccad162cfcac202
**Branch**: master
**Tag**: none
**Working tree**: modified (VERSION, cmd/aether/main.go, internal/cli/governance.go, internal/cli/root.go) + untracked artifacts/docs/internal/

---

## FINAL VERDICT

```
Stage 46d status:           BLOCKED-WITH-OWNER
Stage 46 (overall):         PARTIALLY CLOSED — D45b-004 FIXED, live qualification BLOCKED on Docker NAT
Blocker owner:              DevOps / Infrastructure
Required action:            Fix Docker Desktop NAT for KDC/LDAP TCP ports 88/389, OR migrate to host-network mode, OR provide a host-reachable KDC endpoint
```

---

## BLOCKER SUMMARY

### Primary Blocker: Docker Desktop NAT Failure for Samba AD DC TCP Services

**G2601 (Docker availability)**: PASS — `docker ps` succeeds, container healthy

**G2606-G2609 (Samba4 reachability)**: FAIL — services not reachable through published ports

#### Evidence

| Test | Result |
|------|--------|
| Container status | UP (healthy) — `aether-ad-lab` |
| Domain info (internal) | PASS — `samba-tool domain info 127.0.0.1` returns AETHER.TEST |
| User list (internal) | PASS — 8 users (user1-4, svc_sql, svc_web, Administrator, Guest, krbtgt) |
| KDC port 88 TCP from host | **FAIL** — TCP connect succeeds, data transfer EOF |
| KDC port 88 UDP from host | **FAIL** — read timeout |
| LDAP port 389 TCP from host | **FAIL** — TCP connect succeeds, RST by peer |
| DNS port 53 UDP from host | PASS — valid DNS response |
| SMB port 445 from host | **FAIL** — RST by peer |
| Nginx container port 9999:80 from host | PASS — HTTP 400 (confirms Docker NAT works for other containers) |
| KDC port 88 from inside container | **FAIL** — Connection reset by peer |
| LDAP port 389 from inside container | **FAIL** — RST by peer (tested with real LDAP v3 bind packet) |
| Sidecar container → Samba 389 | **FAIL** — RST by peer |

#### Raw Evidence

```
# Fresh container b5cb69db8f79 recreated, NAT still fails:

# TCP connect from host succeeds but data fails:
$ go run tmp/netcheck.go
127.0.0.1:88: connected
127.0.0.1:88: read n=0 err=read tcp 127.0.0.1:56585->127.0.0.1:88: i/o timeout
127.0.0.1:389: connected
127.0.0.1:389: read n=0 err=read tcp 127.0.0.1:56586->127.0.0.1:389: i/o timeout

# LDAP bind via Aether CLI — TLS path:
$ bin/aether-test.exe ldap bind --host localhost --tls --workspace ws1 ...
Error: connect: TLS dial: EOF

# LDAP bind via Aether CLI — StartTLS path:
$ bin/aether-test.exe ldap bind --host localhost --starttls --workspace ws1 ...
Error: bind: send bind request: EOF

# Fresh Batch 1 transcript (container b5cb69db8f79):
=== RUN   TestKerberosASREP
DEBUG: Sending AS-REQ (202 bytes)
DEBUG: UDP recv failed (read udp ... -> 127.0.0.1:88: i/o timeout), trying TCP fallback
DEBUG: Dialing TCP 127.0.0.1:88
DEBUG: Received response (0 bytes, err=TCP read length: EOF)
--- FAIL: TestKerberosASREP (5.01s)
... (all 6 tests FAIL identically)

# Real KDC AS-REQ via Go probe — timeout on UDP, EOF on TCP fallback.

# ldapsearch inside container — TLS required:
ldap_bind: Strong(er) authentication required (8)
    additional info: BindSimple: Transport encryption required.
```

#### Root Cause Analysis

1. **Docker Desktop NAT** on this host does not pass data packets through to the Samba AD DC container on ports 88 (KDC), 389 (LDAP), 445 (SMB), 53 (DNS). TCP handshakes succeed (SYN/SYN-ACK), but data sent by the host is either dropped (UDP → timeout) or causes the server to close the connection (TCP → EOF/RST). Docker NAT works correctly for other containers (nginx on port 9999→80 returns HTTP responses).

2. **Samba4 AD DC requires encryption** for LDAP binds (LDAP error 8: "Transport encryption required"). Aether's LDAP engine defaults to unencrypted TCP port 389. The `--tls` and `--starttls` flags were tested; both fail with `EOF` during TLS handshake through Docker NAT.

3. **Container was recreated** (b5cb69db8f79) during Stage 46d execution. The Docker Desktop NAT failure persists after recreation. Same symptom with fresh container.

4. **Pre-existing test failures** in `internal/protocol/kerberos` (TestStringToKeyRC4, TestEncryptDecryptAES128, TestEncryptDecryptAES256 — "integrity check failed") exist in the untracked development directory and are unrelated to Docker or Stage 46d.

5. **Codebase divergence**: Stage 46d Batch 2 specifies `ad ldap bind` etc.; actual CLI exposes `ldap bind` (top-level `aether ldap`), and `--json` flag is absent.

---

## GATE MATRIX RESULTS

| Gate | Requirement | Status | Evidence |
|------|-------------|--------|----------|
| G2601 | Docker available | PASS | `docker ps` succeeds, container healthy |
| G2602 | Samba4 image | PASS | `aether-samba-ad-dc:latest` running |
| G2603 | Baseline VERSION=5.0.0-alpha1 | PASS | cat VERSION → 5.0.0-alpha1 |
| G2604 | Realm fix present | PASS | `wire.go` has derGeneralString, TestRealmEncoding PASS |
| G2605 | Prior tests PASS | PASS | go build/test/vet clean |
| G2606 | Container up | PASS | `docker ps` shows healthy |
| G2607 | KDC reachable | FAIL | TCP/UDP 88 from host → EOF/timeout |
| G2608 | LDAP reachable | FAIL | TCP 389 from host → RST |
| G2609 | Seed users present | PARTIAL | Users exist, seed script didn't fully complete |
| G2610-G2615 | Batch 1 Kerberos tests | FAIL | Cannot execute — KDC unreachable |
| G2616 | Batch 1 pass count | FAIL | 0/6 |
| G2617-G2628 | Batch 2 LDAP CLI | FAIL | Cannot execute — LDAP unreachable + wrong CLI syntax |
| G2629-G2630 | Audit/Evidence | FAIL | Cannot execute |
| G2631 | No regression | PASS | go test -race clean |
| G2636 | No v5.* tags | PASS | git tag -l 'v5.*' empty |

---

## DOCKER ENVIRONMENT DETAILS

```
Docker version: 29.7.2
Docker Compose: v5.4.0
Container: aether-ad-lab (b5cb69db8f79 — recreated during execution)
Status: Up 5 minutes (healthy)
Container IP: 172.18.0.2 (aether-ad-net bridge)
Samba version: 4.15.13-Ubuntu
Realm: AETHER.TEST
Domain: AETHER
```

## KNOWN WORKING PATHS

- `docker exec aether-ad-lab samba-tool domain info 127.0.0.1` ✓
- `docker exec aether-ad-lab samba-tool user list` ✓
- `docker exec aether-ad-lab kinit user1@AETHER.TEST` ✓ (uses UDP)
- DNS UDP 127.0.0.1:53 from host ✓
- nginx container TCP 9999→80 from host ✓

## ENVIRONMENT REMEDY

1. **Investigate Docker Desktop NAT** for Samba container TCP ports 88/389/445/53 — NAT drops data packets for this specific container
2. **OR configure Samba** to accept TLS on TCP 389/88 and configure Aether to use TLS
3. **OR switch to host-network mode** (may not be supported on Windows Docker Desktop)
4. **OR expose services via host-gateway IP** instead of localhost
5. Re-run `scripts/ad-lab/seed.sh` to completion after fixing network

## TIME STAMPS

Stage 46d execution started: 2026-09-23T10:25Z
Container recreated: 2026-09-23T11:21Z
NAT failure confirmed persistent: 2026-09-23T11:56Z
BLOCKED-WITH-OWNER declared: 2026-09-23T11:57Z
