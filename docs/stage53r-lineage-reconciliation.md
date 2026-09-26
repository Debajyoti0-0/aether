# Stage 53R — Lineage Reconciliation

Establishing an evidence-backed canonical trunk from two divergent Aether
development lines.

## 1. Authoritative facts

```
ROOT            37c0c09ead560fe034e212bb4d9164527714a694  (2026-09-10)
COMMON HISTORY  first 48 commits
FORK            0dae3ae0292c3a03aefd51b6735decd072b6b4d1  (2026-09-12)
```

Both lines descend from the same root, by the same author
(`Debajyoti Haldar <debjyotih835@gmail.com>`), and share their first 48
commits. They fork at `0dae3ae` ("docs: Stage 7 G0/G26 — Baseline
reconciliation and lineage resolution") and never rejoin.

| | OneDrive workspace | `C:\dev\aether` |
|---|---|---|
| Commits after fork | 16 | 77 |
| HEAD date | 2026-09-26 | 2026-09-19 |
| `VERSION` (pre-reconciliation) | 5.0.0-alpha1 | 5.0.0-alpha2 (uncommitted) |
| Remote | none | `github.com/Debajyoti0-0/aether` |
| Tags | 0 | 11 |
| Dirty tree | all of Stage 52 | 25 entries / 50 files / 9,188 lines |

**Neither line is a superset of the other.** Of the 25 dirty entries in
`C:\dev\aether`, 16 existed nowhere else, including 882 lines of AD CLI
surface, a 603-line MS-WCCE package, and engagement-scope authorization
code.

## 2. Direction decision

`C:\dev\aether` was established as the trunk, merging the workspace
lineage into it. Three facts forced this:

1. Only `C:\dev\aether` had a remote, making it the sole backup in the
   picture.
2. Only `C:\dev\aether` carried the published release lineage
   (`v4.0.0-rc1` … `v4.2.0-ga`); moving the trunk would orphan those tags.
3. Stage 7 had already chosen `C:\dev\aether` as the off-OneDrive path.

Canonicality was then decided per component on evidence, not by recency,
lineage or file count.

## 3. Preservation (performed before any merge)

| Artefact | Location | Contents |
|---|---|---|
| Stranded work | branch `stranded-stage-45-46` | 50 files, 10,529 insertions |
| Workspace lineage | `origin/lineage/workspace-stage52` = `8291fb6` | Stage 52 committed |
| Merged candidate | `origin/reconciliation/stage53r` | — |
| Untouched baseline | `origin/master` = `c75732a` | published v4.x line |

Zero destructive operations. No `reset --hard`, no `clean`, no branch or
directory deletion, no force push, no history rewrite.

## 4. Tag verdict

The brief recorded a contradiction: earlier forensics stated no `v4.x` tags
existed, yet `C:\dev\aether` carried six. Investigation found **eleven**
tags, and resolved the contradiction:

- `VERSION` is self-consistent at every tag (`v4.0.0-rc1` → `4.0.0-rc1`, …
  `v4.2.0-ga` → `4.2.0-ga`).
- All tagger dates (2026-09-15 … 2026-09-19) post-date the fork, so every
  tag belongs to the `C:\dev\aether` line.
- Seven are **published on GitHub**, including `v4.2.0-ga`.
- Tag messages are substantive and evidence-bearing; `v4.2.0-rc1` records
  real gate results and remaining debt, and notes "Local tag — push
  deferred pending authorization".

**Verdict: legitimate, all retained.** The earlier forensic was recorded
against the workspace lineage, which genuinely has zero tags. This is a
successor correction, not a rewrite of the original finding.

`v4.0.0-rc2-broken` is retained deliberately: it is Stage 17 evidence of a
broken release, and destroying it would violate the historical-evidence
rule.

## 5. Merge

Performed on branch `reconciliation/stage53r` with `--no-commit --no-ff`
after a non-destructive `git merge-tree` dry run. Six conflicts, all
resolved semantically and documented in
`artifacts/stage53r/merge/conflict-inventory.md`: `.gitignore`, `VERSION`,
the release binary, `root.go`, `serve_cert.go`, `doctor.go`.

Three resolutions were not mechanical, because a blind side-selection would
have removed a security control:

- `root.go` — workspace structure retained (its module system is required
  by the new CLI files); the workspace's simpler `initConfig` replaced by
  the fail-open config guard and strict `--log-level` validation.
- `serve_cert.go` — workspace base, with `os.MkdirAll(tsCertDir, 0o700)`
  restored. The workspace left the PKI root holding `teamserver-ca.key` at
  default permissions.
- `.gitignore` — the workspace's `*.keytab` credential control kept; the
  other side's blanket `artifacts/` ignore rejected, and proven harmful
  because it blocked staging the signed release artifacts.

A seventh defect was caught after the merge: it silently deleted seven
root-level forensic documents with no archive copy. All seven were
restored byte-exact into `docs/archive/` and verified character-identical.

## 6. Recovered functionality

### Rules-of-engagement scope enforcement — **recovered and extended**

The most consequential finding. `internal/cli/engagement.go` on the
stranded line implemented `IsAuthorized(domain, dc, capability)` with
domain and DC allowlists, a capability allowlist, and RFC 3339 time-window
enforcement, wired into six commands. On the merged trunk, grepping for
`IsAuthorized`, `AuthorizedDomains`, `AuthorizedDCs` and `TimeWindow`
returned **nothing** — the canonical trunk would have run offensive AD
commands against any domain or DC with no engagement and no time limit. The
trunk's capability-file system governs operator capabilities, not domain/DC
scope or time windows, so it did not cover this.

Promoted to `internal/engagement` and made mandatory, fail-closed, on
**15 commands**: `ad enum users|asrep|spn`, `ad kerberoast`, `ad
asreproast`, `ad tgt`, `ldap enum users|groups|computers|ous|spns|all`,
`ldap acl get`, `ldap acl effective`, `ldap path` (reachable at both
`aether ldap` and `aether ad ldap`). Capabilities split so directory reads
(`ad.ldap.read`) can be granted without rights analysis (`ad.ldap.acl`).

Deliberately not gated: `ldap bind` and `ldap rootdse`, which take no
`--domain`, so the engagement domain clause cannot be evaluated. Recorded
rather than left silent.

Deliberate deviation: domain and DC comparison is case-insensitive, because
DNS names are case-insensitive by definition. The recovered original used
exact equality and would have refused an operator who typed a
differently-cased identical name. No unauthorized value is now permitted.

### MS-WCCE — reviewed, deferred

The only AD CS / certificate-template code in either line. **It does not
compile.** Reviewed rather than ported, per instruction. Full defect list
in `artifacts/stage53r/inventory/ms-wcce-review.md`; preserved intact on
`stranded-stage-45-46` for Stage 47.

### LDAP — verified, no loss

The 9-vs-7 file-count asymmetry between the lines was investigated rather
than assumed benign. RootDSE exists on the trunk at
`internal/engine/ad/ldap/engine.go:411`; BER encoding is in `bind.go`; the
trunk additionally has LDAP controls (paging) and security-descriptor
parsing, plus two test files the stranded version lacked. The trunk's LDAP
is strictly stronger.

## 7. Canonical trunk

`reconciliation/stage53r` is the evidence-backed candidate. `master` was
deliberately **not** promoted: two gates are FLAKY and the CLI matrix is
incomplete. Promotion is a separate, explicit decision.
