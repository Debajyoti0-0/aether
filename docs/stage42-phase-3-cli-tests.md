# Stage 42 — Phase 3 Regression Test Coverage

The external report's structural finding — `internal/cli` had ZERO test files
while 5 of its bugs live there — is closed. All new tests are fails-before/
passes-after regressions pinned to the charter fix IDs.

## New test files (before: `internal/cli` had 0, `internal/rl` had 0 regression files)

| File | Tests | Fixes pinned |
| --- | --- | --- |
| `internal/cli/regression_test.go` | `TestServeCertOperatorFlagRegistered` | H1 (fails-before: unknown flag) |
| | `TestRunCmdPassphraseFlagRegistered` | L2 (fails-before: dead var) |
| | `TestResolveServerCAFallback` | H2 (4-assertion unit on `resolveServerCA`) |
| | `TestWorkspaceFromLine` | H3 (space + `=` + absent forms) |
| | `TestIntentWhitelistFailClosed` | M2 (whitelist equality + refusal vocabulary, forever) |
| | `TestProvidersHTTPTimeout` | L5 (positive timeout) |
| | `TestDoctorProbeRoundtrip` | H4 (fails-before on Windows: "file in use") |
| `internal/rl/regression_test.go` | `TestIsSpineGovernable` | H5 (whitelist unit) |
| | `TestGenerateRLPlanSpineOnly` | H5 (fails-before: best learned action `graph build` minted) |

## Updated existing suites (pin the charter fixes)

| File | Change | Fixes pinned |
| --- | --- | --- |
| `internal/api/dashboard_test.go` | query-param block flipped: `?token=` now expects **401** | L4 (channel removal pinned forever) |

## Verified

```
go test ./internal/cli/ ./internal/api/ ./internal/rl/ -count=1 -run "TestServeCert|TestRunCmd|TestResolve|TestWorkspaceFromLine|TestIntentWhitelist|TestProvidersHTTP|TestDoctorProbe|TestDashboard|TestIsLoopback|TestIsSpineGovernable|TestGenerateRLPlanSpineOnly" -v
→ 14 PASS, 0 FAIL (abridged):
  --- PASS: TestServeCertOperatorFlagRegistered
  --- PASS: TestRunCmdPassphraseFlagRegistered
  --- PASS: TestResolveServerCAFallback
  --- PASS: TestWorkspaceFromLine
  --- PASS: TestIntentWhitelistFailClosed
  --- PASS: TestProvidersHTTPTimeout
  --- PASS: TestDoctorProbeRoundtrip (0.06s)
  --- PASS: TestDashboardAuth (+ updated query block)
  --- PASS: TestIsSpineGovernable
  --- PASS: TestGenerateRLPlanSpineOnly
```

Full-suite verification: `go test -count=1 ./...` → 29 packages, 0 failures;
`go test -race -count=1 ./...` → 29 packages, 0 failures.

## Honest limitation

The M1 Action-wiring line (`RiskScore: estimated`) is not unit-intercepted:
`s.Run` executes the mutation, and the four intent executors attempt real
provider calls — an execution-backed unit test would require a provider stub
layer (out of scope). The wiring is verified by the estimator values being
non-zero (60/70) in `cliMutation` and the risk stage vocabulary remaining
fail-closed. Recorded as a known limitation, not a fabricated PASS.
