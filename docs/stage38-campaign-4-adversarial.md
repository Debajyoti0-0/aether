# Stage 38 Completion - Campaign 4: Adversarial (Continuation)

Baseline: master @ `5cc7756`; tag build `v4.2.0-rc1` @ `bb56cf3`. Both
binaries rebuilt with the corrected ldflags binding
(`-X github.com/Debajyoti0-0/aether/internal/version.Version=4.2.0-rc1`)
and print `aether version 4.2.0-rc1`. Sandbox: `AETHER_CONFIG_DIR` isolated
to `%TEMP%\aether-s40\root`; no engagement state touched in the live config
root. Two-column rule: tag column status on the `bb56cf3` build, master
column on the `5cc7756` build. The tag predates ALL four Stage 34 fixes
(`ee76943` is not an ancestor of `bb56cf3` - verified
`git merge-base --is-ancestor` fails), so F-34-1/2/3/4 reproduce on-tag;
master closed them at Stage 35.

Trigger for config attacks: `workspace list` (doctor masks via F-38-1 and
is excluded per the charter).

## 4.1 Config attacks

| Attack | Tag `bb56cf3` (raw) | Master `5cc7756` (raw) |
| --- | --- | --- |
| Malformed JSON via `--config` | exit 0, silent defaults (fail-open - F-34-1 OPEN on-tag) | `Warning: config file ignored: While parsing config: invalid character '\''...` then continues (REMEDIATED at `ee76943`) |
| Unreadable config (ACL deny) | exit 0, silent (F-34-1) | `Warning: config file ignored: ... Access is denied.` (REMEDIATED) |
| Wrong-typed value (`{"log-level": 42}`) | exit 0, silent acceptance (F-34-2 OPEN on-tag) | `Warning: invalid --log-level "42"; using "info"` (REMEDIATED) |
| `--config` pointing at a directory | exit 0, silent | `Warning: config file ignored: Unsupported Config Type ""` (REMEDIATED) |
| Binary garbage config (4 KB random bytes) | exit 0, silent | `Warning: config file ignored: While parsing config: invalid character '\x1d'` (REMEDIATED) |

Verdict: 4.1 PASS - fail-open closed on master; honest warnings, defaults
in effect, exit code 0 (fail-operational by design, visible to operators).

## 4.2 Workspace traversal (must reject)

Nine vectors against `workspace info` on the tag build; typed rejections,
exit 1, no workspace state touched:

| Vector | Raw result |
| --- | --- |
| `..\..\Windows` | `Error: workspace name "..\\..\\Windows": path separators are not allowed` |
| `..\..\..\Windows\System32` | same typed rejection |
| `C:\Windows` | `path separators are not allowed` |
| `/etc/passwd` | `path separators are not allowed` |
| `..%2f..%2fWindows` | treated as literal name -> not found (no decoding, no escape) |
| `sub\..\..` | `path separators are not allowed` |
| `....//....//Windows` | `path separators are not allowed` |
| `.` | `relative-navigation component` |
| `C:\Users\All Users` | `path separators are not allowed` |

Verdict: 4.2 PASS - all vectors rejected; guard identical on master
(predates the tag).

## 4.3 Credential leakage (must be 0)

Full lifecycle in workspace `leakprobe` (passphrase
`S3cr3t-Leakage-Pr0be-777`, argv token `ARARGV-SECRET-leakprobe-999`),
then recursive ASCII scan of every sealed artifact (vault.db, exported
JSONL, report, executive report, SARIF, ATT&CK layer):

| Probe | Hits |
| --- | --- |
| Passphrase in any artifact | 0 |
| Argv token in exported audit JSONL | 0 |
| Argv token in vault.db | 0 |
| Command text in vault.db / audit JSONL | present BY DESIGN (audit payload; contains no credentials - argv secrets are never persisted) |

Verdict: 4.3 PASS - 0 credential leaks.

## 4.4 Audit tampering

| Class | Tag `bb56cf3` (raw) | Master `5cc7756` (raw) |
| --- | --- | --- |
| Byte-flip in live record content (offset 12423, `cmd-1` region) | exit 1, `Error: audit trail integrity: TAMPERED` | exit 1, `TAMPERED` |
| Byte-flip at first structural diff (offset 4128, non-record page) | VERIFIED - correct (structural page, not chain data) | VERIFIED - correct |
| Tail-truncate one page (freelist page; live data intact) | file re-extended on open, VERIFIED (3 entries) - correct self-healing | same |
| Deep truncation to 16384 bytes (removes data pages) | exit 2, **raw panic text**: `panic: invalid freelist page: 0, page type is unknown<00>` (F-34-3 OPEN on-tag) | exit 1, typed: `Error: open vault ...: vault file is corrupt or truncated: invalid freelist page...` - **0 panic occurrences** (REMEDIATED at Stage 35) |
| Wholesale vault substitution (`subsrc/vault.db` copied over `tamperts`) | exit 0, `VERIFIED (3 entries, 0 tampered)` - **substitution UNDETECTED** (F-34-4 OPEN on-tag) | exit 1, typed: `Error: vault belongs to workspace "subsrc" but was opened as "tamperts": vault workspace identity mismatch` (REMEDIATED) |

Verdict: 4.4 PASS with the four-fix lineage reproduced exactly as
evidenced: chain tamper-evidence holds on both builds; the two P2 storage
defects are visible on-tag and fail-closed-typed on master.

## 4.5 Provider impersonation

Local status-code matrix (11 ports, one python mock per class) against
`providers validate okta --domain http://127.0.0.1:<port> --token
impersonation-probe-token`:

| HTTP class | Tag | Master |
| --- | --- | --- |
| 200 valid discovery | exit 1, token validation failed (token is deliberately invalid; 200 never confused with operation success) | same |
| 301 redirect | followed to `http://127.0.0.1/redirected` -> refused cleanly | same |
| 400 / 401 / 403 / 404 / 429 / 500 | exit 1, `okta http <code>: <body>` mapped | same |
| 503 (HTML body) | exit 1, mapped | same |
| 200 + garbage schema | exit 1, JSON parse error | same |
| TLS downgrade (https -> plain port) | exit 1, `tls: first record does not look like a TLS handshake` | same |

Extensions re-proven this stage beyond the Stage 34 matrix: 301, 503,
malformed schema, TLS downgrade. Verdict: 4.5 PASS.

## 4.6 Plugin supply chain

| Control | Tag (raw) | Master |
| --- | --- | --- |
| Traversal name `../../evil` (real registry) | exit 1, `registry http 404` (no local path use) | same |
| Backslash traversal `sub\..\evil` | exit 1, `registry http 404` | same |
| Absolute-path name `C:/Windows/evil` | exit 1, `registry http 404` | same |
| `file://` index URL | exit 1, `unsupported protocol scheme "file"` (no arbitrary file read) | same |
| Unreachable registry | exit 1, clean dial error | same |
| Controlled index: valid manifest | exit 0, installed, SHA-256 verified | same |
| Controlled index: tampered package (badsum) | exit 1, `checksum mismatch for helper-badsum: got 4ffc3e29... want 000...000` | same |
| Manifest name `../escape` | exit 0, installed as sanitized `..-escape-1.0.0.json` - no escape (0 `plugin.so` outside dir) | same |
| No download_url manifest | exit 1, `manifest <name> has no download_url` | same |

Info finding: plugin package download has no size cap (hostile index could
stream unbounded bytes before the checksum verdict) - F-40-4.

Verdict: 4.6 PASS (checksum enforcement holds; traversal structurally
blocked).

## 4.7 Session/token state

| Probe | Raw result |
| --- | --- |
| Identical op recorded twice (replay of audit append) | seq increments 25->26, chain VERIFIED (append-only by design; replay dry-run is the idempotency interface) |
| 4 parallel recorders (race) | all exit 0, chain VERIFIED (30 entries) - bbolt serialization held, no corruption |
| argv token through governed exec, then recursive scan | 0 occurrences in audit JSONL and vault.db |

Verdict: 4.7 PASS - session state consistent.

## 4.8 Resource exhaustion

| Probe | Raw result |
| --- | --- |
| 10 MB config file | parsed in 61 ms, tool continues (bounded; no size cap needed at observed scale) |
| 20,000-level nested JSON array | no crash, tool continues (encoding-json depth limit -> parse error path, silent on-tag / warned on master) |
| 1 MB `--cmd` argument | blocked above the tool layer by CreateProcess limits; chain still VERIFIED |
| Cyclic symlink junction inside a workspace, then `workspace delete --force` | exit 0, securely deleted, no recursion blow-up |

Verdict: 4.8 PASS - bounded.

Campaign 4 verdict: PASS. Findings: F-40-1..F-40-4 (see
docs/stage38-findings-register.md).
