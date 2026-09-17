# Stage 28 — Baseline Lock, Working-Tree Resolution, and Incident RCA

Stage 28 recovers from the Stage 27 withdrawal of `v4.1.0` and qualifies
`4.1.0-rc2`. This document freezes the incident state, resolves every
pre-existing dirty path deliberately, and records the root-cause analysis.
Evidence and certification live in `docs/stage28-certification.md`.

## 1. Incident freeze (captured 2026-09-17, before any change)

| Item | Value |
| --- | --- |
| Repository | `C:\dev\aether` (branch `master`) |
| HEAD at freeze | `35f7a81880fcfce8513cbed8f4f6b7baadcaf7bb` (`chore: bump version to 4.1.0`) |
| Working tree | 69 dirty entries (27 tracked + 42 untracked) |
| Retained baseline tag | `v4.0.0-rc2`: tag object `779cdb6f62075565df376c2b9f04e11f7f29e4c6`, commit `5cd008be1e3e20b039214a911626a6a4426c7838` |
| Baseline VERSION | `4.0.0-rc2` (unchanged, verified `git show v4.0.0-rc2:VERSION`) |
| Withdrawn tag `v4.1.0` | Absent locally; absent from `git ls-remote --tags origin` (remote tags end at `v4.0.0-rc2`) |
| Remote | `origin` = `https://github.com/Debajyoti0-0/aether`; remote `master` at `fc062e0` (Stage 14) |
| Toolchain | go1.27.1 windows/amd64, x86_64 |

## 2. Incident RCA — why the withdrawn `v4.1.0` did not contain the tested code

Incident: the `v4.1.0` tag did not contain the FIX-4/FIX-5 implementation
(`internal/revocation/`, `internal/cli/export_verify.go`, the
`providers validate` surface) that had passed working-tree tests; Stage 27
withdrew the tag.

| Factor | Observed | Root cause | Corrective action (executed in Stage 28) | Preventive control (executable) |
| --- | --- | --- | --- | --- |
| Untracked implementation files | FIX code lived only in the working tree; `git status` showed `?? internal/revocation/`, `?? internal/cli/export_verify.go` | Commits were made with explicit path lists that omitted untracked new files | Every dirty path classified and deliberately committed (Section 3); `git show <tag>:<file>` proves presence | Gate G451: `git show v4.1.0-rc2:internal/revocation/checker.go` must succeed |
| No pre-tag tracked-state validation | Tag was created from a tree whose tracked state was never reconciled | No gate required a clean tree before tagging | WS0 resolved all 69 paths; `git status --porcelain` is empty before tagging | `git status --porcelain` must be empty at tag time |
| Tag/source mismatch | Tag pointed at a commit without the implementation | Tag was cut from HEAD, not from a verified content set | Tag cut only after commit + clean-checkout build + test | Gate G451 (tag content) + G452 (tagged build version) |
| Release build not proven from tag | Binary was built from the working tree, not from the tag | No independent build step | `bin/aether-tagged` built in a fresh clone at the tag | Clean-room clone at tag; `--version` must equal `4.1.0-rc2` |
| No clean-checkout reproduction | Tests always ran against the dirty working tree | Working tree = verification environment | All matrices re-run against the tagged build from `/c/dev/clean-room-aether` | Clean-room protocol (this stage, repeatable) |
| Stale binary confusion | Tracked `bin/aether.exe` was modified but not built from any release commit | Binary committed ad hoc | Binary rebuilt with ldflags `Version=4.1.0-rc2 Commit=<short SHA>` and committed from the release tree | `--version` reports release version, not `dev` |
| Version/tag discrepancy | `VERSION` (working tree) was `4.0.0-rc2` while HEAD claimed 4.1.0 | Version bumps decoupled from release content | `VERSION=4.1.0-rc2` committed before tagging; ldflags match | G452: tagged build `--version` = 4.1.0-rc2 |
| Missing source-to-artifact verification | No check tied artifact to source | Not previously defined | JSON/CLI outputs recorded from the tagged binary itself | Raw CLI output evidence in certification doc |

## 3. Working-tree resolution (69 paths, bucket table)

Rule followed: no `git add -A` / `git commit -am`; every path group staged
explicitly. Buckets per the Stage 28 spec: Delete / Commit / Revert / Archive.
Nothing was silently discarded; nothing unrelated was swept into release
commits (the FIX and hygiene changes are separate commits).

| Bucket | Paths | Action | Commit |
| --- | --- | --- | --- |
| Commit (docs, deleted) | 22 × `docs/stage6-*`, `docs/stage7-*`, `docs/stage12-*` (deleted in tree during earlier archival passes, never staged) | staged deletions | `fbb264d` |
| Commit (docs, untracked) | 30 × `docs/stage19-…`–`docs/stage27-…` + `Dockerfile.race` | added | `93ef483` |
| Commit (feature) | `internal/observability/` (3 files), `internal/cli/connect.go`, `go.mod`, `go.sum` | added | `5eb61b1` |
| Commit (FIX-5) | `internal/cli/v3.go` | added | `dd72f1e` |
| Commit (FIX-4) | `internal/revocation/` (checker.go, checker_test.go), `internal/cli/export_verify.go`, `internal/cli/export_verify_test.go` | added | `05ba793` |
| Commit (tests/fixtures) | `test/integration/{evidence_verification,interop_matrix,protocol_conformance,replay_idempotency}_test.go`, `testdata/` (34 files), `test_pki/` (4 files) | added | `012e8a8` |
| Commit (version) | `VERSION` → `4.1.0-rc2` | modified | `374bb04` |
| Commit (artifact) | `bin/aether.exe` (tracked binary, rebuilt from release source with ldflags) | modified | `34d1558`, `bc674af` |
| Delete (debris, untracked — removed, not committed) | `test_revoked.hex`, `test_revoked.txt`, `testdata/debris/` (2 scratch programs), `testdata/demoCA/` (3 OpenSSL db files), `testdata/dummy.csr`, `testdata/leaf.csr`, `testdata/ocsp-req-dummy.der` (intermediate) | `rm` (scratch/debris; regenerable from committed keys/config) | — |
| Revert | none — no dependency drift was reverted; `go.mod`/`go.sum` changes are the direct Prometheus requirements of the committed observability feature (verified by `go mod tidy` producing no further changes) | — | — |
| Archive | none — the historical docs were committed rather than archived, preserving truth | — | — |

Post-resolution: `git status --porcelain` is empty (excluding only the two
Stage 28 documents, committed after the tag; see below).

## 4. Fixtures regenerated (release-integrity correction)

Stage 27's fixtures were built around the hardcoded dummy certificate
(serial `3039` = decimal 12345). Since Stage 28 removes the hardcoded
certificate, the fixtures were regenerated for the real leaf certificate
(serial `6F5CC0B8D7BB7B37F1FF00FE8A3AF210172119B7`):

* `ca.pem` re-issued from the same `ca.key` with critical
  `keyUsage: digitalSignature, keyCertSign, cRLSign` (the old CA lacked
  `cRLSign`, which the new CRL-signature gate correctly rejects),
  same subject DN; `leaf.pem` / `dummy-12345.pem` re-issued from the same
  keys with unchanged serials.
* `ocsp-good.der` / `ocsp-revoked.der` / `ocsp-unknown.der`: responses for
  the leaf serial (good / revoked / unknown), signed by the test CA.
* `crl.pem` (empty) and `crl-revoked.pem` (leaf revoked), PEM + `crl.der`.
* `revoked.txt` (file-mode list: `12345` + leaf serial).
* `ocsp_responder.py` rewritten to serve any fixture by port (the CLI
  POSTs; a static `python3 -m http.server` cannot answer POST).
* `index.txt`, `index-good.txt`, `index-empty.txt`, `openssl.cnf`,
  `ca.srl`, `crlnumber` committed so every fixture is reproducible.

Test keys in `testdata/` and `test_pki/` are throwaway local PKI material
with no production value; they are committed so fixtures can be reproduced.

## 5. Tag lineage

* Commit sequence (10 commits from freeze HEAD `35f7a81`):
  `fbb264d` → `93ef483` → `5eb61b1` → `dd72f1e` → `05ba793` → `012e8a8` →
  `374bb04` → `34d1558` → `267f797` (exit-code propagation fix) →
  `bc674af` (tracked binary rebuilt).
* Tag `v4.1.0-rc2`: annotated tag object `e8416b5849c26b37a53493782aacfc9bb41f236c`,
  dereferenced commit `bc674af1ab9ac49c8ec154fda103e9e558457301`.
* Tag re-creation disclosure: the tag was created locally twice before
  being finalized — first on `34d1558`, then re-created on `267f797`
  (main.go did not yet propagate the documented exit codes), then re-created
  on `bc674af` (tracked binary rebuilt). None of these instances was ever
  pushed; the finalized tag has never been force-pushed over a published ref.
* `v4.0.0-rc2` untouched: tag object `779cdb6f…`, commit `5cd008be…`,
  `VERSION=4.0.0-rc2` — verified after all Stage 28 work.
* `v4.1.0` (withdrawn, Stage 27) was not recreated and is not reused.
