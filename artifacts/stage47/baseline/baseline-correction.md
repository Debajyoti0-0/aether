# Stage 47 Baseline Correction — G3203

```text
Artifact:    artifacts/stage47/baseline/baseline-correction.md
Purpose:     Correct the stale Stage 47 Phase-0 baseline (dated 2026-09-21)
Corrects:    baseline-summary.md, package-inventory.json, repository-status.json,
             dependency-state.json, gate-B3-G00.json, stage45-correlation.json,
             stage46-correlation.json
Re-taken:    2026-09-26
Gate:        G3203 (Stage 46h committed and certified)
Status:      Stage 47 remains BLOCKED-WITH-OWNER on G3206 only
```

## 1. Why this artifact exists

`baseline-summary.md` was written on 2026-09-21 against commit `a887a0e`. Every
load-bearing conclusion in it is now false. It is retained unmodified as a
forensic record of the earlier attempt; this file is the correction, so that a
reader who opens the baseline directory cannot mistake it for current state.

The original file itself flags the staleness risk ("must be re-taken before
implementation"). This is that re-take, for the dependency portion only. The
`adcs-surface-probe.txt` finding in section 3 was re-probed on 2026-09-26 and is
unchanged.

## 2. Claim-by-claim correction

| # | `baseline-summary.md` claim (2026-09-21) | Verified state (2026-09-26) |
|---|---|---|
| 1 | `VERSION` = `3.4.0-stage3` | `VERSION` = `5.0.0-alpha1` |
| 2 | Commit `a887a0e`, `Git Status: Clean`, tree lacked the AD expansion | HEAD `c07338e`, branch `master`, working tree clean at re-take; AD expansion committed as `a448932` |
| 3 | `internal/protocol/kerberos` missing | **PRESENT** — 19 files (18 `.go` + 1 `testdata`), 1 subdir |
| 4 | `internal/protocol/ldap` missing | **PRESENT** — 7 files, all `.go` |
| 5 | `internal/engine/ad/kerberos` missing | **PRESENT** — 6 files, all `.go` |
| 6 | `internal/engine/ad/ldap` missing | **PRESENT** — 1 file |
| 7 | `internal/engine/ad/acl` missing | **PRESENT** — 1 file |
| 8 | CLI `ad` command not implemented | **PRESENT** — `internal/cli/ad`, 5 files |
| 9 | `scripts/ad-lab/` does not exist | **PRESENT** — 10 files, incl. `adcs-windows-setup.ps1`, `seed.sh`, `docker-compose.yml` |
| 10 | 37 packages total | **45** packages (`go list ./...`) |
| 11 | Stage 45 (Kerberos) NOT IMPLEMENTED | **IMPLEMENTED and live-qualified** — Stage 45b against Samba4 |
| 12 | Stage 46 (LDAP + ACL) NOT IMPLEMENTED | **IMPLEMENTED and live-qualified** — Stages 46d/46f/46g |
| 13 | Stage 46h not started | **COMPLETE** — 6 defects (D1–D6) closed, certified |
| 14 | Test harness / evidence artifacts absent | `test/integration/ad` present; `artifacts/stage46h/` holds evidence `h1`–`h6` |
| 15 | "All 4 Stage 46 blockers are fundamental missing infrastructure" | **All 4 resolved.** BLK-46-001 lab exists; BLK-46-002 fuzz target exists and ran 17,127,579 execs / 0 crashes; BLK-46-003 evidence exists; BLK-46-004 error matrix exists |
| 16 | Gate B3-G00 **FAIL** — Stage 46 dependency missing | **PASS** — the dependency exists, is certified, and its tests are green |
| 17 | `internal/protocol/ms-wcce` missing | still **ABSENT** — correctly, Stage 47 is not started |
| 18 | `internal/engine/ad/adcs` missing | still **ABSENT** — correctly, Stage 47 is not started |
| 19 | AD CS test harness NOT EXIST | still **NOT EXIST** — this is G3206, unchanged, blocking |

Claims 17–19 are the material ones for Stage 47 planning: the *foundation* is
present and the *Stage 47 work itself* is absent, which is the correct state to
be in at an entry gate.

Counting basis, so the numbers above are reproducible: file counts are
`Get-ChildItem <dir> -File -Recurse` at the stated commit and include
`testdata` fixtures. They are not `git ls-files` counts and are not
"number of `.go` files", which is why they differ slightly from the
`entry-gates.md` G3202 table.

## 3. The finding that did not change: no AD CS surface

Re-probed 2026-09-26 against the running `aether-ad-lab`:

```text
CN=Public Key Services,CN=Services,CN=Configuration,DC=aether,DC=test
        -> exists, 0 children
pKICertificateTemplate objects in tree     0
certificationAuthority objects in tree      0
```

Every environment path to a real CA was tested and is unavailable from this
host: no Azure/AWS/GCP CLI or credentials, Hyper-V `Get-VM` denied (session is
not elevated), no VirtualBox/VMware/QEMU, and Windows containers are impossible
because Docker Desktop here runs `linux/amd64` and the Windows `nanoserver`
manifest does not resolve for it. Full matrix and the rejected FreeIPA
substitute: `docs/stage46i-provisioning-attempt.md`.

A Samba-backed `CN=Public Key Services` container is **not** an AD CS CA. It has
no `pKICertificateTemplate` class in schema, no `certificationAuthority` objects,
and no MS-WCCE (DCOM) endpoint. It cannot qualify enrollment, template abuse
(ESC1–ESC15), or PKINIT.

## 4. Corrected dependency verdict

| Dependency | Required by Stage 47 | State | Usable as a live test target? |
|---|---|---|---|
| Kerberos (KDC, AS/TGS, ccache, PKINIT preauth) | yes | implemented, live-qualified | **yes** — Samba KDC at `172.18.0.2`, realm `AETHER.TEST` |
| LDAP (bind, search, base/scope, ACL parsing) | yes | implemented, live-qualified | **yes** — same DC |
| ACL / security-descriptor parsing | yes | implemented, unit-tested | yes (directory-level) |
| AD CS templates (MS-WCCE, ESC detection) | yes | absent | **no** — no CA exists |
| AD CS enrollment (MS-WCCE DCOM) | yes | absent | **no** |
| PKINIT against a CA-issued cert | yes | absent | **no** — no CA, no NTAuth store |

## 5. Resulting gate state

```text
G3201  PASS   VERSION 5.0.0-alpha1, no v5.* tag, tree clean
G3202  PASS   all Batch 1+2 packages present
G3203  PASS   Stage 46h committed and certified (this artifact records the
               dependency state the gate assumed and corrects the stale baseline)
G3204  PASS   fuzz re-executed, 17,127,579 execs / 60s / 0 crashes
G3205  PASS   Docker 29.7.2, aether-ad-lab healthy, aether-test-client up
G3206  FAIL   CA harness: no CA, no templates, no reachable provisioning path
G3207  PASS   go build 0 / go vet 0 / go test 0 failures
```

**G3206 is the only failing gate, and it is not a repository defect.** It cannot
be cleared from this host by any amount of code, because it requires a real
Windows Server 2022 AD DS + AD CS environment. Owner action is specified in
`docs/stage46i-certification.md`. No mock CA, synthetic CA, or fixture
substitution was used to make this gate appear satisfiable.
