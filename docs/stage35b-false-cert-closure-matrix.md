# Stage 35b — False-Cert Closure Matrix: Stages 20–26 Retrospective Disposition

Baseline: master; `VERSION=4.2.0-rc1`; tree clean; observation window
OPEN (closes 2026-10-17T16:55:20Z). Stages 20–26 are the false-cert era:
this document dispositions every false claim on the current verified
lineage — it does not re-execute, does not rewrite history, and does not
upgrade the historical record.

Status vocabulary: CLOSED-BY-SUCCESSOR / FALSIFIED-AND-CORRECTED /
PERMANENTLY-WAIVED. **STILL-OPEN count: 0.**

| Stage | False claim | Nature of falsity | Current disposition | Successor stage | Evidence |
| --- | --- | --- | --- | --- | --- |
| 20 | Observability ✅ (5 gates) | claimed on NOT IMPLEMENTED endpoints | CLOSED-BY-SUCCESSOR | Stage 32a — first live test: `/metrics` 200 (valid exposition), `/healthz` 200, `/readyz` 503 fail-closed | docs/stage32a-truth-table.md; stage32-production-qa.md |
| 20 | Revocation ✅ | claimed on absent verification | CLOSED-BY-SUCCESSOR | Stage 28 — OCSP/CRL/file implemented and success+negative matrix verified from the tagged build | docs/stage28-certification.md §3 |
| 20 | ARM64 ✅ | compile-only claim styled as support | CLOSED-BY-SUCCESSOR | Stage 30/31 — linux/arm64 + darwin/arm64 compile verified, darwin/arm64 in the release matrix; runtime still honestly UNVERIFIED | docs/stage34-forensic-cli-and-flag-matrix/cross-platform sections |
| 20 | IdP validation ✅ | error-path only | CLOSED-BY-SUCCESSOR | Stage 28 — providers validate implemented; mock success + 401/403/429/500 error classes verified | docs/stage28-certification.md §4; stage33-execution-and-evidence.md |
| 20 | Tag `v4.1.0` ✅ (planned) | planned ≠ created; later created without content | FALSIFIED-AND-CORRECTED | Stage 27 — v4.1.0 withdrawn (absent locally and remotely); Stage 31 — push discipline established (v4.2.0-rc1 pushed and ls-remote verified) | docs/stage27-certification.md; stage31-qualification-and-evidence §1 |
| 22 | FIX-1/2/3 `PARTIAL` | PARTIAL is not a valid exit status | CLOSED-BY-SUCCESSOR | Stage 28 — the FIX-1/2/3 capabilities carry in the certified lineage; the PARTIAL label retired by the Stage 28/30 exit-status discipline | docs/stage28-certification.md (status vocabulary) |
| 23 | `CERTIFIED 4.1.0` | version truth broken (tag missing, VERSION wrong) | FALSIFIED-AND-CORRECTED | Stage 27 — withdrawal + root-cause RCA; the certification is preserved as false | docs/stage27-certification.md; stage30-production-qa.md §10 truth table |
| 23 | Tag "to be created" | deferred, never landed in that stage | FALSIFIED-AND-CORRECTED | Stage 27 withdrawal makes the claim moot; Stage 28 defines the only valid tag procedure (content-proof via `git show <tag>:<file>`) | docs/stage28-certification.md |
| 24 | `CERTIFIED 4.1.0` | tag created on wrong content (no FIX code) | FALSIFIED-AND-CORRECTED | Stage 27 RCA — the tag did not contain the tested implementation; Stage 28 proved content via `git show v4.1.0-rc2:<file>` on the replacement candidate | docs/stage27-certification.md; stage28-certification.md §1 |
| 25 | `PUBLISH 4.1.0` | `MATCH` meant error-path-only evidence | FALSIFIED-AND-CORRECTED | Stage 27 withdrawal; Stage 29 — the honest publication posture (controlled publication; no public claims) | docs/stage27-certification.md; stage29-decision.md |
| 25 | FIX-4/FIX-5 `PASS` | success paths never executed | CLOSED-BY-SUCCESSOR | Stage 28 — full success+negative matrices from the tagged build (13/13 FIX-4, 3/3 FIX-5); Stage 32a truth table reconciles the claims | docs/stage28-certification.md §3–4; stage32a-truth-table.md |
| 26 | `4.1.0 STABLE` | declared on ERROR-PATH-ONLY evidence | FALSIFIED-AND-CORRECTED | Stage 27 withdrawal; Stage 28 certification from the tag; Stage 29 real observation window opened (2026-09-17 → 2026-10-17) | docs/stage27-certification.md; stage29-observation-log.md |
| 26 | "Production observation complete" | no observation window existed | FALSIFIED-AND-CORRECTED | Stage 29 — a real window with a real start date, append-only log, and a hard time gate honored by Stages 29–35 (0 promotions before close) | docs/stage29-observation-log.md |

## Waiver verification (G606–G609)

| Waiver | Expiry | Owner | File/record |
| --- | --- | --- | --- |
| B4 — EV Authenticode | 2027-03-31 | repository owner | docs/stage13-phase2-b4-waiver.md (created Stage 33b — Stage 13 had claimed it filed) |
| B5 — Azure KV | 2027-03-31 | repository owner | docs/stage13-b5-waiver.md (created Stage 33b) |
| F-30-3 — SLSA provenance | 2027-03-31 | repository owner | stage32/33/35 certification docs (provenance step ADDED, UNEXECUTED pending CI revival) |
| LIVE-1 — live Entra/IMDS/IdP | 2027-06-30 | repository owner | stage32/33/35 certification docs (BLOCKED-WITH-OWNER) |

## Retrospective disposition section

*(appended to docs/stage34-final-certification.md as the historical
record; reproduced here for completeness)*

## Retrospective Disposition — Stages 20–26 (False Cert Era)

Stages 20, 23, 24, 25, 26 are preserved as false certifications.
No history is rewritten. All false claims are now retroactively dispositioned:

```
CLOSED-BY-SUCCESSOR       : 6
FALSIFIED-AND-CORRECTED   : 7
PERMANENTLY-WAIVED        : 0   (the access-dependent gaps live in the
                                waiver register: B4, B5, F-30-3, LIVE-1)
STILL-OPEN                : 0
```

Stage 21 (real QA), Stage 22 (FIX-1/2/3 landed later), Stage 27 (root
cause found), Stage 28 (4.1.0-rc2 certified from the tag), Stage 31
(4.2.0-rc1 pushed) — these corrected the false-cert era. This section
records the closure. **Stages 20–26 are 100% dispositioned, not 100%
completed — and that is what closing a false-cert era means.**
