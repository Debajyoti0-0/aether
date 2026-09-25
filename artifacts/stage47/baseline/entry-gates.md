# Stage 47 Entry Gate — G3201–G3207 Results

```text
GATE      REQUIREMENT                    RESULT        EVIDENCE
--------  -----------------------------  ------------  ---------------------------------
G3201     Baseline: tree clean,           PASS          git status --porcelain -> empty
          VERSION=5.0.0-alpha1                          VERSION -> 5.0.0-alpha1
                                                        (was 3.4.0-stage3 at a887a0e)
                                                        tags -> none; v5.* -> none
G3202     Batch 1+2 packages present      PASS          internal/protocol/kerberos  20 files
                                                        internal/protocol/ldap       7 files
                                                        internal/engine/ad/kerberos  7 files
                                                        internal/engine/ad/ldap      1 file
                                                        internal/engine/ad/acl       1 file
                                                        internal/cli/ad              5 files
                                                        test/integration/ad          1 file
G3203     Stage 46h committed             PASS (after   a448932 feat(ad): deliver AD Kerberos
                                  action)   + LDAP expansion and close Stage 46h defects
G3204     Fuzz claim resolved             PASS          Re-executed, not dropped.
                                                        17,127,579 execs / 60s / 0 crashes.
                                                        artifacts/stage46h/h6-fuzz-evidence.json
G3205     Docker available                PASS          Docker 29.7.2
                                                        aether-ad-lab     Up (healthy)
                                                        aether-test-client Up
G3206     CA harness ready                ** FAIL **    See section 2. No CA exists.
G3207     Baseline tests                  PASS          build=0 vet=0 test=0 (31 ok, 0 fail)
```

## 1. Findings that contradict the entry assumptions

### 1.1 The entire AD implementation was uncommitted

The entry gate assumed Stages 45/46 were committed and only Stage 46h was
pending. In fact **none** of the AD expansion had ever been committed:

- 5 modified tracked files (`.gitignore`, `VERSION`, `cmd/aether/main.go`,
  `internal/cli/governance.go`, `internal/cli/root.go`)
- 226 untracked files, including every AD protocol/engine/CLI package

`VERSION` was still `3.4.0-stage3` at HEAD; the working tree carried
`5.0.0-alpha1`. Resolved by committing all of it as the Stage 46h closure
(230 files, 22,957 insertions) after verifying: no binaries, no keys, no
certificates, largest added file 32 KB.

`.kilo/` was excluded from that commit. It contains a **full nested git worktree**
(`worktrees/hurricane-henley/`) holding a second complete copy of the source
tree; committing it would nest the repository inside itself. It is now ignored.

### 1.2 The existing Stage 47 baseline is stale and must not be trusted

`artifacts/stage47/baseline/baseline-summary.md` (dated 2026-09-21) concludes:

```text
"Key Finding: The program plan claims Stage 45 is COMPLETE and Stage 46 is
 IMPLEMENTATION COMPLETE, but neither exists in the repository."
"Stage 47 is BLOCKED by missing Stage 45 and Stage 46 implementations."
```

That was true when written and is **false now**. Stages 45, 46 and 46h have
since implemented and live-qualified all of it. The same file also records
`VERSION = 3.4.0-stage3` and `Git Status: Clean`, both superseded.

Anyone reading that baseline without checking dates would wrongly conclude the
AD foundation is missing. It is retained as a forensic record of the prior
attempt and must be re-taken before implementation.

## 2. G3206 — CA harness: FAIL (hard gate)

Entry acknowledgment 4 states a CA harness must exist. It does not.

```text
PROBE                                                    RESULT
--------------------------------------------------------  --------------------------------
internal/protocol/ms-wcce/                                absent
internal/protocol/kerberos/pkinit.go                       absent
internal/engine/ad/adcs/                                  absent
certipy on host                                          not installed
Python (needed to install certipy)                        3.14.5 present
Hyper-V module                                           present, but Get-VM denied:
                                                         "You do not have the required
                                                          permission" (needs elevation)
Accessible Windows Server VM                              none
certreq.exe / certutil.exe                                present (client tooling, not a CA)
samba-tool CA subcommands                                 none
CA material under /var/lib/samba/private                  none
```

### 2.1 The Samba lab provisions the AD CS container but nothing behind it

This is more specific than "Samba has no AD CS", and it matters for scoping.
Probe: `artifacts/stage47/baseline/adcs-surface-probe.txt`

```text
CN=Services,CN=Configuration,DC=aether,DC=test children:
  CN=Public Key Services     <- container EXISTS
  CN=Windows NT, CN=NetServices, CN=RRAS, CN=MsmqServices

CN=Public Key Services,CN=Services,CN=Configuration,DC=aether,DC=test
                             -> result: 0 Success  (exists, but empty)
pKICertificateTemplate objects in tree                    0
certificationAuthority objects in tree                     0
pKICertificateTemplate classSchema in schema              absent
```

### 2.2 What this permits and what it forbids

```text
CAN be live-exercised on this lab:
  - LDAP template/CA enumeration: the container and base DN resolve and the
    query is valid, so the base/scope/filter construction can be proven and an
    empty result set can be proven correct.

CANNOT be live-exercised on this lab:
  - ESC1-15 detection against real template data: a zero-finding result is
    indistinguishable from a correct detector on an unpopulated tree.
  - ESC1/ESC6/ESC15 abuse: no CA to enroll against.
  - PKINIT: needs a CA-issued certificate chaining to a trust anchor the KDC
    accepts. There is no CA and no NTAuth store behind this KDC.
  - Any claim of MS-WCCE interop with a real CA.
```

### 2.3 Consequence

Per hard rule 8 and the exit criteria, Stage 47 must exit `BLOCKED-WITH-OWNER`
on the live-qualification portion. A mock MS-WCCE responder (Option C) can prove
NDR marshalling, request construction, response parsing and disposition
handling, and a synthetic LDAP fixture can prove detection logic keys off the
right attributes. Neither proves interoperability with a real CA, and neither
may be reported as if it did.
