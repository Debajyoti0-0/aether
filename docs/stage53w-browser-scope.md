# Stage 53W — Browser Verification Scope

Captured (UTC): 2026-09-26. HEAD at time of decision: `4364ab4`.
Decision: **Option A — scope-reduce to Chromium, documented.** No code change.

## Correction to the inherited record

Stages 53S through 53V recorded the blocker as *"Firefox is not installed."*
That was wrong, and it was wrong in a way that pointed at the wrong remedy.

Measured on this host:

```
Get-AppxPackage *firefox*   ->  Name: Mozilla.Firefox   Version: 156.0.1.0
firefox --version           ->  Mozilla Firefox 156.0.1
```

Firefox 156.0.1 **is** installed and launchable. It is an AppX/WindowsApps
package, so it lives outside the standard `C:\Program Files\Mozilla Firefox\`
path, which is why every path-based check missed it and reported it absent.

The real blocker is architectural, not environmental.

## The actual constraint

`scripts/browser-verify.mjs` speaks the **Chrome DevTools Protocol** directly.
It has no `package.json` and no `node_modules`; puppeteer and playwright are both
absent. It launches a Chromium-family binary with `--remote-debugging-port`,
parses the `DevTools listening on ws://...` line, and drives the page over that
WebSocket. Its own failure message is explicit:

> `no Chromium-family browser found; set AETHER_BROWSER to one`

**Firefox does not implement the DevTools Protocol.** Installing it changes
nothing. Any amount of additional browser installation effort would not have
closed this gate.

## Why Option A is defensible for the current build

The scope reduction is only honest if the code under test is actually
engine-neutral. Verified rather than assumed:

| Engine-specific API | Present? | Consequence |
| --- | --- | --- |
| WebGL / WebGPU | **No** | No GPU/driver dependency, no silent-software-rasteriser divergence |
| `SharedArrayBuffer` / `Atomics` | **No** | No COOP/COEP cross-origin isolation requirement |
| Web Workers | **No** | No worker-origin differences |
| `WebAssembly` | Yes (4 files) | Baseline-supported in Chrome, Firefox and Safari |

The dashboard's graph renderer is **2D canvas**, and `internal/web/static/graph.js:3-18`
documents why WebGL was deliberately not used: layout runs in `graph.wasm` while
the draw pass is a few thousand line segments, and a WebGL path that silently
falls back to a software rasteriser is slower and less predictable.

So the surfaces exercised by the harness — Canvas 2D, WebAssembly, fetch, DOM —
are standards-based and implemented by all three engines. The harness's
limitation is CDP-specific, not verifier-specific.

**This is an architectural argument, not a test result.** Nothing has been
executed in Firefox or Safari, and this document does not claim otherwise.

## Declared support matrix

| Engine | Installed | Exercised by CI harness | Status |
| --- | --- | --- | --- |
| Chromium / Chrome | Yes | Yes (CDP) | **QUALIFIED** |
| Edge (Chromium-based) | Yes | Yes (CDP-compatible) | **QUALIFIED** by protocol equivalence, not by a separate run |
| Firefox | **Yes** (156.0.1) | **No** — CDP-only harness | **NOT PERFORMED** |
| Safari | Not installable on Windows | No | **NOT PERFORMED** |

**The project does not claim cross-browser support.** The browser-verification
claim is scoped to Chromium-family engines.

## Deferred work

Cross-browser verification requires a harness that can speak something other
than CDP. Two viable routes, both a dedicated stage:

1. **WebDriver BiDi** — Firefox 129+ ships BiDi; no `geckodriver` needed for the
   protocol itself. Fits the existing "no dependencies" posture of the harness
   better than a full WebDriver stack.
2. **Playwright** — one API across Chromium, Firefox and WebKit, but adds a heavy
   dependency tree and bundled browser downloads, which is a supply-chain
   decision in its own right (see `docs/stage53v-linter-scope.md` for the project's
   posture on unpinned tooling).

**Owner:** release engineer. **Target:** Stage 55 or 56, after Stage 52b's
dashboard work settles — rewriting the harness against a moving UI wastes the
effort.

Safari additionally requires macOS or a real device and **cannot be closed on
Windows at all**. If the project requires Safari support, that is a production
blocker requiring infrastructure this project does not control.

## What Stage 53T may and may not conclude

The browser cross-platform gate is `WAIVED-WITH-OWNER` for the purpose of
Stage 53T closure, on the explicit basis that:

- Chromium is qualified by executed runs.
- The unsupported engines are **NOT PERFORMED**, not passing, not merely untested
  in principle — they were never executed and are recorded as such.
- The scope boundary is documented here rather than left implicit in a harness
  that only ever searched for Chromium binaries.

This waiver covers *cross-browser qualification*. It does **not** cover browser
stability, cleanup, or process ownership, which remain separately gated on
Chromium evidence.

## Related known limitation, unchanged

`graph.js:17` records the 50,000-node performance claim as **UNVERIFIED**,
citing `docs/stage52-certification.md`. That remains true and is carried into the
Stage 52b handoff. Nothing in this document improves it.
