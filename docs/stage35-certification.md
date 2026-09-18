# Stage 35 Certification — Audit Vault Identity Binding + Storage Fail-Closed Remediation

```
STAGE 35 FINAL VERDICT

Stage:                      35 (security remediation: F-34-4 + F-34-3)
Execution status:           COMPLETE
Track A — v4.1.0:           BLOCKED — time gate (window closes 2026-10-17T16:55:20Z)
Track B — v4.2.0:           CONTROLLED-PUBLICATION-READY (unchanged scope)
Current version:            4.2.0-rc1
Current HEAD:               master (remediation commits 822fd96…, test fix 87a76ca)
Current tag:                v4.2.0-rc1 → bb56cf3 (on origin, unchanged)
Remote master:              fc062e0 (stale — unpublished stage work; push = operator action)
Working tree:               clean

Observation window:         OPEN, attached to v4.1.0-rc2 (bc674af1)
Window closed:              NO
P0: 0   P1: 0   P2: 0 (F-34-3 and F-34-4 now CLOSED)   P3: 1 (D-29-2, accepted)

Signing:                    IMPLEMENTED — UNEXECUTED (unchanged)
Provenance:                 IMPLEMENTED — UNEXECUTED (unchanged)
LIVE-1:                     BLOCKED-WITH-OWNER (unchanged)

F-34-3:                     CLOSED — corrupt vault → typed ErrVaultCorrupt,
                            clean diagnostic, exit 1, ZERO panic text, handle
                            released; regression test encodes it
F-34-4:                     CLOSED — cross-workspace substitution rejected with
                            identity-named error; same-passphrase and
                            empty-vault classes rejected; positive controls
                            VERIFIED; disclosed boundary: unsealed meta rebindable
                            by a passphrase-held, tooling-capable attacker
F-34-1 / F-34-2:            FIXED (Stage 34), regressions intact
DEBT-1:                     carried (long-tail behavioral comparison, Stage 35+)

Open blockers:              CI revival (gh auth login + push master) — unchanged
Deferred debt:              long-tail flag comparison; ARM64 runtime; LIVE-1
Publication scope:          v4.2.0-rc1 controlled publication; GA-READY remains
                            PROHIBITED (signing/provenance UNEXECUTED)

Final certification:        SECURITY REMEDIATION VERIFIED
                            (both findings closed with reproduction → design →
                            implementation → regression → adversarial re-attack →
                            live CLI verification; full gates green)
```

## Gate results (G581–G600)

| Gate | Result | Evidence |
| --- | --- | --- |
| G581 | Baseline | `stage35-baseline-lock.md`; tags unchanged; tree clean |
| G582–G584 | F-34-3 implemented/verified/regressed | recover boundary spans open+init+bind; `TestVaultTruncatedTypedErrorNoPanic`; CLI: exit 1, 0 panic occurrences |
| G585–G589 | F-34-4 design/impl/attack/control/regressed | Option A; substitution matrix §3; `vault_binding_test.go` (5 tests) |
| G590 | build/vet/unit | PASS (29 packages) |
| G591 | integration | PASS (30.7s full suite — includes the layout-independent torn-write fix) |
| G592 | race | PASS (29 pkgs, 0 DATA RACE, CGO/mingw) |
| G593 | fuzz | 21/21 targets, 0 crashes (60s each) |
| G594 | lint + govulncheck | 0 / 0 affecting |
| G595 | tag integrity | 3 tags unchanged; v4.1.0 absent |
| G596–G597 | observation window | OPEN; events appended; 0 new P0/P1 |
| G598–G599 | docs ≤2; no PARTIAL | certification + remediation + baseline-lock (long-charter names); self-audit clean |
| G600 | Stage 36 handoff | below |

## Historical truth

Stage 34's `PARTIALLY VERIFIED` stands as recorded; Stage 35 upgrades the
two open P2s to CLOSED with the full finding lifecycle (reproduction →
root cause → design → implementation → regression → adversarial re-attack
→ clean evidence). `v4.2.0-rc1` remains the current candidate — **this
remediation does not promote it and does not transfer the observation
window.** A `4.2.0-rc2` is NOT created: the storage hardening is a
behavioral security improvement, and the next candidate cut (after the
4.1.0 promotion gate) will carry it; `v4.2.0-rc1` remains immutable at
`bb56cf3`.

## Stage 36 handoff

Stage 36 begins on/after 2026-10-17T16:55:20Z: verify `v4.1.0-rc2` →
`bc674af1`; audit the observation log (zero P0/P1 required — currently
true); clean-room build `bin/aether-rc2` from `bc674af1`; run unit +
integration + crash-matrix + release-surface matrix; then PROMOTE
(`VERSION=4.1.0` bump commit from `bc674af1`, annotated `v4.1.0`, push,
remote verification, fresh clean-room rebuild) or RETAIN with the
specific reason. Parallel 4.2.0 path: the trust chain (signing,
provenance, vacuous-pass guards, windows-sign alignment) is fully
implemented and needs `gh auth login && git push origin master` to
execute; F-34-3/F-34-4 remediation should ship in the next 4.2.0
candidate (`4.2.0-rc2`) with its own clean-room verification. Rollback
baseline: `v4.0.0-rc2` (`5cd008be`).
> Corrected 23 → 21 in Stage 40; the count discrepancy is documented in docs/stage40-corrections-register.md.