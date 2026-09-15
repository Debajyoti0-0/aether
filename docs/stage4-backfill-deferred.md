# Stage 4 Backfill — Deferred Register (B4-G15 / WS8)

Every storage/spine risk left open by this backfill. No item is closed
by this document; each requires its own executable evidence.

| ID | Description | Owner | Risk | Target stage | Expiry |
|---|---|---|---|---|---|
| B4-DEF-01 | bbolt data pages carry no per-page checksums: a bit flip in **unallocated** vault space is not detected (committed records remain intact and verifiable — asserted by `TestTornWriteRejection/bitflip_unallocated_space_records_intact`). Hardening: store a whole-file digest (e.g. SHA-256 in `meta` bucket, updated per Sync) and verify on open. | aether-maintainers | medium (silent free-space corruption undetectable) | Stage 10 | 2027-06-30 |
| B4-DEF-02 | A truncated vault file can fault the opening process (bbolt mmap) instead of returning a typed error; rejection is fail-closed at process level (asserted with crash-isolated probes) but not a clean `ErrVault` typed failure. Hardening: pre-open size/page-count sanity check in `OpenVault`. | aether-maintainers | low | Stage 10 | 2027-06-30 |
| B4-DEF-03 | Operator PKI revocation is a name-list checked at connection time; OCSP/CRL distribution and fingerprint-level revocation are not implemented (code comments defer this to "Stage 4" scope — explicitly carried instead). | aether-maintainers | medium | Stage 10 | 2027-06-30 |
| B4-DEF-04 | Endurance FD-count assertion is platform-limited: Windows cannot enumerate FDs from pure Go; the goroutine/memory/chain assertions ran locally, FD bounds must be asserted by the CI ubuntu job. | CI maintainers | low | next CI cycle | 2027-06-30 |
| B4-DEF-05 | Race detector requires a C toolchain (gcc absent locally); race evidence remains CI-authoritative (ubuntu + windows matrix). golangci-lint and govulncheck were run locally and passed. | CI maintainers | low | next CI cycle | 2027-06-30 |
| B4-DEF-06 | Audit-chain signing key is stored in the vault `meta` bucket (base64 Ed25519 seed) — same trust domain as the data it protects. The Azure KV provider landed for HSM/KMS custody but `GetSigningKey` is fail-closed (Azure KV exports no private material; Ed25519 unsupported); wiring server-side signing (ECDSA or KV `Sign`) into the audit chain is open. | aether-maintainers | medium | Stage 10 | 2027-06-30 |
| B4-DEF-07 | Vault `ErrVaultLocked` wraps `bolt.ErrTimeout` with a 2 s wait; a "wait longer / retry queue" policy for multi-process contention is not implemented (losers fail fast by design). | aether-maintainers | low | Stage 10 | 2027-06-30 |

Waiver rule: any gate closed by waiver must cite owner, risk, impact,
expiry ≤ 2027-06-30, approval, and a public release-note line. None of
the items above were used to waive a failing gate in this backfill.
