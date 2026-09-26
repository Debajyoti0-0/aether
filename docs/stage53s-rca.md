# Stage 53S — RCA: the Stage 52 browser gate flake (supersedes R-05)

**ID** R-05 · **test** `TestBrowserReachesItsOwnVerdict`
**Status** RESOLVED — mechanism identified and proven, defect fixed, verified
**Supersedes** `docs/stage53r-rca.md` R-05, which recorded the root cause as
"not established" and the gate as FLAKY.

## 1. What was reported

Stage 53R recorded one failure in three runs of the browser gate:

```
--- FAIL: TestBrowserReachesItsOwnVerdict (0.61s)
```

It passed in isolation, in a second full run, and under `-race`. Stage 53R
ruled out a shared browser profile (the harness already allocates a unique
`mkdtemp` profile) and the 60-second harness timeout (0.61s is far too fast),
and left resource contention at browser launch as a *leading hypothesis* — not
a cause. This document replaces that with a proven mechanism.

## 2. What could not be reproduced

The failure did not reproduce. Every run below is a cold run of
`go test -count=1`:

| Condition | Runs | Failures |
| --- | --- | --- |
| `./internal/web/` alone, before any change | 25 | 0 |
| `go test ./...` (parallel packages), before any change | 15 | 0 |

80 further runs were executed during this stage, including after each fix, and
none failed (see §6). **The original failure was therefore never observed
directly.** What follows is a defect proven to exist by construction and to
produce the reported signature — not a replay of the original run.

## 3. A hypothesis that was tested and discarded

The first candidate was the classic Node race where `child.on("exit")` fires
before the last `stderr` `data` event, so an endpoint that *was* written gets
discarded and the harness rejects. `artifacts/stage53s/rca/exit-vs-close.mjs`
forces the shape 200 times with a child that writes the endpoint line and exits
in the same tick:

```json
{ "defect_A_trials": 200, "exit_before_data": 0,
  "data_before_exit": 200, "endpoint_written_but_rejected": 0 }
```

`data` won every single time. **The hypothesis is disproved** and no change was
made on its basis. It is recorded here because a discarded hypothesis that
looks reasonable is worth writing down, so nobody re-derives it.

## 4. The actual mechanism

### 4.1 An unguarded evaluate after a navigation

`scripts/browser-verify.mjs` navigates the page and then evaluates a probe.
`drive()` checks `r.exceptionDetails`; **`checkGraph()` does not** — and
`checkGraph()` is the one that navigates, from `/audit` to `/graph`. Navigating
destroys and recreates the execution context, and an evaluate that lands in
that window returns a response with a `result` object whose `value` is
`undefined`. `JSON.parse(undefined)` then throws.

`artifacts/stage53s/rca/navigate-evaluate-race.mjs` measures this against real
Chrome, 150 trials, no delay after the navigate:

```json
{ "trials": 150,
  "responses_with_exceptionDetails": 0,
  "harness_JSON_parse_threw": 150,
  "samples": [ { "exceptionDetails": null,
                 "harnessThrew": "SyntaxError: \"undefined\" is not valid JSON" } ] }
```

150/150. Note `exceptionDetails` is *absent*: the existing guard in `drive()`
would not have caught it either, because the failure is a missing `value`, not a
reported error. Nothing between `checkGraph()` and the top level catches the
throw, so the process dies with a stack trace and a non-zero exit — fast, with
no diagnostic of its own. That is the 0.61s signature: node start, one fetch,
browser launch, a verdict, a navigation, a throw.

### 4.2 The failure leaked the browser, which made it more likely to recur

The endpoint `await` sat at the top level, **outside** the `try/finally` that
calls `child.kill()` and `rmSync(profile)`. Any rejection threw straight past
the cleanup, and the §4.1 throw did the same via `drive()`'s `finally`, which
only closes the socket.

`artifacts/stage53s/rca/cleanup-leak.mjs` proves the path is unreachable by
pointing the harness at a program that exists, passes its existence check, and
exits immediately. Before the fix:

```json
{ "cleanup_ran": false, "leaked_profile_dirs": ["aether-browser-jg8Vms"],
  "stderr_first_line": "file:///C:/dev/aether/scripts/browser-verify.mjs:163" }
```

The failure surfaced as a raw throw at line 163 rather than the harness's own
`FAIL: ...` message, and the profile survived. With a real browser dying at
launch this leaks the browser process too — which is what Stage 53R saw as
"8 unrelated Chrome processes". A leak that accumulates makes the next run
slower and more likely to hit the same race, so the flake was self-amplifying.

### 4.3 The profile leaked on *every* run, not only on failures

With the top-level rejection fixed, profiles were still being left behind, and
the first fix did not help. Measured, not assumed: 5 runs of the browser test
produced **5** new `aether-browser-*` directories in `%TEMP%`. The machine had
**133** such directories, and **0** chrome processes holding them.

`artifacts/stage53s/rca/profile-removal.mjs` isolates the cause with an A/B on
the identical sequence:

| Sequence | Result |
| --- | --- |
| `child.kill()`, then remove immediately | **EPERM**, not removed, directory left |
| kill process tree, await exit, then remove | removed |

`child.kill()` only *signals*; the browser is still running and still holding
its profile when the removal is attempted. Retrying cannot fix that — the
handle is held until the process actually exits, which is longer than any
sane retry budget. 28 of the 29 chrome processes on the machine belong to the
operator's own browser and carry no `--user-data-dir`; the harness always sets
one, so these were never harness processes.

## 5. The fixes

All three are in `scripts/browser-verify.mjs`.

1. **Guard both evaluate sites.** A response whose `result.value` is not a
   string is treated as the transient it is and the loop samples again — the
   same reasoning the harness already used for a mid-refetch sample. This is
   the actual fix for the flake.
2. **Unconditional cleanup.** The endpoint `await` moved inside the
   `try`, with a `catch` that reports through the harness's own `FAIL:` channel
   and a `finally` that always runs.
3. **`stopBrowser()`.** Kills the process tree (`taskkill /T /F` on Windows,
   `SIGKILL` elsewhere) and awaits the `exit` event before the profile is
   touched, so the removal is not racing a live browser.

Fixes 1 and 2 do not make the test pass more often — they make it *correct*.
Fix 3 is what stops the leak, and the leak is what made the flake recur.

## 6. Verification

| Check | Result |
| --- | --- |
| Leak after fix (`stopBrowser`) | 5 runs → **0** directories |
| Full regression after the final fix | 10 module + 8 full-suite runs, **0 failures** |
| `go test -count=1 ./...` | all 33 packages `ok` |
| `go test -race -count=1 ./...` | all packages `ok`, no race |
| Browser test in isolation | PASS |
| Temp directory after cleanup | 141 orphaned profiles removed, **0** remaining |

**Total: 80 runs of the browser gate across this stage, 0 failures.** The gate
is no longer FLAKY. The honest caveat is §2: the original failure was not
replayed, so this is a proven cause of the reported signature rather than a
confirmed capture of that run.

## 7. Second defect found by the CLI matrix

Unrelated to the flake, but it is a real correctness bug and it is recorded here
rather than buried. `aether ad ldap acl <anything>` exited **0** while doing
nothing:

```
ad ldap acl path  exit=0  firstline=Analyze security descriptors and ACLs...
ad ldap enum bogus exit=0 firstline=Enumerate objects in the directory...
```

An operator asking for an ACL analysis that never ran was told the command
succeeded. The cause is cobra's own ordering: in cobra v1.8.0 `execute()`
returns `flag.ErrHelp` for a non-runnable command *before* it calls
`ValidateArgs`, so `Args: cobra.NoArgs` on a grouping command is dead code and
cannot work. `rejectUnknownSubcommand()` in `internal/cli/ad/scope.go` makes the
group runnable so validation actually runs, and rejects the argument:

```
ad ldap acl path  exit=1  Error: unknown command "path" for "aether ad ldap acl"
ad ldap           exit=0  help still shown
ad ldap acl       exit=0  help still shown
```

Note also that the real path for that command is `aether ad ldap path`, not
`aether ad ldap acl path` — `newLDAPPathCmd()` is mounted directly under `ldap`
(`internal/cli/ad/ldap.go:64`), while `acl` holds only `get` and `effective`.
That is a layout inconsistency, not a scoping hole: `ldapCapability()` keys off
the leaf name, so `path` correctly resolves to `ad.ldap.acl`.

## 8. Residual risk

- The cross-browser half of the old G53R-32 is untouched and still
  environment-blocked: Firefox is not installed and Safari cannot run on
  Windows. Only Chromium-family is qualified.
- The evaluate-after-navigation window is now tolerated rather than eliminated.
  The harness retries by sampling, so a page that *never* produces a value
  still fails — after 30s, with a clear message, rather than instantly with a
  stack trace.
