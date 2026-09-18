# Waiver LIVE-1 - Live Entra / IMDS / IdP Validation Absent

| Field | Value |
| --- | --- |
| Waiver ID | LIVE-1 |
| Owner | Repository owner (Debajyoti Haldar) |
| Risk | All identity-provider validation (live Entra ID tenant, IMDS endpoint from an authorized cloud-hosted VM, live IdP round-trips) is mock-verified only; real-tenant behavior, rate limits, header normalization, and endpoint drift are not exercised by any automated gate. |
| Impact | Protocol and provider code paths are proven against local mocks and fixture matrices; first live contact may expose integration defects that mocks cannot represent. Claims to "live IdP validated" are NOT made; posture is mock-verified. |
| Compensating controls | (1) Mock success + negative-path matrices for Okta/GitLab/K8s/IMDS/OCSP/CRL (testdata mocks + Stage 28-32 qualification); (2) offline provider error-class matrix (401/403/400/404/429/500/503/301/malformed-schema/TLS-downgrade - re-proven Stage 40 Campaign 4.5); (3) LIVE-1 waiver carried as register entry with owner through Stages 32-39; (4) 21-target fuzz suite over protocol parsers (SAML/WS-Trust/MSOAP-X/token/IMDS - 0 crashes, re-proven Stage 40). |
| Expiry | 2027-06-30 |
| Approval | Approved by repository owner as an access-dependent Production-Limited posture item (no authorized Entra lab tenant exists; `ENTRA_LAB_TENANT` and `AZURE_DEV_VM` unset - re-verified raw at Stage 39). Not valid for GA publication. |

## History

- Registered as LIVE-1 register entry at Stage 32; carried inline in the Stage 35b closure matrix waiver table (stage35b-false-cert-closure-matrix.md section waiver-verification; stage35-certification.md:22); re-verified Stage 39 (no standalone file).
- Promoted to standalone waiver file in Stage 40 (this document), so the on-disk waiver-file rule applies uniformly (B4/B5 model).

## Closure procedure

Provision an authorized Entra ID lab tenant (set `ENTRA_LAB_TENANT`) and an authorized cloud-hosted VM for IMDS (set `AZURE_DEV_VM`), run the live-qualification matrix (Entra auth round-trip, IMDS identity token fetch + parse, one live IdP validate per provider), and record raw outputs. Close the waiver in the next release-candidate cycle, or re-approve with updated expiry.
