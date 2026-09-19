# Phase 4 — Version & Documentation Alignment

## 4.1 Version decision — RECORDED: Option A (`3.4.0-ga`), pending release ceremony

Rationale: the GA candidate is the same code line as `3.4.0-stage3` plus three defect fixes (D-001..D-003) and evidence artifacts. No breaking API/CLI change justifies 4.0.0; the abandoned 4.2.0 numbering belongs to the foreign lineage and must not be resurrected. The `VERSION` file is intentionally **not** mutated in this stage: GA tagging belongs to a release ceremony that has not been authorized to pass (independent audit, board approval, and a remote are still outstanding — see final report).

## 4.2 Four-source agreement (current, verified)

| Source | Value |
|---|---|
| `VERSION` file | `3.4.0-stage3` |
| README heading | `Aether v3.4.0-stage3` |
| CHANGELOG head | `v3.4.0-stage3 — Teamserver V2` |
| Binary `--version` | `aether version 3.4.0-stage3` |

All four agree. For the eventual GA ceremony, all four must be flipped to `3.4.0-ga` in one commit.

## 4.3 Security model — verified as documented (spot-checks this stage)

| Claim | Evidence this stage |
|---|---|
| Fail-closed defaults | empty passphrase rejected; missing PKI refuses serve; garbage cert refuses serve; wrong CA rejected; non-whitelisted intent rejected; capability denial; empty graph typed error |
| Insecure opt-ins explicit + warning-producing | `--allow-empty-passphrase` flag help says "strongly discouraged; emits warnings on every open"; `--insecure` requires `--i-know-what-im-doing` (flag help verified in CLI matrix) |
| File-based revocation | live-verified incl. D-003 fix; limits (connection-time, name-based, plain-text) match `docs/stage3-threat-model.md` |
| Sealed workspace crypto | AES-256-GCM + Argon2id per README/doctor output; corrupt/wrong-key fail-closed live-verified |
| Credential redaction | dashboard token never echoed into logs by tests (masked as [REDACTED] in artifacts); revoked.txt serials redacted in evidence |

New findings to record in the threat model on next edit: revocation list is name-based plain text (operator can read who is revoked); revocation enforcement now re-reads the file per connection (D-003); dashboard token is per-start random 32 bytes.
