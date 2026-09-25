# Stage 46i — Provisioning Path Attempt

```text
Stage 46i:        BLOCKED-WITH-OWNER
Primary gate:     G3306 (owner AD CS environment)
HEAD:             c07338e
Version:          5.0.0-alpha1 (unchanged — no release decision made)
Tag:              NONE
Stage 47 code:    NOT STARTED (correct; charter prohibits mock-shaped code)
```

## 1. Path evaluation

Every path in the charter's ranking table was tested, not assumed. Each result
is from an executed command.

| Path | Mechanism | Test executed | Result |
|---|---|---|---|
| **A** Azure | `az account show` | `Get-Command az` | **UNAVAILABLE** — CLI not installed |
| **B** AWS | `aws sts get-caller-identity` | `Get-Command aws` | **UNAVAILABLE** — CLI not installed |
| **C** GCP | `gcloud auth list` | `Get-Command gcloud` | **UNAVAILABLE** — CLI not installed |
| **D** Local Hyper-V | `Get-VM` | `Get-VM` | **UNAVAILABLE** — access denied |
| **E** VirtualBox/VMware | `VBoxManage`/`vmrun` | `Get-Command` | **UNAVAILABLE** — not installed |
| **F** Operator handoff | — | — | **SELECTED** |
| **G** Certipy fixtures | detection only | not installed | **Does not clear G3206** — cannot issue certificates |

Corroborating evidence gathered for each rejection:

```text
Elevation:        IsAdministrator = False
                  -> Hyper-V Get-VM denied; cannot be fixed without elevation

Hyper-V feature:  Get-WindowsOptionalFeature -> "The requested operation
                  requires elevation." (cannot even query feature state)

Docker:           OSType = linux / Docker Desktop
                  docker pull mcr.microsoft.com/windows/nanoserver:ltsc2022
                    -> "no matching manifest for linux/amd64/v4"
                  Windows containers are therefore impossible on this host.
                  (Windows containers require a Windows host in process-
                  isolation mode, which in turn requires Hyper-V.)

Cloud creds:      ~/.azure/azureProfile.json    absent
                  ~/.aws/credentials            absent
                  gcloud credentials.db        absent
                  ~/.kube/config                absent
                  AZURE_*/AWS_*/GOOGLE_* env    none set

Other hypervisors: VBoxManage, vmrun, qemu-system-x86_64, multishare
                    all absent
```

**Conclusion: paths A through E are all unavailable, and F is the only
executable option.** There is no partially-viable path to rank higher, and no
route by which a real Windows AD CS CA can be created from this environment
without operator action or elevation.

## 2. FreeIPA considered and rejected

FreeIPA was evaluated as a possible non-Windows CA, because it ships a real CA
(Dogtag/389-ds) and supports PKINIT. It was rejected:

- **It does not implement MS-WCCE.** AD CS enrollment is DCOM/RPC over
  MS-RPCE. FreeIPA has no `ICertRequestD`/`ICertRequestD2` equivalent, so it
  cannot qualify the MS-WCCE half of Batch 3, and it has no
  `pKICertificateTemplate` objects to detect ESC1–ESC15 against. Its templates
  are IPA-specific and not AD CS templates.
- **The WSL distro has no `freeipa` packages** and the install would still
  require root package installation, and a Samba KDC in a separate container
  could not be made to trust an IPA CA without manual, undocumented
  configuration of the Samba KDC's NTAuth equivalent.

Using FreeIPA would clear a *portion* of the PKINIT requirement while leaving
MS-WCCE, enrollment and all ESC detection unqualified. That is the same
substitution of partial coverage for the real thing that the charter prohibits,
so it was not pursued.

## 3. What was repaired while the paths were being tested

Testing the DCOM/CA paths required establishing whether an *independent* Kerberos
reference implementation was available. It was not, and the reason was a
previously-misdiagnosed environment fault.

### 3.1 WSL was misdiagnosed and is now corrected

An earlier Stage 46h note claimed WSL was unusable because it had no
`/bin/bash`. That was wrong. WSL's **default** distribution is `docker-desktop`,
which has no shell; invoking `bash` from PowerShell routes through the WSL
interop shim to that default. Selecting the distro explicitly works:

```text
wsl -d Ubuntu -- uname -a
  Linux DARKPURPLE-01 6.6.114.1-microsoft-standard-WSL2 ... x86_64 GNU/Linux

wsl -d Ubuntu -- ls -la /bin/bash
  -rwxr-xr-x 1 root root 1540520 Feb 13 2026 /bin/bash

/etc/os-release: Ubuntu 26.04.1 LTS
```

The certification has been corrected in place
(`docs/stage46h-certification.md`, known-limitations section) rather than the
false claim being silently deleted.

### 3.2 The interrupted `krb5-user` install was a real fault, now fixed

The related claim that `krb5-user` installation "hung holding the dpkg lock" was
**substantively correct**. The package state proved it:

```text
BEFORE
  iF  krb5-config   2.7ubuntu1        all
  iU  krb5-user     1.22.1-2ubuntu4.1  amd64     <- unpacked, NOT configured
  ii  krb5-locales  1.22.1-2ubuntu4.1  all
```

Root cause: `krb5-user`'s postinst raises a **whiptail prompt for the default
realm**. With no TTY attached the prompt never resolves and `dpkg` blocks
indefinitely — which is exactly what both the original install and a naive
`dpkg --configure -a` did. Seeding the debconf answers removes the prompt:

```sh
printf '%s\n' \
  'krb5-user krb5-common/default_realm string AETHER.TEST' \
  'krb5-user krb5-common/admin_server string' \
  'krb5-user realm/sync_password boolean false' \
  'krb5-config strings/default_realm string AETHER.TEST' \
  'krb5-config strings/admin_server string' \
  | sudo debconf-set-selections
DEBIAN_FRONTEND=noninteractive sudo dpkg --configure -a
```

```text
AFTER
  ii  krb5-config   2.7ubuntu1        all
  ii  krb5-user     1.22.1-2ubuntu4.1  amd64
  kinit /usr/bin/kinit   klist /usr/bin/klist   kvno /usr/bin/kvno
  kdestroy /usr/bin/kdestroy
```

### 3.3 `krb5-pkinit` installed — the independent PKINIT reference

```text
dpkg: krb5-pkinit 1.22.1-2ubuntu4.1
/usr/lib/x86_64-linux-gnu/krb5/plugins/preauth/pkinit.so   128056 bytes
/usr/lib/x86_64-linux-gnu/krb5/plugins/preauth/spake.so
OpenSSL 3.5.5 27 Jan 2026
```

This satisfies the charter's requirement that PKINIT be validated against an
implementation other than Aether. A PKINIT-shaped client certificate was minted
and verified to prove the local toolchain can produce the material PKINIT needs:

```text
subject=CN=pkinit-probe
X509v3 Extended Key Usage:  TLS Web Client Authentication
X509v3 Subject Alternative Name: othername: UPN:pkinit-probe@PROBE.TEST
Signature Algorithm: sha256WithRSAEncryption
private key matches certificate: MATCH
```

Note: the `id-pkinit-san` (`otherName:1.3.6.1.5.2.2;prn:...`) form is **not**
supported by this OpenSSL 3.5.5 build; the Microsoft UPN otherName form
(`1.3.6.1.4.1.311.20.2.3`) was used instead. This is recorded because Stage 47's
CMS/AuthPack work must emit whichever form the target KDC accepts, and the two
are not interchangeable.

**What this does and does not prove.** It proves the *local* PKINIT toolchain is
installed, functional and able to mint the required client identity. It proves
nothing about any PKINIT exchange: `kinit` stops at `Cannot contact any KDC`,
which happens before PKINIT is reached. A PKINIT exchange requires a KDC that
trusts a CA, which is precisely the open blocker.

## 4. Provisioning script static review

`scripts/ad-lab/adcs-windows-setup.ps1` could not be executed here (no Windows
domain, no elevation). It was therefore reviewed statically and via the AST.

```text
G3303  syntax errors        0
        functions            7
        parameters           3  (DomainFQDN:String,
                                 DomainAdminPassword:String,
                                 SafeMode:SwitchParameter)
        literal secrets      none  (no password or key literal anywhere;
                                 the only matches for a pass/key scan were the
                                 attribute name pKIExtendedKeyUsage)
        external commands    Install-WindowsFeature, Install-ADCSertificationAuthority,
                             New-ADCertificateTemplate, Register-ADCSecurityTemplate,
                             Add-ADObjectAce, Set-ADObject, Set-ItemProperty,
                             Restart-Service, Get/Set-WindowsFeature, AD cmdlets
```

### 4.1 What `-SafeMode` actually does

Determined by reading the control flow, as required before it is relied upon:

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

`-SafeMode` performs **real provisioning** — it installs the roles and creates a
real Enterprise CA. It is not a simulation and must not be described as one. What
it omits is exclusively the intentionally-vulnerable configuration. A CA created
with `-SafeMode` is hardened and is **not** suitable for ESC qualification; it
would be suitable for PKINIT trust-chain work only.

### 4.2 Idempotency assessment (G3335, by inspection)

The script is idempotent in its three mutating phases:

- `Install-CertificateAuthority` returns early if `Get-ADCSertificationAuthority`
  already returns a CA.
- `Enable-Esc6CaFlag` returns early when the current flag value already equals
  the desired value.
- `New-VulnerableTemplate` returns early when the template `cn` already exists,
  and the Enroll ACE grant happens *after* that early return, so a second run
  does not duplicate the ACE.

**One partial-state weakness, disclosed rather than fixed blind:** because the
early return precedes the ACE grant, a run that created the template but failed
before the ACE was added will, on retry, skip the ACE permanently. If a
provisioning run reports a template as present but enrollment still fails with
access denied, delete the template and re-run. This cannot be tested without a
domain, so it is documented as a known limitation of the script rather than
claimed as verified.

### 4.3 Two observations worth recording

- `ADCS-Web-Enrollment` is installed but never configured or used. Web
  enrollment is the ESC8 attack surface, so its presence is harmless for this
  stage but will matter when ESC8 detection is implemented. It is noted so its
  presence is not later mistaken for ESC8 having been qualified.
- The script requires the PSPKI module and fails fast without it, because base
  AD cmdlets cannot *publish* a template to a CA. This is correct behaviour: the
  alternative would be a half-configured CA.

## 5. Path selected

**Path F — operator handoff.** No provisioning path is executable from this
environment. The operator remedy, with exact commands, is in
`docs/stage46i-certification.md`.

No mock CA, synthetic CA, or partial substitute was created. No Stage 47 code was
written. No Aether source file was modified.
