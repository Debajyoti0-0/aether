# Stage 53R — Merge Conflict Inventory

Three-way reconciliation of the two Aether development lines.

```
BASE   = 0dae3ae0292c3a03aefd51b6735decd072b6b4d1  (fork point, 2026-09-12)
SIDE A = workspace/master = 8291fb6                ("World B" re-based line)
SIDE B = master           = c75732a                (published v4.x line)
RESULT = 54abb17 (merge), refined by f436b6e, 3a51a1c, 76685d2
```

Common ancestry: both lines descend from root `37c0c09` and share their
first 48 commits. Neither line is a superset of the other.

## Dry run

`git merge-tree --write-tree --name-only c75732a workspace/master` (git
2.54.0) reported **6 conflicts** and auto-merged everything else. Totals
for the resulting tree: 330 files added, 12 modified, 7 deleted.

## Conflicts

### C1 — `.gitignore` (add/add)

| | |
|---|---|
| Side B (master) | goreleaser-generated: `dist/`, `bin/`, `*.exe`, `*.tar.gz`, `*.zip`, `artifacts/`, `.goreleaser.yaml` |
| Side A (workspace) | 65 lines of specific rules, including `*.keytab` ("Keytabs are credential material and must never be committed regardless of which lab produced them"), per-stage evidence exclusions, stray-binary list, Rust `target/`+`pkg/` exclusions |
| Semantic difference | Not cosmetic. Side B blanket-ignores `artifacts/` and `*.exe`; Side A deliberately **tracks** signed release artifacts and evidence. |
| Security impact | Side A's `*.keytab` rule is a credential-leak control. Side B has no equivalent. |
| **Resolution** | **Union.** Kept Side A verbatim; appended `dist/`, `*.tar.gz`, `*.zip`, `*.exe`. |
| Rejected alternative | Side B's `artifacts/` was **deliberately not carried over.** It was empirically proven harmful: during this merge it blocked staging `artifacts/release/dist/aether-windows-amd64.exe`, i.e. it would have made the release evidence uncommittable. |
| Note | Side B's `.goreleaser.yaml` entry is a no-op — the real file is `.goreleaser.yml`, which is tracked on both lines. Verified, so no release infrastructure was at risk. |

### C2 — `VERSION` (content)

| | |
|---|---|
| Base | `3.4.0-stage3` |
| Side B | `4.2.0-ga` (published, tagged) |
| Side A | `5.0.0-alpha1` |
| **Resolution** | **`5.0.0-alpha2`** |
| Reason | Release-engineering conclusion, not a merge shortcut. Side A set 5.0.0-alpha1 in a single commit (`ad0c0ae`) and never carried the 4.x lineage, so its CHANGELOG stops at v3.4.0-stage3. Side B has a clean tagged progression 4.0.0-rc1 → 4.2.0-ga. The merged trunk strictly supersedes both. `5.0.0-alpha2` was Side B's own recorded next intent (preserved on `stranded-stage-45-46`). |
| Tests | None applicable (declarative). Verified `internal/version` still injects via ldflags and defaults to `dev`. |
| Note | **No `v5.*` tag was created.** |

### C3 — `artifacts/release/dist/aether-windows-amd64.exe` (binary)

| | |
|---|---|
| Side B | `bin/aether.exe` only — the 4.2.0-ga Windows build |
| Side A | Full five-platform `artifacts/release/dist/` set plus `checksums.txt`, `provenance.json`, `release-manifest.json`, signed `supply-chain/*.sig` + `release.pub`, and SBOMs |
| **Resolution** | **Side A** (workspace) |
| Reason | Strictly richer release record, and the rename matches the workspace's `stage*/ → release/` convention. |
| Loss analysis | None. Verified `git ls-tree v4.2.0-ga` contains `bin/aether.exe`, so the GA binary remains permanently recoverable at the published tag. |
| Tests | `go build ./...` unaffected; the tracked binary is a build artifact, not a source input. |

### C4 — `internal/cli/root.go` (content, 5 hunks)

| | |
|---|---|
| Side A | Module-registration system (`Module` interface, `RegisterModule`, `loadModules`, `modulesMu`) + a simple `initConfig` |
| Side B | Hardened `initConfig`: warns when a config file exists but cannot be read (fail-open prevention) and rejects invalid `--log-level` (finding F-34-2), with `validLogLevels` + `normalizeLogLevel` |
| **Resolution** | **Side A structure, Side B semantics.** |
| Reason | The module system is required by the new CLI files (`ad.go`, `ldap.go` call `cli.RegisterModule`). The config hardening is a security control Side A lacks. |
| Security impact | Taking Side A wholesale would have deleted the fail-open config guard and the strict `--log-level` contract — a §36 "compiles but removes a security control" failure. |
| Deviation | Side B's `initConfig` body was rewritten to fit Side A's `SetConfigFile`/search-path model, and its `errors.As` guard was corrected: Side B placed the check inside an `if cfgFile != ""` branch where `cfgFile != ""` was always true, making the `ConfigFileNotFoundError` test dead. On the merged structure the corrected form warns for an explicit `--config` that fails to load and stays silent when no config exists. |
| Tests | `go build`, `go vet`, `go test ./...` all pass. Import set is the union (`errors`, `fmt`, `os`, `sync`); both `errors.` and `sync.` confirmed used. |

### C5 — `internal/cli/serve_cert.go` (content)

| | |
|---|---|
| Surface read | Appears comment-only, but a code-only diff disproved that |
| Side B extra code | `os.MkdirAll(tsCertDir, 0o700)` before `api.InitCA` — creates the **PKI root** owner-only, fail-closed |
| Side A | Creates only the `operators` subdir at `0700`; the root that holds `teamserver-ca.key` is left to default permissions |
| **Resolution** | **Side A base + Side B's `MkdirAll` restored**, with an explanatory comment. |
| Security impact | Taking Side A wholesale would have deleted a directory-permission control protecting the CA private key. |
| Historical evidence | The revocation-flag fix is recorded under **both** finding IDs — `D-002` (workspace) and `H1` (master) — as a single combined comment. Same defect, two independent findings; neither record discarded (§39). |
| Tests | `go build`, `go vet`, `go test ./...` pass. |

### C6 — `internal/cli/doctor.go` (content)

| | |
|---|---|
| Side B | `closeProbe()` |
| Side A | Explicit `w.Close()` with error reporting, documented: "The vault holds an open file handle; on Windows an open file cannot be unlinked, so the handle must be released before Delete shreds it." |
| **Resolution** | **Side A** |
| Reason | Documented-correct Windows behaviour. `closeProbe` has 0 remaining references in Side A, so no dead code is left behind. |
| Tests | `go test ./...` passes, including `internal/cli`. |

## Deletions introduced by the merge (7 files, unarchived)

The merge deleted 7 root-level documents via Side A's cleanup commit
`682f3d6` ("chore: remove obsolete development artifacts"). None had an
archive copy, so accepting that deletion would have destroyed Stage 1-3
implementation reports, the v3.2.0 forensic baseline RCA, and the v1.0.0
engineering blueprint from the canonical trunk.

Restored byte-exact from parent `c75732a` into the line's existing
`docs/archive/stageN/` convention, verified character-identical 7/7
(commit `f436b6e`, 2,525 lines):

| Original | Archived to |
|---|---|
| `AETHER_STAGE1_SAFETY_WIRING_IMPLEMENTATION_REPORT.md` | `docs/archive/stage1/stage1-safety-wiring-implementation-report.md` |
| `AETHER_v3.2.0_FORENSIC_BASELINE_RCA_REPORT.md` | `docs/archive/stage1/v3.2.0-forensic-baseline-rca-report.md` |
| `AETHER_STAGE2_ARCHITECTURE_BASELINE.md` | `docs/archive/stage2/stage2-architecture-baseline.md` |
| `AETHER_STAGE2_STORAGE_AND_SPINE_IMPLEMENTATION_REPORT.md` | `docs/archive/stage2/stage2-storage-and-spine-implementation-report.md` |
| `AETHER_STAGE3_TEAMSERVER_ARCHITECTURE_BASELINE.md` | `docs/archive/stage3/stage3-teamserver-architecture-baseline.md` |
| `AETHER_STAGE3_TEAMSERVER_V2_IMPLEMENTATION_REPORT.md` | `docs/archive/stage3/stage3-teamserver-v2-implementation-report.md` |
| `Aetherv1.0.0—Complete-Engineering-Blueprint` | `docs/archive/v1.0.0/v1.0.0-complete-engineering-blueprint.md` |

## Clean auto-merges worth auditing

`CHANGELOG.md` merged without conflict, which is exactly where a clean
merge can silently corrupt. Audited: one `# Changelog` header, headings in
correct descending order (v4.2.0-ga → v1.0.0), and byte-level UTF-8
verified (em dash `U+2014` present, no double-encoding). Both sides only
prepended above their shared v3.4.0-stage3 history, so the textual merge
was semantically correct.
