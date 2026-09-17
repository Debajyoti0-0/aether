# B5 Waiver — Live Azure Key Vault Validation (formalized Stage 33b)

> Provenance note: Stage 13's B5 reassessment (docs/stage13-phase3-b5-
> reassessment.md, line 189) instructed "File docs/stage13-b5-waiver.md
> with… expiry ≤ 2027-03-31", but no waiver file existed in the
> repository. This file was created in Stage 33b to complete that
> instruction. Environment check at filing time:
> `AZURE_KEYVAULT_URI` not configured — live round-trip not executable.

## Waived requirement

Live Azure Key Vault round-trip validation of the Azure KV key provider
(generate EC-P256 → sign → verify → rotate → sign → verify → revoke old
→ confirm rejection) against a real vault.

## Risk

The Azure KV provider is verified by unit/integration tests against
emulated and contract-level surfaces only; live tenant behavior
(certificate/secret API versions, Key Vault throttling, Managed Identity
token quirks, regional endpoints) is unexercised.

## Impact

Capability correctness for the Azure KV backend in production tenants is
unproven. All other key-provider paths and the local round-trip contract
are tested. Fail-closed behavior is verified: the provider refuses
operation on contract violations.

## Compensating controls

1. Unit + integration coverage of the KV provider contract (fix verified
   in Stage 13: dedupe `extractVersionFromKID`, fail-closed Ed25519
   contract).
2. Capability-aware provider interface — the CLI reports provider
   capability state instead of assuming.
3. `AZURE_KEYVAULT_URI`-gated live round-trip procedure documented in
   this waiver: when an authorized vault exists, run the provider
   round-trip (generate → sign → verify → rotate → revoke → confirm
   rejection) and close this waiver.

## Expiry

**2027-03-31.**

## Owner

Repository owner (Debajyoti Haldar).

## Approval

Accepted for the Production-Limited / controlled publication scope.
Recorded 2026-09-18, Stage 33b.
