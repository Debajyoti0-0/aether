# Stage 4 Backfill — Release Surface (B4-G16 / WS9)

## 1. Version discipline

| Item | Value | Rule |
|---|---|---|
| Current `VERSION` | `4.0.0-rc1` | **UNTOUCHED** — no regression; backfill does not modify it |
| Live tag | `v4.0.0-rc1` | **UNTOUCHED** (re-pointed by concurrent work to `28bcb99` before this stage; not moved again) |
| Historical tag | `v3.5.0-stage4-backfill` | created on the backfill commit; historical marker only |

Version-monotonicity: acknowledged in `docs/stage4-backfill-baseline.md`
§1. The historical tag documents the backfill lineage position; it does
not and must not be consumed as a current version by any tooling.

## 2. Backfill changes on the release surface

**Code (compatible, additive):**
- `internal/store/vault.go` — new exported sentinel `ErrVaultLocked`;
  lock-timeout error now wraps it (`errors.Is`-compatible; message text
  preserved as prefix). No behavior change otherwise.
- `internal/api/identity.go` — `validateOperatorName` now rejects
  non-ASCII characters (homoglyph/IA5String guard). Previously accepted
  names containing non-ASCII now fail closed. **Behavior change, security
  hardening** — release note required.
- `internal/api/ca.go` — `IssueOperatorCert` gains an IA5String issuance
  guard + post-issue DER reparse/SAN assertion. Rejects at issuance what
  the identity layer would reject. No valid-name behavior change.

**Tests (additive, build tag `integration`):**
- `test/integration/storage_safety_test.go` — multi-process lock, 8-cell
  crash matrix, torn-write rejection, spine tamper detection.
- `test/integration/pki_lifecycle_test.go` — 10 PKI lifecycle subtests.
- `test/integration/endurance_test.go` — 300 governed runs + 20
  reconnect cycles.

**Docs (additive):** `docs/stage4-backfill-*.md`, `docs/stage4-admission-semantics.md`.
**Artifacts (additive):** `artifacts/stage4-backfill/` (logs, tool output, checksums).

**Schema/migration impact:** none. Vault `SchemaVersion` remains `1`; no
store format changed; no CLI flags changed; no configuration keys changed.
