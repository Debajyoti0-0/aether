# Stage 34 Forensic Review — Security and Reliability Findings

Every finding reproduced with raw evidence. Severity per the charter's
P0–P3 scale. No suspected-without-reproduction items are marked CONFIRMED.

## Confirmed findings

### F-34-1 — Malformed config silently ignored (P2, FIXED)

```text
COMPONENT : internal/cli root initConfig (root.go:64-76 pre-fix)
REPRO     : printf '{"this is": not valid json' > %APPDATA%/aether/aether.json
            aether doctor  → exit 0, no diagnostic, defaults in effect
EXPECTED  : visible warning (config present but unusable)
ACTUAL    : silent fallback to defaults — fail-open configuration: a
            security-relevant setting with a typo silently reverts
WHY MISSED: no test exercised an unreadable config; viper errors were
            discarded (`if err == nil`)
FIX       : commit ee76943 — stderr warning whenever a config file exists
            but cannot be read; missing-file in search paths stays silent
REGRESSION: covered by live verification + TestNormalizeLogLevel suite scope
```

### F-34-2 — Invalid --log-level silently accepted (P3, FIXED)

```text
REPRO     : aether --log-level notalevel doctor → exit 0, silent
ROOT CAUSE: flag bound to viper, never consumed, never validated (FLAG-004)
FIX       : commit ee76943 — normalizeLogLevel rejects invalid values
            (warns, falls back to info); flag documented as reserved
            (no leveled logger exists in the codebase)
REGRESSION: TestNormalizeLogLevelAcceptsValid / RejectsInvalid
```

### F-34-3 — Corrupted vault produces raw panic text (P2, OPEN)

```text
COMPONENT : store (paged KV) surfaced through `audit verify`
REPRO     : truncate vault.db to 16KB → aether audit verify
ACTUAL    : exit 2 (fail-closed, automation-safe) BUT the output contains
            "panic: assertion failed: Page expected to be: 4, but self
            identifies as 0" — a storage-engine assertion escaping to the CLI
EXPECTED  : clean diagnostic, no panic text
IMPACT    : reliability/cosmetic (automation-safe exit; frightening output;
            potential stack-trace content in crash output)
ROOT CAUSE: storage layer panics on page-identity assertion; CLI does not
            recover at the command boundary
WHY MISSED: integration crash tests assert exit codes, not output cleanliness
FIX       : recover at the audit verify / store-open boundary and map to a
            typed error (recommended; not rushed this stage — touching the
            storage error path needs its own regression pass)
STATUS    : OPEN (P2)
```

### F-34-4 — Workspace vault substitution undetected (P2, OPEN)

```text
COMPONENT : store/workspace identity binding
REPRO     : create ws A and ws B with the same passphrase;
            copy B/vault.db over A/vault.db; aether audit verify --workspace A
ACTUAL    : "Audit trail integrity: VERIFIED (0 entries, 0 tampered)" exit 0 —
            wholesale replacement of the audit trail is undetected
EXPECTED  : the vault should be cryptographically bound to the workspace
            identity (salt/nonce/name), and a mismatch must fail loudly
IMPACT    : an attacker with filesystem write access can silently replace an
            audit trail with an empty one; the tamper-evident chain covers
            record modification (proven: byte-flip in live data → exit 1
            "corrupt audit entry") but NOT wholesale substitution/deletion
ROOT CAUSE: vault identity is not bound to the workspace directory; the
            key derives from passphrase (+vault-internal salt), so any vault
            encrypted with the same passphrase opens in any workspace
WHY MISSED: integration tests tamper records in place; none substitute
            vaults across workspaces
FIX       : bind salt.bin ↔ vault.db (store the workspace salt inside the
            vault and verify on open) — design change, own stage
STATUS    : OPEN (P2)
```

## Survived attacks (documented per the charter)

| Attack | Result |
| --- | --- |
| Workspace name path traversal (`../../escaped`) | rejected: "path separators are not allowed" — PASS |
| Audit chain byte-flip in LIVE data region (64 bytes, offset chosen by diffing vs an empty vault) | exit 1, "corrupt audit entry: invalid character" — tamper DETECTED — PASS |
| Audit byte-flip in free/meta region | VERIFIED (correct: region identical to empty vault; bbolt meta replicas tolerate) — not a bypass |
| Truncated vault | fail-closed exit 2 (but F-34-3 panic text) |
| Synthetic token `TEST_TOKEN_123` through provider validate at debug level | 0 occurrences in stdout/stderr — no leakage — PASS |
| Hardcoded dummy cert/serial search in FIX-4 path | absent (removed Stage 28; verified by source + behavior) — PASS |
| Malformed certs / wrong issuer / wrong serial / revoked / unknown | full matrix PASS (Stage 28–32) |
| Provider 401/403/400/404/429/500 classes | all exit 1 with mapped errors; 200 never confused with operation success — PASS |
| Secret scan (client_secret/token/password/private-key patterns, non-test code) | 0 real secrets; redaction set present in store/log.go — PASS |

## Test-quality observations

* Integration crash tests assert exit codes, not output cleanliness —
  why F-34-3 survived (panic text vs clean error).
* No test substitutes vaults across workspaces — why F-34-4 survived.
* The audit tamper-detection suite (evidence_verification_test.go) mutates
  records in place and passes — assertion quality there is good.
