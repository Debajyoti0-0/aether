# RELEASING — Aether v3.4.0-ga Operator Runbook & Resume Prompt

**Terminal state (restated, verified 2026-09-20 @ `4c5d0b2`):**

```text
PRODUCT READY:       YES
RELEASE AUTHORIZED:  NO
RELEASE PUBLISHED:   NO
```

Engineering and qualification are complete and evidenced in the committed evidence chain (process history pruned to `artifacts/PROCESS-HISTORY.md`; full records recoverable from git history). Only governance remains, and every remaining action belongs to a human with authority, credentials, hardware, or signing power. This file is the standing execution order; the authoritative probe is always the repository itself (`git status`, `git tag`, `git remote`, `VERSION`), never this file.

---

## Part 1 — Standing Operator Execution Order

**Preconditions:** a human with release authority identified; credentials for the target remote/channel in hand; independent reviewer confirmed reachable; CI runners (or waiver signatories) reachable.

**Step 1 — Close B-1 (independent audit).**
Hand `artifacts/stage48/auditor-handoff/auditor-handoff.md` to a reviewer independent of this codebase's development and QA. Reviewer executes the full scope against the current commit and signs `artifacts/stage46/independent-audit-report-v2.md` with an independence attestation. Critical/High findings: closed or signed-waived before proceeding.

**Step 2 — Close B-2 (remote + channel).**
```bash
git remote add origin <authorized-url>   # provided by the operator; never guessed
git fetch origin && git push -u origin master
```
Designate the release channel (GitHub Releases / S3 / internal store). Dry-run upload + retrieval + delete to prove reachability.

**Step 3 — Close B-3 (cross-platform runtime).**
```bash
gh workflow run ga-smoke.yml && gh run watch && gh run download
```
Capture logs from `macos-13`, `macos-14`, `windows-11-arm`. If runners are truly unavailable, sign the build-only waiver (drafted in `artifacts/stage48/b3-hard-blocker.md`) with Product + Security + QA signatures.

**Step 4 — Close B-4 (release authorization).**
Convene the release board; present `artifacts/stage48/board-dossier.md` plus the Step 1–3 evidence; record explicit APPROVED / REJECTED with signatures. Silence is not approval.

**Step 5 — GA ceremony (only if Steps 1–4 all PASS).**
Execute `artifacts/stage48/publish-runbook.md` mechanically: version flip (`VERSION`/README/CHANGELOG/binary → `3.4.0-ga`, single commit) → clean-tree `-buildvcs=false` build of all six targets with the reproducibility check → re-sign everything and regenerate checksums + manifest → tag `v3.4.0-ga` and push → publish artifacts + SBOM + provenance + signatures + manifest → retrieve from a clean environment and verify signatures post-publish → release notes → post-GA monitoring window.

**Step 6 — Final state statement.**
```
PRODUCT READY:       YES
RELEASE AUTHORIZED:  YES
RELEASE PUBLISHED:   YES
```

---

## Part 2 — Resume Prompt (paste when the external inputs arrive)

> **AETHER — GA CEREMONY EXECUTION (post-governance).**
>
> The four governance blockers have been closed externally:
> - B-1: signed `artifacts/stage46/independent-audit-report-v2.md` present with independence attestation.
> - B-2: remote `<url>` configured, `master` pushed, channel `<channel>` designated and reachability-verified.
> - B-3: `ga-smoke.yml` runs complete on darwin×2 + windows-arm64, or signed waiver on file.
> - B-4: signed APPROVED decision in `artifacts/stage48/board-dossier.md`.
>
> **Role:** Release operator. Execute the rehearsed GA ceremony from `artifacts/stage48/publish-runbook.md`.
>
> **Do not re-derive.** Do not re-audit. Do not re-qualify. The engineering and qualification work is complete and evidenced in the committed evidence chain (process history pruned to `artifacts/PROCESS-HISTORY.md`; full records recoverable from git history). Your only task is the mechanical ceremony:
> 1. Re-probe the repository (`git status`, HEAD, tags, remotes, `VERSION`) — trust the repository, not this prompt.
> 2. Verify the four governance inputs are present and signed.
> 3. Version flip to `3.4.0-ga` in VERSION / README / CHANGELOG / binary; commit.
> 4. Clean-tree `-buildvcs=false` build of all six targets; verify SHA-256 stability across two builds.
> 5. Re-sign artifacts; regenerate checksums + manifest; verify positive/tamper/wrong-key.
> 6. Tag `v3.4.0-ga`; push; confirm the remote tag resolves to the GA commit.
> 7. Publish to the designated channel; verify from a clean environment.
> 8. Publish release notes; announce; open monitoring window.
> 9. Report final state with the three lines kept distinct.
>
> **Rules:** No fabrication. No signature on anyone's behalf. No tag before all four inputs verified. If any input is missing or unsigned, stop and report the exact gap with owner and date.
>
> **ULTRA GOD MAXX directive:** This is the last mile. Be mechanical, be honest, be brief. `EXECUTED ≠ ACHIEVED`. Do not blur `PRODUCT READY`, `RELEASE AUTHORIZED`, and `RELEASE PUBLISHED`.

---

## Part 3 — What an agent must NOT do from here

- Invent a further engineering stage (Stage 50+).
- Fabricate an auditor, remote, CI run, or approval.
- Flip VERSION or create a tag without all four governance inputs signed.
- Re-run qualification to "improve" the gate matrix.
- Produce percentages that fold untested items into the total.
- Treat the dossier's blank signature block as approval.

**The correct agent behavior from this point is to stop.** The repository is clean, the evidence chain is intact (`artifacts/` — see `PROCESS-HISTORY.md`), and every remaining action belongs to a human.
