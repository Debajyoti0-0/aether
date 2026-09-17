# Stage 35 — Remediation Evidence: Vault Identity Binding (F-34-4) and Storage Fail-Closed Boundary (F-34-3)

## 1. Design (chosen before implementation)

**Option A (workspace-identity binding), architecture-consistent form.**
The vault's plaintext `meta` bucket (bolt KV, where `schema_version` and
`created_at` already live) records the name of the workspace directory
the vault was created in. Every `OpenVault` verifies the bound name
against `filepath.Base(filepath.Dir(path))`.

Consequences, chosen deliberately and documented in-code:

| Behavior | Rationale |
| --- | --- |
| Substituted vault (any other workspace, same passphrase) | REJECTED — the Stage 34 attack class |
| Renaming/relocating a workspace directory | binding invalidated (rejected) — the vault is bound to exactly one workspace name |
| Pre-binding vaults (created before this change) | one-time migration: first open with the new binary adopts the current directory name; substitution is enforced from then on |
| Old binary + new vault | forward-compatible: the old code ignores the new meta key |

**Security boundary, stated honestly (G5):** the binding metadata is not
passphrase-sealed. An attacker who can rewrite the vault file with
storage-layer tooling (bbolt is a library) AND knows the workspace name
can rebind the meta record. The property delivered is: *accidental or
lazy substitution — file copies with the same passphrase, the exact
Stage 34 attack — is detected.* Forging a complete false audit history
requires deliberate passphrase-held forgery of both the binding and the
sealed record chain; the sealed chain still detects record tampering.
This boundary is disclosed rather than overclaimed.

**Why not a passphrase-sealed binding (Option B):** `OpenVault` does not
receive the passphrase (records are sealed per-record with an embedded
salt under `keyprovider_impl.go`); adding a sealed binding requires
threading the passphrase into the store-open path — an API change across
every caller. Deferred as a hardening option; disclosed limitation.

## 2. F-34-3 — panic boundary (G15-compliant)

The panic (`assertion failed: Page expected to be: 2, but self
identifies as 0`, bbolt v1.3.8 `_assert`) fires during initialization
writes — **not only during `bolt.Open`**. The first fix (recover around
`bolt.Open` only) was insufficient: the test corrupting a page header
panicked through `initBuckets`. Final shape: one narrow recover boundary
(`openAndBindVault`) spanning open + bucket init + schema check + identity
binding, converting the panic to `ErrVaultCorrupt` and **closing the file
handle on the recovered path** (a leaked lock otherwise blocks temp-dir
cleanup on Windows). No blanket suppression elsewhere.

Error mapping: `audit verify` on a corrupt vault → exit 1, clean
diagnostic (`vault file is corrupt or truncated: …`), zero panic text.
(The Stage 34 exit was 2 because Go exits 2 on an uncaught panic; the
typed error now exits 1 via the normal error path — documented.)

## 3. Substitution matrix (G8/G9/G10) — executed

| Attack | Expected | Actual |
| --- | --- | --- |
| A vault → A workspace | ACCEPT | ACCEPT (exit 0, VERIFIED) |
| B vault → B workspace | ACCEPT | ACCEPT (positive control, exit 0) |
| B vault → A workspace | REJECT | **REJECT** — `exit 1, "vault belongs to workspace \"wb\" but was opened as \"wa\": vault workspace identity mismatch"` |
| B vault → A, CLI (Stage 34 exact repro) | REJECT | REJECT (0 "panic" occurrences; 2 "identity/mismatch" occurrences) |
| vault-only swap, both directions (unit) | REJECT ×2 | REJECT ×2 (`ErrVaultIdentityMismatch`, identities named) |
| Truncated vault (16KB, CLI) | CLEAN FAILURE | `exit 1, "vault file is corrupt or truncated: invalid freelist page"` — 0 panic text |
| Page-header corruption (unit) | CLEAN FAILURE | `ErrVaultCorrupt`, no panic escape, handle closed |
| Pre-binding vault migration | ADOPT then enforce | ADOPT then enforce (unit-verified) |
| Post-migration substitution | REJECT | REJECT (unit-verified) |
| Meta-name altered to match directory (deliberate rebind) | disclosed limit | ACCEPT — out of the unsealed-binding boundary (§1); requires passphrase-held deliberate forgery |

## 4. Adversarial re-attack (G32) — attempted defeats and results

* *Copy B's vault only* → rejected (identity mismatch).
* *Delete/rename A's sidecar* → no sidecar exists in this design; nothing
  to delete (binding is inside the vault).
* *Restore an old A vault* → accepted only if its binding says "wa" —
  legitimate same-workspace recovery preserved (G13).
* *Copy the entire workspace directory* → the copy is bound to its
  original name; verification under a new name fails. Documented
  consequence of name-binding (G12 decision).
* *Rewrite meta with bbolt tooling* → succeeds (disclosed boundary §1).
* *TOCTOU (verify-then-replace)* → binding is re-evaluated on every open;
  the CLI verifies by opening fresh each run — no cached identity.
  Full TOCTOU-on-mmap protection is not claimed (documented).

## 5. Regression tests (G37) — permanent encoding

`internal/store/vault_binding_test.go`:
`TestVaultBindingAcceptsSameWorkspace`, `TestVaultBindingRejectsSubstitution`,
`TestVaultBindingRejectsVaultOnlySwapBothDirections`,
`TestVaultTruncatedTypedErrorNoPanic` (asserts `ErrVaultCorrupt`; a panic
escape fails the test), `TestVaultBindingMigratesPreBindingVault`
(migration + post-migration enforcement). Plus
`TestTornWriteRejection/bitflip_live_data` (integration; corruption
located by diffing against a same-code empty vault — layout-independent,
see §6).

## 6. Collateral fix discovered by the gates

The first post-fix integration run failed
`TestTornWriteRejection/bitflip_first_data_page`: the test hard-coded
offset 8192 as "first data page"; the identity binding shifted the page
layout and the flipped byte landed in dead space where ACCEPTED was
correct. The subtest now locates live bytes by diffing against a
same-code empty vault (layout-independent), mirroring the Stage 34
forensic methodology. Full integration suite: PASS (30.7s).
