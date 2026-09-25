# Stage 46g — Root Cause Verdict

**Status:** `DOCKER_NAT` **REFUTED** · `SAMBA_CONFIG` **REFUTED** · **ROOT CAUSE = AETHER CODE DEFECTS**
**Date:** 2026-09-25 · **Version:** 5.0.0-alpha1 (unchanged) · **Tag:** NONE

---

## 1. The decisive test (WS-A1)

Stage 46f left one question open: is Samba loopback-bound (Aether fixable / config defect) or externally
bound (Docker NAT defect)? Stage 46g ran the decisive test.

```
docker exec aether-ad-lab netstat -tlnp
```

```
tcp  0  0  0.0.0.0:389   0.0.0.0:*  LISTEN  23/samba: task[ldap
tcp  0  0  0.0.0.0:88    0.0.0.0:*  LISTEN  31/samba: task[kdc]
tcp  0  0  0.0.0.0:445   0.0.0.0:*  LISTEN  14/smbd
tcp  0  0  0.0.0.0:636   0.0.0.0:*  LISTEN  23/samba: task[ldap
tcp  0  0  0.0.0.0:53    0.0.0.0:*  LISTEN  53/samba: task[dns]
tcp  0  0  0.0.0.0:464   0.0.0.0:*  LISTEN  31/samba: task[kdc]
```

**Output A.** Samba binds `0.0.0.0` on every relevant port. `smb.conf` confirms
`interfaces = lo eth0` / `bind interfaces only = No` — not loopback-only.

**Verdict: `SAMBA_CONFIG` is REFUTED. Part B1 is not the fix. No `smb.conf` change was made.**

## 2. The control test (WS-A4 / G2907)

A TCP handshake proves nothing. The control is whether *application data* flows.

```
docker exec aether-test-client ldapsearch -x -H ldap://172.18.0.2:389 -b '' -s base
```

```
dn:
supportedLDAPVersion: 2
supportedLDAPVersion: 3
namingContexts: DC=aether,DC=test
...
search: 2   result: 0 Success   numEntries: 1
```

Authenticated simple bind with the same credentials also returns `result: 0 Success` and a full
`DC=aether,DC=test` entry. Reference `kinit` (MIT, after pointing `/etc/krb5.conf` at the container)
returns `Authenticated to Kerberos v5`.

**Verdict: `DOCKER_NAT` is REFUTED.** Container-to-container application data flows on 389, 88 and
464. Part B2 is not reached. No operator remedy is required or proposed.

## 3. What actually caused the Stage 46f blocker

The Stage 46f symptom "TCP handshake OK, data dropped" was **real but mis-attributed**. The failure
was never in the network path. Two independent causes:

### 3a. CLI contract misuse (primary)

Stage 46f/46e invoked a CLI that does not exist. The commands in those stages
(`aether ad ldap bind AETHER.TEST <ip> --json`) were rejected before any socket was opened:

```
Error: unknown flag: --dc
Error: required flag(s) "workspace" not set
Error: required flag(s) "bind-dn", "bind-pass" not set
Error: bind credentials required: expected exactly 2 positional arguments <username> <password>, got 0
```

Aether's `ad ldap` surface is flag-based (`--host`/`--dc`, `--domain`, `--workspace`, `--bind-dn`,
`--bind-pass`) with credentials as **two positional arguments**, and has **no `--json` flag anywhere**.
Errors were read as transport failures. They were argument-validation failures.

### 3b. Six real code defects

Found only because the environment was finally made to work. See
`docs/stage46g-certification.md` §4. In short:

| # | Defect | Class | Severity |
|---|--------|-------|----------|
| D1 | AS-REQ pre-auth → `KDC error 24` while reference `kinit` with the same password succeeds | PROTOCOL | **Critical** |
| D2 | `asreproast` reports roasts for accounts that are **not** roastable; hash is the whole wire reply, etype hardcoded 23 | CODE | **Critical** |
| D3 | ccache parser rejects valid MIT ccache (`unexpected EOF`) → blocks `ccache show` and `kerberoast` | CODE | High |
| D4 | `rootdse` queries a non-empty base and rewrites `SCOPE_BASE`→`SUBTREE`, returning a ypservers object | CODE | High |
| D5 | `acl get` prints raw Go struct pointers (`&{ S-1-5-21-… }`) and `objectClass` = `top` | CODE | Medium |
| D6 | Read-only `ccache show` classified as mutating; forced `--workspace` | GOVERNANCE (fail-safe) | Low |

## 4. Environment defect (isolated, not Aether's)

Docker's embedded resolver does not forward `aether.test` to Samba:

```
getent hosts aether.test   -> rc=2 (NXDOMAIN)
dig +short @172.18.0.2 aether.test A -> 172.18.0.2 / 192.168.65.3 / 172.18.0.4
dig +short @172.18.0.2 SRV _ldap._tcp.dc._msdcs.aether.test -> 0 100 389 dc01.aether.test.
dig +short @172.18.0.2 aether.test SOA -> dc01.aether.test. hostmaster.aether.test. 4964 900 600 86400 3600
```

Samba's DNS is fully correct; the **client container** lacks `aether.test` in its resolver path.
This is a Docker container DNS configuration defect, class `ENVIRONMENT`. It is **not** an Aether
defect and it is **not** the Stage 46 blocker. Remedy: run the test client with `--dns 172.18.0.2`
or add a `network-alias`. All Stage 46g qualification was therefore run against the literal IP
`172.18.0.2`, and the host is not an input to any result below.

## 5. Environment-qualified matrix

| Protocol | Port | Binding | TCP | Reference client | Aether | Status |
|---|---:|---|---|---|---|---|
| DNS | 53/udp | `0.0.0.0` | — | `dig` SOA/SRV/A **OK** | n/a (no Aether DNS resolver in this path) | ENV-DEFECT (client resolver) |
| Kerberos | 88/tcp | `0.0.0.0` | OK | `kinit` **OK** | `ad enum users` **OK** / `ad tgt` **FAIL (D1)** | AETHER-DEFECT |
| LDAP | 389/tcp | `0.0.0.0` | OK | `ldapsearch` **OK** | `bind`/`rootdse`/`enum` **OK** (rootdse has D4) | MIXED |
| LDAPS | 636/tcp | `0.0.0.0` | — | not tested | not tested | NOT RUN |
| SMB | 445/tcp | `0.0.0.0` | — | not tested | not tested | NOT RUN |

## 6. Corrected attribution

```text
STAGE 46f SAID :  Docker Desktop NAT is the blocker. BLOCKED-WITH-OWNER.

STAGE 46g FOUND:  Docker NAT is fine. Samba is fine. The AD lab is fully functional and
                  was functional the whole time. The blocker was a mis-invoked CLI plus
                  six genuine Aether code defects that could not be seen while the
                  network was being blamed for them.

STAGE 46g VERDICT: BLOCKED-WITH-OWNER is WITHDRAWN as an infrastructure claim.
                    Stage 46 is BLOCKED-ON-CODE, not blocked-on-infrastructure.
                    No operator remedy is required. Stage 47 remains BLOCKED.
```
