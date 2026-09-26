# Stage 53T — Certification

**Exit status: `BLOCKED-WITH-OWNER`.**

No gate is PARTIAL. Every gate is PASS, BLOCKED-WITH-OWNER, or NOT PERFORMED
with the reason named. The two work items that were blocked on entry — the
unpushed fix commit and the inert `log-level` — are both closed. What remains
blocked is the promotion decision, and the blockers are specific and actionable.

## Required final output

```text
Stage 53T status:           BLOCKED-WITH-OWNER
Version:                    5.0.0-alpha2
Tag:                        NONE (0 v5.*; all 10 historical tags intact)
Working tree:               clean (tracked)

PUSH:
  Branch                    : reconciliation/stage53r
  Push outcome              : SUCCESS (1b2a16e, then d14e847)
  Verified on origin        : YES — origin/reconciliation/stage53r = d14e847

G53R-25 RESOLUTION:
  Resolution chosen         : A (add consumer)
  slog wired                : YES
  Behavioral tests          : 6/6 CLI cases PASS + 7/7 Go regression tests
  Final status              : PASS

PROMOTION POLICY:
  Resolution chosen         : 3 (stricter governs until explicitly relaxed)
  Rationale                 : this project's settled practice resolves
                             ambiguity toward declining to act (Stages 20-26
                             false-gate corrections, Stage 53R refusing to
                             promote on a flaky test, Stage 53S holding master,
                             and the charter's own "no partial promotion" rule).
                             Recency was explicitly rejected as a rationale.
  Implication               : master advances only on positive instruction plus
                             closed reachable gates.

PROMOTION OUTCOME:
  Pre-conditions met        : NO
  Master stays at           : c75732a
  Specific blocker          : Stage 54 §38 forbids promotion until its
                             production matrix passes, and that matrix has not
                             been run; plus G53R-34 (dependency/vulnerability
                             audit) NOT PERFORMED and G53R-26 (security
                             differential) BLOCKED — both internal, both
                             closable now.
  Remedy                    : run the G53R-34 and G53R-26 analyses, obtain a
                             Stage 54 matrix decision, then promote on
                             explicit instruction.

DEFECTS VERIFIED:
  ad ldap acl reject        : PASS (3 commands, all non-zero, actionable msgs)
  Audit key ignored         : PASS (git check-ignore confirms both files)

HANDOFF:
  Stage 52b readiness       : unblocked, start from reconciliation/stage53r
                             @ d14e847; browser harness now trustworthy
  Stage 47 readiness        : BLOCKED-WITH-OWNER on G3206; remedy script
                             present; ms-wcce 3 files stranded with 5 open
                             defects, 2 confirmed absent

QUALITY GATES:
  build/vet/unit            : PASS (34 packages)
  race                      : PASS (34 packages, 0 races)
  lint                      : NOT PERFORMED — golangci-lint unavailable
  govulncheck               : NOT PERFORMED — govulncheck unavailable
  No regression             : PASS

TAG INTEGRITY:
  No v5.* tags              : confirmed

DOCUMENTS PRODUCED:         3
  1. docs/stage53t-promotion-policy.md
  2. docs/stage53t-handoff.md
  3. docs/stage53t-certification.md

FINAL VERDICT:              BLOCKED-WITH-OWNER — master retained at c75732a
                            Both fix commits pushed; both defects verified;
                            G53R-25 closed as PASS; policy documented
                            Stage 52b cleared to begin; Stage 47 blocked on G3206
```

## Gate matrix

### Pre-flight

| Gate | Result | Evidence |
| --- | --- | --- |
| G3901 baseline | PASS | branch `reconciliation/stage53r`, `VERSION=5.0.0-alpha2`, 0 v5 tags, 0 tracked modifications |
| G3902 fix commit state | PASS | `1b2a16e` on `reconciliation/stage53r` |
| G3903 current master | PASS | `c75732a`, equal to `origin/master` |
| G3904 remote state | PASS | `origin` = github, `workspace` = OneDrive |
| G3905 test baseline | PASS | build, vet, test, race all clean across 34 packages |
| G3906 defects in commit | PASS | `git show 1b2a16e` carries both `.gitignore` and `rejectUnknownSubcommand` |
| G3907 log-level consumer | PASS | confirmed absent before the fix — sole `Get` fed `normalizeLogLevel` itself |
| G3908 reconciliation branch | PASS | present |

### WS1 — push

| Gate | Result | Evidence |
| --- | --- | --- |
| G3909 push attempted | PASS | `artifacts/stage53t/push.log` |
| G3910 branch pushed | PASS | `origin/reconciliation/stage53r` = `d14e847`, confirmed by `git ls-remote` |

### WS2 — G53R-25

| Gate | Result | Evidence |
| --- | --- | --- |
| G3911 resolution chosen | PASS | Resolution A |
| G3912 consumer added | PASS | `initLogging` + `slogLevelFor` in `internal/cli/root.go` |
| G3913 behavioural tests | PASS | 6 CLI cases + 7 Go tests, below |
| G3915 G53R-25 resolved | **PASS** | chain now flag > environment > default, all observable |

### WS3 / WS4 — policy and promotion

| Gate | Result | Evidence |
| --- | --- | --- |
| G3916 policy documented | PASS | `docs/stage53t-promotion-policy.md` |
| G3917 resolution named | PASS | Resolution 3 |
| G3918 rationale recorded | PASS | settled-practice argument; recency explicitly rejected |
| G3919 pre-conditions | **FAIL (not met)** | §39 of the policy doc |
| G3920 promotion attempted | N/A | deliberately not attempted |
| G3921 outcome | PASS | master retained at `c75732a`, blocker named |
| G3922 tags resolve | N/A | not promoted; 10 historical tags verified intact |
| G3923 post-promotion tests | N/A | not promoted |

### WS5 — defect verification

| Gate | Result | Evidence |
| --- | --- | --- |
| G3924 `ad ldap acl` rejects | PASS | `artifacts/stage53t/defects-verified.log` |
| G3925 audit key ignored | PASS | `git check-ignore` confirms both paths |

### WS6 / WS7 — documents

| Gate | Result | Evidence |
| --- | --- | --- |
| G3926 handoff | PASS | `docs/stage53t-handoff.md` |
| G3927 certification | PASS | this document |
| G3934 document limit | PASS | 3 of a permitted 4 |

### Quality

| Gate | Result | Evidence |
| --- | --- | --- |
| G3928 build/vet/unit | PASS | 34 packages `ok` |
| G3929 race | PASS | 34 packages `ok`, 0 races |
| G3930 lint | **NOT PERFORMED** | `golangci-lint` not installed |
| G3931 govulncheck | **NOT PERFORMED** | `govulncheck` not installed |
| G3932 no regression | PASS | suite green before and after both commits |
| G3933 no v5 tag | PASS | `git tag -l 'v5.*'` empty |
| G3935 clean tree | PASS | no tracked modifications at exit |

## G53R-25: what was actually wrong

The blocker was worse than "the env tier is unobservable". It was **inoperative**.

`internal/cli/root.go` called `viper.AutomaticEnv()` with no prefix and no key
replacer. viper derives an environment variable name from the key by
upper-casing it, so the hyphenated key `log-level` mapped to `LOG-LEVEL` — a name
no shell will ever set. An operator could export `AETHER_LOG_LEVEL=debug`, be
told nothing was wrong, and receive the default. Isolated directly:

```
AETHER_LOG_LEVEL=debug
  AutomaticEnv() alone                 -> "info"     <- setting ignored
  SetEnvPrefix(AETHER) + key replacer  -> "debug"    <- reachable
```

Separately, `--log-level` had no consumer at all: parsed, validated, written
back to viper, then read by nothing. The flag could not change behaviour either.

Both are fixed. `applyEnvWiring` declares the prefix and replacer;
`initLogging` installs a `log/slog` handler at the resolved threshold and records
the loaded configuration at debug, which makes every tier observable.

Verified against the built binary:

| Case | Expected | Observed |
| --- | --- | --- |
| default | no debug record | 0 |
| `AETHER_LOG_LEVEL=debug` | debug record | 1, `log_level=debug` |
| `AETHER_LOG_LEVEL=error` | no debug record | 0 |
| env=debug + `--log-level=error` | flag wins, no record | 0 |
| env=error + `--log-level=debug` | flag wins, record | 1, `log_level=debug` |
| `--log-level=notalevel` | warn, fall back to info | warned, exit 0 |

Plus 7 Go regression tests in `internal/cli/loglevel_test.go`, including
`TestEnvTierIsIgnoredWithoutTheWiring`, which pins the original defect so it
cannot silently return.

**One design note worth recording.** The first version of these tests called
`initConfig()` directly and two of them failed. The cause is real and not a test
artefact: `initConfig` ends by pinning the level with `viper.Set`, which sits at
the *highest* precedence in viper. A second call in the same process therefore
cannot observe a new environment. The tests were rebuilt against a fresh viper
per case, with the wiring extracted as `applyEnvWiring` and `slogLevelFor`.

## Secret hygiene (Stage 54 §18, G54-031)

`aether-audit.key` was **never committed** — `git log --all -- aether-audit.key`
returns nothing. No exposure, so no rotation is required for repository history.

Tracked key material was classified rather than assumed:

| Path | Classification |
| --- | --- |
| `testdata/ca.key`, `ca.pem` | synthetic self-signed `O=Aether, CN=Aether Test CA`, valid 2026-09-17 → 2036-09-14 |
| `testdata/leaf.key`, `leaf.pem`, `dummy.key`, `dummy-12345.pem`, `crl.pem`, `crl-revoked.pem` | companion fixtures to that test CA |
| `internal/protocol/msoapx/prt_test.go` | inline key in a `_test.go` file |
| `internal/protocol/oauth2/cae_handler.go:41` | **not a secret** — an elided example inside a comment |
| `oauth2/client_test.go`, `engine/token/confuse_test.go` | JWT test vectors |

**No real secret is present in the release surface.** The one genuine hazard —
a real Ed25519 seed written into the working tree by a test run — is now ignored,
verified by `git check-ignore`.

## Carried forward from Stage 53S

Unchanged by this stage, and still owned elsewhere:

| Gate | Status | Owner |
| --- | --- | --- |
| G53R-08 ms-wcce stranded | BLOCKED-WITH-OWNER | Stage 47 |
| G53R-26 security differential | BLOCKED-WITH-OWNER | security reviewer |
| G53R-32 cross-browser | BLOCKED-WITH-OWNER | release engineer |
| G53R-34 dependency audit | BLOCKED-WITH-OWNER / NOT PERFORMED | toolchain owner |
| G53R-36 artifact classification | BLOCKED-WITH-OWNER | release engineer |
| G53R-37 release artifact labels (`3.4.0-stage3` vs `alpha2`) | BLOCKED-WITH-OWNER | release manager |
| G3206 AD CS | BLOCKED-WITH-OWNER | operator provisions CA |

## Why this stage is not COMPLETE

`COMPLETE` requires G3901–G3960 to pass. Three cannot:

1. **G3930 / G3931** require `golangci-lint` and `govulncheck`, which are not
   installed. Recording these as PASS would be exactly the conversion Stage 53S
   refused to make.
2. **G3919** (promotion pre-conditions) is not met, for the reasons in
   `docs/stage53t-promotion-policy.md`.

Declaring `COMPLETE` would require either waiving the two unavailable tools or
promoting against a not-yet-run production matrix. Neither is this stage's call,
and both are the owner's. **The user should note that Stage 52b's stated
unblocking condition — "Stage 53T exits COMPLETE" — is therefore not met.** The
substantive work Stage 52b needs is done and pushed; the residual block is
tooling availability and a promotion decision, neither of which Stage 52b
depends on. Stage 52b can start from `reconciliation/stage53r` @ `d14e847`
without waiting, which is the recommendation in the handoff. If the owner wants
the formal gate cleared first, installing `golangci-lint` and `govulncheck` and
authorising promotion are the two actions that would do it.
