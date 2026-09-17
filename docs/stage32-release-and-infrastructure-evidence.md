# Stage 32 — Release and Infrastructure Evidence

## 1. G502–G503 — Keyless cosign in the CI release workflow (implemented)

`release.yml` already installed cosign and granted `id-token: write` — but
**never signed anything**: the goreleaser job produced no signatures, so
the verify job's signature loops passed vacuously (no `.sig` files → loop
bodies skipped → green without testing anything). This was the F-30-2
pattern surviving inside CI.

Fix (commit `45c56bf`):

* New `Sign artifacts (keyless cosign)` step after `goreleaser release`:
  `cosign sign-blob --yes` for `checksums.txt`, every archive, and every
  SBOM, emitting `.sig` + `.pem` (keyless via the job's `id-token: write`).
* New `Upload signatures` step so the verify job's signature-verification
  loops and tamper tests now operate on real signature files.
* Workflow YAML validated (`WORKFLOW_YAML_VALID`); goreleaser config
  re-validated.

New findings recorded (not silently fixed):

| ID | Finding | Severity | Disposition |
| --- | --- | --- | --- |
| F-32-2 | `windows-sign` job downloads artifact name `aether-windows-amd64.zip`, which no step uploads (goreleaser names archives `aether_<version>_windows_amd64.zip` and does not use upload-artifact) — the job would fail on any real run | HIGH | OPEN — fix requires a CI run to validate; the whole release workflow has likely never executed end-to-end |
| F-32-3 | Whether the release workflow has EVER run is unverifiable: it triggers on tag push, both release tags were pushed, but remote run state requires `gh auth` | HIGH (trust chain) | OPEN — BLOCKED-WITH-OWNER (see §2) |

## 2. G504–G506 — CI revival: BLOCKED-WITH-OWNER

```text
gh auth status → "You are not logged into any GitHub hosts."
origin/master  → fc062e0 (Stage 14) — 20+ commits behind local master
```

Remote workflow runs cannot be listed, triggered, or verified
unauthenticated. No CI evidence is fabricated. Exact operator remedy:

```bash
gh auth login                      # interactive OIDC/device flow
git push origin master             # publish stages 19–32; triggers ci.yml
# then: gh run watch; gh run view <id> --log
# race (ci.yml 'test (-race)' jobs) and provenance results follow
```

Race (local CGO/mingw) and provenance (local limitation F-30-3) statuses
are unchanged from Stage 31; CI-local results are BLOCKED-WITH-OWNER.

## 3. G507–G509 — INT-1 CLOSED (crash-matrix harness)

Classification (per the Stage 32 long-form charter): **Category 1 — test
harness defect.** The product guarantee (no partial record, contiguous
sequence, verifiable chain across a kill) is correct; the test assumed
instantaneous termination.

Fix (commit `45c56bf`): the exact upper count bound gained a documented
constant (`crashKillLatencyTolerance = 2`, the observed Windows
TerminateProcess latency in fsync'd-appends units). **The durability
assertions remain strict and untouched**: contiguous `Seq`, full chain
`Verify`, no partial record readable.

Verification: `TestCrashMatrix` **5/5 consecutive PASS**
(0.59–0.66s each). INT-1 = CLOSED.

## 4. DEBT-1 — long-tail flag surface: bounded

Enumeration (from the `v4.2.0-rc1` binary help): 26 top-level commands;
export (9 subcommands), providers (4), token/workspace/validate subtrees;
~120 distinct flags incl. persistent flags. Coverage disposition:

| Scope | Status |
| --- | --- |
| Release surface (`export verify-evidence` 14 flags, `providers validate` 3 providers × 15 case classes) | VERIFIED (Stages 28–31 matrices + 32a spot checks) |
| All 26 commands: help / bare / unknown-flag / unknown-arg class | VERIFIED (scripted matrix, Stage 29 + re-run 30/31) |
| Remaining long-tail: per-flag value-class sweeps (empty/negative/max/duplicate/conflict) for ~20 non-release commands | **DEFERRED-WITH-OWNER** — owner: repository owner; target: Stage 33+; method: scripted per-flag sweep of the enumerated inventory |

No PARTIAL exit: each scope is VERIFIED or DEFERRED-WITH-OWNER.

## 5. G510 — 4.2.0-rc1 independent candidate health

```text
v4.2.0-rc1 is a separate candidate line.
It is not the source of v4.1.0 promotion.
It does not inherit the v4.1.0-rc2 observation window.
```

Health: tag on origin (`bd0d9636`→`bb56cf3`), VERSION correct, D-29-1
verified from its own build (Stage 31), race/fuzz/lint/vulncheck green
(Stages 30–31), signing = none (truthful), provenance = none (F-30-3),
integration = green post-INT-1 (5/5 crash-matrix + full suite PASS),
live = BLOCKED. Healthy as an RC; publication readiness in the 32a truth
table.

## 6. Observation window (G513–G514)

OPEN — continuing. Events appended this stage: CI signing step added;
INT-1 closed; CI revival BLOCKED-WITH-OWNER; 32a truth table produced.
New P0/P1: **0**.
