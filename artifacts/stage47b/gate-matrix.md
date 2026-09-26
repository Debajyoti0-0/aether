# Stage 47B — Gate Matrix (G47B-01 … G47B-50)

```text
HEAD:   ebb8c8bfb64f52bc0205351366495088c3772e63
TREE:   CLEAN
DATE:   2026-09-26

RESULT LEGEND
  PASS          executed, criterion met, evidence captured
  PARTIAL       part executed and met; remainder impossible here (reason given)
  NOT SATISFIED the criterion is a real requirement and it is NOT met
  NOT PERFORMED the test could not be executed at all (environment absent)
  NOT REACHED   depends on a hard gate that failed; never attempted
  DENIED        an explicit authorization decision, recorded
```

## Hard-dependency gates (§47)

`G47B-09 G47B-10 G47B-12 G47B-14 G47B-16 G47B-18 G47B-20 G47B-21`

All eight **FAIL / NOT SATISFIED**. Per §47, Stage 47 stays `BLOCKED`.

---

## Repository integrity and prior evidence (G47B-01 … G47B-08)

| Gate | Requirement | Result | Evidence |
|---|---|---|---|
| G47B-01 | HEAD integrity | **PASS** | HEAD `ebb8c8b…` matches briefed `ebb8c8b`; `git describe` → `ebb8c8b` |
| G47B-02 | Clean tree | **PASS** | `git status --porcelain` → empty |
| G47B-03 | Stage 46i verification | **PASS** | all 5 documents present, sizes + SHA-256 prefixes recorded; none altered |
| G47B-04 | Historical correction verification | **PASS** | the 7 files dated 2026-09-21 are untouched; `baseline-correction.md` present and consistent with the tree |
| G47B-05 | Provisioning script syntax | **PASS** | PowerShell AST: **0 parse errors**, 1061 tokens, **7** functions, **3** parameters |
| G47B-06 | Provisioning safety | **PARTIAL** | static: **0** password/private-key literals; 11 distinct AD/CA mutating cmdlets enumerated. **Live safety NOT PERFORMED** — no Windows domain, session not elevated |
| G47B-07 | SafeMode | **PARTIAL** | control flow read from the AST: `-SafeMode` still installs roles **and provisions a real Enterprise CA**; it skips only `Enable-Esc6CaFlag` + the 5 templates + the Enroll ACE. **Live NOT PERFORMED** |
| G47B-08 | Idempotency | **PARTIAL** | static: early-return guards confirmed in all 3 mutating phases. **Live partial-state tests NOT PERFORMED** (§9 requires creating a controlled partial state on a real domain). Known ACE-order weakness re-confirmed, not fixed |

Evidence: `provisioning/script-static-verify.txt`, `baseline/baseline.md`.

## Environment gates — the G3206 block (G47B-09 … G47B-26)

These are the gates that define G3206. Every one requires a real Windows Server
2022 AD DS + AD CS environment.

| Gate | Requirement | Result | Evidence / reason |
|---|---|---|---|
| G47B-09 | **AD DS** | **NOT SATISFIED** | no Windows domain exists. `Get-VM` → *"You do not have the required permission"*; `Get-WindowsOptionalFeature` → *"requires elevation"*. A Samba4 DC is not Windows AD DS |
| G47B-10 | **DNS** | **NOT SATISFIED** | no Windows DC to host DNS. Samba's internal DNS is qualified in Stage 45/46 but is not the Windows DNS this gate requires |
| G47B-11 | LDAP | **NOT SATISFIED** | Aether's LDAP client works live against the Samba DC (Stage 46h, still passing), but there is no Windows LDAP to qualify against |
| G47B-12 | **KDC** | **NOT SATISFIED** | no Windows KDC. MIT Kerberos tooling is installed and interop-certified against Samba, but that is not a Windows KDC |
| G47B-13 | Public Key Services | **NOT SATISFIED** | `CN=Public Key Services` **exists but is empty** (0 children). `CN=Enrollment Services` and `CN=Certificate Templates` also exist, also empty |
| G47B-14 | **Enterprise CA** | **NOT SATISFIED** | **0** `certificationAuthority` objects. `Install-ADCSertificationAuthority` has never been executed — no Windows domain |
| G47B-15 | CA configuration | **NOT SATISFIED** | no CA, therefore no `CertSvc\Configuration\<CAName>\Flags` registry key to read. `EDITF_ATTRIBUTESUBJECTALTNAME2` is **not observed**, only *intended* |
| G47B-16 | **Certificate templates** | **NOT SATISFIED** | **0** `pKICertificateTemplate` objects. `ESC1-Test`, `ESC2-Test`, `ESC3-Test`, `ESC6-Test`, `ESC15-Test` all absent |
| G47B-17 | Template ACLs | **NOT SATISFIED** | no templates exist to carry security descriptors |
| G47B-18 | **Enrollment** | **NOT SATISFIED** | no CA to enrol against. No `ICertRequest`/MS-WCCE endpoint exists anywhere reachable |
| G47B-19 | Certificate validation | **NOT SATISFIED** | no CA certificate, so no chain to validate |
| G47B-20 | **KDC trust** | **NOT SATISFIED** | no CA certificate exists to publish into `NTAuthCertificates`. Classified as **ENVIRONMENT CONFIGURATION DEFECT** (absent precondition), **not** an Aether protocol defect |
| G47B-21 | **Independent PKINIT** | **NOT SATISFIED** | independent implementation **is** installed and confirmed (`krb5-pkinit 1.22.1`, plugin 128056 bytes, OpenSSL 3.5.5) and a PKINIT-shaped certificate was minted and verified — but the exchange cannot be attempted: `kinit` → `Cannot contact any KDC`. A minted certificate is **not** a PKINIT exchange |
| G47B-22 | Negative PKINIT #1 (wrong identity) | **NOT SATISFIED** | no issued certificate to present as Identity A/B |
| G47B-23 | Negative PKINIT #2 (untrusted CA) | **NOT SATISFIED** | no trusted CA to contrast against |
| G47B-24 | Negative PKINIT #3 (bad cert properties) | **NOT SATISFIED** | no issued certificate lacking required properties |
| G47B-25 | Environment reproducibility | **NOT SATISFIED** | reproducibility of the *qualification environment* cannot be demonstrated for an environment that was never created. `scripts/ad-lab/adcs-windows-setup.ps1` is the reproducibility artifact and it is **untested live** |
| G47B-26 | Environment evidence | **PARTIAL** | evidence **that the environment is absent** is complete and reproducible (`adcs/surface-reprobe.txt`, `provisioning/path-reprobe.txt`, `provisioning/windows-container-probe.txt`). Evidence **of a qualified environment** does not exist and was not fabricated |

Evidence: `adcs/surface-reprobe.txt`, `provisioning/path-reprobe.txt`,
`provisioning/windows-container-probe.txt`.

## Authorization (G47B-27)

| Gate | Requirement | Result | Reason |
|---|---|---|---|
| G47B-27 | Implementation authorization | **DENIED** | §22 permits the `BLOCKED-WITH-OWNER` → `IMPLEMENTATION-AUTHORIZED` transition only after G3206-A…Q pass. G3206-F/G/H/I/J/K/L/M/N/O do not. The transition is **not** taken. Stage 47 remains `BLOCKED` |

## Stage 47 implementation gates (G47B-28 … G47B-50)

Not reached. These describe AD CS + PKINIT implementation, which §22 and §23
prohibit before G3206 closes. No Stage 47 source file was created.

| Gate | Requirement | Result |
|---|---|---|
| G47B-28 | Protocol implementation (MS-WCCE) | NOT REACHED |
| G47B-29 | ASN.1 / DER | NOT REACHED |
| G47B-30 | X.509 | NOT REACHED |
| G47B-31 | CMS | NOT REACHED |
| G47B-32 | PKINIT | NOT REACHED |
| G47B-33 | Kerberos (PKINIT path) | NOT REACHED |
| G47B-34 | ccache | NOT REACHED |
| G47B-35 | CLI | NOT REACHED |
| G47B-36 | Flags | NOT REACHED |
| G47B-37 | Positional arguments | NOT REACHED |
| G47B-38 | Help / unknown flags | NOT REACHED |
| G47B-39 | JSON | NOT REACHED |
| G47B-40 | Human output | NOT REACHED |
| G47B-41 | Security | NOT REACHED |
| G47B-42 | Fuzz | NOT REACHED |
| G47B-43 | Race | **PASS** — executed, `go test -race -count=1 ./...` exit 0, 31 ok, 0 FAIL, **0 DATA RACE** reports (`race/go-test-race.txt`) |
| G47B-44 | Static analysis | **PARTIAL** — `go vet ./...` exit 0, 0 findings. `golangci-lint`, `staticcheck` and `govulncheck` are **not installed** on this host, so errcheck/unused/gosimple and dependency CVE reachability were **not** run and are recorded as an open gap, not a pass (§39 skipped-tool disclosure in `docs/stage47b-certification.md` §5.1) |
| G47B-45 | Regression | **PASS** — 31 packages ok / 0 FAIL / 14 no-test; Stage 46h AD packages included; no baseline behaviour changed (Stage 47B authored no source) |
| G47B-46 | Clean checkout | **PASS** — HEAD is committed, tree clean, no uncommitted source; Stage 47B changed documentation/evidence only |
| G47B-47 | Reproducibility | **PARTIAL** — build/vet/test/race reproduce exactly from this checkout. Full live qualification does not reproduce because it was never performed |
| G47B-48 | Evidence integrity | **PASS** — every result above cites a file under `artifacts/stage47b/`; no expected configuration value was copied forward as an observation |
| G47B-49 | Documentation | **PASS** — see `docs/stage47b-certification.md`; Stage 46i documents preserved unedited |
| G47B-50 | Production readiness | **NOT READY** — G3206 open; 10 of 50 gates not satisfied, 4 partial, 21 not reached |

## Tally

```text
PASS           10    01 02 03 04 05 43 45 46 48 49
PARTIAL         6    06 07 08 26 44 47
NOT SATISFIED  17    09 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25
DENIED          1    27
NOT REACHED    16    28 29 30 31 32 33 34 35 36 37 38 39 40 41 42 50
                --
TOTAL          50
```

The single fact that matters: **no gate that requires a real Windows AD CS
environment passed, and none was made to pass.**
