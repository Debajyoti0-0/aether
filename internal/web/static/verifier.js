// The browser-side, independent audit verification.
//
// This module is the whole point of the read-only dashboard: the server that
// serves the chain is not trusted to tell you whether the chain is intact. The
// Go side computes one verdict for the initial render; this computes a second
// one, in this tab, from the same bytes over an independent implementation
// (verify.wasm, Rust, no shared code with internal/store). Both verdicts are
// shown side by side and never merged, so a disagreement is visible instead of
// being resolved in the server's favour.
//
// It is an ES module rather than an inline script because the dashboard's CSP
// is `script-src 'self' 'wasm-unsafe-eval'`, which blocks inline script
// outright.

import init, { selftest, verify_chain_ndjson } from "./verify.js";

// The result of the last verification pass, kept so it can be re-painted after
// the partial runtime swaps the chain-status banner and resets the chip to its
// "verifying" placeholder. Without this, every WebSocket push would visibly
// flash the banner back to unverified.
let last = null;

// Promise for the harness in tests/ and for anything else that wants to wait
// for a verdict without polling the DOM.
const ready = (async () => {
  // The generated glue would otherwise resolve its default module name
  // relative to itself, i.e. under /static/, and the server publishes the
  // module at /wasm/. The URL is given explicitly so it loads from /wasm/.
  await init(new URL("/wasm/verify.wasm", window.location.origin));
  await run();
  return last;
})();

// A load failure must be visible, not silent. An uncaught rejection here would
// leave the chip reading "verifying…" forever, which is the one state this
// module exists to eliminate.
ready.catch((err) => {
  last = { valid: false, error: String(err) };
  paint();
});

// Exported for a headless check that the module really does load the real
// artifact and produce a real verdict, rather than a stub.
window.__aetherVerify = {
  ready,
  selftest,
  get result() {
    return last;
  },
};

// run fetches the chain and the key, then hands both to the WASM verifier.
async function run() {
  // NDJSON rather than the aggregate JSON: one entry per line is what a
  // streaming client can verify incrementally, and it carries nothing this
  // module did not ask for. The server's own verdict is deliberately not
  // passed in.
  const [ndjsonRes, keyRes] = await Promise.all([
    fetch("/api/audit/chain?format=ndjson", { credentials: "same-origin" }),
    fetch("/api/audit/pubkey", { credentials: "same-origin" }),
  ]);

  if (keyRes.status === 503 || ndjsonRes.status === 503) {
    // No chain or no key. The banner is already in its pending state, which is
    // the honest rendering of "nothing to verify".
    last = null;
    paint();
    return;
  }
  if (!ndjsonRes.ok || !keyRes.ok) {
    throw new Error(
      `fetch failed: chain=${ndjsonRes.status} pubkey=${keyRes.status}`
    );
  }

  const ndjson = await ndjsonRes.text();
  const { pubkey_base64: pubkey } = await keyRes.json();

  let parsed;
  try {
    parsed = JSON.parse(verify_chain_ndjson(ndjson, pubkey, false));
  } catch (err) {
    // The WASM boundary returns a JSON string. A parse failure means the module
    // returned something that is not a verdict, which is a failure of the
    // verifier itself and must not be reported as a valid chain.
    throw new Error(`verifier returned unparseable output: ${err}`);
  }

  last = {
    valid: parsed.valid === true,
    entryCount: parsed.entry_count,
    validCount: parsed.valid_count,
    tamperedSeq: parsed.tampered_seq || [],
    brokenChainAt: parsed.broken_chain_at ?? null,
    error: parsed.error || null,
    notes: parsed.notes || [],
    verifiedBy: parsed.verified_by,
    fingerprint: parsed.pubkey_fingerprint,
    at: new Date().toISOString(),
  };
  paint();
}

// paint writes the browser verdict into the banner. It is idempotent and safe
// to call when the banner is not on the page.
function paint() {
  const chip = document.getElementById("verdict-browser");
  if (!chip) {
    paintedChip = null;
    return;
  }
  paintedChip = chip;
  chip.classList.remove("ok", "bad", "wait");
  if (last === null) {
    chip.classList.add("wait");
    chip.textContent = "browser: nothing to verify";
  } else if (last.error) {
    chip.classList.add("bad");
    chip.textContent = "browser: ERROR";
    chip.title = last.error;
  } else if (last.valid) {
    chip.classList.add("ok");
    chip.textContent = `browser: valid (${last.validCount}/${last.entryCount})`;
    chip.title = `Computed by verify.wasm in this browser tab at ${last.at}`;
  } else {
    chip.classList.add("bad");
    chip.textContent = "browser: INVALID";
    const where = last.brokenChainAt === null ? "" : ` — linkage breaks at ${last.brokenChainAt}`;
    chip.title = `Computed by verify.wasm in this browser tab. ${
      last.tamperedSeq.length
        ? `${last.tamperedSeq.length} entries failed hash or signature checks${where}`
        : last.error || "verification failed"
    }`;
  }

  const server = document.getElementById("verdict-server");
  const agree = document.getElementById("verdict-agree");
  const disagree = document.getElementById("verdict-disagree");
  if (!server || !agree || !disagree) {
    return;
  }
  // Agreement is only claimed when both sides have actually reported. A server
  // that says "valid" while this tab has not finished is not agreement.
  if (last === null) {
    agree.hidden = true;
    disagree.hidden = true;
    return;
  }
  const serverOK = server.classList.contains("ok");
  agree.hidden = !(serverOK && last.valid);
  disagree.hidden = serverOK === (last.valid && !last.error);
}

// The chip this module last painted. The chain-status banner is replaced
// wholesale by the partial runtime every few seconds, and a replacement node
// comes back with the server's "verifying…" placeholder. Without tracking the
// element identity, the banner would visibly fall back to "verifying" five
// seconds after proving itself and stay there until the next push.
let paintedChip = null;

// Re-paint whenever the banner has been replaced. This is a DOM observation
// rather than an event subscription because the swap path is not the same code
// that pushes chain updates: the poller swaps silently, and making it announce
// itself would also make the graph refetch on every poll.
const chipObserver = new MutationObserver(() => {
  const chip = document.getElementById("verdict-browser");
  if (chip && chip !== paintedChip) {
    paint();
  }
});

if (document.documentElement) {
  chipObserver.observe(document.documentElement, { childList: true, subtree: true });
}

// Re-verify whenever the chain changes: the partial runtime replaces the banner
// (so the chip must be repainted from the retained result) and the WebSocket
// pushes new entries.
document.addEventListener("aether:chain", () => {
  paint();
  run().catch((err) => {
    last = { valid: false, error: String(err) };
    paint();
  });
});
