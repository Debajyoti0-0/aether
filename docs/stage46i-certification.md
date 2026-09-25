# Stage 46i — Certification (Provisioning Path + Owner Handoff)

```text
Stage 46i:     BLOCKED-WITH-OWNER
Blocking gate: G3206 — real Windows AD DS / KDC / AD CS environment
Satisfied:     G3201, G3202, G3203, G3204, G3205, G3207
HEAD:          c07338e
Version:       5.0.0-alpha1 (unchanged — no release decision made)
Tag:           NONE
Code changed:  NONE (Stage 46i is an assessment stage; no Aether source file was
               modified, and no Stage 47 code was written)
```

Stage 46i exists to answer one question before Stage 47 begins: **can a real AD CS
CA be provisioned, validated, and used to qualify PKINIT and MS-WCCE from this
environment?** The answer is no, and the reason is environmental, not
repository-side. This document certifies that finding, records what *was*
established, and specifies the owner action that unblocks it.

## 1. CA validation — NOT PERFORMED (the blocking item)

This is the section the stage exists to produce, so it is stated first and
without softening.

```text
CA validation status:   NOT PERFORMED
Reason:                 no CA exists, and no provisioning path is executable here
Substitute evidence:    NONE — no mock CA, no synthetic CA, no Certipy fixture
                        was created to make this section look complete
```

An empty CA-validation section is the honest result. Writing one would require
either a real CA or a fabricated stand-in, and the charter prohibits the
substitution. A mock CA proves NDR marshalling; it proves nothing about
interoperability, and reporting it as validation would be a false certification.

### 1.1 What was checked, and what each check showed

| Check | Result |
|---|---|
| `internal/protocol/ms-wcce/` | absent — Stage 47 not started (correct) |
| `internal/engine/ad/adcs/` | absent — Stage 47 not started (correct) |
| `certipy` on host | not installed (and insufficient: it is a client, not a CA) |
| `certreq.exe` / `certutil.exe` | present — client tooling, cannot issue from a CA database |
| `samba-tool CA` subcommands | none |
| CA material under `/var/lib/samba/private` | none |
| `CN=Public Key Services` container | **exists, 0 children** |
| `pKICertificateTemplate` objects in tree | **0** |
| `certificationAuthority` objects in tree | **0** |
| `pKICertificateTemplate` in classSchema | absent |

Re-probe evidence: `artifacts/stage47/baseline/adcs-surface-probe.txt`.

The `CN=Public Key Services` container existing is worth stating precisely,
because it is the most likely thing to be mistaken for a CA. It is an empty
LDAP container. There is no `pKICertificateTemplate` schema class, no
`certificationAuthority` object, no CA signing key, no certificate templates, and
no DCOM `ICertRequest` endpoint. A Samba DC cannot become an AD CS CA, and no
Samba configuration flag changes that.

### 1.2 The procedure that must be run by the owner

The environment must be a **throwaway Windows Server 2022** machine with AD DS
and AD CS. Not a client with the AD CS management tools, not a Samba DC, and not
an existing production domain.

```powershell
# 1. On the throwaway DC, in an elevated PowerShell, from the repository:
#    (run from the Aether source tree, mounted or copied to the host)

# 2. Confirm the prerequisite module is present - the script fails fast without it,
#    because base AD cmdlets cannot PUBLISH a template to a CA:
Install-WindowsFeature RSAT-AD-PowerShell, RSAT-ADCS-Mgmt

# 3. Provision. Omit -SafeMode to create the intentionally vulnerable templates;
#    the vulnerable templates are the entire point of the qualification.
.\scripts\ad-lab\adcs-windows-setup.ps1 `
    -DomainFQDN aether.test `
    -DomainAdminPassword '<owner-supplied>' `
    -Verbose

# 4. Capture the environment state the script prints (CA name, CA hostname,
#    CAConfigurationFlags, Esc6CaFlagPresent, TemplateCount, per-template
#    nameFlag / enrollFlag / schema / eku). This IS the CA validation evidence.
```

The script's own verification block (`Test-QualificationEnvironment`) prints
exactly the fields CA validation must record. Capture that output verbatim:

```powershell
.\scripts\ad-lab\adcs-windows-setup.ps1 ... -Verbose 4>&1 |
    Tee-Object -FilePath artifacts/stage46i/ca-validation.txt
```

Expected shape of a qualified environment:

```text
TemplateCount   = 5
Templates       = ESC1 Test   (nameFlag supplies subject, enrollFlag=0, schema=2, eku=Client Authentication)
                  ESC6 Test   (nameFlag=CT_FLAG_ENROLLEE_SUPPLIES_SUBJECT..., eku=Client Authentication)
                  ESC15 Test  (schema=1, eku=Client Authentication)
                  ESC2 Test   (eku=Any Purpose)
                  ESC3 Test   (eku=Client Authentication)
Esc6CaFlagPresent = True
```

Five templates, `Esc6CaFlagPresent = True`, one template per ESC class named
`ESC1-Test`, `ESC6-Test`, `ESC15-Test`, `ESC2-Test`, `ESC3-Test`. Anything less
is a provisioning failure, not a Stage 47 defect.

A CA created with `-SafeMode` is **real** — roles installed, Enterprise CA
created — but hardened. It is unsuitable for ESC qualification and suitable for
PKINIT trust-chain work only. It is not a simulation and must not be described
as one; it is also not a substitute for the vulnerable configuration.

## 2. Independent PKINIT validation — TOOLING READY, EXCHANGE BLOCKED

The charter requires PKINIT to be validated against an implementation other than
Aether. The independent implementation is now installed and confirmed working.

```text
WSL distro:        Ubuntu 26.04.1 LTS (kernel 6.6.114.1-microsoft-standard-WSL2)
krb5 packages:     krb5-user 1.22.1-2ubuntu4.1, krb5-config 2.7ubuntu1,
                   krb5-locales 1.22.1-2ubuntu4.1
PKINIT plugin:     /usr/lib/x86_64-linux-gnu/krb5/plugins/preauth/pkinit.so (128056 bytes)
OpenSSL:           3.5.5 27 Jan 2026
binaries:          kinit klist kvno kdestroy
```

### 2.1 The blocker

```text
$ kinit pkinit-probe@AETHER.TEST
Cannot contact any KDC while getting initial credentials
```

This fails **before** PKINIT is reached — no KDC is reachable from WSL, because
the Samba lab is a Docker network and the minted certificate is self-signed with
no chain to any anchor the KDC accepts. The failure is a harness/topology fact,
not evidence about Aether and not evidence that PKINIT works.

### 2.2 What the local mint proved, and what it did not

```text
subject=CN=pkinit-probe
X509v3 Extended Key Usage:   TLS Web Client Authentication
X509v3 Subject Alternative Name: othername: UPN:pkinit-probe@PROBE.TEST
Signature Algorithm:         sha256WithRSAEncryption
private key matches certificate: MATCH
```

This proves the toolchain can produce PKINIT-shaped material. It proves nothing
about a PKINIT exchange. The two must not be conflated in any later report.

One finding is carried forward because it affects Stage 47's CMS/AuthPack work:
this OpenSSL build does **not** support the MIT `id-pkinit-san` form
(`otherName:1.3.6.1.5.2.2;prn:...`); the Microsoft UPN otherName form
(`1.3.6.1.4.1.311.20.2.3`) was used instead. The forms are not interchangeable,
and Stage 47 must emit whichever the target KDC accepts.

### 2.3 The validation that must be run once the CA exists

Two runs are required, and the negative control is not optional. A single
successful `kinit` proves only that a permissive path works.

```bash
# POSITIVE: a certificate issued by the qualified CA, chaining to an anchor the
# KDC trusts, with the SAN form the KDC accepts.
kinit -X X509_user_identity=FILE:/etc/krb5/pkinit.pem pkinit-probe@AETHER.TEST
klist -e
# Expect: a fresh ccache whose client principal is pkinit-probe@AETHER.TEST with
#         a preauth indicator showing PKINIT, not ENCRYPTED-TIMESTAMP.

# NEGATIVE CONTROL 1: right cert, revoked/expired trust -> must FAIL.
# NEGATIVE CONTROL 2: cert whose SAN UPN does not match the principal, against
#                    the KDC -> must FAIL. This is the control that distinguishes
#                    "PKINIT is honoured" from "the KDC accepted anything".
# NEGATIVE CONTROL 3: cert issued by a CA the KDC does not trust -> must FAIL.
```

Aether's own PKINIT path must then be exercised against the same CA and produce
an equivalent ticket, with the negative controls failing identically. Aether's
failure on a control that MIT also fails on is a harness result, not a defect;
divergence between the two implementations on any case is a defect.

### 2.4 The KDC-trust prerequisite

PKINIT requires the KDC to trust the issuing CA. The script's closing guidance
repeats this because it is the single most likely cause of a false negative:
publish `NTAuthCertificates` for the domain, or enroll the KDC with a certificate
from a CA already in its trust store. Without it, chain validation fails in a way
that looks exactly like an Aether bug.

## 3. Provisioning path evaluation

Every path in the charter's ranking table was executed, not assumed. Full
evidence, including the FreeIPA evaluation and why it was rejected, is in
`docs/stage46i-provisioning-attempt.md`.

```text
Path A  Azure        UNAVAILABLE  (no az CLI, no ~/.azure/azureProfile.json)
Path B  AWS          UNAVAILABLE  (no aws CLI, no ~/.aws/credentials)
Path C  GCP          UNAVAILABLE  (no gcloud, no credentials.db)
Path D  Hyper-V      UNAVAILABLE  (Get-VM: access denied; session not elevated)
Path E  VirtualBox / VMware / QEMU  UNAVAILABLE (not installed)
Path F  Operator handoff           SELECTED
Path G  Certipy fixtures           does not clear G3206 — a client, not a CA
FreeIPA                            rejected — no MS-WCCE, no AD CS templates
WSL Ubuntu 26.04.1                 recovered; Kerberos tooling installed
Windows containers                 impossible (Docker is linux/amd64;
                                   nanoserver:ltsc2022 has no such manifest)
```

FreeIPA deserves the explicit note because it is the tempting shortcut: it ships
a real CA and supports PKINIT, so it would clear *part* of the PKINIT
requirement. It was rejected because it has no `ICertRequestD` equivalent, so
MS-WCCE enrollment is unqualified, and it has no `pKICertificateTemplate`
objects, so ESC1–ESC15 detection would be unqualified. Partial coverage offered
as qualification is the same substitution the charter prohibits.

## 4. Provisioning script review (G3303/G3335, by inspection)

`scripts/ad-lab/adcs-windows-setup.ps1` cannot be executed here (no Windows
domain, no elevation), so it was reviewed statically and via the AST. It passed
every static check available:

```text
G3303  syntax errors     0
        functions         7
        parameters        3  (DomainFQDN:String, DomainAdminPassword:String,
                               SafeMode:SwitchParameter)
        literal secrets   none — no password or private-key literal anywhere;
                          the only regex hits for pass/key were the attribute
                          name pKIExtendedKeyUsage
G3335  idempotency       confirmed by control-flow inspection in all three
                        mutating phases (CA install, CA flag, template create)
```

### 4.1 `-SafeMode` semantics

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

### 4.2 Known limitation, disclosed rather than fixed blind

Idempotency holds, but there is one partial-state weakness. `New-VulnerableTemplate`
returns early when the template already exists, and the Enroll ACE grant happens
*after* that early return. A run that created the template but failed before the
ACE was added will, on retry, skip the ACE permanently.

Operator instruction: if a template is reported present but enrollment fails with
access denied, delete the template and re-run. This cannot be tested without a
domain, so it is documented as a known limitation rather than claimed as verified.

### 4.3 Two observations recorded for later stages

- `ADCS-Web-Enrollment` is installed but never configured or used. Its presence is
  harmless here, and it is noted explicitly so it is not later mistaken for
  ESC8 having been qualified. ESC8 remains unqualified.
- The script requires PSPKI and fails fast without it, because base AD cmdlets
  cannot publish a template. This is correct: the alternative is a
  half-configured CA that looks provisioned.

## 5. Baseline correction (G3203)

`artifacts/stage47/baseline/baseline-summary.md` (2026-09-21, against `a887a0e`)
concluded that Stages 45 and 46 were "NOT IMPLEMENTED" and that Stage 47 was
blocked by missing foundations. Every load-bearing claim in it is false today.

The correction is a separate artifact rather than an edit, so the forensic record
of the earlier attempt is preserved: `artifacts/stage47/baseline/baseline-correction.md`.
It corrects 19 claims individually, re-probes the AD CS surface, and restates the
corrected dependency verdict: Kerberos, LDAP, and ACL are present and
live-qualifiable; AD CS templates, MS-WCCE enrollment, and PKINIT are absent
because Stage 47 has not started.

The baseline was **not** deleted or rewritten in place. Silently editing history
would have destroyed the evidence that the earlier attempt's assumptions were
wrong.

## 6. Quality gates

Re-executed on 2026-09-26 against `c07338e`:

```text
go build ./...                    0 errors
go vet ./...                      0 findings
go test -count=1 ./...            0 failures
                                  31 packages ok, 14 with no test files
                                  45 packages total (go list ./...)
FuzzCCacheParse, 60s              17,127,579 execs, 0 crashes
                                  (carried forward from Stage 46h; not re-run
                                  here, and not claimed as re-run)
```

The fuzz figure is cited from `artifacts/stage46h/h6-fuzz-evidence.json` and is
labelled as such. It was not re-executed in this stage, and G3204's
re-execution is recorded in `artifacts/stage47/baseline/entry-gates.md`.

## 7. What was deliberately not done

Each of these was available and would have made this stage look further along.
Each was declined.

```text
No mock / synthetic AD CS CA          G3206 would appear satisfied falsely
No Certipy or hand-built fixture      client-side only; proves no interop
No fabricated credentials or keys      would not authenticate against anything
No change to G3206's definition        the gate is correct; the environment is absent
No VERSION bump                        5.0.0-alpha1 stands; nothing was released
No v5.* tag                           Stage 47 has not produced releasable work
No Stage 47 source written            charter prohibits mock-shaped code
No Aether source file modified        Stage 46i is an assessment stage
No Stage 46h evidence edited          prior certification stands as certified
```

The one prior document that was corrected is
`docs/stage46h-certification.md`, which contained a **factually false** claim
about WSL. The false claim (WSL unusable, no `/bin/bash`) is corrected in place
with an explanation of the actual cause (WSL's default distro is
`docker-desktop`, which has no shell; selecting the distro explicitly works),
rather than being quietly deleted. A certification that silently drops an
inconvenient finding is not a certification.

## 8. Owner handoff — what unblocks Stage 47

```text
1. Provision a throwaway Windows Server 2022 VM with AD DS promoted to a DC
   (realm name aether.test, to match the existing lab and artifacts).

2. Install AD CS on that DC:
     Install-WindowsFeature ADCS-Cert-Authority, ADCS-Web-Enrollment `
       -IncludeManagementTools
   or run scripts/ad-lab/adcs-windows-setup.ps1, which does this plus the
   vulnerable template configuration.

3. Run the script WITHOUT -SafeMode so the 5 vulnerable templates and the ESC6
   CA flag are created. -SafeMode produces a hardened CA and cannot qualify ESC.

4. Capture the script's environment-state output as the CA validation evidence.

5. Publish the domain NTAuthCertificates so the KDC trusts the CA, otherwise
   PKINIT chain validation fails for reasons that are not Aether defects.

6. Provide network reachability from this host to the CA (DCOM/RPC 135 + dynamic
   ports for MS-WCCE) and from a test client to the KDC.

7. Record the CA hostname for two required places:
     - the engagement file's authorized_cas array
     - the --confirm-ca argument on certificate operations
```

## 9. Required artifacts once the environment exists

None of these exist yet. They are listed so the owner's return is unambiguous
about what closes the gate.

```text
artifacts/stage46i/ca-validation.txt              script environment-state output;
                                                  5 templates, ESC6 flag present
artifacts/stage46i/pkinit-mit-success.txt          MIT kinit + klist, PKINIT
                                                  preauth indicator
artifacts/stage46i/pkinit-mit-negative.txt         MIT fails all 3 controls
artifacts/stage46i/pkinit-aether.txt               Aether equivalent ticket
artifacts/stage46i/pkinit-aether-negative.txt      Aether fails identically
artifacts/stage47a/baseline-correction.md          (superseded by
                                                   artifacts/stage47/baseline/
                                                   baseline-correction.md)
docs/stage46i-ca-validation.md                     CA validation section, filled
docs/stage47a-certification.md                     Stage 47A verdict
```

## 10. Residual risk

| Risk | Impact | Mitigation |
|---|---|---|
| G3206 never satisfied | Stage 47 cannot start | Owner handoff is the only path; no local substitute exists |
| Partial provisioning (template without ACE) | enrollment fails, looks like a bug | documented in §4.2; delete template and re-run |
| KDC does not trust the CA | PKINIT fails for non-Aether reasons | documented in §2.4; publish NTAuthCertificates first |
| SAN form mismatch | PKINIT rejected on a correct implementation | recorded in §2.2; Stage 47 must emit the form the KDC accepts |
| Prior baseline still on disk | a reader may take it as current | superseded by `baseline-correction.md`, which states the staleness explicitly |

**Verdict: BLOCKED-WITH-OWNER.** G3206 requires infrastructure this environment
cannot provide. Everything within reach of this stage has been executed,
verified, and recorded; the remainder is explicitly and specifically the owner's
to supply.
