# Stage 38 Findings Register (Campaigns 4-6, Continuation)

Baseline: master @ `5cc7756`; tag build `v4.2.0-rc1` @ `bb56cf3`.
Two-column rule: on-tag = reproduced on the `bb56cf3` build; on-master =
status on the `5cc7756` build. The tag predates all four Stage 34 fixes
(`ee76943` is not an ancestor of `bb56cf3`); the four mapped findings
reproduce on-tag with the master column REMEDIATED at Stage 35.

Status vocabulary: OPEN / REMEDIATED / NEW / NEW-INFO. Severity scale
P0-P3 per the charter.

## Mapped findings (Stage 34 lineage, four-fix reproduction)

| ID | Sev | Finding | On-tag `bb56cf3` | On-master `5cc7756` |
| --- | --- | --- | --- | --- |
| F-34-1 | P2 | Malformed/unreadable/binary config silently ignored | OPEN - reproduced: exit 0, no diagnostic, defaults in effect (Campaign 4.1) | REMEDIATED at Stage 35 (`ee76943`): visible `Warning: config file ignored` for all four classes |
| F-34-2 | P3 | Invalid `--log-level` silently accepted | OPEN - reproduced: `{"log-level": 42}` accepted silently (4.1) | REMEDIATED: rejected with valid-value list, falls back to `info` |
| F-34-3 | P2 | Corrupt vault escapes raw storage-engine panic text | OPEN - reproduced: deep truncation -> `panic: invalid freelist page: 0...` exit 2 (4.4) | REMEDIATED at Stage 35: typed `vault file is corrupt or truncated`, exit 1, 0 panic occurrences |
| F-34-4 | P2 | Workspace vault substitution undetected | OPEN - reproduced: `subsrc` vault over `tamperts` -> `VERIFIED (3 entries)` exit 0 (4.4) | REMEDIATED at Stage 35: typed `vault workspace identity mismatch`, exit 1 |

## New findings (this stage)

| ID | Sev | Finding | On-tag `bb56cf3` | On-master `5cc7756` | Evidence |
| --- | --- | --- | --- | --- | --- |
| F-40-1 | P2 | Provider runCommand transport mismatch: client advertises ALPN h2 but the Go transport runs HTTP/1.1 over the custom dialer; real h2-capable endpoints (management.azure.com) reply with an h2 SETTINGS frame that the h1 parser rejects: `net/http: HTTP/1.x transport connection broken: malformed HTTP response "\x00\x00\x12\x04..."`. Local/h1-only mocks and all qualification gates are unaffected. Governed intent, audit, and rollback records still commit before the provider call; only the provider call fails. | NEW - reproduced | NEW - reproduced (same build lineage) | Campaign 4.3: governed exec against management.azure.com, raw response bytes captured |
| F-40-2 | P3 | Invalid `AETHER_CONFIG_DIR` (e.g. `NUL:`) yields a silent empty workspace view (exit 0, `No workspaces...`), no diagnostic on either build. | NEW | NEW | Campaign 6.6a |
| F-40-3 | P3 | Environment hygiene: unit/integration test runs leave residue - ~200 test-named workspaces (e.g. `EventSigWS-Test...`, `RollbackWS-...`) accumulate in the live config root because tests do not isolate `AETHER_CONFIG_DIR`; `go test` also writes `ad_sampledata/` fixtures into the repo CWD (created during this stage's suite run; left in place awaiting operator approval for removal). | PRE-EXISTING | PRE-EXISTING | Campaign 4.1 (workspace list residue); git status during this stage |
| F-40-4 | Info | Plugin package download has no size cap; a hostile index could stream unbounded bytes before the checksum verdict. Content integrity is gated by SHA-256 (badsum rejected raw); only size is unbounded. | NEW | NEW | Campaign 4.6 |

## Survived-attacks matrix (documented per the charter)

| Attack | Result |
| --- | --- |
| Workspace name traversal (9 vectors, Campaign 4.2) | all typed-rejected, exit 1 - PASS |
| Credential leakage scan (Campaign 4.3) | 0 hits (passphrase, argv token, key material) - PASS |
| Audit chain byte-flip in live record content (4.4) | DETECTED exit 1 `TAMPERED` on both builds - PASS |
| Provider error-class matrix + 301/503/malformed/TLS-downgrade extensions (4.5) | all exit 1 mapped; 200 never confused with success - PASS |
| Plugin checksum enforcement (4.6) | tampered package rejected with named got/want raw - PASS |
| 4-way concurrent audit append race (4.7) | all exit 0, chain VERIFIED - PASS |
| 10 MB config / 20k-deep JSON / 1 MB arg / symlink loop (4.8) | bounded, no crash - PASS |
| Rollback determinism + undo fail-safe + replay dry-run idempotency (Campaign 5) | byte-identical outputs, honest retained cleanup, no journal pollution - PASS |
| Empty/surrogate passphrase, read-only vault/salt, interrupt kill (Campaign 6) | typed errors, VERIFIED chain, lock released, no panic - PASS |

## Counts

Mapped lineage: 4 findings (2 x P2, 1 x P2, 1 x P3) - all OPEN on-tag, all
REMEDIATED on-master.
New this stage: 4 findings (1 x P2, 2 x P3, 1 x Info) - NEW on both
builds; F-40-1 carries an observation-window event.
New P0: 0. New P1: 0.
