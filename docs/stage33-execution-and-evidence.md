# Stage 33 — Execution and Evidence

## 1. F-32-2 — windows-sign artifact alignment: CLOSED (implemented; CI-unexecuted)

Traced defect chain (three independent mismatches, any one fatal):

1. `windows-sign` downloaded artifact `aether-windows-amd64.zip` — no step
   uploaded anything under that name (goreleaser writes
   `dist/aether_<version>_windows_amd64.zip` and does not upload).
2. After extraction the job signed `aether-windows-amd64.exe` — the
   archive contains `aether.exe`.
3. The "signed" upload republished an `.exe` path that never existed.

Fix (commit series on master; YAML validated):

* goreleaser job now uploads the Windows archive under the exact name the
  signing job downloads (`aether-windows-amd64.zip`).
* Sign/verify target the extracted `aether.exe`.
* Signed binary is repackaged (`aether-windows-amd64-signed.zip`) and
  uploaded under the name downstream steps reference.
* Signing remains gated on `AUTHENTICODE_CERT != ''` — **no certificate
  secret exists**, so the job truthfully skips signing; it can never
  claim signed output while unsigned.

Status: `IMPLEMENTED`, **not** `EXECUTED` — the release workflow has no
run history (see §2); validation requires a real CI run. No signature is
claimed for any artifact (none exists anywhere).

## 2. F-32-3 — release workflow run history: BLOCKED-WITH-OWNER (re-affirmed)

```text
gh auth status → "You are not logged into any GitHub hosts."
origin/master  → fc062e0 (Stage 14) — master's 25+ later commits unpublished
```

`gh run list` is impossible unauthenticated; whether `release.yml` has
ever executed remains **unknown, not assumed**. Disposition:
BLOCKED-WITH-OWNER (owner: repository owner). Remedy unchanged:
`gh auth login` → `git push origin master` → `gh run list --workflow=release.yml`.
Both release tags (`v4.1.0-rc2`, `v4.2.0-rc1`) were tag-push events that
should have triggered the workflow; the operator should check the Actions
tab after authenticating. **The next `v*` tag push will execute the
workflow with the corrected signing and windows-sign steps.**

## 3. F-32-4 — VERIFY.md regenerated: CLOSED

* All `4.0.0-rc2` references replaced with `4.2.0-rc1` (16 references; 0
  stale remain).
* Header: release date 2026-09-17, commit `bb56cf3`, candidate tag
  `v4.2.0-rc1`.
* Artifact table regenerated for the actual 5-target release matrix
  (incl. `darwin_arm64`); per-artifact sizes/checksums point at
  `checksums.txt` rather than stale hard-coded values.
* Signing-status banner retained (Stage 31); cosign/Authenticode sections
  remain labeled NOT CURRENTLY PRODUCED until signatures exist.
* Command-validity sweep: VERIFY.md documents **zero** `aether <cmd>`
  invocations (it is an artifact-verification guide only) — 0 missing
  commands by scope.

## 4. DEBT-1 continuation

Optional per charter; the DEFERRED-WITH-OWNER disposition carries forward
unchanged (owner: repository owner; target: Stage 34+). No new sweep
performed this stage; no PARTIAL exit issued.

## 5. Quality gates (Stage 33 HEAD)

| Gate | Result | Detail |
| --- | --- | --- |
| build / vet | PASS | |
| unit | PASS | 29 packages |
| integration | **PASS** | 21.9s — full suite green post-INT-1 (first fully-green integration run since the flake surfaced in Stage 31) |
| fuzz | 21/21 targets exit 0 | 60s per target (raw log /tmp/fuzz33.log) |
| lint | PASS | 0 issues |
| govulncheck | PASS | 0 affecting |

## 6. Observation window

OPEN — continuing. Events appended: F-32-2 closed, F-32-3
re-affirmed-blocked, F-32-4 closed, integration suite fully green.
New P0/P1: **0**.
> Corrected 23 → 21 in Stage 40; the count discrepancy is documented in docs/stage40-corrections-register.md.