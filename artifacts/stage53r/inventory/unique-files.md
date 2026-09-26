# Stage 53R — Unique File Inventory

Pre-merge state of `C:\dev\aether` (125 commits, `VERSION 5.0.0-alpha2`,
dirty tree) against the OneDrive workspace (64 commits, `VERSION
5.0.0-alpha1`, dirty with Stage 52).

25 dirty entries / 50 files / 9,188 lines. **9 entries also existed in the
workspace** (all divergent — zero byte-identical); **16 existed only in
`C:\dev\aether`**.

All 50 files were committed to branch `stranded-stage-45-46`
(10,529 insertions) before any merge. Nothing here was lost.

## Existed only in `C:\dev\aether` (16 entries) — the at-risk set

| Path | Lines | Disposition |
|---|---|---|
| `internal/cli/ad.go` | 52 | superseded — trunk `internal/cli/ad` module |
| `internal/cli/engagement.go` | 83 | **RECOVERED** → `internal/engagement`, now fail-closed on 15 commands (`3a51a1c`, `76685d2`) |
| `internal/cli/enum.go` | 219 | superseded — trunk `ad enum users/asrep/spn` |
| `internal/cli/ldap_acl.go` | 73 | superseded — trunk `ad ldap acl get/effective` |
| `internal/cli/ldap_enum.go` | 185 | superseded — trunk `ad ldap enum` |
| `internal/cli/roast.go` | 152 | superseded — trunk `ad kerberoast`, `ad asreproast` |
| `internal/cli/tgt.go` | 118 | superseded — trunk `ad tgt` |
| `internal/protocol/ms-wcce/` (3 files) | 603 | **DEFERRED to Stage 47** — does not compile; defect list in `ms-wcce-review.md` |
| `engagement.json` | 1 | superseded by `engagements/*.json` |
| `engagement_example.json` | 8 | superseded by `engagements/example.json` |
| `engagement_no_roast.json` | 8 | superseded by `engagements/example-readonly.json` |
| `engagement_other.json` | 8 | superseded by `engagements/example-other-domain.json` |
| `docs/stage45-ws23-engines-cli.md` | 96 | **LOST from trunk** — no workspace equivalent; only on `stranded-stage-45-46` |
| `docs/stage45-ws56-test-evidence.md` | 94 | **LOST from trunk** — no workspace equivalent; only on `stranded-stage-45-46` |
| `docs/stage45-certification.md` | 133 | **LOST from trunk** — no workspace equivalent; only on `stranded-stage-45-46` |
| `docs/stage46-certification.md` | 144 | **LOST from trunk** — no workspace equivalent; only on `stranded-stage-45-46` |
| `docs/stage46-implementation.md` | 132 | **LOST from trunk** — no workspace equivalent; only on `stranded-stage-45-46` |

**Open item:** five Stage 45/46 documents (559 lines) exist on no canonical
branch. They are safely preserved on `stranded-stage-45-46` but are not on
the trunk. Unlike the seven forensic reports the merge deleted, these were
never on the workspace lineage, so the merge did not delete them — they
simply never arrived. Archiving them onto the trunk is outstanding work.

## Existed in both trees (9 entries), all divergent

Zero byte-identical. Workspace versions are generally the newer/larger.

| Path | Workspace | `C:\dev\aether` | Disposition |
|---|---|---|---|
| `internal/protocol/kerberos/` | 19 files | 10 files | workspace canonical (merged cleanly) |
| `internal/engine/ad/` | 8 files | 5 files | workspace canonical (merged cleanly) |
| `internal/protocol/ldap/` | 7 files | 9 files | workspace canonical (merged cleanly); note `C:\dev\aether` had **2 more** LDAP files — file-count review owed |
| `scripts/ad-lab/` | 10 files | 3 files | workspace canonical (merged cleanly) |
| `docs/stage45-ws0-structure.md` | present | 35 lines | workspace canonical |
| `docs/stage45-ws1-kerberos-protocol.md` | present | 32 lines | workspace canonical |
| `docs/stage45-ws4-spine.md` | present | 63 lines | workspace canonical |
| `docs/stage45-certification.md` | present | 133 lines | **DIVERGENT** — workspace has a different version; both retained on their branches |
| `VERSION` | 5.0.0-alpha1 | 5.0.0-alpha2 (uncommitted) | reconciled to `5.0.0-alpha2` |

## Follow-up

1. **Open:** archive the five orphaned Stage 45/46 documents
   (`stage45-ws23-engines-cli.md`, `stage45-ws56-test-evidence.md`,
   `stage45-certification.md`, `stage46-certification.md`,
   `stage46-implementation.md`, 559 lines) onto the trunk. They exist on no
   canonical branch; only on `stranded-stage-45-46`.

2. **Resolved — LDAP file-count asymmetry (9 vs 7): no capability lost.**
   The two LDAP implementations are organized differently, not
   supersets. Stranded-only files: `ber.go`, `client.go`, `connection.go`,
   `constants.go`, `errors.go`, `rootdse.go`. Trunk-only: `controls.go`,
   `sd.go`, `sd_test.go`, `search_test.go`. Verified on the trunk:
   - RootDSE is present at `internal/engine/ad/ldap/engine.go:411`
     (`func (e *Engine) GetRootDSE`), i.e. it moved from the protocol layer
     to the engine layer rather than disappearing.
   - BER encoding is present in `internal/protocol/ldap/bind.go` and
     `internal/engine/ad/ldap/engine.go`.
   - The trunk adds LDAP **controls** (paging) and **security-descriptor**
     parsing (`sd.go`), neither of which exists on the stranded line.
   - The trunk adds **two test files** (`sd_test.go`, `search_test.go`);
     the stranded LDAP package had **zero** tests.

   Conclusion: the trunk's LDAP is strictly stronger. The stranded version
   is an older, untested skeleton. The workspace implementation is
   correctly canonical, and the asymmetry was organizational rather than
   functional.
