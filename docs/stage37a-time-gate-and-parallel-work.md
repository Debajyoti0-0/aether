# Stage 37a — Time Gate Result and Parallel Work

**Stage 37 was invoked before its time gate and converted to Stage 37a,
per the charter's own pre-flight rule.**

## 1. Time gate

```text
Current UTC   : 2026-09-17T20:01:23Z
Window close  : 2026-10-17T16:55:20Z
Window total  : 30 days
Elapsed       : ~3 hours of 30 days — NOT ELAPSED (2,580,836 seconds remain)
Decision gate : BLOCKED — TIME NOT ELAPSED
Conversion    : Stage 37 → Stage 37a (parallel work only)
```

No promotion. No `v4.1.0` created. No evidence backdated. The window
remains attached to `v4.1.0-rc2` (`bc674af1`).

## 2. Observation log audit (independent recount — G2)

27 event rows since window open. Recounted row-by-row (not from the
summary line):

```text
P0 : 0
P1 : 0
P2 : 1  (D-29-1 silent positional args — CLOSED via cobra.NoArgs,
         commit 36be959; regression-tested)
P3 : 1  (D-29-2 parent help semantics — documented, accepted)
INFO/event rows : 25 (all CLOSED except 1 DEFERRED harness item
                  [carried, owner assigned] and 1 BLOCKED-WITH-OWNER
                  [CI revival, remedy documented])
```

Note: a naive `grep "P0\|P1"` over the log returns 3/3 hits — those are
the severity *definitions* in the document preamble and the exit-criteria
text, not defect events. The row-level recount above is the authoritative
count: **zero P0/P1 events; the 4.1.0 promotion path remains clean.**

## 3. CI revival retry (G655)

```text
gh auth status → "You are not logged into any GitHub hosts."
```

**BLOCKED-WITH-OWNER** — re-confirmed. Remedy unchanged and exact:
`gh auth login` → `git push origin master` (now 36 reviewed commits,
all classified stage work) → `gh run list` provides run URLs that
convert signing/provenance/race from IMPLEMENTED to EXECUTED.

## 4. F-32-4 re-confirmation

`VERIFY.md`: 0 stale `4.0.0-rc2` references; 16 current `4.2.0-rc1`
references; regeneration from Stage 33 intact. CLOSED (carried).

## 5. DEBT-1 continuation (batch 2): 5 more commands, 16 flags

Commands: `plan`, `rollback`, `relay`, `ztna`, `watch` (raw per-flag
results in the Stage 36 sweep format, `/tmp/debt1b.txt`).

| Command | Flags | Bare-flag accepted | Empty-value accepted | Anomalies |
| --- | --- | --- | --- | --- |
| plan | 2 | 0 | 2 (`--config=`, `--log-level=` on a parent command → help, exit 0) | none |
| rollback | 2 | 0 | 2 (same class) | none |
| relay | 2 | 0 | 2 (same class) | none |
| ztna | 3 | 0 | 3 (adds `--browser-preset=` string default) | none |
| watch | 7 | 0 | 0 — all flags reject empty values deterministically | none |

All empty-value acceptances are parent-command help paths or global-flag
empty strings falling back to defaults (F-34-2 normalization active) —
correct behavior, no security-relevant empties. **No new defects.**

Cumulative DEBT-1 state: **10 commands fully swept** (Stage 36 + 37a),
56 parse-class flags closed, release surface fully matrixed; remaining
long-tail behavioral comparison DEFERRED-WITH-OWNER (repository owner,
Stage 38+).

## 6. Re-deferral

Stage 37 (the PROMOTE/RETAIN decision) re-defers to its time gate:
**on or after 2026-10-17T16:55:20Z**, execute the documented procedure
(clean-room build from `bc674af1`, observation-log audit, unit +
integration + release-surface matrix, then PROMOTE with a deliberate
`VERSION=4.1.0` bump commit + annotated tag + push, or RETAIN with the
specific evidenced reason). Until then, no tag, no promotion, no
fabricated elapsed time.
