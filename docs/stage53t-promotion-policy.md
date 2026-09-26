# Stage 53T — Promotion policy

**Decision:** Resolution 3 — the stricter policy governs until explicitly
relaxed.
**Consequence for this stage:** `master` is retained at `c75732a`. Nothing is
promoted.

## 1. The conflict, stated exactly

Two briefs govern overlapping scope and disagree about the default branch.

**Stage 53S**, §1E and WS5:

> Promote the trunk to `master` — if and only if all reachable gates pass.

**Stage 54**, closing absolute rule:

> Do not promote `reconciliation/stage53r`. Do not move `master`.

Stage 54 §38 restates it as a standing constraint:

> `master` remains untouched until all promotion requirements are satisfied.

These cannot both be obeyed in a single stage: one authorises a conditional
promotion, the other forbids promotion outright.

## 2. The three candidate resolutions

| # | Resolution | Argument for | Argument against |
| --- | --- | --- | --- |
| 1 | The later brief governs | Simple, deterministic tie-break | Recency is a coincidence of arrival order, not a statement of authority. Stage 54 is scoped to production qualification, not to Stage 53S's trunk |
| 2 | The stage-specific brief governs | Each brief speaks to its own stage; Stage 53T needs its own policy anyway | Leaves the two documents formally inconsistent, so every future stage re-litigates it |
| 3 | The stricter policy governs until explicitly relaxed | Consistent with this project's recorded practice; makes the safe outcome the default | Requires a positive act to promote, which can be read as friction |

## 3. Resolution chosen, and why it is authoritative

**Resolution 3.** The rationale is not that Stage 54 arrived later. It is that
the stricter reading is the one this project has actually applied, repeatedly,
under exactly the conditions that are in play now:

- **Stages 20–26** corrected a run of false gates. A test that reported success
  without proving the operation was treated as a failure, not a pass, even
  though the suite was green.
- **Stage 53R** declined to promote while one test was flaky, and recorded the
  root cause as "not established" rather than promoting on "it passed twice".
- **Stage 53S** held `master` at `c75732a` with reachable gates blocked, and
  declined to resolve the contradiction by omission.
- The governing charter states the rule directly: *"No partial promotion. If any
  reachable gate is not PASS, master stays."*

In every prior instance the ambiguity was resolved toward the reading that
**declines to act**. That is a settled practice, not a preference, and it is the
reason the stricter policy is authoritative here. Choosing "the later document"
would have produced the same outcome *by accident*; this produces it by
consistency, and — more importantly — it tells the next stage what to do without
re-deriving the argument.

## 4. The decisive constraint for Stage 53T specifically

Even setting the general principle aside, promotion fails on the facts:

- **Stage 54 §38 forbids promotion until its full production matrix passes.
  That matrix has not been run.** Stage 53T is a push-and-blocker-resolution
  stage; it is not the production qualification. Promoting the reconciliation
  lineage to `master` now would place unqualified code on the default branch
  while the document that governs promotion explicitly withholds authorisation.
- **G53R-34 remains NOT PERFORMED.** `golangci-lint`, `staticcheck` and
  `govulncheck` are unavailable, so the vulnerability scan has not been done.
  An unmeasured security posture is not a passing posture.
- **G53R-26 remains BLOCKED-WITH-OWNER.** The systematic control-by-control
  differential audit across both lineages was not performed.
- **Stage 54 §39 lists automatic blockers** including unexplained flakiness and
  false-success commands. Both were *found and fixed* here, but Stage 54's
  matrix — which is what §39 is scoped to — has not certified them.

## 5. The nuance about BLOCKED gates, and why it does not rescue promotion

Stage 53T §3.3 argues that a `BLOCKED-WITH-OWNER` gate does not block promotion
when the block is external, and that once G53R-25 is resolved "promotion is
permitted". That reasoning is sound in general and is adopted here — with one
qualification that matters:

> A block is only *external* when the remaining work requires something outside
> the codebase. G3206 (a provisioned Windows CA) and the Firefox/Safari matrix
> are external. G53R-34 and G53R-26 are **not**: they are unperformed analysis
> of code that exists and is readable today. They are closable now, and were not
> closed.

So the external blocks (G3206, cross-browser) do not by themselves prevent
promotion. The internal ones do. That distinction is the operative finding, and
it is why `master` stays put for reasons that are actionable rather than
atmospheric.

## 6. What would change this decision

Promotion becomes permissible when all of the following hold:

1. G53R-34 is closed — the dependency and vulnerability audit is actually run,
   or waived in writing by its owner with an expiry.
2. G53R-26 is closed — the control-by-control security differential is
   performed.
3. Stage 54's production matrix reaches a decision, since §38 ties promotion to
   it.
4. An explicit instruction authorises promotion. Under Resolution 3 that
   instruction must be positive; the absence of a prohibition is not consent.

`1b2a16e` and `d14e847` are pushed and safe on
`origin/reconciliation/stage53r`, so none of this work is at risk from holding
the branch.

## 7. What this decision does not claim

`master` is retained because the evidence does not support promotion, **not**
because the reconciliation branch is unfit. That branch is the only qualified
trunk, it is fully green, and every reachable gate on it is either PASS or
blocked on work that is identified and owned. Retaining `c75732a` is a
statement about the *evidence threshold for the default branch*, not a judgement
on the code.
