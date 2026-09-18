# Waiver F-30-3 - SLSA Provenance Absent

| Field | Value |
| --- | --- |
| Waiver ID | F-30-3 |
| Owner | Repository owner (Debajyoti Haldar) |
| Risk | No SLSA provenance is generated for release artifacts; supply-chain attestation (build source, builder identity, build recipe) is absent for shipped binaries. |
| Impact | A consumer cannot verify that a released artifact was built from the tagged source by the documented pipeline. Artifact provenance currently rests on tag traceability and pipeline configuration rather than signed attestation. |
| Compensating controls | (1) Release artifacts carry checksums published alongside them; (2) SPDX SBOM generated for each release; (3) annotated-tag traceability discipline (tag content proven via `git show <tag>:<file>` at every stage freeze since Stage 28); (4) release pipeline (release.yml/goreleaser) pinned to documented steps in-repo. |
| Expiry | 2027-03-31 |
| Approval | Approved by repository owner as a Production-Limited posture item (Stage 32/33 posture; re-affirmed Stage 39). Not valid for GA publication. |

## History

- Registered as register entry F-30-3 at Stage 32 (stage32-certification.md:18, stage33-certification.md:44); re-verified Stage 39 (register-verified, no standalone file).
- Promoted to standalone waiver file in Stage 40 (this document), so the on-disk waiver-file rule applies uniformly (B4/B5 model).

## Closure procedure

Generate SLSA Build L3 provenance for release artifacts (SLSA GitHub generator or goreleaser provenance integration), publish signed provenance alongside checksums + SBOM, and re-run the artifact-trust report with provenance checks green. Close the waiver in the next release-candidate cycle before expiry, or re-approve with updated expiry.
