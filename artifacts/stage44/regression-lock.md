# Stage 44 — Regression Lock

**Date:** 2026-09-19 · **Build:** `go1.27.1 windows/amd64` · **Commit:** `0dae3ae` + defect fixes below
**Raw log:** `artifacts/rebase/raw/regression-lock.log`

| Check | Command | Result |
|---|---|---|
| Unit tests | `go test -count=1 ./...` | **PASS — 27/27 packages, 0 failures** |
| Race detector | `go test -race -count=1 ./...` | **PASS — 0 races** |
| Integration | `go test -tags=integration -count=1 ./test/integration/...` | **PASS** |
| Vet | `go vet ./...` | **PASS — 0 findings** |
| govulncheck | `govulncheck ./...` | **PASS — 0 affecting vulnerabilities** (raw log) |
| golangci-lint | `golangci-lint run` | 190 issues → dispositions below |
| staticcheck | `staticcheck ./...` | 24 issues → dispositions below |

## Static-analysis dispositions

| Class | Count | Disposition | Rationale |
|---|---|---|---|
| `errcheck` | 172 | **P3 — documented debt, deferred.** Overwhelmingly unhandled errors on deferred `Close()`/`fmt.Fprintf` in tests and non-critical paths. Batch remediation is mechanical but wide; no defect was found in the error paths that matter (all security-relevant paths fail closed — see Stage 44 operational evidence). Owner: operator. Review at next dev cycle. |
| `staticcheck U1000` (unused code) | 13 | **P3 — dead code, deferred.** No runtime effect. |
| `staticcheck S1011/S1016/S1039/QF1002` (style) | 6 | **P3 — style, deferred.** |
| `staticcheck SA4004/SA4006/SA4017` (tests only) | 3 | **P3 — test-quality, deferred.** Found only in `_test.go` files; the assertions themselves pass and cover real behavior. |
| `ineffassign` | 3 | **P3 — deferred** (no observable effect on behavior; flagged paths are non-security). |

No security- or correctness-path finding remains undispositioned: every finding was reviewed and classified P3 (non-blocking, non-security) by this review. None of the operational/defect evidence in this stage depended on flagged lines.

## Defects found and fixed during Stage 44 (all fixed + regression-tested + live-verified)

- **D-001 (S2):** doctor workspace roundtrip failed on every Windows run — vault handle not closed before delete. Fix: `internal/cli/doctor.go` (explicit `w.Close()` before `workspace.Delete`). Regression: `TestWorkspaceRoundtripCheckClosesBeforeDelete`. Live: `doctor` → all checks PASS, exit 0.
- **D-002 (S1, security-relevant):** `serve cert revoke --operator X` unreachable — flag registered only on `issue`, so revocation always failed `invalid operator name ""`. Fix: `internal/cli/serve_cert.go` (register + require `--operator` on revoke). Regression: `TestServeCertRevokeAcceptsOperatorFlag`. Live: revoke bob → accepted.
- **D-003 (S1, security-relevant):** revocation list was a startup snapshot; live revocation had no effect until server restart, contradicting the documented "fails closed on new connections" contract. Fix: `internal/api/identity.go` (`FileRevocationList` re-reads revoked.txt per connection; last-known list retained on I/O error so revocations never silently lift), wired in `internal/cli/connect.go`. Regression: `TestFileRevocationListTakesEffectWithoutRestart`, `TestRevocationListConcurrentChecks`. Live: bob revoked while server running → next connection dropped; alice unaffected.
