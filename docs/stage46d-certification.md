# Stage 46d — AETHER 5.0.0 Live Qualification Certification

**Status**: BLOCKED-WITH-OWNER
**Reason**: Docker Desktop NAT failure for Samba AD DC TCP services (ports 88/389/445/53) — confirmed persistent after container recreation
**Remedy**: Fix Docker Desktop NAT for Samba container, OR configure Samba for TLS on all ports, OR use host-network mode, OR provide a host-reachable KDC endpoint
**Date**: 2026-09-23T11:57Z (UTC+5:30)
**Container**: b5cb69db8f79 (recreated 11:21Z, healthy, NAT still failing)
**Version**: 5.0.0-alpha1
**Tag**: none (required by stage rules)
**Working tree**: modified (VERSION, cmd/aether/main.go, internal/cli/governance.go, internal/cli/root.go) + untracked

---

## DOCKER STATUS

| Component | Status |
|-----------|--------|
| Docker daemon | RUNNING |
| Samba4 container | UP (healthy) |
| Container IP | 172.18.0.2 |
| Network | aether-ad-net (bridge) |
| Realm | AETHER.TEST |
| Domain | AETHER |
| Samba version | 4.15.13-Ubuntu |

## KDC (port 88) — TCP AND UDP (fresh container b5cb69db8f79)

| Path | Result |
|------|--------|
| Host localhost:88 TCP | FAIL — EOF after connect |
| Host localhost:88 UDP | FAIL — i/o timeout |
| Container internal 127.0.0.1:88 | FAIL — Connection reset by peer |
| Container internal 172.18.0.2:88 | FAIL — Connection reset by peer |
| Sidecar container → :88 | FAIL — Connection reset by peer |
| Fresh Batch 1 (6 tests) | FAIL 0/6 — TCP read length: EOF |

## LDAP (port 389) — TCP (fresh container)

| Path | Result |
|------|--------|
| Host localhost:389 TCP | FAIL — RST / EOF |
| Host localhost:389 StartTLS | FAIL — TLS dial EOF |
| Host localhost:389 TLS | FAIL — bind: send bind request: EOF |
| Container internal 127.0.0.1:389 | FAIL — RST |
| ldapsearch (internal) | FAIL — "Transport encryption required" |

## WORKING PATHS

- DNS UDP 53 from host: PASS
- Nginx TCP 80 via NAT: PASS
- Container internal `samba-tool domain info`: PASS
- Container internal `samba-tool user list`: PASS
- Container internal `kinit` (UDP): PASS (prompts for password)

## BATCH 1 (KERBEROS) — NOT EXECUTED

Fresh Batch 1 transcript confirms all 6 tests FAIL with `TCP read length: EOF`:
- TestKerberosASREP: FAIL (KDC EOF, 5.01s)
- TestKerberosTGSREP: FAIL (KDC EOF)
- TestKerberoast: FAIL (KDC EOF)
- TestASREPRoast: FAIL (KDC EOF)
- TestCcacheRoundTrip: FAIL (KDC EOF)
- TestEngagementBoundary: FAIL (KDC EOF)

## BATCH 2 (LDAP + ACL) — NOT EXECUTED

- LDAP port 389 unreachable from host (TCP EOF/RST)
- CLI syntax divergence: commands are `aether ldap bind` not `ad ldap bind`
- `--json` flag does not exist in current codebase

## DOCKER ENVIRONMENT

- Container: b5cb69db8f79 (recreated during execution, healthy)
- Docker: 29.7.2, Compose: v5.4.0
- KDC/LDAP/SMB/DNS ports: NAT fails for Samba container specifically
- Nginx container: NAT works normally (control)

## STAGE 45 CERTIFICATION

Status: UNCHANGED (was PARTIALLY CLOSED in Stage 46r, wire encoding fix verified)

## STAGE 46 CERTIFICATION

Status: PARTIALLY CLOSED (D45b-004 FIXED, live qualification BLOCKED)

## STAGE 47 ENTRY GATE (TB47-G00)

Status: STILL-BLOCKED — Stage 46d not COMPLETE

## DOCUMENTS PRODUCED (≤ 2)

1. `docs/stage46d-certification.md` (this document)
2. `artifacts/stage46d/EXECUTION-REPORT.md` (detailed evidence)

---

## STAGE 46 OVERALL DISPOSITION

```
Stage 46: PARTIALLY CLOSED
  - Realm GeneralString fix (D45b-004): IMPLEMENTED, VERIFIED, EXECUTED, PASSED
  - Wire encoding (AS-REQ, TGS-REQ, AP-REQ, Authenticator): IMPLEMENTED, VERIFIED
  - Unit tests, race tests, vet, build: PASSED
  - LIVE QUALIFICATION (Kerberos + LDAP + ACL against real AD): BLOCKED
  - Reason: Docker Desktop NAT does not pass data to Samba AD DC container
  - Stage 46d: BLOCKED-WITH-OWNER

Stage 46d: BLOCKED-WITH-OWNER
```

## STAGE 47 ENTRY

```
Stage 47: BLOCKED — awaiting Stage 46d COMPLETION
```

---

**Signed**: Stage 46d QA Lead
**Disposition**: BLOCKED-WITH-OWNER — requires Docker infrastructure remediation before live qualification can proceed
