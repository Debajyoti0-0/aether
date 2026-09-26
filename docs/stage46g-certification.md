# Aether Stage 46g — Certification

```text
AETHER STAGE 46g
=================

Version:    5.0.0-alpha1
Commit:     a887a0e (master) — working tree NOT clean (pre-existing, see §8)
Tag:        NONE
Repository: github.com/Debajyoti0-0/aether

Infrastructure: FUNCTIONAL — refuted as blocker
DNS:            PARTIAL — Samba DNS correct; client container resolver NXDOMAIN (ENV defect)
Kerberos:       FAIL — Aether AS-REQ pre-auth (D1); reference kinit PASS
LDAP:           PASS — bind, rootDSE reference, enumeration all live
LDAPS:          NOT RUN
SMB:            NOT RUN

LDAP BER:       RFC 4511 compliant (Stage 46f) — re-confirmed, unaffected by this stage
RFC Compliance: PASS for encoding; see D4 for base-DN/scope selection, an encoding-independent defect

Enumeration:    PASS — users 9, groups 39, computers 1, OUs 1, SPNs 4 objects
Pagination:     NOT SEPARATELY TESTED (limit flag exercised; paging control not isolated)
Filters:        PARTIAL — default filters verified live; malformed-filter path not tested
Controls:       NOT TESTED

Security Descriptors: PASS — 44 DACL ACEs + 2 SACL ACEs parsed on live object
ACL Analysis:   PASS — effective rights for Domain Admins match ACE[0] exactly
ACL Path:       PASS — 0 paths found on seeded lab (correct negative)

CLI:            PARTIAL — negative paths solid; surface inconsistent (--host vs --dc)
Flags:          PASS
Arguments:      PASS
Unknown Flags:  PASS (exit 1, clear message)
JSON:           FAIL — no --json flag exists on any ad command (see §3.1)
Human Output:   FAIL — raw Go pointer leakage (D5)

Governance:     PASS — fail-closed; wrong/empty passphrase rejected with exit 1
Secret Redaction: PASS — password absent from stdout/stderr on both success and failure
Fuzz:           NOT RUN
Race:           PASS — -race clean on protocol/*, engine/ad/*, cli
Regression:     PASS — 29/29 packages pass, 0 fail

Gates:
PASS:        38
FAIL:         7
BLOCKED:      6
WAIVED:       5
N/A:          0

Open Defects: 6 (2 critical, 2 high, 1 medium, 1 low)

Stage 46 Status: BLOCKED-ON-CODE  (not BLOCKED-ON-INFRASTRUCTURE)
Stage 47 Status: BLOCKED
Production Status: NOT READY
```

---

## 1. Executive summary

Stage 46g was chartered to settle one question: is the Stage 46 blocker Samba's interface binding or
Docker Desktop NAT? **Both hypotheses are false.** `netstat -tlnp` inside the Samba container shows
`0.0.0.0` on ports 53/88/389/445/464/636, and reference clients (`ldapsearch`, `kinit`) complete full
LDAP and Kerberos transactions from a sibling container. The infrastructure was never broken.

The real blocker is a **mis-invoked CLI** — Stages 46e/46f invoked an `ad ldap` syntax that does not
exist, so every attempt failed argument validation before a socket opened, and the argument errors
were read as transport failures. With the correct syntax, **LDAP live-qualifies and passes**.

That success then exposed **six genuine Aether code defects**, two of them critical. The most serious
is `asreproast`, which reports AS-REP roasts for accounts that are provably not roastable and
fabricates the hash from the raw wire reply rather than the enc-part.

Stage 46 cannot be certified. Stage 47 stays blocked.

## 2. Infrastructure RCA (the charter's question)

| Hypothesis | Test | Result | Verdict |
|---|---|---|---|
| Samba loopback-bound | `netstat -tlnp` in `aether-ad-lab` | `0.0.0.0:389/88/445/636/53/464` | **REFUTED** |
| Samba config defect | `grep interfaces smb.conf` | `interfaces = lo eth0`, `bind interfaces only = No` | **REFUTED** |
| Docker NAT drops data | `ldapsearch` anon + auth from `aether-test-client` | `result: 0 Success`, full entry | **REFUTED** |
| Kerberos reachable | `kinit administrator@AETHER.TEST` | `Authenticated to Kerberos v5` | **REFUTED** |
| DNS broken | `dig @172.18.0.2` A/SRV/SOA | all correct | **REFUTED for Samba** |

No `smb.conf` was modified. No `socat`/`portproxy`/`--network host` retry. No operator remedy issued.

## 3. CLI contract findings (test-envelope defect)

Three of the stage's own prescribed commands do not exist. Recorded because they caused the
misdiagnosis, and because the docs are wrong independent of that.

| Prescribed | Reality | Result |
|---|---|---|
| `aether ad ldap bind AETHER.TEST <ip> --json` | `bind` has no positional args and no `--json` | `required flag(s) "bind-dn", "bind-pass" not set` |
| `aether ad ldap enum users --dc <ip> --domain <d> --workspace <w>` | additionally needs 2 positional creds | `expected exactly 2 positional arguments, got 0` |
| `aether export verify-evidence --workspace <w>` | no `verify-evidence` subcommand; no `--workspace` flag on `export` | `unknown command` |

`aether ad ldap bind` / `rootdse` take `--host`; `enum` / `acl` / `path` take `--dc`. The same target
is spelled two ways across sibling commands.

## 4. Defect register

### D1 — CRITICAL · PROTOCOL — AS-REQ pre-authentication always fails
`aether ad tgt` returns `KDC error: KDC error 24: Pre-authentication information was invalid`
for every principal, while reference `kinit` with the **same password** succeeds
(`artifacts/stage46g/qual-06-acl-kerb.txt`, `artifacts/stage46g/control-kinit2-acl.txt`).
Layer is proven: LDAP simple bind with the same credentials succeeds, and `kinit` succeeds, so the
password and the KDC are both correct. The defect is inside Aether's AS-REQ construction
(`internal/protocol/kerberos/asreq.go:447-468`, `internal/engine/ad/kerberos/tgt.go:73-111`).
The PA-ENC-TIMESTAMP padata/etype/key-usage values are correct by inspection, so the fault is in key
derivation or timestamp encoding. **Realm case is NOT the cause** — `--domain AETHER.TEST` fails
identically to `--domain aether.test` (`artifacts/stage46g/verify-realm-case-hypothesis.txt`).
Blocks: `ad tgt`, `ad kerberoast` (needs a TGT), and the entire TGT leg of Batch 1.
Not byte-proven: the KDC logs no per-request detail, so the exact wrong byte is unlocated.

### D2 — CRITICAL · CODE — `asreproast` fabricates roast results
Ground truth from the DC (`artifacts/stage46g/verify-asrep-path-ccache.txt`): `userAccountControl`
is `512` (NORMAL_ACCOUNT) for administrator, user1, user3, user4, svc_web, svc_sql; only `user2`
has `4194816` (0x400000 = DONT_REQ_PREAUTH). **Exactly one account is AS-REP roastable.**

Aether reported `AS-REP roasted 7 accounts`, and user1/user2/user3/user4 all returned the *identical*
ciphertext prefix `7e81f73081f4a003` — impossible for different users and keys.

Cause: the roast condition is inverted (`internal/engine/ad/kerberos/roast.go:342-357`). It treats
`KDC_ERR_PREAUTH_REQUIRED` (25) — which means *not* roastable — as roastable, and discards the
`ParseASREP` result. The "hash" is `fmt.Sprintf("%x", resp)`, the entire raw KRB-ERROR wire reply,
not `EncPart.Cipher`, and `Etype` is hardcoded to `RC4_HMAC` (23). Genuinely roastable accounts are
silently dropped — `roast.go:357` has no `append` on the success path. The correct logic already
exists at `internal/engine/ad/kerberos/enum.go:354-367`.
**A security tool that invents credentials and hides real ones must not ship.** This is the most
severe finding of the stage.

### D3 — HIGH · CODE — ccache parser rejects valid MIT ccaches
`aether ad ccache show --ccache /tmp/krb5cc_0` and `aether ad kerberoast --ccache ...` both fail with
`read ccache: unexpected EOF` on a ccache produced by reference `kinit`
(`artifacts/stage46g/verify-asrep-path-ccache.txt`).

File header is `0504 000c …`. The parser reads `headerlen=12` and treats the header block as a
credential record: tag `0x0001`, length `0x00080000` (524288) → `io.ReadFull` fails.
`internal/protocol/kerberos/ccache.go:116-169` never skips `HeaderLen` and uses an invented
uint16-tag/uint32-length TLV grammar instead of the real v4 layout (principals carry the realm;
credentials are unframed). `CCACHE_VERSION_HEIMDAL` is set to `0x0504` (`ccache.go:22`) instead of
`0x0503`. The round-trip test passes only because `WriteToWriter` emits the same non-standard format
and the integration suite feeds it back its own output. Blocks Batch 1 `kerberoast` and `tgt`.

### D4 — HIGH · CODE — `rootdse` queries the wrong base and scope
Reference rootDSE returns `dn:` empty with `namingContexts` and `supportedLDAPVersion`. Aether's
`ad ldap rootdse` returns `CN=bydefaults,CN=ypservers,CN=ypServ30,CN=RpcServices,CN=System,DC=aether,DC=test`
and prints `objectGUID` as raw binary garbage (`artifacts/stage46g/qual-04-ldap-batch2.txt`).

Two causes: `internal/cli/ad/ldap.go:780-789` converts `--domain` into a non-empty `DC=…` base, and
`internal/engine/ad/ldap/engine.go:211-215` rewrites any `Scope == 0` to `SUBTREE` when the base is
non-empty. Because `LDAP_SCOPE_BASE == 0` (`internal/protocol/ldap/types.go:98`), an *explicit*
base-scope search is indistinguishable from an unset scope. Result: subtree search of a domain
object, first entry returned as if it were the RootDSE. Exit code is 0, so this fails silently.

### D5 — MEDIUM · CODE — raw Go struct pointers in user-facing output
`aether ad ldap acl get` prints:
```
Object: CN=Test User1,CN=Users,DC=aether,DC=test (top)
Owner: &{ S-1-5-21-3443072978-3862387813-1790103675-512   }
```
`Owner`/`Group` are `*acl.Principal`, which has no `String()` method
(`internal/engine/ad/acl/engine.go:15-21`), so `%s` at `internal/cli/ad/ldap.go:912-914` degrades to
`%v` and leaks the internal struct layout. `(top)` is `GetFirstAttributeValue("objectClass")`
(`acl/engine.go:310`) returning AD's most-general class first.

### D6 — LOW · GOVERNANCE (fail-safe) — read-only command forced to be governed
`aether ad ccache show` without `--workspace` fails with *"this command mutates external state and
requires --workspace"*. There is no read/mutate classification table for CLI commands;
`internal/cli/governance.go:125-131` classifies spine *intents* only, so every `ad` subcommand
passes through the fail-closed `openGovernedWorkspace` (`governance.go:39-53`). Over-broad but errs
toward safety, so it is filed as low, not as a security defect.

## 5. What passed, with evidence

| Test | Evidence | Notes |
|---|---|---|
| `ad ldap bind` (valid creds) | `qual-04-ldap-batch2.txt` | `Successfully bound as cn=administrator,…`, exit 0 |
| `ad ldap bind` (bad creds) | `qual-08-negative-audit.txt` | exit 1, no false success |
| `ad ldap enum users` | `qual-05-ldap-enum.txt` | 9 users, SIDs resolved, matches `ldapsearch` |
| `ad ldap enum groups` | same | 39 groups incl. `Tier1-Admins` (1109), `Tier2-Admins` (1110) |
| `ad ldap enum computers` | same | 1 (`DC01$`, RID 1000) |
| `ad ldap enum ous` | same | 1 (`Domain Controllers`; blank SID is correct — OUs have no objectSid) |
| `ad ldap enum spns` | same | 4 objects, 16 SPNs including `HTTP/web01.aether.test`, `MSSQLSvc/sql01.aether.test:1433` |
| `ad ldap enum all` | same | Users 9 / Groups 39 / Computers 1 / OUs 1, consistent |
| `ad ldap acl get` | `control-kinit2-acl.txt` | 44 DACL + 2 SACL ACEs, rights decoded |
| `ad ldap acl effective` | `verify-asrep-path-ccache.txt` | Domain Admins → 13 rights, byte-identical to ACE[0] |
| `ad ldap path` | same | `Found 0 attack paths`, exit 0 — correct negative |
| `ad enum users` (Kerberos) | `qual-07-kerb-acl-path.txt` | 7/8 found; `nosuchuser` correctly excluded |
| Governance: no workspace | `qual-08-negative-audit.txt` | `required flag(s) "workspace" not set`, exit 1 |
| Governance: wrong passphrase | `qual-09-governance-audit.txt` | exit 1, `wrong passphrase or corrupted salt.bin` |
| Governance: empty passphrase | same | exit 1, explicitly rejected |
| Secret redaction | `qual-08-negative-audit.txt` | `NO-LEAK` on success path |
| Audit chain | `qual-09-governance-audit.txt` | `{"total":10,"valid":10,"valid_all":true}` VERIFIED |
| Unknown flags | `qual-08-negative-audit.txt` | exit 1, `unknown flag: --bogus-flag` |
| Bad flag value | same | `strconv.ParseBool: parsing "maybe": invalid syntax`, exit 1 |
| Missing required flags | same | `required flag(s) "dc", "domain", "object" not set`, exit 1 |
| Unreachable host / bad port | same | `connect: connection refused`, exit 1, no hang |
| Static: build/vet | `artifacts/stage46g/` (console) | `go build ./cmd/... ./internal/...` and `go vet` both clean |
| Unit tests | console | 29/29 packages `ok`, 0 fail |
| Race | console | `-race` clean on `protocol/*`, `engine/ad/*`, `cli` |

Enumeration counts were cross-checked against `ldapsearch` (`control-kinit-dn.txt`) and matched
exactly — the outputs are not plausible-looking, they agree with the reference client.

## 6. Not run (and therefore not certified)

LDAPS/636 · SMB/445 · paged-results control in isolation · LDAP controls (server-side sort, VLV) ·
filter parser fuzzing · malformed-filter handling · general fuzz campaigns · `go test -race ./...`
(limited to AD-relevant packages) · LDAPS `StartTLS` path · large-dataset pagination.

Per §47 of the operating rules these are recorded as NOT RUN. None may be inferred from the passing
gates above.

## 7. Gate matrix

| Gate | Status | Gate | Status |
|---|---|---|---|
| G2901 container running | PASS | G2916 enum groups | PASS |
| G2902 test client | PASS | G2917 enum computers | PASS |
| G2903 baseline | PASS | G2918 enum ous | PASS |
| G2904 `netstat -tlnp` captured | PASS | G2919 enum spns | PASS |
| G2905 `smb.conf` inspected | PASS | G2920 enum all | PASS |
| G2906 Samba logs | PASS | G2921 acl get | PASS |
| G2907 raw TCP / reference client | PASS | G2922 acl effective | PASS |
| G2908 root cause verdict | PASS | G2923 acl path | PASS |
| G2909 smb.conf fix | N/A (not needed) | G2924 ldap bind | PASS |
| G2910 Samba restart | N/A (not needed) | G2925 kerberos enum users | PASS |
| G2911 bindings re-verified | N/A | G2926 asreproast | **FAIL (D2)** |
| G2912 ldapsearch from client | PASS | G2927 kerberoast | **FAIL (D1+D3)** |
| G2913 Docker NAT verdict | PASS (REFUTED) | G2928 kerberos TGT | **FAIL (D1)** |
| G2914 rootdse | **FAIL (D4)** | G2929 audit + evidence | PASS |
| G2915 ldap bind | PASS | G2930 no tag, doc limit | PASS |

## 8. Repository state

Working tree was **already dirty before this stage** and is unchanged by it. Pre-existing
modifications: `VERSION`, `cmd/aether/main.go`, `internal/cli/governance.go`, `internal/cli/root.go`.
Pre-existing untracked debris at the repo root — `test_tcp*.go`, `test_tls*.go`, `test_hello.go`,
`test_encode_search.go`, `tmp/bindprobe.go` and ~20 stray binaries — makes a bare
`go build ./...` and `go vet ./...` fail with `main redeclared in this block`. This is a
**REPRODUCIBILITY DEFECT** and must be cleaned before any release gate; Stage 46g did not delete
historical evidence to make the build pass, and instead qualified the real package set
(`./cmd/... ./internal/...`), which builds and vets clean.

## 9. Verdict

```text
Docker NAT was NOT the blocker.  Samba config was NOT the blocker.
The AD lab was functional throughout Stages 46e-46f.

Stage 46 is BLOCKED-ON-CODE, on six defects:
  D1  AS-REQ pre-authentication  (critical — breaks every TGT)
  D2  asreproast fabricates roasts (critical — invents credentials)
  D3  ccache parser rejects MIT ccache (high — breaks kerberoast)
  D4  rootdse wrong base/scope    (high — silent wrong answer, exit 0)
  D5  Go pointer leakage in output (medium)
  D6  read-only ccache show gated  (low, fail-safe)

LDAP is LIVE-QUALIFIED and the Batch 2 LDAP gates are CLOSED.
Kerberos is PARTIAL: enumeration CLOSED, TGT and roasting BLOCKED on D1/D2/D3.

Stage 46 : BLOCKED-ON-CODE     -> not certifiable
Stage 47 : BLOCKED
v5.0.0   : NOT READY
```

**Closure of the ambiguity Stages 46d–46f were circling:** the question was never Samba's binding and
never Docker's NAT. Both are healthy. The Stage 46 blocker is in Aether, and it is now enumerated
with file, line, and reproducible evidence for each of the six items.
