# Stage 34 Forensic Review — CLI and Flag Matrix

## 1. Command inventory

```text
Top-level commands : 26 (audit cap completion connect dashboard doctor exec export
                     graph pivot plan plugins providers prt relay replay rollback
                     run serve simulate token tunnel validate watch workspace ztna)
Subcommands        : export(9), providers(4), token(4: issue/spoof/…), workspace(4+),
                     audit(2: record/verify), cap, prt, relay, ztna subtrees
Positional args    : documented per command; required-arg commands validated
Global flags       : --config, --log-level (reserved, see F-34-2), --help, --version
```

## 2. Flag enumeration and class results (587 flags)

| Class | Count | Result |
| --- | --- | --- |
| Total enumerated (root + 2 levels, deduplicated) | 587 | inventory complete |
| Parse-validated types (int/bool/duration) | 56 | **56/56 reject invalid values at parse, exit 1** |
| Runtime-validated (string/URL/path/enum) | 529 | invalidity is semantic; release surface fully matrixed (Stages 28–31); long-tail behavioral comparison DEFERRED-WITH-OWNER |
| Sweep anomalies | 2 | both investigated → sweep detector false positives (documented in stage34-execution-and-trust-evidence §2), not product defects |

## 3. Global-flag forensics (new this review)

| Probe | Result |
| --- | --- |
| `--config <malformed-json>` | **F-34-1 (P2): silently ignored → FIXED** (now warns on stderr; missing-file in search paths stays silent) |
| `--config /nonexistent.json` | warns (explicit path), runs with defaults — reasonable, now visible |
| `--config <valid>` | loads ("Using config file: …") — unchanged |
| `--log-level notalevel` | **F-34-2 (P3): silently accepted → FIXED** (warns + normalizes to info) |
| Placement (before/after command) | cobra persistent-flag semantics: both positions parse — PASS |
| Duplicate/conflicting global flags | last-wins (cobra standard) — documented, PASS |

## 4. Command black-box results (new probes; prior coverage cited)

| Command | New adversarial probes | Result |
| --- | --- | --- |
| audit record/verify | full tamper matrix (§security-findings) | mixed — see findings |
| workspace create | name traversal `../../escaped` | **rejected** ("path separators are not allowed") — PASS |
| providers validate | synthetic token `TEST_TOKEN_123` at debug level | **0 occurrences in stdout/stderr** — no leakage — PASS |
| providers validate | HTTP error-class extension | 401/403 previously verified; 400/404/429/500 classes exercised via mock variants — all exit 1 with mapped errors — PASS |
| doctor | malformed config / invalid log-level / missing config | now diagnostics visible (F-34-1/2 fixes) — PASS |
| tunnel | documented as "reachability probe, not a tunnel" | source review matches claim: single proxy probe path, no tunnel setup — PASS (claim vs implementation consistent) |
| exec | `os/exec` usage reviewed | governed execution via engine mutation with undo specs; no raw shell invocation in CLI layer — PASS (source) |
| export verify-evidence | prior 13-scenario matrix + JSON | PASS (carried) |

## 5. Exit-code contract (re-verified)

```text
0 = success/GOOD · 1 = validation/network/config error · 2 = REVOKED · 3 = UNKNOWN
New case: audit verify on corrupted vault → exit 2 (fail-closed) — but via a raw
storage-engine panic message (F-34-3, P2): exit code is safe for automation; the
diagnostic is not operator-clean.
```
