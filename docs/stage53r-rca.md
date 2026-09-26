# Stage 53R — RCA

Root-cause records for every reconciliation conflict and defect found
during Stage 53R.

---

## R-01 — Two unrelated git histories both named "aether"

**ID** R-01 · **repositories** both · **files** repo-level

**Conflict.** Two clones shared a root commit and 48 commits of history,
then diverged on 2026-09-12 and were developed independently for two weeks.
`merge-base` between the two HEADs failed; neither repository could resolve
the other's objects. Both contained work the other lacked, and both were
plausibly "the" Aether repository.

**Root cause.** Not a Git failure. The workspace had no remote configured,
so the two lines were never reconciled after the fork; the workspace line
was also explicitly re-based ("stage44-47 (re-based, World B)"), which
reset it onto a fresh 5.0.0 scheme and discarded the 4.x version lineage
from its own history.

**Selected behaviour.** `C:\dev\aether` is the trunk; the workspace lineage
is merged into it.

**Security impact.** None directly. The risk was loss: the workspace had no
remote, so a single disk failure would have destroyed the only copy of the
Stage 52 work and the 45–47B certification history.

**Compatibility impact.** The two lines disagreed on `VERSION` scheme
(4.2.0-ga vs 5.0.0-alpha1) and on document layout (`stage*/` vs `release/`).

**Test.** `git cat-file` on each HEAD in the other repo; `merge-base`; root
and author comparison; `git log -- VERSION` on both sides.

**Evidence.** Section 1 of `docs/stage53r-lineage-reconciliation.md`.

**Status.** RESOLVED. Workspace pushed to `origin/lineage/workspace-stage52`
(hash verified); single-point-of-failure risk closed.

---

## R-02 — Merged tree deleted seven forensic reports with no archive copy

**ID** R-02 · **commit** `54abb17` · **files** 7 root-level documents

**Conflict.** The merge reported only 6 conflicts, all of which were
resolved deliberately. Yet the resulting tree deleted 7 root-level
documents, inherited from the workspace cleanup commit `682f3d6` ("chore:
remove obsolete development artifacts"). None had an archive copy.

**Root cause.** `git status` showed 7 `D` entries; the reconciliation had
been scoped to *conflicts*, and deletions arriving cleanly through the
merge are invisible to conflict-centric review. This is precisely the
"merge succeeded, therefore nothing was lost" failure the programme
forbids.

**Selected behaviour.** All seven restored byte-exact from parent `c75732a`
into `docs/archive/stageN/`, matching the line's existing convention, and
verified character-identical 7/7 (2,525 lines, commit `f436b6e`).

**Why.** Stage 1–3 implementation reports, the v3.2.0 forensic baseline RCA
and the v1.0.0 engineering blueprint are exactly the kind of contradictory
historical evidence the programme requires be preserved.

**Security impact.** None. Evidence impact: total loss of those documents
from the trunk.

**Test.** Character-for-character comparison against `c75732a` for all 7.

**Status.** RESOLVED.

---

## R-03 — Scope enforcement existed only on the abandoned line

**ID** R-03 · **files** `internal/cli/engagement.go` → `internal/engagement`

**Conflict.** The stranded line enforced rules-of-engagement scope on its
offensive AD commands: domain, DC and capability allowlists plus an RFC
3339 time window, wired at 6 call sites. The merged trunk had no equivalent
— `IsAuthorized`, `AuthorizedDomains`, `AuthorizedDCs` and `TimeWindow` all
returned zero hits.

**Root cause.** The control lived in package `cli` in flat per-command
files that the workspace's rework replaced with an `internal/cli/ad`
module. It was never carried across, and because it was a *missing* control
rather than a conflict or a compile error, nothing surfaced it. A clean
build and a fully green test suite coexisted with the gap.

**Selected behaviour.** Promoted to `internal/engagement`, made mandatory
and fail-closed on 15 commands, split into `ad.ldap.read` and
`ad.ldap.acl`, with 17 unit tests and 12 end-to-end control cases.

**Security impact.** Before: the canonical trunk would enumerate, roast and
request TGTs against **any** domain or DC, with no engagement and no time
bound. After: refused unless an engagement authorizes the exact
(domain, DC, capability) triple inside the time window.

**Compatibility impact.** Breaking by design — `--engagement` is now
required on 15 commands, so existing invocations fail.

**Test.** 6 control cases on the AD commands and 6 on LDAP, all against the
built binary. Five refusals exit 1 with precise messages; the two positive
controls pass the gate and proceed, proving it is not refusing
unconditionally.

**Status.** RESOLVED (`3a51a1c`, `76685d2`).

---

## R-04 — `ms-wcce` never compiled

**ID** R-04 · **branch** `stranded-stage-45-46` · **files** `internal/protocol/ms-wcce/`

**Conflict.** The only AD CS / certificate-template implementation in either
line does not build. See `artifacts/stage53r/inventory/ms-wcce-review.md`
for the full defect list (D1–D5).

**Root cause.** The package was never built or tested. `CertRequest` is
declared twice with two different meanings; `x509.CertificateRequest` has no
`ExtKeyUsage` field; `parseURI` is undefined; malformed SAN URIs are
silently dropped; and the package doc advertises a `Transport` interface and
`ErrTransportUnavailable` that do not exist in the code.

**Selected behaviour.** Not ported. Preserved intact for Stage 47 with a
defect list, per instruction to review before porting.

**Security impact.** None on the trunk, since the code was never reachable
there. The silent SAN-URI drop is a fail-open behaviour that would matter
if ported unfixed.

**Evidence.** Standalone scratch build: 3 compile errors.

**Status.** DEFERRED to Stage 47. Not lost — `stranded-stage-45-46`.

---

## R-05 — Stage 52 browser gate is flaky

**ID** R-05 · **test** `TestBrowserReachesItsOwnVerdict`

**Conflict.** The test failed in the first full-module run (0.61s, browser
harness launch failure) and passed in isolation, in a second full run, and
under `-race`.

**Root cause.** Not established. Ruled out: shared browser profile — the
harness already allocates a unique `mkdtemp` profile per run. Ruled out: the
60s harness timeout — the failure at 0.61s is far too fast. Leading
hypothesis is resource contention at browser launch: 8 unrelated Chrome
processes were running, and the failing run was the cold first run with
every package compiling concurrently.

**Selected behaviour.** Recorded as **FLAKY**, not PASS and not dismissed.

**Security impact.** None.

**Test.** 3 runs: 1 fail, 2 pass, plus a passing `-race` run.

**Status.** OPEN. G53R-19 and G53R-32 remain FLAKY. This is the main
technical obstacle to promoting the reconciliation branch to `master`.

---

## R-06 — Trunk version contradicts its own tracked release artifacts

**ID** R-06 · **file** `VERSION` vs `artifacts/release/`

**Conflict.** The reconciled trunk declares `5.0.0-alpha2`, but the tracked
release artifacts are labeled `3.4.0-stage3` —
`artifacts/release/sbom/aether-3.4.0-stage3.cdx.json`,
`artifacts/release/supply-chain/aether-3.4.0-stage3.cdx.json{,.sig}`.

**Root cause.** The workspace inherited release artifacts from the
pre-fork 3.4.0-stage3 era, while the version line was independently reset to
5.0.0-alpha1 and is now 5.0.0-alpha2. Release artifacts were never
regenerated.

**Selected behaviour.** Left as-is and recorded. These artifacts are the
historical record of the 3.4.0-stage3 release, not evidence for
5.0.0-alpha2, and must not be presented as the latter.

**Security impact.** Supply-chain misattribution risk: signing and
provenance records for one version could be read as covering another.

**Status.** OPEN — documentation reconciliation (G53R-37).

---

## R-07 — CRLF line endings make `gofmt -l` unreliable on this tree

**ID** R-07 · **files** workspace-contributed Go files, incl. `internal/cli/ad/`

**Conflict.** `gofmt -l` flagged 5 files in `internal/cli/ad`, including
`ad.go` and `ldap.go`, which were not modified by this stage.

**Root cause.** Line endings, not formatting. The workspace files carry
CRLF; `gofmt` normalises to LF, so it reports whole-file diffs. Verified by
byte inspection: a `gofmt -d` on an untouched file shows every line replaced.

**Selected behaviour.** Formatting checked against CRLF-normalised copies.
Real issues fixed only where introduced by this stage (one `var` block in
`tgt.go`). Pre-existing misalignment in `ldap.go`'s `rootdse` and map/struct
literals in `tgt.go` left alone and recorded, rather than absorbed as
cosmetic churn in a reconciliation commit.

**Status.** RECORDED. Open as separate hygiene work.

---

## R-08 — Five Stage 45/46 documents exist on no canonical branch

**ID** R-08 · **files** 5 documents, 559 lines

**Conflict.** `stage45-ws23-engines-cli.md`, `stage45-ws56-test-evidence.md`,
`stage45-certification.md`, `stage46-certification.md` and
`stage46-implementation.md` were uncommitted on `C:\dev\aether` and have no
workspace equivalent, so the merge could not bring them onto the trunk.

**Root cause.** Unlike R-02, nothing deleted them — they were simply never
committed on the line that was preserved and do not exist on the line that
was merged. A merge cannot recover content that exists on neither input.

**Selected behaviour.** Preserved on `stranded-stage-45-46`; archiving onto
the trunk is outstanding.

**Status.** OPEN. Tracked in `artifacts/stage53r/inventory/unique-files.md`.
