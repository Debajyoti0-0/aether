# Stage 52 browser evidence

Everything in this directory was produced by driving a real Chromium browser
against a real `aether dashboard view` process. No step here inspects a
screenshot or trusts a status line: the harness reads the page's own state over
the DevTools protocol and fails if the page has not reached a verdict.

## How it was produced

```text
# 1. A chain and a graph to serve. This is evidence tooling, not a product
#    command: no shipped subcommand appends to a plain chain file (see
#    docs/stage52-dashboard.md, "Where the chain comes from").
go run ./scripts/stage52-seed %TEMP%\aether-s52-live

# 2. The real binary, over the real assets.
go build -o bin/aether-stage52.exe ./cmd/aether
bin\aether-stage52.exe dashboard view ^
  --graph  %TEMP%\aether-s52-live\aether-graph.json ^
  --audit  %TEMP%\aether-s52-live\aether-audit.jsonl ^
  --audit-key %TEMP%\aether-s52-live\aether-audit.key ^
  --port 18443 --workspace stage52-evidence

# 3. The browser check, in another shell.
node scripts/browser-verify.mjs http://127.0.0.1:18443 --timeout-ms 60000
```

`scripts/browser-verify.mjs` does three things and requires all of them:

1. Loads `/audit` and waits for `window.__aetherVerify.result` **and** for the
   banner chip to settle. Requiring both is deliberate: a chain update can
   replace the banner between a verdict being stored and being painted, and
   sampling one without the other reports a healthy page as broken, or a broken
   page as healthy.
2. Requires the browser chip to read `browser: valid` and the server chip to
   read `server: valid`, so the two verdicts are actually being compared.
3. Navigates to `/graph` and requires the renderer to hold as many nodes and
   edges as `/api/graph` reports, with every layout coordinate finite. This is
   the check that the layout module ran; a blank canvas is not a pass.

`verify-output.json` is the captured output of that run.

## The tamper run

`tamper-output.txt` is the same harness against a dashboard serving a chain whose
second entry had its `result` rewritten after signing, with the recorded `hash`
and `signature` left untouched (`scripts/tamper-chain.mjs`).

The harness exits non-zero, which is the correct outcome. The captured verdict:

```json
{
  "valid": false,
  "validCount": 23,
  "tamperedSeq": [2],
  "brokenChainAt": null,
  "notes": ["seq 2: recomputed hash does not match the recorded hash, so this
             entry was altered after signing"]
}
```

Two things are worth stating plainly about this run:

- The verdict came from `verify.wasm` in the browser tab, not from the server.
  The server's own verdict agreed, and the CLI said so on stderr before it
  started serving, but the browser chip is the one that matters for the claim
  that the operator's own machine checks the chain.
- The dashboard still *served* the tampered chain. That is a deliberate choice:
  hiding the evidence would make the failure invisible, which is worse. The
  banner is red, both chips say `INVALID`, and the disagreement chip is what an
  operator sees first.

## Not covered here

- No frame-rate or 50,000-node measurement was taken. The renderer is Canvas 2D,
  not WebGL, so the Stage 52 performance gate is `NOT SATISFIED` rather than
  `UNVERIFIED`; see `../gate-matrix.md`.
- Only Chrome was exercised. Firefox and Safari are `NOT PERFORMED`: neither is
  installed here, and neither was claimed.
