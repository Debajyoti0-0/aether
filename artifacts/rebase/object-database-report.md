# Object-Database Forensics (Stage 44-R Phase 1)

**Date:** 2026-09-19
**Method:** full object DB sweep including unreachable/dangling objects, all refs, and complete reflog.

## Commands and results

```text
$ git cat-file -t 9a03334   -> fatal: Not a valid object name 9a03334
$ git cat-file -t bb56cf3   -> fatal: Not a valid object name bb56cf3
$ git cat-file -t d250d52   -> fatal: Not a valid object name d250d52

$ git fsck --full --no-reflogs
dangling tree d760958312a5f4242469101ce88ff39e75ad9d0a
dangling tree eeaad29cc411c0cdcadeff53e3b6948f54cd668d
(two dangling trees only — normal GC artifacts, same finding as docs/stage7-baseline.md §1)

$ git show-ref
a2404aa... refs/cline/checkpoints/1789195746823_on1ix/1   (agent checkpoint refs)
7b03090... refs/cline/checkpoints/1789195746823_on1ix/2
547ef11... refs/cline/checkpoints/1789471604030_7tgr9/1
806b49b... refs/cline/checkpoints/1789640408826_e7nd1/1
0dae3ae... refs/heads/master
990426b... refs/heads/onyx-crepe      (stale branch at Stage 3 fix commit)
990426b... refs/heads/sincere-plume   (stale branch at Stage 3 fix commit)

$ git reflog --all | wc -l  -> 103 entries, none referencing foreign lineage
$ git log --oneline --all | wc -l -> 56 commits total
```

## Verdict on claimed objects

| Object | Claimed role | Status |
|---|---|---|
| `9a03334` | Stage 43 HEAD | **ABSENT** — not reachable, not unreachable, not dangling |
| `bb56cf3` | `v4.2.0-rc1` tag target | **ABSENT** |
| `d250d52` | Stage 5 baseline (prior adjudication) | **ABSENT** |

**OBJECT ABSENT — NO RECOVERY POSSIBLE FROM THIS REPOSITORY.**
No tag `v4.2.0-rc1` exists; no tag of any kind exists. No history was fabricated to fill the gap. Whether another clone elsewhere holds that lineage cannot be inferred from this object database and was not assumed.

## Reachable structure notes

- `refs/cline/checkpoints/*` (4 refs) are AI-agent checkpoint refs from earlier sessions; not part of the release lineage.
- `onyx-crepe` and `sincere-plume` both point at `990426b` (Stage 3 flaky-test fix) — already merged into master history; candidates for deletion but preserved untouched by this stage.
