# Stage 52 — read-only dashboard design, and what it does not do

This is the design record for `aether dashboard view`. It exists because the
implementation deliberately departs from the Stage 52 specification in five
places, and a departure that is not written down reads as a completed
requirement.

The gate-by-gate result is in `artifacts/stage52/gate-matrix.md`.

## What the thing is

A single Go process that serves, over loopback HTTP:

- a graph view (Canvas 2D, layout in WebAssembly),
- an audit view with time travel and differential mode, both anchored to real
  chain sequence numbers,
- a two-source verification banner: the server's verdict and the browser's, shown
  side by side and never merged,
- JSON and partial-HTML endpoints for the same data, and a push-only WebSocket
  that carries chain notifications and nothing else.

No route accepts a write. There is no POST, PUT, PATCH or DELETE handler, and
the WebSocket has no command path; `TestWriteMethodsAreRefused` and
`TestWebSocketAcceptsAClientAndRefusesCommands` assert both, because a
read-only claim that is only enforced by convention is not a read-only claim.

## The three things that matter

### 1. The operator's browser checks the chain, not the server

The server renders a verdict from `internal/store`. A second verdict is computed
in the operator's tab by `verify.wasm` (Rust, no dependencies), over the NDJSON
form of the chain and the public key, and both are displayed.

This is the part that makes the dashboard worth having. A server that both serves
the evidence and judges it can be wrong in exactly the way that matters. Two
implementations that share no code, over the same bytes, with disagreement shown
prominently, is a much weaker claim to make.

`selftest()` is a known-answer test, not a stub: it verifies an embedded chain
that was signed by Node's `crypto` — a third implementation — and then requires
that same chain to be **rejected** under an unrelated key. A module that had lost
its verification step would fail the second half.

### 2. Time travel and diffs are anchored to signed history

The slider is a chain sequence number, not an index into a client-side array. The
server reconstructs the graph at that position by taking the nearest checkpoint
at or before it and replaying forward, so what is drawn is the state the signed
history attests to. Nothing about the picture is computed in the browser from an
untrusted hint.

`RemoveNode` and `RemoveEdge` were added to the graph builder for this. Without
them a chain that recorded a removal could not be replayed, and the time-travel
control would have quietly shown a state that the history contradicts.

### 3. Choke-point analysis is exact, bounded, and says when it stopped early

Paths are enumerated with an explicit hop and count budget. If the budget is hit
the result is marked truncated with the cause, and an empty result is reported as
an empty result — never as "no choke point found", which is a different and much
stronger claim.

## Deliberate departures

| Specified | Shipped | Why, and what it costs |
|---|---|---|
| WebGL | Canvas 2D | The expensive part of a force-directed layout is the O(n log n) force computation, and that is in `graph.wasm`. The draw pass is a few thousand line segments, which Canvas 2D handles. The cost is that the 50,000-node/60 FPS gate is **not met and not measured**. A real WebGL renderer would need a second pipeline (buffers, shaders, picking) for a draw pass that is not the bottleneck. |
| Web Worker layout | main thread | Saves a worker protocol and a duplicated graph copy in exchange for blocking input during layout. For a large graph this is a visible stall. Not measured. |
| Vendored HTMX | ~300 lines of `hx-*` runtime in `app.js` | Only `hx-get`, `hx-trigger`, `hx-target`, `hx-swap` and `hx-include` are implemented, because those are the only attributes the templates use. The behaviour that matters — bind once, never rebind a swapped element, clear the poll timer when the element leaves the document — is covered by the runtime's own comments. The risk is that a future template uses an attribute that silently does nothing; there is no test that fails when that happens. |
| Embedded web fonts | system font stack | No font file means no external request and no FOUT. The dashboard looks like the operator's OS, which is arguably better. |
| HTMX + CSP | stricter CSP than needed | `style-src 'self'` with no `unsafe-inline`, so the templates carry no `style` attributes at all; the one dynamic value (a coverage bar's width) travels as `data-w` and is applied through the CSSOM. This was found the hard way: the browser run logged real `style-src` violations that were being dropped silently. |

## Where the chain comes from — and a gap worth naming

`aether dashboard view` reads a plain chain file and a key file
(`--audit`, `--audit-key`). The dashboard appends its own lifecycle entries
(`dashboard.start`, `dashboard.stop`) to that chain, so a dashboard that has been
started has a real, verifiable chain of its own.

But **no shipped CLI subcommand appends to a plain chain file.** `aether audit
record` writes into an encrypted workspace vault, and `aether export audit` emits
the chain *without* the signing key. So the practical effect is:

- time travel and differential mode have nothing to replay unless some other
  tooling writes graph operations to that file, and
- the only entries a stock run produces are the dashboard's own lifecycle ones.

This is a real integration gap, not a documentation nicety. It was left as-is
rather than papered over with a new CLI verb invented during an evidence pass;
`scripts/stage52-seed` exists only to produce the evidence chain, and says so in
its own package comment. Closing the gap properly means deciding where graph
operations are meant to be recorded and exposing that path — a product decision,
not a dashboard one.

## Security posture

- Loopback by default. `--bind-all` alone is refused; `--bind-all` **and**
  `--allow-remote-binding` are both required, and the CLI prints a warning naming
  what becomes reachable.
- `--operator` is an authorisation claim, and it is checked against that
  operator's capability file. `dashboard.read` is deliberately absent from the
  default capability set: the CLI already needs `read.graph` and `read.audit` to
  do its job, while this dashboard puts the whole engagement on one network
  surface. A missing capability file is a refusal, not a pass.
- `--require-capability` gates every request on `X-Aether-Capability`.
- TLS is opt-in and both `--tls-cert` and `--tls-key` are required together.
- The WebSocket is a dependency-free RFC 6455 implementation that accepts a
  client, sends notifications, and closes. It has no opcode path for anything
  else.

## Known weaknesses

- The partial runtime has no test that fails when a template uses an
  `hx-*` attribute it does not implement. A typo would be a dead control.
- The poll timer for the banner runs every 5 s and re-renders the banner. The
  browser verdict chip is repainted by a `MutationObserver` rather than by the
  swap path announcing itself, because making the poller announce itself would
  also make the graph refetch on every poll. That coupling is worth knowing about
  before changing either.
- There is no test that the WebSocket reconnect backoff behaves; there is a test
  that a client is accepted and that a command frame is refused.
- Only Chrome has been exercised.
