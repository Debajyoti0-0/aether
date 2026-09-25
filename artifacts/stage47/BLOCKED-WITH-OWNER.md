# Stage 47 — BLOCKED-WITH-OWNER

```text
Stage 47 status:     BLOCKED-WITH-OWNER
Version in:          5.0.0-alpha1
Version target:      5.0.0-alpha3   (NOT produced)
Tag:                 NONE
Working tree:        clean at a448932 + this commit

BLOCKING GATE:       G3206 — CA test harness unavailable
OWNER:               stage operator (requires Windows Server 2022 eval VM)
```

## Why this stage is blocked

Stage 47 hard rule 8 and the exit criteria state:

> **CA test harness is a hard gate.** Without a working CA, ESC1 abuse cannot be
> verified. If no CA harness is available, exit `BLOCKED-WITH-OWNER` with the
> specific remedy.

> **Hard rule:** if the CA harness is unavailable, exit `BLOCKED-WITH-OWNER` with
> the specific remedy. Do not substitute static verification for live CA testing.

No CA exists in any available environment. Verified directly, not inferred
(`artifacts/stage47/baseline/entry-gates.md`, `adcs-surface-probe.txt`):

- No Windows domain with AD CS. Hyper-V is present but `Get-VM` is denied
  without elevation, and no VM is reachable.
- Certipy is not installed. Installing it would not clear this gate: it cannot
  issue certificates, so enrollment, ESC abuse and PKINIT remain unexercisable.
- The Samba4 lab provisions an **empty** `CN=Public Key Services` container:
  zero `pKICertificateTemplate` objects, zero `certificationAuthority` objects,
  and the `pKICertificateTemplate` schema class is absent from the schema.

## Remedy (owner action)

Provision a Windows Server 2022 evaluation domain with the AD CS role and the
vulnerable templates Stage 47 needs to qualify against:

```powershell
# On the evaluation domain controller, as Administrator.
Install-Module PSPKI -Scope AllUsers      # required: template publication

.\scripts\ad-lab\adcs-windows-setup.ps1 `
    -DomainFQDN adcs-lab.test `
    -DomainAdminPassword 'Passw0rd123!'
```

The script installs the Enterprise CA, sets `EDITF_ATTRIBUTESUBJECTALTNAME2`
(`0x00040000`) for the CA-level ESC6 condition, and publishes five deliberately
vulnerable templates: `ESC1-Test`, `ESC6-Test`, `ESC15-Test` (schema version 1),
`ESC2-Test` (Any Purpose EKU) and `ESC3-Test` (Certificate Request Agent EKU).
It is idempotent, has a `-SafeMode` switch that provisions a hardened CA with no
weaknesses, and validates with the PowerShell AST parser: 0 syntax errors,
7 functions, 3 parameters.

Two conditions the script cannot satisfy on its own:

1. **The KDC must trust the issuing CA.** PKINIT validates the client
   certificate's chain against a trust anchor the KDC accepts. If the KDC does
   not trust the CA, PKINIT fails chain validation — that is a harness fault and
   must not be recorded as an Aether defect.
2. **Domain isolation.** The templates are intentionally exploitable. Use a
   throwaway evaluation domain containing no real identities or data.

## What is NOT blocked, and what must not be claimed

Because the blocker is confined to live CA qualification, this much is provable
without a CA and can proceed once one exists:

| Area | Status without a CA |
|---|---|
| LDAP template/CA enumeration against the Samba lab | **Live-exercisable now.** `CN=Public Key Services` exists and the base DN resolves, so base/scope/filter construction and correct empty-result handling can be proven. |
| MS-WCCE NDR marshalling, request/response construction | Protocol-correctness only, against a mock responder. **Not** interoperability. |
| CMS SignedData, PKINIT AuthPack/PA-PK-AS-REQ/REP, DH/ECDH, KDF | Protocol-correctness only. |
| ESC1–15 detection | **Not** validatable against real template data. A zero-finding result on an unpopulated tree is indistinguishable from a correct detector. |
| ESC1/ESC6/ESC15 abuse, certificate issuance | Impossible: no CA. |
| PKINIT to TGT | Impossible: no CA, no KDC trust anchor. |
| Spine, capability authz, engagement boundary, `--confirm-ca`, audit, evidence | Fully implementable and testable without a CA. |

Explicitly prohibited claims while blocked: that ESC detection or ESC abuse is
"live-qualified", or that MS-WCCE/PKINIT "interoperates". A mock responder and a
synthetic LDAP fixture share no code, no authentication path and no issuance
policy engine with Windows AD CS.

## Resolved before this point

The three carry-forward items from Stage 46h closure are complete:

1. **Stage 46h committed.** `a448932 feat(ad): deliver AD Kerberos + LDAP
   expansion and close Stage 46h defects` — 230 files, 22,957 insertions. This
   included the entire previously-uncommitted AD implementation, not just the
   Stage 46h delta. `.kilo/` was excluded because it contains a full nested git
   worktree.
2. **Fuzz claim verified, not dropped.** `FuzzCCacheParse` re-executed:
   17,127,579 executions in a 60s budget, 0 crashes, full provenance in
   `artifacts/stage46h/h6-fuzz-evidence.json`. The certification cites the
   measured figure and no longer repeats the unverifiable 7.4M claim.
3. **Lab limitation scope documented.** `docs/stage46h-certification.md` §7.1 now
   states, with evidence, that this lab can live-qualify Batches 1 and 2 but no
   part of Batch 3.

Entry gate result: G3201, G3202, G3203, G3204, G3205, G3207 **PASS**;
**G3206 FAIL** (this blocker). Gates G3208–G3280 are not reached.
