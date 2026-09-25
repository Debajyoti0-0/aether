# Aether Stage 46h — Certification

```text
AETHER STAGE 46h
==================

Version:    5.0.0-alpha1
Commit:     a887a0e (master) — working tree modified (all changes staged for review)
Tag:        NONE (deliberately not created; see §8)
Repository: github.com/Debajyoti0-0/aether

Lab:        aether-ad-lab (Samba 4.15.13 AD DC) at 172.18.0.2
            aether-test-client (Alpine) at 172.18.0.3
            Realm AETHER.TEST

Infrastructure: FUNCTIONAL — the Stage 46g "ENV defect" was a client resolver
                misconfiguration and is fixed and pinned (D6)
Kerberos:       PASS — AS-REQ pre-auth, ccache v4 read+write verified against
                MIT kinit/klist in both directions (D1, D2, D3)
LDAP:           PASS — bind, RootDSE from an empty base DN, enumeration
LDAPS:          NOT RUN
SMB:            NOT RUN

RFC Compliance: PASS — Kerberos AS-REQ / KDC-ERROR / AS-REP (RFC 4120/4121),
                ccache v4 (MIT format), LDAP (RFC 4511, re-confirmed)

Regression:     PASS — 31/31 packages ok, 0 fail
Fuzz:           PASS — ccache parser, 17,127,579 executions in a recorded 60s
                run, 0 crashes (evidence: h6-fuzz-evidence.json)
Race:           PASS (inherited from 46g; no concurrency changes in 46h)

Gates:
PASS:        31 packages
FAIL:         0
BLOCKED:      0
WAIVED:       2 (documented, §7)
N/A:          0

Defects: 6 of 6 CLOSED (D1-D6) + 1 found and closed in 46h (D7)

Stage 46 Status: CODE COMPLETE — no open code defects
Stage 47 Status: UNBLOCKED (not started; deliberately out of scope)
Production Status: NOT READY — see §7
```

---

## 1. Executive summary

Stage 46g certified six open defects (D1–D6) spanning Kerberos protocol
handling, LDAP scope selection, repository hygiene and lab DNS. **All six are
closed and verified live.** A seventh defect (D7) was found during final
verification and closed in the same stage.

The stage's central result is that the two critical Kerberos defects (D1, D2)
were genuinely Aether code faults, and that the AS-REP roasting defect (D2)
was more serious than "reports wrong counts" — it **fabricated a hash from the
raw KDC error reply** and presented it as a crackable credential. That is an
integrity failure, not a cosmetic one, and it is now fixed and cross-checked
against MIT's own client.

The previously blocked native MIT interop test was unblocked this stage. The
Samba DC image ships `kinit` and `klist`, so ccache interoperability could be
tested against a real MIT implementation without WSL. Both directions now pass:
Aether reads a ccache written by MIT `kinit`, and MIT `klist` reads a ccache
written by Aether, with every field agreeing.

Two Stage 46g conclusions were **refuted or corrected** by live evidence:

| Stage 46g claim | Stage 46h evidence | Verdict |
|---|---|---|
| Client container resolver NXDOMAIN is an "ENV defect" in the infrastructure | The client inherited Docker's embedded resolver, which forwards to the *host* resolver; pointing it at the Samba DNS fixes every AD zone and SRV lookup | Infrastructure was never broken — misconfiguration, now fixed and pinned |
| `user2` is AS-REP roastable (`DONT_REQ_PREAUTH` set) | MIT `kinit -n` demands a password for `user2` despite `userAccountControl=0x400200`; this Samba ignores the flag entirely | Lab limitation, not an Aether defect |
| Kerberos interop untestable (WSL `krb5-user` install blocked) | The DC image ships `kinit`/`klist` | Interop now fully tested |

## 2. Defect register — all closed

| ID | Severity | Component | Root cause | Status |
|---|---|---|---|---|
| D1 | CRITICAL | `internal/protocol/kerberos/asreq.go`, `etypes.go` | PA-DATA OCTET STRING + DER sequence mis-parsed; salt hardcoded instead of read from KDC `ETYPE-INFO2`; KDC options written little-endian; `rtime[6]` omitted on renewable requests | **CLOSED** |
| D2 | CRITICAL | `internal/engine/ad/kerberos/roast.go` | KDC error replies were hashed as if they were AS-REP enc-parts, fabricating crackable hashes | **CLOSED** |
| D3 | HIGH | `internal/protocol/kerberos/ccache.go` | ccache v4 reader/writer did not follow the documented MIT grammar (principal layout, keyblock framing) | **CLOSED** |
| D4 | MEDIUM | `internal/engine/ad/ldap/engine.go` | CLI derived a `DC=` base DN from `--domain`; explicit base scope not preserved | **CLOSED** |
| D5 | LOW | repository | Scratch files and build debris at repo root | **CLOSED** |
| D6 | MEDIUM | `scripts/ad-lab/` | Docker subnet not pinned, client resolver not pointed at Samba DNS | **CLOSED** |
| D7 | LOW | `internal/cli/ad/tgt.go` | `ccache show` appended the realm twice, printing `user1@AETHER.TEST@AETHER.TEST` | **CLOSED** |

### D1 — AS-REQ pre-authentication (CRITICAL, closed)

Four independent faults, all required for a TGT to be issued:

1. **PA-DATA framing.** The padata `padata-value` is an OCTET STRING wrapping a
   DER SEQUENCE; the parser read it as a bare sequence, so the pre-auth request
   was malformed and the derived key never matched.
2. **Salt source.** The salt was hardcoded rather than taken from the KDC's
   `ETYPE-INFO2` entry. Verified live: etype `18` (`aes256-cts-hmac-sha1-96`),
   salt `AETHER.TESTAdministrator`, s2kparams `00 00 10 00` (4096 iterations).
3. **KDC options byte order.** Written little-endian; the wire format is
   big-endian.
4. **Renewable request.** `rtime[6]` was not sent when renewable lifetime was
   requested, so the KDC did not set `RENEWABLE`.

**Live verification** (`artifacts/stage46h/h1-d1-d2-d4-live.txt`):

```text
user1          OK   Valid: 2026-09-25T18:33:57Z to 2026-09-25T18:38:57Z
Administrator  OK   Valid: 2026-09-25T18:33:57Z to 2026-09-25T18:38:57Z
administrator  OK   Valid: 2026-09-25T18:33:57Z to 2026-09-25T18:38:57Z
UsEr1          OK   Valid: 2026-09-25T18:33:58Z to 2026-09-25T18:38:58Z
```

Mixed-case principals all succeed, confirming case-insensitive handling.
Negative paths remain correct:

```text
wrong password  -> KDC error 24: Pre-authentication information was invalid
bad principal   -> KDC error 6:  Client not found in Kerberos database
```

A golden `kdcErrPreAuthRequiredLive` vector is byte-exact against a captured
`_scratch/kdc-preauth.bin`. `--output` now honours the requested path via
`TGTInput.OutputPath`.

**Realm case was refuted**, not assumed: `--domain AETHER.TEST` and
`--domain aether.test` fail identically without the fix, so realm case is
irrelevant; the KDC-supplied salt and case-insensitive principal handling are
what matter.

### D2 — AS-REP roasting fabricates hashes (CRITICAL, closed)

The old code hashed the KDC's raw reply regardless of whether the reply was an
AS-REP or a KDC-ERROR. A KDC error is not a credential, so Aether printed
`$krb5asrep$...` hashes for accounts that were provably not roastable. The fix
hashes **only** the encrypted part of a successful AS-REP, and reports every
skipped account with the reason it was skipped.

**Live verification** — all three lab accounts correctly reported as not
roastable, with reasons:

```text
AS-REP roasted 0 of 3 accounts (mode 18200/13100)
Not roastable (3):
  user1: KDC error 25: Additional pre-authentication required
  Administrator: KDC error 25: Additional pre-authentication required
  user2: KDC error 25: Additional pre-authentication required
```

**Cross-checked against MIT's own client.** `kinit -n` is the canonical AS-REP
roasting request — it offers no pre-authentication and asks for a TGT. Results
must agree per account (`artifacts/stage46h/h2-d2-mit-crosscheck.txt`):

| Account | `userAccountControl` | MIT `kinit -n` | Aether `asreproast` | Agree |
|---|---|---|---|---|
| `Administrator` | `0x200` | password demanded | KDC error 25 | yes |
| `user1` | `0x200` | password demanded | KDC error 25 | yes |
| `user2` | `0x400200` | password demanded | KDC error 25 | yes |
| `nosuchuser` | — | not in KDB | (not enumerated) | yes |

`user2` carries `DONT_REQ_PREAUTH` (`0x400000`) and is *still* prompted for a
password by MIT's own client. **This Samba build ignores the flag**, so no
account in this lab is roastable and the positive roasting path is not
exercisable here. Aether's 0-of-3 is therefore **correct**, and D2's fix cannot
be positively demonstrated on this lab — only negatively, and negatively it is
proven. This is a lab limitation, recorded in §7, not an open Aether defect.

### D3 — ccache v4 reader/writer (HIGH, closed)

Rewritten against the documented MIT v4 grammar. Four real defects, each of
which desynchronised the byte stream:

1. **Principal layout.** v4 is `name_type`, `count`, then counted data fields
   where **the realm is the first data field**, not the last component. The old
   code read a bare component count and treated the final string as the realm,
   desynchronising by 4 bytes per principal.
2. **Keyblock framing.** The keyblock is `uint16` enctype followed by a
   **counted** key (`uint32` length). Reading a second `uint16` stole two bytes
   of the following `authtime`.
3. **No credential count, no config section.** Records run to EOF.
4. **Config entries.** `X-CACHECONF:` records are ordinary credentials in the
   stream and must be skipped by `GetDefaultEntry()`.

**Native MIT interop, both directions** — this was the evidence blocked by the
WSL failure, and it is now obtained from the DC image's own `kinit`/`klist`.

*Direction 1 — MIT writes, Aether reads*
(`artifacts/stage46h/h3-d3-mit-interop.txt`). MIT `kinit` produced a 1591-byte
ccache; MIT's own `klist` and Aether's reader agree field for field:

```text
MIT    klist:  Default principal: Administrator@AETHER.TEST
                 valid 09/25/26 18:23:30 -> 09/26/26 04:23:30
                 Etype (skey, tkt): aes256-cts-hmac-sha1-96, aes256-cts-hmac-sha1-96
                 Flags: FRIA
Aether ccache:  [0] Administrator@AETHER.TEST -> krbtgt/AETHER.TEST@AETHER.TEST
                 (etype=18, flags=0x40e00000, valid=2026-09-25T18:23:30Z to 2026-09-26T04:23:30Z)
                 [1] ... -> krb5_ccache_conf_data/pa_type/krbtgt/...@X-CACHECONF:
                     (etype=0, flags=0x0)   <- config record, correctly isolated
```

*Direction 2 — Aether writes, MIT reads*
(`artifacts/stage46h/h4-d3-aether-written-mit-read.txt`). Aether wrote a
1189-byte ccache; MIT `klist` reads it with no complaint and reports values
identical in shape and content to MIT's own `kinit` output:

```text
MIT klist -e:  Default principal: user1@AETHER.TEST
               valid 09/25/26 18:35:04 -> 09/25/26 18:40:04, renew until 10/02/26
               Etype (skey, tkt): aes256-cts-hmac-sha1-96, aes256-cts-hmac-sha1-96
MIT klist -f:  Flags: FRIA
```

Supporting regression coverage in `internal/protocol/kerberos/stage46h_regression_test.go`:
`TestCcacheParseMIT`, `TestCcacheRoundTripMIT`, `TestCcacheWriterIsByteExact`
plus malformed-input cases. The MIT fixture in
`internal/protocol/kerberos/testdata/mit-kinit-user1.ccache` is genuine MIT
`kinit` output, including a real `X-CACHECONF:` record. Parser fuzzing was
re-executed and recorded: **17,127,579 executions in a 60s budget (64.6s wall
clock, 16 workers), 0 crashes, 0 panics, 0 unique failures**, growing the
interesting-input corpus from 77 to 99. Full provenance — command, timestamps,
Go version, commit SHA, platform — is in
`artifacts/stage46h/h6-fuzz-evidence.json`. Aether-written ccaches were
additionally decoded by an independent Python parser.

### D4 — LDAP base DN and scope (MEDIUM, closed)

`SearchOptions.ScopeSet` and `ResolveScope` were added so an explicitly supplied
base scope survives resolution. RootDSE now queries with an **empty DN**,
requests operational attributes, validates the response, and derives no `DC=`
base. The CLI no longer manufactures a base DN from `--domain`.

**Live verification** — RootDSE returns correct naming contexts with no
synthesised base:

```text
defaultNamingContext: DC=aether,DC=test
rootDomainNamingContext: DC=aether,DC=test
configurationNamingContext: CN=Configuration,DC=aether,DC=test
schemaNamingContext: CN=Schema,CN=Configuration,DC=aether,DC=test
dnsHostName: dc01.aether.test
supportedLDAPVersion: 2, 3
```

### D5 — repository hygiene (LOW, closed)

All scratch files were moved under `_scratch/` (the Go tool ignores
underscore-prefixed directories, so `go build ./...` is unaffected). The repo
root held **73.4 MB + 140.2 MB of debug debris** from this stage's
cross-compiles and probes; all of it was moved to `_scratch/root-strays/`
rather than deleted, preserving the debugging history. `.gitignore` was
hardened so the same debris cannot return, including `*.keytab` (credential
material must never be committed regardless of origin) and
`/test_integration_ad_linux`.

`cmd/asn1probe` was **kept**: it is a deliberate, documented verification tool
(`docs/rfc-verification.md`), and `internal/protocol/kerberos/testhook.go` exists
specifically to serve it. Two unreferenced probe commands (`cmd/ldapdebug`,
`cmd/ldaptest`) were quarantined.

### D6 — lab networking and DNS (MEDIUM, closed)

Two distinct problems, both now fixed **in the repository** so they cannot
regress on rebuild.

*Pinned addressing.* The docker network is pinned to `172.18.0.0/16` with
static DC (`172.18.0.2`) and client (`172.18.0.3`) addresses. Automatic pool
assignment could otherwise hand out a different address after a rebuild,
silently breaking every documented command.

*Client resolver.* The client was querying Docker's embedded resolver
(`127.0.0.11`), which forwards everything except container names to the **host**
resolver — so `aether.test` and every AD SRV record returned NXDOMAIN. It is
now pointed at the Samba DNS.

*Stale A records (new this stage).* The `samba_data` volume outlives networking
changes, and Samba's DNS update task re-registers the container's current
addresses **without purging the old ones**. The zone had accumulated two dead
addresses. `samba-tool dns query` showed the provenance unambiguously — the
correct record is `serial=1`, the stale ones `serial=2927` and `serial=2949`:

```text
Name=, Records=4:
  A: 172.18.0.2     (serial=1)    <- correct, pinned
  A: 192.168.65.3   (serial=2927) <- stale, earlier lab run
  AAAA: fdc4:...::3 (serial=2928) <- the DC's real current Docker IPv6, kept
  A: 172.18.0.4     (serial=2949) <- stale, earlier lab run
```

`scripts/ad-lab/seed.sh` now removes any A record that is not the pinned DC
address, and adds the **per-DC SRV records** that provisioning omitted
(`_kerberos._tcp.dc01`, `_kerberos._udp.dc01`, `_ldap._tcp.dc01` — Windows-style
clients query those directly and previously could not discover the KDC at all).
The block is idempotent and was validated: syntax clean, `awk` parse correct,
0 removals needed, all three SRVs detected as already present.

**Verified end state:**

```text
_kerberos._tcp.aether.test        0 100 88  dc01.aether.test.
_kerberos._tcp.dc01.aether.test   0 100 88  dc01.aether.test.
_kerberos._udp.dc01.aether.test   0 100 88  dc01.aether.test.
_ldap._tcp.dc01.aether.test       0 100 389 dc01.aether.test.
_ldap._tcp.aether.test            0 100 389 dc01.aether.test.
aether.test / dc01.aether.test / dc01  ->  172.18.0.2  (single address)
SOA: dc01.aether.test. serial 5150
```

*Bind-mount defect (new this stage).* The compose file mounted the repo with a
WSL-style host path, `/mnt/c/Users/.../aether:/aether:ro`. That path form only
exists when Docker runs **inside WSL**; on Docker Desktop for Windows it does
not exist, so the mount silently resolved to an empty directory and `/aether`
was unusable. Replaced with a relative path, `- ../..:/aether:ro`, which Compose
resolves against the compose file's own directory and which is portable across
Docker Desktop, WSL and native Linux. Verified: the repository is now visible at
`/aether` in the client, and the DC was left untouched during the recreate
(`--no-deps`).

### D7 — `ccache show` realm duplication (LOW, closed, found in 46h)

`internal/cli/ad/tgt.go` printed `%s@%s` with the realm, while the value passed
in was already produced by `PrincipalName.FullName`, which already appends
`@realm`. Output read `user1@AETHER.TEST@AETHER.TEST`. The file itself was
always correct — MIT `klist` read it fine — so this was display-only.

The redundant append is removed. The remaining slash-separated rendering is
exactly what MIT's own `klist` prints for the same credential:

```text
before:  [0] user1@AETHER.TEST@AETHER.TEST -> krbtgt/AETHER.TEST@AETHER.TEST
after:   [0] user1@AETHER.TEST -> krbtgt/AETHER.TEST
```

## 3. Additional Kerberos fixes

Two defects were found while verifying D1 that were not in the 46g register:

- **Ticket flags were truncated.** `ticketFlagsSafe` was dropping flags,
  producing a value that disagreed with MIT. It now preserves all flags, giving
  the MIT-compatible `0x40e00000` that `klist` renders as `FRIA`.
- **`Ticket.Raw` held the wrong bytes.** It was populated with the encrypted
  enc-part rather than the DER `KerberosTicket`. Live tickets now carry the
  proper APPLICATION 1 tag, which is what the ccache interop in §D3 depends on.

## 4. Verification matrix

| Check | Method | Result |
|---|---|---|
| AS-REQ pre-auth | 4 principals, mixed case, live KDC | PASS |
| AS-REQ negative | wrong password, bad principal | PASS (KDC 24, KDC 6) |
| Golden KDC-ERROR vector | byte-compare vs captured reply | PASS |
| AS-REP roast negative | 3 accounts, live KDC | PASS (0 hashes, reasons given) |
| AS-REP roast positive | MIT `kinit -n` cross-check | **NOT EXERCISABLE** — lab KDC ignores `DONT_REQ_PREAUTH` (§7) |
| ccache read | MIT `kinit` file, both entries | PASS |
| ccache write | MIT `klist` on Aether file | PASS |
| ccache byte-exactness | round-trip + golden fixture | PASS |
| ccache fuzz | recorded 60s re-run, full provenance | PASS (17,127,579 execs, 0 crashes) |
| RootDSE | empty base DN, live LDAP | PASS |
| DNS zone | `dig` A/SRV/SOA, authoritative | PASS |
| Lab reproducibility | `seed.sh` idempotence, `bash -n` | PASS |
| `go build ./...` | clean tree | PASS |
| `go vet ./...` | clean tree | PASS |
| `go test -count=1 ./...` | full suite | PASS (31 ok, 0 fail) |

## 5. Evidence index

All under `artifacts/stage46h/`:

| File | Contents |
|---|---|
| `h1-d1-d2-d4-live.txt` | D1 TGT acquisition (4 principals) + both negatives, D4 RootDSE, D2 roast |
| `h2-d2-mit-crosscheck.txt` | D2 per-account cross-check vs MIT `kinit -n`, with `userAccountControl` |
| `h3-d3-mit-interop.txt` | Direction 1: MIT `kinit` writes, Aether reads |
| `h4-d3-aether-written-mit-read.txt` | Direction 2: Aether writes, MIT `klist` reads |
| `h5-gates.txt` | `go build`, `go vet`, `go test -count=1` with versions and counts |
| `h6-fuzz-evidence.json` | `FuzzCCacheParse` re-run: 17,127,579 execs, 0 crashes, full provenance |
| `d1-live-verify.txt`, `d1-output-flag-verify.txt`, `d1-salt-hypothesis.txt` | Earlier D1 working evidence |

Supporting scripts and captures are under `_scratch/` (git-ignored).

## 6. Test coverage added

- `internal/protocol/kerberos/stage46h_regression_test.go` — ccache v4 parse,
  round-trip, byte-exact writer, malformed-input rejection, MIT fixture
  (with a real `X-CACHECONF:` record), fuzz entry point.
- `internal/engine/ad/kerberos/stage46h_flags_test.go` — ticket-flag
  preservation, asserting the `0x40e00000` value MIT writes.

## 7. Known limitations (waived, not defects)

1. **AS-REP roasting cannot be positively demonstrated on this lab.** This
   Samba build ignores `DONT_REQ_PREAUTH`; MIT's own `kinit -n` is prompted for
   a password for every account including `user2`, which has the flag set. A
   lab with a KDC that honours the flag is required to test the success path.
   Aether's behaviour is proven correct by agreement with the reference client.
2. **LDAPS, SMB and AD CS were not exercised** this stage. They remain untested
   surface, not fixed defects.

Two environment notes recorded for the next operator:

- **`seed.sh` had an invalid `userAccountControl`.** It set `4194304`
  (`0x400000` alone), omitting `NORMAL_ACCOUNT` (`0x200`). The correct value is
  `4194816` (`0x400200`); this was corrected. It makes no behavioural difference
  on this Samba (see limitation 1) but is now correct.
- **WSL is not usable for Kerberos testing** on this host — no `/bin/bash`, and
  `krb5-user` installation previously hung holding the dpkg lock. Use the DC
  container's `kinit`/`klist` instead; it is the same MIT implementation.

### 7.1 This limitation is not limited to AS-REP roasting

The scope of limitation 1 is broader than D2 and must not be read narrowly. It
is a general property of the lab, and it bounds what any future stage can claim:

| Capability | Behaviour on Windows AD | Behaviour on this Samba4 lab | Why |
|---|---|---|---|
| AS-REP roasting eligibility (D2) | `DONT_REQ_PREAUTH` is honoured; AS-REP is returned and can be roasted | Flag is **ignored**; MIT `kinit -n` is prompted for a password for every account, including `user2` which has the flag set | Samba ignores the `userAccountControl` bit |
| AD CS / ESC1–15 detection | Fully supported; the real attack surface | **Container only.** `CN=Public Key Services` exists but holds zero templates and zero CA objects, the `pKICertificateTemplate` schema class is absent, and `samba-tool` has no CA subcommands | Samba provisions the AD CS container but implements none of the service behind it |
| ESC1/ESC6/ESC15 abuse | Issues certificates with attacker-supplied SANs | **Impossible on this lab.** There is nothing to enroll against | no CA |
| PKINIT | Works with a CA-issued certificate the KDC trusts | **Not exercisable.** PKINIT requires the certificate to chain to a trust anchor the KDC accepts, and this KDC has no CA behind it | no CA, no NTAuth store |

The practical consequence, stated plainly so it is not rediscovered as a
surprise mid-stage:

> **This Samba4 lab can live-qualify the Kerberos and LDAP protocol work of
> Batches 1 and 2. It cannot live-qualify any part of Batch 3 (AD CS + PKINIT).**
> AD CS and PKINIT work is therefore protocol-correctness-verifiable at best
> unless a real CA is provisioned. Any claim of ESC detection or ESC abuse
> being "live-qualified" against this lab would be false.

This was confirmed directly during the Stage 47 entry gate, not inferred. The
probe is preserved at `artifacts/stage47/baseline/adcs-surface-probe.txt` and the
result is more specific than "Samba has no AD CS":

```text
CN=Services,CN=Configuration,DC=aether,DC=test children:
  CN=Public Key Services   <- the AD CS container EXISTS
  CN=Windows NT, CN=NetServices, CN=RRAS, CN=MsmqServices

CN=Public Key Services,CN=Services,CN=Configuration,DC=aether,DC=test -> result: 0 Success
  ...but it contains zero pKICertificateTemplate objects
  ...and zero certificationAuthority objects
  ...and the pKICertificateTemplate classSchema is not in the schema at all
```

So Samba provisions the AD CS *container* but provides no CA, no templates, no
issuance policy, and not even the schema class needed to hold them. The
practical split this implies, which matters for how future stages are scoped:

- **LDAP template/CA enumeration can be live-exercised here.** The container and
  base DN resolve and the query is valid, so enumeration code can be proven to
  build the correct base, scope and filter, and to return an empty set
  correctly. It cannot be proven to return a *populated* set, because there is
  nothing to return.
- **ESC detection cannot be validated against real directory data here.** A
  detector that returns zero findings is indistinguishable from a correct
  detector on an unpopulated tree unless tested against synthetic template
  objects.
- **Enrollment and abuse are impossible**, and PKINIT remains unexercisable,
  because there is no CA and no trust anchor.

A consequence for the *fidelity* of any mock harness used instead: a mock
MS-WCCE responder can prove that Aether's NDR marshalling, request construction,
response parsing and disposition handling are correct, and a synthetic LDAP
fixture can prove that detection logic keys off the right attributes. Neither
proves interoperability with a real CA, because a mock shares no code, no
authentication path and no certificate-issuance policy engine with Windows AD
CS, and a synthetic fixture shares no schema with a real one. Those claims must
be labelled as protocol-correctness only.

## 8. Deliberate non-actions

- **No `v5.*` tag was created.** Tagging is a release decision and is out of
  scope for a defect-certification stage.
- **Stage 47 was not started.** Unblocking it is recorded, not acted upon.
- **No scratch or debug artifact was deleted.** Everything was moved under
  `_scratch/` to satisfy D5 without destroying evidence.
- **No `smb.conf`, `krb5.conf` or Samba configuration inside the container was
  modified** to make a test pass. The one file added to the DC
  (`/etc/krb5-aether.conf`) is a *client* krb5.conf used to point MIT's tools at
  the lab realm, not a change to the KDC under test.

## 9. Conclusion

All six Stage 46g defects are closed and verified live, with a seventh found and
closed in passing. The two critical Kerberos defects are fixed with
root-cause-level explanations and cross-checked against MIT's own client, which
is the strongest available evidence short of a second independent
implementation. The ccache format now passes native MIT interoperability in
both directions, evidence that was previously blocked by an unrelated
environment problem.

**No open code defects remain.** Production readiness is still gated on the
untested surface in §7, not on anything found in this stage.
