// End-to-end check of the dashboard's in-browser audit verification.
//
// Every other gate in this repository can be satisfied by a server that sends
// correct bytes. This one cannot: it drives a real browser, and it fails unless
// the WASM module loads under the dashboard's own CSP, the bootstrap reaches the
// URLs it claims to, and the banner's browser chip leaves its "verifying…"
// placeholder with a verdict derived from verify.wasm.
//
// Usage:
//
//   node scripts/browser-verify.mjs <base-url> [--timeout-ms 30000]
//
// The URL must serve a dashboard with a real audit chain attached. On success it
// prints the verdict as JSON and exits 0; on failure it prints why and exits 1.
//
// It talks to the browser over the DevTools protocol rather than using
// --dump-dom, because --dump-dom combined with --virtual-time-budget advances a
// *virtual* clock: the budget can be spent in a fraction of a second, so the
// dump routinely lands before a real fetch or a WASM compile has finished. That
// failure looks exactly like a broken verifier, which is the one thing this
// script must never be ambiguous about. Polling for the result instead means a
// slow machine gets more time rather than a wrong answer.

import { spawn } from "node:child_process";
import { existsSync, mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const args = process.argv.slice(2);
const base = args.find((a) => !a.startsWith("--") && !/^\d+$/.test(a));
if (!base) {
  console.error("usage: node scripts/browser-verify.mjs <base-url> [--timeout-ms N]");
  process.exit(2);
}
const timeoutIdx = args.indexOf("--timeout-ms");
const timeoutMs = timeoutIdx >= 0 ? Number(args[timeoutIdx + 1]) : 30000;
const auditURL = `${base.replace(/\/$/, "")}/audit`;
// What the graph page is expected to lay out, read from the server before the
// browser starts. Deriving it rather than hard-coding it means the check fails
// if the page and the API disagree, which is the bug worth catching.
let expectedNodes = 0;
let expectedEdges = 0;
try {
  const g = await (await fetch(`${base.replace(/\/$/, "")}/api/graph`)).json();
  expectedNodes = Array.isArray(g.nodes) ? g.nodes.length : 0;
  expectedEdges = Array.isArray(g.edges) ? g.edges.length : 0;
} catch (e) {
  console.error(`could not read /api/graph from ${base}: ${e.message}`);
  process.exit(4);
}
let graphReport = null;
// Browser-side diagnostics, collected for the whole session and printed on any
// failure. A verdict that never arrives is otherwise indistinguishable from a
// module that threw on the first line, and the thrown error is the only thing
// that distinguishes them.
const browserOutput = [];

function dumpBrowserOutput() {
  if (!browserOutput.length) return;
  console.error("browser output:");
  for (const l of browserOutput) console.error("  " + l);
}

// The browsers this script will drive, in preference order. Chromium's headless
// mode is required; a full browser with a window would work but is rude to run
// unattended.
function findBrowser() {
  const candidates = [
    process.env.AETHER_BROWSER,
    "chrome",
    "msedge",
    "google-chrome",
    "chromium",
    "chromium-browser",
  ].filter(Boolean);
  const fixed = [
    ["C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe"],
    ["C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe"],
    ["C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe"],
    ["C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe"],
    ["/usr/bin/google-chrome"],
    ["/usr/bin/chromium"],
    ["/usr/bin/chromium-browser"],
  ].map((p) => p[0]);
  for (const c of [...candidates, ...fixed]) {
    if (c.includes("/") || c.includes("\\")) {
      try {
        if (statFile(c)) return c;
      } catch {
        /* not there */
      }
      continue;
    }
    const found = which(c);
    if (found) return found;
  }
  return "";
}

function statFile(p) {
  return existsSync(p);
}
function which(cmd) {
  const dirs = (process.env.PATH || "").split(";").filter(Boolean);
  const exts = process.platform === "win32" ? [".exe", ".cmd", ""] : [""];
  for (const d of dirs) {
    for (const e of exts) {
      const p = join(d, cmd + e);
      try {
        if (statFile(p)) return p;
      } catch {
        /* keep looking */
      }
    }
  }
  return "";
}

const browser = findBrowser();
if (!browser) {
  console.error("no Chromium-family browser found; set AETHER_BROWSER to one");
  process.exit(3);
}

const profile = mkdtempSync(join(tmpdir(), "aether-browser-"));
const child = spawn(
  browser,
  [
    "--headless=new",
    "--disable-gpu",
    "--no-first-run",
    "--no-default-browser-check",
    "--disable-extensions",
    `--user-data-dir=${profile}`,
    // Port 0 lets the OS pick a free port, so parallel runs cannot collide. The
    // actual port is read from the DevTools line the browser prints.
    "--remote-debugging-port=0",
    "about:blank",
  ],
  { stdio: ["ignore", "pipe", "pipe"] }
);

let stderrBuf = "";
const wsURL = await new Promise((resolve, reject) => {
  const timer = setTimeout(
    () => reject(new Error(`the browser never reported a DevTools endpoint:\n${stderrBuf}`)),
    30000
  );
  child.stderr.on("data", (d) => {
    stderrBuf += d.toString();
    const m = stderrBuf.match(/ws:\/\/[^\s]+/);
    if (m) {
      clearTimeout(timer);
      resolve(m[0]);
    }
  });
  child.on("error", (e) => {
    clearTimeout(timer);
    reject(e);
  });
  child.on("exit", (code) => {
    clearTimeout(timer);
    reject(new Error(`the browser exited with code ${code} before reporting an endpoint:\n${stderrBuf}`));
  });
});

let code = 1;
try {
  code = await drive(wsURL);
} finally {
  child.kill();
  try {
    rmSync(profile, { recursive: true, force: true });
  } catch {
    // A leftover temp profile is not worth failing the run over.
  }
}
process.exit(code);

// connect opens a DevTools session on a fresh target and returns a small
// request/response helper over it.
async function connect(wsURL) {
  // The browser reports a ws:// endpoint. The HTTP endpoints that create and
  // list targets live on the same host and port, so the scheme is swapped rather
  // than a second endpoint being discovered; fetch rejects a ws:// URL outright.
  const httpBase = wsURL
    .replace(/^ws:\/\//, "http://")
    .replace(/\/devtools\/browser\/.*$/, "");
  const res = await fetch(`${httpBase}/json/new?${encodeURIComponent(auditURL)}`, {
    method: "PUT",
  });
  const target = await res.json();
  const ws = new WebSocket(target.webSocketDebuggerUrl);
  await new Promise((resolve, reject) => {
    ws.addEventListener("open", resolve, { once: true });
    ws.addEventListener("error", () => reject(new Error("the DevTools socket refused")), {
      once: true,
    });
  });
  let next = 1;
  const pending = new Map();
  const consoleLines = browserOutput;
  ws.addEventListener("message", (ev) => {
    const msg = JSON.parse(ev.data);
    if (msg.id && pending.has(msg.id)) {
      const { resolve, reject } = pending.get(msg.id);
      pending.delete(msg.id);
      if (msg.error) reject(new Error(`${msg.error.message} (${JSON.stringify(msg.error.data ?? null)})`));
      else resolve(msg.result);
      return;
    }
    // Page errors and console output are collected rather than ignored: a
    // verifier that threw would otherwise be indistinguishable from one that
    // never ran.
    if (msg.method === "Runtime.exceptionThrown") {
      const d = msg.params.exceptionDetails;
      consoleLines.push(`exception: ${d.exception?.description || d.text}`);
    }
    if (msg.method === "Runtime.consoleAPICalled") {
      consoleLines.push(
        `${msg.params.type}: ${msg.params.args.map((a) => a.value ?? a.description ?? "").join(" ")}`
      );
    }
    if (msg.method === "Log.entryAdded") {
      const e = msg.params.entry;
      // A CSP violation or a refused script is the most likely reason a verdict
      // never arrives, so these are captured verbatim.
      consoleLines.push(`log(${e.level}): ${e.text}${e.url ? ` ${e.url}` : ""}`);
    }
  });
  const send = (method, params = {}) =>
    new Promise((resolve, reject) => {
      const id = next++;
      pending.set(id, { resolve, reject });
      ws.send(JSON.stringify({ id, method, params }));
    });
  return { send, consoleLines, close: () => ws.close() };
}

async function drive(wsURL) {
  const { send, consoleLines, close } = await connect(wsURL);
  try {
    await send("Runtime.enable");
    await send("Log.enable");
    await send("Page.enable");
    await send("Page.navigate", { url: auditURL });

    const deadline = Date.now() + timeoutMs;
    let result = null;
    while (Date.now() < deadline) {
      const r = await send("Runtime.evaluate", {
        expression: `(() => {
          const v = window.__aetherVerify;
          return JSON.stringify({
            present: !!v,
            result: v ? v.result : null,
            chip: (document.getElementById("verdict-browser") || {}).textContent || null,
            serverChip: (document.getElementById("verdict-server") || {}).textContent || null,
            ready: document.readyState,
          });
        })()`,
        returnByValue: true,
        awaitPromise: false,
      });
      if (r.exceptionDetails) {
        throw new Error(`evaluating the probe failed: ${r.exceptionDetails.text}`);
      }
      const state = JSON.parse(r.result.value);
      // Both conditions are required, not just the verdict. A chain update can
      // replace the banner between the moment the verdict is stored and the
      // moment it is painted, so sampling the result alone would report a
      // placeholder chip as a broken verifier. Waiting for the chip to settle
      // is also what proves the repaint-after-swap path works.
      const settled = /browser:\s*(valid|INVALID|ERROR)/.test(state.chip || "");
      if (state.result && settled) {
        result = state;
        break;
      }
      await sleep(150);
    }

    if (!result) {
      console.error(`FAIL: the browser produced no verdict within ${timeoutMs}ms`);
      dumpBrowserOutput();
      return 1;
    }

    // A verdict that is an error is still a verdict, and it still fails: the
    // module ran and reported that it could not verify.
    if (result.result.error) {
      console.error(`FAIL: the verifier reported an error: ${result.result.error}`);
      return 1;
    }
    if (result.result.valid !== true) {
      console.error(
        `FAIL: the verifier rejected the chain: ${JSON.stringify(result.result, null, 2)}`
      );
      return 1;
    }
    if (!/browser:\s*valid/.test(result.chip || "")) {
      console.error(`FAIL: the banner chip did not show a valid verdict: ${JSON.stringify(result.chip)}`);
      return 1;
    }
    if (!/server:\s*valid/.test(result.serverChip || "")) {
      console.error(
        `FAIL: the server verdict chip is not "valid", so the two sides cannot be compared: ${JSON.stringify(
          result.serverChip
        )}`
      );
      return 1;
    }

    // The audit banner being green says nothing about the graph, so the second
    // module is checked in the same session.
    if (await checkGraph(send, expectedNodes, expectedEdges) !== 0) {
      dumpBrowserOutput();
      return 1;
    }

    console.log(
      JSON.stringify(
        {
          ok: true,
          url: auditURL,
          browser: browser,
          verified_by: result.result.verifiedBy || result.result.verified_by,
          entry_count: result.result.entryCount ?? result.result.entry_count,
          valid_count: result.result.validCount ?? result.result.valid_count,
          pubkey_fingerprint: result.result.fingerprint ?? result.result.pubkey_fingerprint,
          chip: (result.chip || "").trim(),
          server_chip: (result.serverChip || "").trim(),
          graph: graphReport,
        },
        null,
        2
      )
    );
    return 0;
  } finally {
    close();
  }
}

// checkGraph proves the second module ran. A dashboard whose audit banner is
// green but whose graph canvas never rendered is a dashboard that looks healthy
// and shows nothing, so this is checked in the same browser session rather than
// left to a screenshot.
//
// The expectation is derived from the server rather than hard-coded: whatever
// /api/graph reports is what the renderer must have laid out.
async function checkGraph(send, expectedNodes, expectedEdges) {
  await send("Page.navigate", { url: `${base.replace(/\/$/, "")}/graph` });
  const deadline = Date.now() + 30000;
  let state = null;
  while (Date.now() < deadline) {
    const r = await send("Runtime.evaluate", {
      expression: `(() => {
        const g = window.__aetherGraph;
        if (!g) return JSON.stringify({ present: false, ready: document.readyState });
        return JSON.stringify({
          present: true,
          nodes: g.nodeCount(),
          edges: g.edgeCount(),
          allFinite: g.allFinite(),
          overlay: g.overlay(),
        });
      })()`,
      returnByValue: true,
    });
    state = JSON.parse(r.result.value);
    // allFinite is part of the wait condition, not just an assertion: a chain
    // update can trigger a refetch, and a sample taken mid-refetch would report
    // a loaded node list with no layout yet. That is a normal transient, not a
    // broken module.
    if (state.present && state.nodes > 0 && state.allFinite) break;
    await sleep(150);
  }
  if (!state || !state.present) {
    console.error("FAIL: the graph page never exposed its renderer state");
    return 1;
  }
  if (!state.allFinite) {
    console.error(
      `FAIL: the WASM layout did not produce finite coordinates for every node (overlay: ${JSON.stringify(
        state.overlay
      )})`
    );
    return 1;
  }
  if (state.nodes !== expectedNodes) {
    console.error(
      `FAIL: the renderer laid out ${state.nodes} nodes but /api/graph reports ${expectedNodes}`
    );
    return 1;
  }
  if (state.edges !== expectedEdges) {
    console.error(
      `FAIL: the renderer holds ${state.edges} edges but /api/graph reports ${expectedEdges}`
    );
    return 1;
  }
  graphReport = {
    nodes: state.nodes,
    edges: state.edges,
    all_coordinates_finite: state.allFinite,
    overlay: (state.overlay || "").trim(),
  };
  return 0;
}

function sleep(ms) {
  return new Promise((r) => setTimeout(r, ms));
}
