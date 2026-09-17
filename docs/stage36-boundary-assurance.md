# Stage 36 — Boundary Assurance: Adversarial Validation of the F-34-4/F-34-3 Remediation

Objective: does the Stage 35 remediation hold when the attacker stops
using the exact path that was tested? Every result below is from the
freshly built `4.2.0-rc1` binary (current master tree).

## 1. G2/G3 — call-graph audit: binding enforced at the trusted layer

```text
grep OpenVault|bolt.Open (non-test) →
  internal/store/vault.go       : bolt.Open (inside openAndBindVault's recover boundary)
  internal/workspace/workspace.go:154 : store.OpenVault (the only external caller)
```

**Exactly one vault-open path exists.** The identity binding lives inside
`OpenVault` — the lowest trusted layer — so every consumer (CLI commands,
engine, future callers) inherits it. No alternate entry point, no
CLI-only enforcement. Classification: **BINDING ENFORCED** (single path);
no BYPASS, no UNTESTED paths remain in the enumeration.

## 2. G1 — Stage 35 controls re-verified

| Control | Result |
| --- | --- |
| A→A, B→B (same passphrase) | ACCEPT, exit 0 — positive controls |
| B vault → A directory (populated) | REJECT, exit 1, identity-named error |
| B vault → A directory (**empty-state class**) | REJECT, exit 1 — **empty history is not an identity bypass** (G4) |

## 3. G5/G6 — plaintext-meta rebind attack (the documented limitation, exercised)

Attack: open the substituted vault with bbolt tooling and rewrite
`meta.workspace_name` from "wb" to "wa", then verify:

```text
REBOUND verify → exit 0, "VERIFIED (0 entries, 0 tampered)"
```

ACCEPTED — exactly the documented Stage 35 boundary. Consequence analysis
(G6):

* The attacker holds file-write **and** the workspace passphrase. With
  those, they can already forge the entire sealed audit chain (the
  record-layer key derives from the passphrase). The rebind therefore
  grants **no new capability class** — it makes erasure *cheaper*
  (rebind + empty vault vs. forging a full chain).
* What the binding does protect: every actor **without** the passphrase
  (the class that Stage 34's attack demonstrated) is now blocked.
* Classification (G29): **A — ACCEPTED ARCHITECTURAL BOUNDARY**, with
  the sharpened caveat recorded: the binding defeats substitution by
  non-passphrase holders and all accidental/lazy copies; it is not
  tamper-proof against the passphrase-holder. A passphrase-sealed
  binding (threading the passphrase into `OpenVault`) remains the
  documented hardening option and is deferred with rationale (API change
  across every caller; no demonstrated attacker without the passphrase
  is affected).

## 4. G8/G9 — rename and clone matrices

| Scenario | Result |
| --- | --- |
| `wb` renamed to `wc`, verify as `wc` | REJECT (exit 1) — name-bound, as documented |
| Renamed back to `wb`, verify | ACCEPT (exit 0) — no hidden rebinding occurred |
| `A→B→C→A` rename cycle | returns to ACCEPT — deterministic, no drift |
| Clone `wa` → `wd` (full directory copy), verify as `wd` | REJECT — a copied workspace does not acquire the new name's identity |

Clone semantics: a copied workspace directory is a *backup of the
original identity*, not a new workspace; to reuse it, the operator
creates a fresh workspace (new binding). Documented.

## 5. G16 — corruption campaign (60 randomized mutations)

Mutation classes: single/multi-byte flips, random truncation, page-aligned
truncation, 64-byte zero fills — seeded, isolated APPDATA, fresh binary.

```text
mutations: 60
rejected (non-zero exit): 60
accepted: 0
raw panic text: 0
false VERIFIED (populated history after corruption): 0
```

The F-34-3 boundary holds across all mutation classes; no corruption
escapes as a raw panic and no corrupted state produces success.

## 6. DEBT-1 reduction (G630) — 20 flags across 5 commands

Commands: `audit verify`, `audit record`, `workspace delete`, `tunnel`,
`cap list`. Per-flag: bare flag, `--flag=` empty value, plus prior
parse-class coverage.

| Probe class | Result |
| --- | --- |
| Empty `--workspace ""` on `audit record` | exit 1, "workspace name: name is required" — clean validation |
| `workspace delete --workspace "" --force` | **name validated before any destructive action; nothing deleted** — fail-closed PASS |
| Global `--config ""` / `--log-level ""` | empty values fall back correctly (F-34-2 normalization active) — PASS |
| Remaining bare/empty combinations | all deterministic, no silent acceptance of security-relevant empties |

Deferred remainder (long-tail behavioral comparison on the other ~21
commands): DEFERRED-WITH-OWNER (repository owner, Stage 37+).

## 7. G13 — symlink/reparse (Windows)

Junction/symlink redirection of `vault.db` requires elevated privileges
on this host to create reliably; marked **UNEXECUTED** (not PASS). The
TOCTOU property is likewise NOT claimed: the binding is re-evaluated on
every open (no cached identity), but a same-directory file swap between
open and read is not demonstrated as impossible.

## 8. G31 — no regression against Stage 35

Substitution matrix, empty-state, truncation, rename/clone, corruption
campaign, config fail-open warning, log-level normalization, token-leak
probe, workspace traversal — all re-executed this stage on the current
binary: **all green**. F-34-1/F-34-2 regression tests pass.
