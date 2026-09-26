# Stage 52 — Certification

```text
date:     2026-09-25
HEAD:     a7eab345fb641dedb2f45f059b2ce2a6bae968c4 (working tree DIRTY)
VERSION:  5.0.0-alpha1  (unchanged)
tags:     no v5.* tag exists; none created
verdict:  STAGE 52 — PARTIAL
          STAGE 47 — BLOCKED
          STAGE 47B — BLOCKED-WITH-OWNER
          G3206 — OPEN
```

## What is certified

A read-only operator dashboard, served by one Go process over loopback, that:

- shows an identity graph with a Barnes-Hut layout computed in WebAssembly and
  drawn on a Canvas 2D context;
- shows time travel and differential mode anchored to real audit chain sequence
  numbers, reconstructed on the server from signed history;
- shows **two** independent verification verdicts — one computed by
  `internal/store` in Go, one computed by `verify.wasm` in the operator's own
  browser tab — and never merges them;
- detects tampering end to end, which was demonstrated rather than asserted;
- ships every asset embedded, with a strict CSP and no external request.

Every gate executed for this verdict is recorded in
`artifacts/stage52/gate-matrix.md`, and every gate passed.

## What is not certified, and is not claimed

1. **G3206 remains open.** Stage 47 and 47B are unchanged. No Stage 52 work
   touched AD DS, KDC, AD CS, enrollment, or PKINIT, and nothing here should be
   read as partial progress on them.
2. **Five specified requirements were not met as written**: WebGL rendering, a
   Web Worker layout, the 50,000-node/60 FPS target, vendored HTMX, and embedded
   web fonts. They are recorded as `NOT SATISFIED` in the gate matrix with the
   substitute that shipped and what it costs. They are deviations, not pending
   tasks.
3. **The performance gate was not measured at all.** There is no node-count
   sweep and no frame-time capture in this evidence. Claiming "works at scale"
   would be unsupported.
4. **The tree is dirty.** This certifies a working tree, not a commit. A
   reviewer reproducing these results from HEAD alone will not get them.
5. **One browser.** Chrome only. Firefox and Safari are not installed here and
   were not exercised.
6. **The dashboard's chain has no producer in the shipped CLI.** A stock run
   yields only the dashboard's own lifecycle entries, so time travel and
   differential mode have nothing to replay unless other tooling writes graph
   operations to the same file. This is named in `docs/stage52-dashboard.md`
   rather than hidden behind a fixture.

## Bugs this stage found and fixed

Recorded because each was invisible to the tests that existed before it, and each
was found by a gate that was written to look for exactly that failure:

| Bug | Symptom | Found by |
|---|---|---|
| `html/template` name collision | The whole template set failed to parse; no page would render | `TestTemplatesRenderWithoutError` |
| Partials rendered as empty pages | Every partial executed its *file* template (a few whitespace characters) instead of its `define` block, and returned 200 | `TestTemplatesRenderWithoutError` with content markers |
| Directory listing served | `/static/` and `/wasm/` listed the embedded tree, because the guard sat inside `StripPrefix` and saw a path with no trailing slash | `TestDirectoryListingIsRefused` |
| `graph.wasm` refused a coincident seed | The dashboard seeds every node at the origin on a first render; the ABI returned an error and the graph page sat on "loading graph…" with no explanation | `an_all_coincident_seed_still_lays_out` |
| `WebAssembly.Instance` used as its own exports | Every layout call threw `aether_alloc is not a function`; the graph page never drew | `TestBrowserReachesItsOwnVerdict` |
| Module-load race | The first graph fetch could run before `graph.wasm` finished loading, handing the layout a null module | same |
| Browser verdict lost on every poll | The chain-status banner is replaced every 5 s and came back with the "verifying…" placeholder, so a proven chain looked unverified five seconds after proving itself | same |
| CSP silently dropping styles | ~30 `style` attributes across the templates were being discarded by `style-src 'self'`, with the violations visible only in the browser console | the same run, then `TestTemplatesContainNoInlineHandlers` |
| `selftest()` always returned `"ok"` | A stub that could report health for a module that had lost its verification step | review, while making the smoke-test assertion stronger |
| Encoding damage in three templates | An em dash had been lost to a `Set-Content` round trip and rendered as U+FFFD | `TestTemplatesContainNoInlineHandlers` |
| Misleading tamper warning | "linkage first breaks at seq 0" was printed for a chain whose only fault was a rewritten payload | reading the captured evidence |

## Recommended next steps

In priority order, and none of them is "continue Stage 47":

1. **Commit the stage**, so the evidence refers to something reproducible. The
   gate matrix currently certifies a dirty tree and says so.
2. **Decide where graph operations are recorded**, and expose a shipped path for
   it. Until then the dashboard's headline features are demonstrated but not
   reachable in ordinary use.
3. **Decide the WebGL question on the merits.** The Canvas 2D renderer works and
   is verified; whether it is fast enough at the required scale is a measurement
   nobody has taken. A node-count and frame-time sweep would turn this from a
   judgement call into a fact.
4. **Add a test that fails when a template uses an unimplemented `hx-*`
   attribute**, so the hand-written runtime cannot silently grow dead controls.
5. **Stage 47 stays blocked** until an owner provisions Windows Server 2022 with
   AD DS, KDC and AD CS, and produces the evidence in
   `artifacts/stage47b/gate-matrix.md`.
