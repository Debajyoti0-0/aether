# Stage 47B — Certification

```text
Stage 47B:    BLOCKED-WITH-OWNER
Primary gate: G3206 — real Windows AD DS + KDC + AD CS qualification environment
HEAD:         ebb8c8bfb64f52bc0205351366495088c3772e63
TREE:         CLEAN
VERSION:      5.0.0-alpha1 (unchanged — no release decision made)
Tag:          NONE
Code changed: NONE. No Aether source file was created, modified or deleted.
              Stage 47B is an assessment and evidence stage.
```

## 1. What Stage 47B was asked to do, and what it did

Stage 47B's primary objective was to **resolve G3206** by provisioning and
qualifying a real Windows Server 2022 AD DS + AD CS environment, and to
authorize Stage 47 implementation only if G3206-A…Q all passed.

G3206 was not resolved. It cannot be resolved from this host, and this stage
re-tested that conclusion rather than restating it. The evidence for the
refusal is new, dated, and independently reproducible.

What Stage 47B *did* establish:

```text
1. Forensic baseline re-captured at HEAD ebb8c8b: clean tree, VERSION
   5.0.0-alpha1, no v5.* tag.  -> baseline/baseline.md
2. All 5 Stage 46i documents verified present and unaltered (sizes + SHA-256).
   The 7 historical 2026-09-21 baseline files confirmed untouched.  -> G47B-03/04
3. All 9 provisioning paths re-probed on 2026-09-26. Still 0 executable.
                                                             -> provisioning/
4. Windows-container path closed on TWO independent grounds (new, decisive).
                                    -> provisioning/windows-container-probe.txt
5. AD CS surface re-probed. Two Stage 46i findings found to be WRONG and
   corrected in place-of-record, without editing the originals.
                                              -> adcs/surface-reprobe.txt
6. Provisioning script re-verified statically: 0 AST errors, 0 secret literals,
   -SafeMode semantics re-derived from the AST, idempotency guards re-confirmed,
   known ACE-order weakness re-disclosed and NOT silently fixed.
                                    -> provisioning/script-static-verify.txt
7. Full quality gate set executed: build 0, vet 0, test 31 ok / 0 FAIL,
   race 0 / 0 DATA RACE.  -> baseline/, race/
8. G47B-01..50 matrix recorded gate by gate, 11 PASS / 5 PARTIAL /
   17 NOT SATISFIED / 1 DENIED / 16 NOT REACHED.  -> gate-matrix.md
9. Rust + wasm32 + wasm-pack toolchain installed (absent at Stage 46i) so the
   Stage 52 WASM workstream is not itself blocked.  -> baseline/baseline.md
```

## 2. G3206 — why it is still OPEN

### 2.1 Provisioning path re-probe, 2026-09-26

Every path in the §6 list was re-executed. Nothing changed since Stage 46i.

| Path | Mechanism | Result | Evidence |
|---|---|---|---|
| Azure | `az` | **UNAVAILABLE** — CLI absent, `~/.azure/azureProfile.json` absent | `provisioning/path-reprobe.txt` |
| AWS | `aws` | **UNAVAILABLE** — CLI absent, `~/.aws/credentials` absent | same |
| GCP | `gcloud` | **UNAVAILABLE** — CLI absent, `credentials.db` absent | same |
| Hyper-V | `Get-VM`, `Get-WindowsOptionalFeature` | **UNAVAILABLE** — both denied; `IsAdministrator = False` | same |
| VirtualBox | `VBoxManage` | **UNAVAILABLE** — not installed | same |
| VMware | `vmrun` | **UNAVAILABLE** — not installed | same |
| QEMU | `qemu-system-x86_64` | **UNAVAILABLE** — not installed | same |
| Windows container | `docker pull …/nanoserver:ltsc2022` | **UNAVAILABLE** — see §2.2 | `provisioning/windows-container-probe.txt` |
| Certipy | — | **INSUFFICIENT** — a client, not a CA; cannot issue | Stage 46i |
| FreeIPA | — | **REJECTED** — no MS-WCCE, no AD CS templates | Stage 46i |
| WSL | `wsl -d Ubuntu` | **TOOLING ONLY** — MIT krb5-pkinit installed; no reachable AETHER.TEST KDC, no CA | §2.4 |
| **Operator handoff** | — | **SELECTED** — the only executable path | §7 |

Also probed and absent, ruling out infrastructure-as-code shortcuts:
`terraform`, `packer`, `Vagrant`, `~/.kube/config`.

### 2.2 The Windows-container path is now closed on two independent grounds

Stage 46i reported this path as unavailable. Stage 47B found the earlier
evidence was imprecise and re-ran it properly, because `docker manifest inspect`
and `docker pull` answer different questions:

```text
docker manifest inspect mcr.microsoft.com/windows/nanoserver:ltsc2022
  -> SUCCESS. schemaVersion 2, a windows/amd64 manifest for os.version
     10.0.20348.5622. The manifest EXISTS in the registry.

docker pull mcr.microsoft.com/windows/nanoserver:ltsc2022
  -> FAIL: "no matching manifest for linux/amd64/v4 in the manifest list
     entries: no match for platform in manifest: not found"

docker info --format '{{.OSType}}'   -> linux
docker context show                  -> desktop-linux
```

The honest reading: the earlier claim "no matching manifest" was **true of the
pull** and **false of the registry**. The image is real; this daemon simply
cannot run it. Recording only the pull error would have overstated the
obstruction, and recording only the manifest would have understated it.

**Ground 1 — the daemon is Linux.** A Windows image requires a Windows daemon.
This daemon is `linux/amd64`, so no Windows container can start here at all.

**Ground 2 — and it would not help.** AD DS and AD CS are **not supported as
Windows container roles**. A Windows container cannot host a domain controller
or a certification authority, even on a Windows host with Hyper-V. So this path
is closed by a product limitation, not merely by local configuration.

Ground 2 is the stronger argument and is the reason no amount of Docker work
would have unblocked G3206. It is recorded so a future engineer does not
re-attempt the container route believing it is merely misconfigured here.

### 2.3 AD CS surface — the Samba lab, and two corrections

Re-probed on 2026-09-26 with a **control query** proving the bind and query
path work, so that every empty result is a real absence and not a tool failure.

```text
CONTROL   (objectClass=domainDNS) on DC=aether,DC=test
          -> dn: DC=aether,DC=test                      [bind + path WORK]

G3206-G   CN=Public Key Services .............. EXISTS, empty
          CN=Enrollment Services ............... EXISTS, empty
          CN=Certificate Templates ............. EXISTS, empty
G3206-H   (objectClass=pKICertificateTemplate)  -> 0 entries
          (cn=ESC1-Test | ESC2-Test | ESC3-Test | ESC6-Test | ESC15-Test)
                                                -> 0 entries
G3206-F   (objectClass=certificationAuthority)  -> 0 entries
```

**Correction 1 — Stage 46i recorded a false negative.** Both
`docs/stage46i-certification.md` and `artifacts/stage47/baseline/adcs-surface-probe.txt`
state that `pKICertificateTemplate` is absent from the schema. It is present:

```text
dn: CN=PKI-Certificate-Template,CN=Schema,CN=Configuration,DC=aether,DC=test
objectClass: top
objectClass: classSchema
cn: PKI-Certificate-Template
lDAPDisplayName: pKICertificateTemplate
```

**Correction 2 — Stage 46i's inventory was incomplete.** It listed only
`CN=Public Key Services` as present. `CN=Enrollment Services` and
`CN=Certificate Templates` are also present beneath it.

Both original files are **unmodified**. The correction lives in
`artifacts/stage47b/adcs/surface-reprobe.txt` §4, per §2 (preserve evidence) and
§57 (never silently move a status).

**Neither correction weakens the verdict — both sharpen it.** The classes
`pKIEnrollmentService`, `certificationAuthority` and `pKIExtendedKeyUsage` *are*
absent. So Samba ships the AD PKI *schema* and the PKI *container skeleton*, and
then provisions nothing into it. The absence of a CA here is a
**runtime/servicing** absence, not a schema absence. That distinction is exactly
why schema presence plus empty containers cannot be substituted for a real
Windows AD CS deployment: a detection tool run against this tree would find zero
templates, and a zero-finding result on an unpopulated tree is
indistinguishable from a correct detector.

### 2.4 Independent PKINIT — tooling ready, exchange impossible

The independent implementation exists and is confirmed working:

```text
WSL Ubuntu 26.04.1, kernel 6.6.114.1-microsoft-standard-WSL2
krb5-user 1.22.1-2ubuntu4.1, krb5-config 2.7ubuntu1, krb5-locales 1.22.1-2ubuntu4.1
/usr/lib/x86_64-linux-gnu/krb5/plugins/preauth/pkinit.so   128056 bytes
OpenSSL 3.5.5 27 Jan 2026
kinit klist kvno kdestroy
```

A PKINIT-shaped certificate was minted and verified:

```text
subject=CN=pkinit-probe
X509v3 Extended Key Usage:  TLS Web Client Authentication
X509v3 Subject Alternative Name: othername: UPN:pkinit-probe@PROBE.TEST
Signature Algorithm:  sha256WithRSAEncryption
private key matches certificate: MATCH
```

The exchange cannot be attempted:

```text
$ kinit pkinit-probe@AETHER.TEST
Cannot contact any KDC while getting initial credentials
```

This is the §20 STOP condition, and it is correctly classified as an
**ENVIRONMENT CONFIGURATION DEFECT**, not an Aether defect: the failure occurs
before PKINIT is reached, because no KDC is reachable and no CA anchor exists.
A minted certificate is not a PKINIT exchange, and the difference is recorded
here so a later report cannot quietly collapse it.

One carry-forward finding for Stage 47: this OpenSSL build does not support the
MIT `id-pkinit-san` form (`otherName:1.3.6.1.5.2.2;prn:…`); the Microsoft UPN
otherName form (`1.3.6.1.4.1.311.20.2.3`) was required. The two forms are not
interchangeable, and Stage 47's AuthPack must emit whichever the target KDC
accepts.

### 2.5 G3206-A…Q, gate by gate

| Sub-gate | Requirement | Status | Blocking reason |
|---|---|---|---|
| G3206-A | Windows Server exists | NOT SATISFIED | no hypervisor reachable; not elevated |
| G3206-B | AD DS exists | NOT SATISFIED | no Windows domain; Samba is not AD DS |
| G3206-C | DNS works | NOT SATISFIED | no Windows DC to host DNS |
| G3206-D | KDC works | NOT SATISFIED | no Windows KDC |
| G3206-E | LDAP works | NOT SATISFIED | Aether's LDAP works against Samba, but there is no Windows LDAP |
| G3206-F | Enterprise CA exists | NOT SATISFIED | 0 `certificationAuthority` objects; installer never executed |
| G3206-G | Public Key Services populated | NOT SATISFIED | 3 containers exist, 0 children |
| G3206-H | Five expected test templates exist | NOT SATISFIED | 0 `pKICertificateTemplate` objects |
| G3206-I | Template ACLs correct | NOT SATISFIED | no templates to carry descriptors |
| G3206-J | CA configuration correct | NOT SATISFIED | no `CertSvc` registry key; ESC6 flag **not observed**, only intended |
| G3206-K | Certificate enrollment works | NOT SATISFIED | no CA, no MS-WCCE endpoint |
| G3206-L | Certificate validation works | NOT SATISFIED | no CA certificate, no chain |
| G3206-M | KDC trusts issuing CA | NOT SATISFIED | no CA certificate to publish to `NTAuthCertificates` |
| G3206-N | PKINIT works independently | NOT SATISFIED | tooling ready; no reachable KDC, no CA anchor |
| G3206-O | Negative controls work | NOT SATISFIED | no issued certificate to negate |
| G3206-P | Environment reproducible | NOT SATISFIED | cannot reproduce an environment never created; the setup script is untested live |
| G3206-Q | Evidence complete | NOT SATISFIED | evidence of absence is complete; evidence of a qualified environment does not exist |

**`TemplateCount=5` and `Esc6CaFlagPresent=True` are expected configuration
values. They were NOT copied into any artifact as observations.** Per §14, they
may be recorded only if actually observed. They were not observed, so they
appear nowhere in this stage's evidence as results.

## 3. Authorization decision (G47B-27)

§22 permits `BLOCKED-WITH-OWNER` → `IMPLEMENTATION-AUTHORIZED` only after
G3206-A…Q pass. Seventeen of the seventeen environment sub-gates relevant to
G3206 fail. **The transition is DENIED and is not taken.**

Stage 47 remains `BLOCKED`. No `internal/protocol/ms-wcce/`,
`internal/engine/ad/adcs/`, `internal/protocol/pkinit/` or CMS package was
created. Both were confirmed still absent, which is the correct state at this
gate.

## 4. Provisioning script — static re-verification

`scripts/ad-lab/adcs-windows-setup.ps1` cannot be executed here, so §8's live
validation (fresh / repeat / partial / failed / SafeMode / recovery deployment)
is **NOT PERFORMED**. The static portion was re-executed rather than cited:

```text
AST parse errors        0
tokens                  1061
functions               7
parameters              3  (DomainFQDN:String, DomainAdminPassword:String, SafeMode:SwitchParameter)
secret literals         0  (no password literal, no PEM private-key block;
                          the only "pass" substring hits are the attribute
                          name pKIExtendedKeyUsage, which is not a credential)
mutating cmdlets        Install-WindowsFeature, Install-ADCSertificationAuthority,
                        New-ADCertificateTemplate, Register-ADCSecurityTemplate,
                        Add-ADObjectAce, Set-ADObject, Set-ItemProperty,
                        Restart-Service
```

**`-SafeMode` semantics, re-derived from the AST:**

| Phase | `-SafeMode` | Default |
|---|---|---|
| Assert PSPKI present | runs | runs |
| Assert domain matches `-DomainFQDN` | runs | runs |
| Install AD CS role features | **runs** | runs |
| Install Enterprise CA | **runs** | runs |
| Set `EDITF_ATTRIBUTESUBJECTALTNAME2` | skipped | runs |
| Create + publish 5 vulnerable templates | skipped | runs |
| Grant Domain Users Enroll ACE | skipped | runs |
| Print environment state | runs | runs |

A `-SafeMode` CA is a **real, hardened** CA. It is not a simulation and must
never be described as one; it is also **not valid for ESC qualification**.

**Idempotency (G47B-08), static only.** Early-return guards are present in all
three mutating phases. The known weakness is re-disclosed and deliberately not
blind-fixed: `New-VulnerableTemplate` returns early when the template exists, and
the Enroll ACE grant happens *after* that return — so a run that created the
template and then failed before adding the ACE will skip the ACE permanently on
retry. Operator remedy: delete the template and re-run. This cannot be tested
without a domain, so it is documented as a known limitation and **not** claimed
as verified. §9's controlled partial-state experiments were NOT PERFORMED.

`ADCS-Web-Enrollment` is installed but never configured. Its presence is
harmless here and is recorded so it is not later mistaken for ESC8 having been
qualified.

## 5. Quality gates

Executed on 2026-09-26 at `ebb8c8b`:

```text
go build ./...                     exit 0, no output
go vet ./...                       exit 0, no findings
go test -count=1 ./...             exit 0
                                   31 packages ok
                                    0 FAIL
                                   14 packages with no test files
                                   45 packages total (go list ./...)
go test -race -count=1 ./...       exit 0
                                   31 packages ok
                                    0 FAIL
                                    0 DATA RACE reports
                                   (gcc from mingw-winlibs supplies the
                                    race runtime on windows/amd64)
```

Evidence: `baseline/quality-gates.txt`, `baseline/go-test-output.txt`,
`race/go-test-race.txt`.

### 5.1 Skipped tools — reason, impact, replacement evidence (§39)

| Tool | State | Reason | Impact | Replacement evidence |
|---|---|---|---|---|
| `golangci-lint` | **not installed** | not present on host; installing would add a toolchain dependency for no gate-critical signal | lower-cased linters (errcheck, staticcheck, unused, gosimple) would not run | `go vet ./...` exit 0 covers the vet analyser family; `go build` covers compilation. Staticcheck-class findings remain **unverified**, not assumed absent |
| `govulncheck` | **not installed** | not present on host; requires network module graph analysis | no CVE reachability analysis of dependencies | dependency set is small and pinned (9 direct, all in `go.mod`). This is **not** equivalent to a clean vulnerability scan, and is recorded as an open gap, not a pass |
| `staticcheck` | not run | same toolchain absence | see above | as above |

These are recorded as gaps in §11 residual risk. None of them affects the G3206
verdict, and none is reported as a pass.

## 6. What was deliberately not done

```text
No mock / synthetic / hand-rolled AD CS CA       would make G3206 appear satisfied
No Certipy or fixture substitution                client-side only; proves no interop
No fabricated credentials, certificates or keys    would authenticate against nothing
No change to the G3206 definition                 the gate is correct; the environment is absent
No change to any prior stage's status              Stage 46i stays ACHIEVED
No edit or deletion of contradicted evidence      corrections recorded alongside originals
No Stage 47 source written                        §22/§23 prohibit it before G3206 closes
No Aether source file modified                    Stage 47B authored documentation only
No VERSION bump                                   5.0.0-alpha1 stands
No v5.* tag                                       no release decision reached
No copy of expected values into results           TemplateCount / Esc6CaFlag stayed unrecorded
```

## 7. Owner handoff — the only route to G3206

Use the recorded procedure in `docs/stage46i-certification.md` §1.2 and §8. In
summary, on a **throwaway, isolated, non-production Windows Server 2022**
machine:

```powershell
# AETHER AD CS QUALIFICATION LAB - WINDOWS SERVER 2022 - INTENTIONALLY
# VULNERABLE - NON-PRODUCTION - DISPOSABLE
# In an elevated PowerShell, from the Aether source tree:

Install-WindowsFeature RSAT-AD-PowerShell, RSAT-ADCS-Mgmt

# OMIT -SafeMode. It provisions a hardened CA and cannot qualify ESC.
.\scripts\ad-lab\adcs-windows-setup.ps1 `
    -DomainFQDN aether.test `
    -DomainAdminPassword '<owner-supplied>' `
    -Verbose 4>&1 | Tee-Object artifacts\stage47b\ca\ca-validation.txt
```

Then, per §19/§21, with **three mandatory negative controls**:

```text
1. publish the CA to the domain's NTAuthCertificates, else PKINIT fails chain
   validation for reasons that are NOT an Aether defect
2. MIT kinit with the issued cert  -> must succeed, and the preauth indicator
   must show PKINIT, not ENCRYPTED-TIMESTAMP
3. negative control 1: cert for Identity A must not authenticate as Identity B
4. negative control 2: cert from an untrusted CA must fail
5. negative control 3: cert lacking required auth properties must fail
6. only then: Aether's own PKINIT path against the same CA, with identical
   negative-control behaviour. Divergence on any case = Aether defect.
```

Full per-gate evidence requirements for the owner's return are in
`docs/stage46i-certification.md` §9.

## 8. Reproducibility of this stage's own claims (§58)

Every claim above answers yes to: *could an independent senior engineer
reproduce this from a clean checkout and the documented environment without
trusting this report?*

| Claim | Reproducible from |
|---|---|
| HEAD / tree / VERSION / tag | `git rev-parse`, `git status --porcelain`, `VERSION`, `git tag -l 'v5.*'` |
| Build/vet/test/race green | the four commands in §5, with gcc present |
| No provisioning path available | `provisioning/path-reprobe.txt`; every command is listed in the file |
| Windows container closed on 2 grounds | `provisioning/windows-container-probe.txt`; both `docker manifest inspect` and `docker pull` outputs are pasted verbatim |
| AD CS absent + 2 corrections | `adcs/surface-reprobe.txt`; bind DN, base DNs, filters and a working control query are all recorded, and the lab credential is documented in `scripts/ad-lab/docker-compose.yml` |
| PKINIT tooling ready, exchange blocked | WSL package versions + `kinit` error text in §2.4 and Stage 46i |
| Script static properties | `provisioning/script-static-verify.txt`; the AST is re-parsed by the recorded command |

Nothing in this certification depends on a credential that is not already
committed to the repository, a Docker volume, a hosts entry, or an uncommitted
file. The lab password is redacted in the evidence and its source is cited.

## 9. Evidence index

```text
artifacts/stage47b/
  baseline/baseline.md                 forensic baseline, corrections, evidence hashes
  baseline/quality-gates.txt           build / vet / test summary
  baseline/go-test-output.txt          full go test output
  provisioning/path-reprobe.txt        all 9+ provisioning paths, 2026-09-26
  provisioning/windows-container-probe.txt   manifest-vs-pull, two-ground closure
  provisioning/script-static-verify.txt      AST, secrets, -SafeMode, idempotency
  adcs/surface-reprobe.txt             AD CS surface + control query + 2 corrections
  race/go-test-race.txt                go test -race, 0 DATA RACE
  gate-matrix.md                       G47B-01..50
  reports/                             (this certification is at docs/stage47b-certification.md)
```

The remaining Stage 47B subdirectories (`owner/ ad-ds/ dns/ kdc/ ca/ templates/
enrollment/ certificate/ pkinit/ negative/ cli/ flags/ arguments/ security/ fuzz/
regression/ reproducibility/ evidence/ final/`) are intentionally **empty**. They
exist so the owner's evidence lands in a defined place. Empty is the correct
state for every one of them: populating them would mean fabricating a Windows AD
CS qualification.

## 10. §56 final status

```text
AETHER STAGE 47B
================

HEAD:                    ebb8c8bfb64f52bc0205351366495088c3772e63
TREE:                    CLEAN
VERSION:                 5.0.0-alpha1

Stage 46i:               ACHIEVED

G3201:                   PASS
G3202:                   PASS
G3203:                   PASS
G3204:                   PASS
G3205:                   PASS
G3206:                   OPEN
G3207:                   PASS

AD DS:                   NOT SATISFIED   (no Windows domain)
DNS:                     NOT SATISFIED   (no Windows DC)
KDC:                     NOT SATISFIED   (no Windows KDC)
AD CS:                   NOT SATISFIED   (0 certificationAuthority objects)
Templates:               NOT SATISFIED   (0 pKICertificateTemplate objects)
Enrollment:              NOT SATISFIED   (no CA, no MS-WCCE endpoint)
Certificate Validation:  NOT SATISFIED   (no CA certificate, no chain)
KDC Trust:               NOT SATISFIED   (no CA certificate to publish)
Independent PKINIT:      NOT SATISFIED   (tooling ready; no reachable KDC, no CA)

CLI:                     NOT REACHED
Flags:                   NOT REACHED
Arguments:               NOT REACHED
Security:                NOT REACHED
Fuzzing:                 UNVERIFIED   (Stage 46h figure of 17,127,579 execs /
                                       0 crashes is carried forward from
                                       artifacts/stage46h/h6-fuzz-evidence.json
                                       and was NOT re-executed in this stage)
Race:                    PASS
Static Analysis:         PARTIAL   (go vet clean; golangci-lint / staticcheck /
                                    govulncheck not installed - open gap)
Regression:              PASS
Reproducibility:         PARTIAL   (build/test/race reproduce; live
                                    qualification never performed)
Evidence:                PASS       (for the absence finding; no evidence of a
                                    qualified environment exists or was faked)

Stage 47:
BLOCKED   (G47B-27 DENIED - G3206-A..Q do not pass)

Stage 48:                PENDING
Stage 49:                PENDING
Stage 50:                PENDING
Stage 51:                PENDING

Production Readiness:
NOT READY
```

## 11. Residual risk

| Risk | Impact | Mitigation |
|---|---|---|
| G3206 never satisfied | Stage 47 stays blocked indefinitely | operator handoff is the only path; no local substitute exists and none will be attempted |
| Windows-container route re-attempted in future | wasted effort on a dead path | §2.2 closes it on a product limitation, not local config |
| Prior evidence read without the corrections | a reader concludes the PKI schema class is absent | corrections in `adcs/surface-reprobe.txt` §4; originals preserved per §2 |
| `TemplateCount=5` / `Esc6CaFlagPresent=True` copied forward | a false PASS (§14, §57) | neither value appears in any Stage 47B evidence file as a result |
| PKINIT chain validation blamed on Aether | false defect report | §19 and §2.4 classify it as an environment configuration defect |
| `govulncheck` / `golangci-lint` absent | unknown dependency CVEs and linter findings | §5.1 records the gap; it is not reported as a pass |
| Rust toolchain installed mid-stage | environment drift between baselines | installation is recorded in `baseline/baseline.md` §2 so the next baseline is not misled |

**Verdict: BLOCKED-WITH-OWNER.** The repository is not the blocker and was not
made to look like the blocker. Everything reachable from this host was executed,
verified and recorded; the remainder is explicitly, specifically the owner's to
supply, with exact commands.
