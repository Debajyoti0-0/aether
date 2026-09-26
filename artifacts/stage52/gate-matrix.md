# Stage 52 — Gate Matrix

```text
HEAD:   a7eab345fb641dedb2f45f059b2ce2a6bae968c4
TREE:   DIRTY (Stage 52 is uncommitted; see "Repository state" below)
VERSION: 5.0.0-alpha1  (unchanged; no tag created)
DATE:   2026-09-25

RESULT LEGEND
  PASS          executed, criterion met, evidence captured
  PARTIAL       part executed and met; remainder impossible here (reason given)
  NOT SATISFIED the criterion is a real requirement and it is NOT met
  NOT PERFORMED the test could not be executed at all (environment absent)
  NOT REACHED   depends on a hard gate that failed; never attempted
  DENIED        an explicit authorization decision, recorded
```

Stage 52 is **PARTIAL**. The dashboard, the in-browser verifier and the graph
renderer work and are checked end to end in a real browser. Three requirements
were not met as written, and none of them is presented as met.

---

## Hard dependency: G3206

| Gate | Requirement | Result | Note |
|---|---|---|---|
| G3206 | Windows Server 2022 AD DS + KDC + AD CS evidence | **NOT SATISFIED** | Unchanged and still open. Stage 47 and 47B remain `BLOCKED` / `BLOCKED-WITH-OWNER`. No Stage 52 work closes it, and nothing in this stage was allowed to imply otherwise. |

Stage 52 does not depend on G3206 for anything except the *identity data* it
displays. The chain, the signatures and the layout are all proven here with
locally generated data; the AD-specific evidence remains outstanding.

## Delivered surface (G52-01 … G52-30)

| Gate | Requirement | Result | Evidence |
|---|---|---|---|
| G52-01 | Read-only routes only | **PASS** | `TestWriteMethodsAreRefused`; no POST/PUT/PATCH/DELETE route is registered at all |
| G52-02 | Loopback by default | **PASS** | `TestMustAddrDefaultsToLoopbackAndRejectsRemoteWithoutBothSwitches` |
| G52-03 | Remote binding requires two switches | **PASS** | same test; `bind-all` alone is refused |
| G52-04 | All assets embedded, no CDN, no runtime dependency | **PASS** | `go:embed` over `static/`, `templates/`, `wasm/`; `AssetPresence` gate refuses to start on a missing asset |
| G52-05 | Startup reports what is embedded | **PASS** | `aether dashboard view` prints the asset inventory with sizes; captured in `browser/README.md` |
| G52-06 | CSP present and strict | **PASS** | `TestSecurityHeaders`: `script-src 'self' 'wasm-unsafe-eval'`, no inline script or eval |
| G52-07 | Templates carry nothing the CSP refuses | **PASS** | `TestTemplatesContainNoInlineHandlers` — this test was added after the browser run showed real `style-src` violations being dropped at runtime |
| G52-08 | No directory listing of embedded assets | **PASS** | `TestDirectoryListingIsRefused`; the wrapper had to be moved outside `StripPrefix` to make it effective |
| G52-09 | Untrusted labels are escaped | **PASS** | `TestGraphNodeEscapesUntrustedLabel` |
| G52-10 | Genuine Ed25519/SHA-256 verification, not a stub | **PASS** | `internal/store/audit.go`; Node smoke test signs with `node:crypto` and verifies through `verify.wasm` |
| G52-11 | Two independent verdicts, both shown | **PASS** | `verify-output.json`: `browser: valid (22/22)` and `server: valid` on one page |
| G52-12 | Disagreement is surfaced, not resolved | **PASS** | separate `verdict-agree` / `verdict-disagree` chips; see G52-20 for the tamper run |
| G52-13 | Browser module actually loads under the CSP | **PASS** | `TestBrowserReachesItsOwnVerdict` and `browser/verify-output.json` |
| G52-14 | Verifier self-test is real | **PASS** | `selftest()` verifies an embedded vector signed by an independent implementation **and** requires the same vector to be rejected under an unrelated key |
| G52-15 | Time travel from signed history | **PASS** | `TestDiffIsAnchoredToChainEntries`, `TestTimeTravelIsAnchoredToARealEntry`, `TestReplayUsesTheNearestCheckpointPlusForwardOps` |
| G52-16 | Differential mode is chain-anchored | **PASS** | same tests; both endpoints are chain sequence numbers |
| G52-17 | Deterministic choke-point analysis | **PASS** | `TestChokePointAnalysisIsDeterministic`, `TestChokePointAnalysisIsBounded` |
| G52-18 | WASM layout is real and deterministic | **PASS** | `cargo test` 17/17; `smoke-test.mjs` pins the C ABI and layout determinism |
| G52-19 | Graph page renders in a real browser | **PASS** | `browser/verify-output.json`: 5 nodes, 5 edges, all coordinates finite, quadtree cells reported |
| G52-20 | Tampering is detected, end to end | **PASS** | `browser/tamper-output.txt`: browser verdict `valid: false`, `tamperedSeq: [2]`, with the reason named |
| G52-21 | WebSocket is push-only | **PASS** | `TestWebSocketAcceptsAClientAndRefusesCommands`; no command path exists |
| G52-22 | `dashboard.read` is not a default capability | **PASS** | `internal/api/capabilities.go`; `checkDashboardViewCapability` refuses an operator without it |
| G52-23 | Proof bundles are self-contained | **PASS** | `TestProofBundleIsSelfContained` |
| G52-24 | Go build / vet / test / race | **PASS** | `baseline/quality-gates.txt`, all exit 0 |
| G52-25 | Rust tests for both crates | **PASS** | `wasm/cargo-test-graph.txt` 17/17, `wasm/cargo-test-verify.txt` 14/14 |
| G52-26 | Cross-implementation ABI and crypto smoke test | **PASS** | `wasm/smoke-test.txt` 56/56 |
| G52-27 | Fuzzing of the four input surfaces | **PASS** | `race/fuzz-*.txt`; 4 targets × 20 s, no crash |
| G52-28 | Shipped JavaScript parses | **PASS** | `TestEmbeddedJavaScriptParses` runs `node --check` on every shipped script |
| G52-29 | `VERSION` unchanged, no tag | **PASS** | `5.0.0-alpha1`; `git tag` lists no `v5.*` |
| G52-30 | Evidence is reproducible | **PASS** | `scripts/stage52-evidence.ps1` regenerates every gate in this table except the live browser run, which is scripted in `browser/README.md` |

## Requirements not met as written (G52-31 … G52-36)

| Gate | Requirement | Result | What was done instead |
|---|---|---|---|
| G52-31 | 50,000 nodes at 60 FPS | **NOT SATISFIED** | Not measured, and not claimed. The layout runs in WASM and the draw pass is Canvas 2D. No frame-time or node-count benchmark exists in this evidence. |
| G52-32 | WebGL rendering | **NOT SATISFIED** | Canvas 2D with a Barnes-Hut layout in `graph.wasm`. The choice and its cost are documented in `docs/stage52-dashboard.md`. |
| G52-33 | Web Worker layout | **NOT SATISFIED** | No worker. Layout runs on the main thread; a large graph will block input, and nothing here measures how badly. |
| G52-34 | Vendored HTMX | **NOT SATISFIED** | A ~300-line `hx-*` runtime in `static/app.js` covering exactly the attributes the templates use. `htmx.min.js` is not shipped. |
| G52-35 | Embedded web fonts | **NOT SATISFIED** | System font stack. No `.woff2` is shipped, so the dashboard makes no external request. |
| G52-36 | Stage 47 AD CS evidence | **NOT REACHED** | Depends on G3206, which is `NOT SATISFIED`. Never attempted. |

`G52-31` through `G52-35` are recorded as deviations, not as pending work with a
due date: they are the result of choosing a smaller, dependency-free
implementation over the specified one. Anyone deciding whether that was the right
call should read `docs/stage52-dashboard.md` and judge it on the merits.

## Tooling that was not available

| Tool | Result | Consequence |
|---|---|---|
| `golangci-lint` | **NOT PERFORMED** | Not installed. `go vet` and `gofmt` were run instead; `gofmt` reports pre-existing misalignment in ~100 untouched files, which was left alone. |
| `staticcheck` | **NOT PERFORMED** | Not installed. |
| `govulncheck` | **NOT PERFORMED** | Not installed. No dependency-vulnerability claim is made for Stage 52. |
| `clippy` (stable toolchain) | **NOT PERFORMED** | Not installed for the stable toolchain. |
| `wasm-opt` (via wasm-pack) | **DENIED** | The bundled binary rejects the bulk-memory operations the verifier emits. Builds use `--no-opt`; the Rust release profile still applies LTO, size optimisation and stripping. This is a tooling limitation, not a code change. |
| Firefox, Safari | **NOT PERFORMED** | Not installed. Only Chrome was exercised. |

## Repository state

The work is **uncommitted**: `internal/web/` and `internal/cli/dashboard_view.go`
are untracked additions, and `internal/store/audit.go`,
`internal/engine/graph/graph_builder.go`, `internal/api/capabilities.go`,
`internal/cli/v3c.go`, `Makefile.wasm` and `.gitignore` are modified. `git status
--porcelain` at the time of writing lists 12 entries.

This is stated rather than hidden because the gate matrix above is otherwise
read as a certification of a specific commit, and it is not: it is a
certification of a working tree.

## Evidence index

```text
baseline/environment.txt        toolchain and HEAD
baseline/quality-gates.txt      every gate above, with exit codes
baseline/go-*.txt               raw build, vet and test output
baseline/asset-checksums.txt    SHA-256 of every shipped asset
wasm/cargo-test-graph.txt       17 tests
wasm/cargo-test-verify.txt      14 tests
wasm/smoke-test.txt             56 cross-implementation checks
race/go-test-race-web.txt       -race on internal/web
race/fuzz-*.txt                 four fuzz targets, 20 s each
browser/README.md               how the browser run was produced
browser/verify-output.json      the real verdict, with graph report
browser/tamper-output.txt       the tampered chain, rejected by the browser
```
