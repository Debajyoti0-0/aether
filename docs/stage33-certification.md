# Stage 33 Certification — Dual-Track Verdict

Companion: `stage33-baseline-lock.md` (time gate),
`stage33-execution-and-evidence.md` (F-32-2/3/4, gates).

## 1. Track A — v4.1.0 promotion decision

**Verdict: `BLOCKED`.**

Exact reason: the observation window attached to `v4.1.0-rc2`
(`bc674af1`) closes 2026-10-17T16:55:20Z; the current date is
2026-09-17. Per the charter's G0 time gate, no promotion, no `v4.1.0`
creation, and no window closure may occur before that timestamp — a
clean window is necessary but not sufficient, and the clock has not
reached the deadline. The promotion procedure (clean worktree from
`bc674af1`, deliberate `VERSION=4.1.0` bump, end-of-window suites, fresh
clean-room rebuild, annotated tag, push, remote verification) is fully
specified in the Stage 32/33 handoffs and will execute in Stage 34.

| Track A precondition | State |
| --- | --- |
| Observation window elapsed | **NO (0.3/30 days)** — dispositive |
| Observation log clean (0 P0/P1) | yes, so far |
| v4.1.0-rc2 immutable | yes (`e8416b58`/`bc674af1`) |
| End-of-window tests | pending (must run on/after close) |
| Rollback baseline `v4.0.0-rc2` | intact (`5cd008be`) |

## 2. Track B — v4.2.0 qualification

**Verdict: `CONTROLLED-PUBLICATION-READY` (scope-accurate; not GA).**

| Area | Classification (charter vocabulary) |
| --- | --- |
| Candidate identity (`v4.2.0-rc1` → `bb56cf3`, on origin) | INDEPENDENTLY_VERIFIED |
| Binary correctness (CLI matrix, exit codes 0/1/2/3, JSON, D-29-1) | INDEPENDENTLY_VERIFIED (Stages 28–33) |
| Race / fuzz / lint / vulncheck / unit | PASSED (reconfirmed; fuzz 21/21) |
| Integration suite | PASSED (fully green post-INT-1) |
| Crash-matrix (INT-1) | CLOSED (5/5 + full suite) |
| Observability endpoints | PASSED (first live test: 200/200/503 fail-closed) |
| SBOM | PASSED (SPDX-2.3, verified in snapshot) |
| CI signing | IMPLEMENTED, **UNEXECUTED** (G5 verdict: no EXECUTED-AND-VERIFIED claim) |
| CI workflow history (F-32-3) | BLOCKED-WITH-OWNER (auth) |
| Windows Authenticode (F-32-2) | IMPLEMENTED pipeline, **no certificate** — unsigned, truthfully |
| Provenance (F-30-3) | UNEXECUTED — limitation with expiry 2027-03-31 |
| Live Entra/IMDS/IdP (LIVE-1) | BLOCKED — no authorized access; mocks never labeled live |
| ARM64 runtime | NOT VERIFIED (compile only, both arches) |
| Long-tail flag sweeps (DEBT-1) | DEFERRED-WITH-OWNER (Stage 34+) |

Not claimed: GA-ready, signed, provenance-attested, live-qualified,
ARM64-runtime-qualified. Nothing was converted from BLOCKED/UNEXECUTED
into PASS.

## 3. Defect and debt register (delta)

| ID | Item | Status |
| --- | --- | --- |
| F-32-2 | windows-sign artifact mismatch | **CLOSED** (implemented; validation needs a CI run) |
| F-32-3 | release workflow run history unverifiable | BLOCKED-WITH-OWNER (auth; remedy documented) |
| F-32-4 | VERIFY.md stale version refs | **CLOSED** (regenerated for 4.2.0-rc1; 0 stale refs; 0 missing commands) |
| INT-1 | crash-matrix flake | CLOSED (carried; suite fully green) |
| F-30-2/F-30-1/F-30-3/LIVE-1/DEBT-1/D-29-1/D-29-2 | unchanged from Stage 32 dispositions |

## 4. Historical truth

Preserved: Stages 20–26 false certifications remain marked false; Stage
27 withdrawal; Stage 28 recovery; Stage 29 window establishment; Stage
30/31 candidate hardening and push; Stage 32 vacuous-CI-verification
finding. Nothing retroactively certified. Stage 32's implemented claims
are now separately classified from Stage 33's independently verified
ones (see §2 vocabulary).

## 5. Final release decision

```text
Track A: v4.1.0 BLOCKED — observation window not elapsed (2026-10-17)
Track B: v4.2.0 CONTROLLED-PUBLICATION-READY — explicit limits: unsigned
         artifacts, no provenance, mock-only live status, runtime
         verified on windows/amd64 only
```

## 6. Stage 34 handoff

Stage 34 begins on/after 2026-10-17T16:55:20Z and executes Track A's
promotion procedure exactly as specified: verify `v4.1.0-rc2` →
`bc674af1` unchanged; audit the full observation log (0 P0/P1 required);
clean-room build `bin/aether-rc2` from `bc674af1`; run unit +
integration + crash-matrix + release-surface matrix; then PROMOTE
(deliberate `VERSION=4.1.0` bump commit from `bc674af1`, annotated
`v4.1.0`, push, `ls-remote` verification, fresh clean-room rebuild from
the pushed tag) or RETAIN with the specific reason. `v4.0.0-rc2`
(`5cd008be`) remains the rollback baseline throughout. In parallel, the
4.2.0 GA path requires CI revival (`gh auth login`; `git push origin
master`) so the corrected signing/windows-sign workflow executes with a
run URL, provenance attestation, and LIVE-1 disposition when access
exists.
> Corrected 23 → 21 in Stage 40; the count discrepancy is documented in docs/stage40-corrections-register.md.