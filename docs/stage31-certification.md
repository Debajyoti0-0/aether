# Stage 31 — Certification: RC Lineage, Defect Closure, Release-Surface Hardening

Companion: `stage31-baseline-lock.md` (delta, continuity decision),
`stage31-qualification-and-observation.md` (evidence).

## 1. Gate matrix

| Gate | Description | Status | Evidence |
| --- | --- | --- | --- |
| G0 | Repository/candidate freeze | PASS | baseline-lock §1 |
| G1 | Candidate delta audit | PASS | fixes-only delta (NoArgs + docs); no undocumented change |
| G2 | D-29-1 closure | **PASS — CLOSED** | 6/6 cases from clean-room candidate build; NoArgs in tagged source |
| G3 | Full CLI regression | PASS | Stage 30 full matrix + Stage 31 candidate re-run of the D-29-1 and exit-code surfaces |
| G4 | Config precedence | PASS (carried) | Stage 30 §4 verification; no config code in delta |
| G5 | Security/failure regression | PASS (carried + delta-audited) | no security-relevant code in delta; secret scan clean |
| G6 | FIX-4 revocation/evidence | PASS (carried) | Stage 30 §2 re-verification on the post-fix build; delta contains no revocation code |
| G7 | FIX-5 providers | PASS (mock; live BLOCKED) | carried; LIVE-1 unchanged |
| G8 | Race/fuzz | PASS | race: 29 ok, 0 races (reconfirmed this stage); fuzz: 23/23 targets |
| G9 | Cross-platform | PASS (compile) / BLOCKED (non-Windows runtime) | 5/5 goreleaser targets build incl. darwin/arm64; runtime claims unchanged |
| G10 | Artifacts/SBOM/provenance/signing | PASS (claims now match pipeline) | F-30-2 CLOSED; F-30-1 CLOSED; F-30-3 documented limitation w/ expiry+owner |
| G11 | Observation continuity | PASS — Outcome B | window stays on `v4.1.0-rc2`; candidate needs no full re-window (fixes-only delta) |
| G12 | Historical truth | PASS | false stages preserved as false; all stage records intact |

## 2. 4.1.0 line disposition (G487)

**Decision: Option A — DEFERRED PROMOTION.**

`v4.1.0-rc2` is the observed production-limited baseline. Its 30-day
observation window (opened 2026-09-17T16:55:20Z) continues to
2026-10-17T16:55:20Z. If the window closes clean (zero P0/P1; no P2
requiring code change during the window; end-of-window integration and
unit runs PASS against the tagged binary), `v4.1.0` will be created from
`bc674af1` with an explicit version-bump commit and pushed, per the
Stage 29 decision criteria. The line is neither abandoned nor terminated:
`v4.2.0-rc1` is the next-minor candidate line carrying the D-29-1 fix and
will proceed to its own promotion path independently. Rationale: the
observation investment in `v4.1.0-rc2` is real elapsed time attached to
an immutable, fully verified tag; abandoning it would waste the only
temporally-qualified evidence in the 4.x cycle, while promoting it early
would repeat the Stage 26 failure.

## 3. Blocker register (current)

| ID | Blocker | Status | Next action |
| --- | --- | --- | --- |
| D-29-1 | Silent positional-argument acceptance | **CLOSED** (verified from candidate) | — |
| F-30-2 | Signing claim ≠ pipeline | **CLOSED** (instructions removed; claim matches output) | Implement cosign keyless `signs:` in the CI release workflow before any public-distribution claim |
| F-30-1 | darwin/arm64 missing | **CLOSED** (matrix exclusion removed; artifact builds) | Apple Silicon runtime qualification when a host is available |
| F-30-3 | No SLSA provenance | OPEN — documented limitation, expiry 2027-03-31, owner: repository owner | Add `actions/attest-build-provenance` to the release workflow |
| LIVE-1 | Live Entra/IMDS/Okta/GitLab/Kubernetes | BLOCKED — no authorized access | Execute when access is granted |
| OBS-1 | 30-day observation window | OPEN — 0 days elapsed of 30; closes 2026-10-17 | End-of-window runs on/after close; then promotion decision |
| INT-1 | TestCrashMatrix timing flake (harness) | DEFERRED-WITH-OWNER (repository owner) | Replace kill-timing with a synchronization handshake; do not weaken assertions |
| D-29-2 | Parent unknown-subcommand help exits 0 | ACCEPTED RISK (documented cobra pattern) | — |
| DEBT-1 | Long-tail per-flag boundary matrix | DEFERRED-WITH-OWNER (repository owner; Stages 32/33) | Scripted per-command sweeps |

## 4. Final verdict

**`PARTIAL — CANDIDATE QUALIFIED WITH DEBT` (in the charter's own state
vocabulary): `v4.2.0-rc1` is a legitimate, externally verifiable release
candidate — pushed, delta-audited, defect-closed, all executable gates
green — and `v4.1.0-rc2` remains the immutable observed baseline with its
window intact.** No GA promotion is issued; no GA language is used. The
promotion path requires: window close (2026-10-17) + end-of-window runs
for whichever line is promoted first, F-30-2 signing implementation for
public distribution, F-30-3 provenance, LIVE-1 disposition, and the
deferred qualification debt.

## 5. Stage 32 handoff

The repository now has two clean lines: `v4.1.0-rc2` (observed baseline,
window closing 2026-10-17) and `v4.2.0-rc1` (pushed, fixes-only successor,
all gates green). Next actions in order: (1) on/after 2026-10-17, run the
end-of-window integration and unit suites against `bin/aether-rc2` built
from `bc674af1` and issue the PROMOTE/RETAIN decision for `v4.1.0`; a
clean window promotes `4.1.0` with an explicit version-bump commit;
(2) implement cosign keyless signing in the GitHub Actions release
workflow so the release surface advertises only what it produces, then
cut `4.2.0-rc2` (or promote `4.2.0-rc1` as appropriate) with its own
observation start; (3) revive the CI runners (`gh auth login`, push
master) so race and provenance run where they belong; (4) close INT-1
(crash-matrix harness handshake) and DEBT-1 (long-tail boundary sweeps);
(5) LIVE-1 remains open until authorized access exists. Rollback baseline
at all times: `v4.0.0-rc2` (`5cd008be`). Earliest valid promotion date for
`4.1.0`: 2026-10-17T16:55:20Z.
