# B4 Waiver — EV Authenticode Signing (formalized Stage 33b)

> Provenance note: Stage 13's final report recorded B4 as "WAIVED
> (2027-03-31) — formal waiver filed," but no waiver file existed in the
> repository. This file was created in Stage 33b to make the recorded
> disposition real. No prior waiver file existed; none was deleted.

## Waived requirement

Windows release binaries are signed with an EV Authenticode certificate.

## Risk

Unsigned Windows executables trigger SmartScreen/AV warnings and cannot
be verified to a publisher identity; a malicious re-hosted binary is
harder for an operator to distinguish from an official one.

## Impact

Distribution-trust only. Binary behavior, checksums, and SBOMs are
unaffected. Compensating controls below give operators an integrity
verification path that does not depend on Authenticode.

## Compensating controls

1. `checksums.txt` (SHA-256) per release — verified in the Stage 30
   goreleaser snapshot run.
2. SPDX-2.3 SBOM per archive.
3. Keyless cosign signing of checksums/archives/SBOMs implemented in
   `.github/workflows/release.yml` (Stage 32; execution pending CI
   revival — F-32-3). When executed, artifact-level signatures bound to
   the repository/workflow OIDC identity become available.
4. Tag → commit → source traceability via git (`v4.2.0-rc1` → `bb56cf3`).

## Expiry

**2027-03-31.** At expiry: either an EV certificate is procured and the
`windows-sign` workflow executes with the `AUTHENTICODE_CERT` secret, or
the waiver must be re-approved explicitly with a new expiry and rationale.

## Owner

Repository owner (Debajyoti Haldar).

## Approval

Accepted as release policy for the Production-Limited / controlled
publication scope (Stages 28–33 posture: public GA remains blocked
independently of this waiver). Recorded 2026-09-18, Stage 33b.
