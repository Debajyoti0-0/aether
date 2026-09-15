# Stage 4 Backfill — Final Report (B4-G18 equivalent / directive §9)

Final commit: b807ad2 (stage4 backfill: storage & spine hardening evidence)
Historical tag: v3.5.0-stage4-backfill · VERSION: 4.0.0-rc1 (untouched)

## A. Baseline
- Audit start: `58e6432` (v4.0.0-rc1); lineage root `990426b` (3.4.0-stage3) verified as ancestor.
- Vault backend: bbolt v1.3.8, SchemaVersion 1, exclusive flock, fsync-per-audit-append.
- Pre-existing storage tests: 6 files / ~1531 lines (inventoried in baseline doc).

## B. Audit findings
- Stage 4: ⊘ ABSENT (no commits, no tags — `git log --all` empty).
- Archived Stage 5 report `d250d52`: ABSENT (invalid object in both repos) → WS0 = re-execute.
- Directive citation `66b3600` contradicted (not a valid object; live truth is `v4.0.0-rc1` → `58e6432`/`28bcb99`).
- Real gaps found and closed: no typed lock error; operator names not ASCII-restricted; no IA5String issuance guard; no multi-process/crash/torn-write/endurance executable evidence.

## C. Implementation
- Code: `ErrVaultLocked` sentinel (vault.go); ASCII-only operator names (identity.go); IA5String issuance guard + post-issue DER reparse (ca.go).
- Tests: storage_safety_test.go (multi-process, 8-cell crash matrix, torn-write, spine tamper), pki_lifecycle_test.go (10 subtests), endurance_test.go (300 runs + 20 reconnects).
- Docs: baseline, lineage, admission semantics, deferred register, release surface.
- Schema/migrations: none. CLI: none. Config: none.

## D. Security
- Typed fail-closed multi-process lock errors; ASCII/homoglyph operator identity guard; IA5String guard; tamper detection proven (pinpointed seq); torn writes rejected; failed/crashed writers never yield false positives; no secrets in any artifact (synthetic names only).
- Remaining limitations: B4-DEF-01 (free-space flips), B4-DEF-02 (truncated-vault fault isolation), B4-DEF-03 (name-list revocation only), B4-DEF-06 (audit key same trust domain).

## E. Tests (actual)
- `go build ./...` exit 0 · `go vet ./...` exit 0 · `go test -count=1 ./...` 27/27 ok
- `go test -tags=integration -count=1 ./test/integration/...` ok (12.78s)
- Crash matrix 8/8 · PKI 10/10 · Torn-write 5/5 · Multi-process PASS · Spine tamper PASS · Endurance PASS (300+20)
- golangci-lint: 0 issues · govulncheck: 0 affecting
- Race: CI-authoritative (gcc absent; B4-DEF-05)

## F. Compatibility
- API: additive only (`ErrVaultLocked` new). One intentional security behavior change: non-ASCII operator names now rejected (release-note required).
- Store/schema: unchanged (SchemaVersion 1). CLI/config: unchanged.

## G. Remaining gaps
- See docs/stage4-backfill-deferred.md (7 items, owner/risk/target/expiry).
- Stage 5 backfill: NOT EXECUTED in this session (B5-G01.4 prerequisite `d250d52` remains ABSENT; B5 re-execution scope — 32 protocol fixtures, interop matrix, fuzz corpus, SBOM — is the next session's work).

## H. Release decision

STAGE 4 COMPLETE WITH EXPLICIT LIMITATIONS

(All 16 gates PASS or CI-AUTHORITATIVE; limitations are registered deferred items, none used to waive a gate.)
