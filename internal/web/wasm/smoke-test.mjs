// Smoke test for the dashboard's WebAssembly modules, run under Node.
//
// # Why this exists
//
// The dashboard's two WASM modules are the only parts of Aether that execute on
// the operator's machine rather than in the CLI. Neither can be reached from the
// Go test suite without embedding a WebAssembly runtime in the product, and a
// browser is not always available on a build machine. This script therefore
// exercises the exact artifacts that `go:embed` ships, over the exact ABI
// `internal/web/static/graph.js` and `verify.js` use.
//
// It is a test harness, not a runtime dependency: nothing in the shipped
// dashboard imports Node, and `go build` does not need this file.
//
// The verifier test is the important one. It builds a real signed chain with
// Node's own Ed25519 implementation, using the wire contract documented in
// `internal/web/audit.go` and `internal/store/audit.go`, and then has
// `verify.wasm` check it. If the Rust verifier's hash preimage, its signature
// coverage, or its base64 handling drifted from the Go side, this fails.
//
// Usage:  node internal/web/wasm/smoke-test.mjs

import { readFile } from "node:fs/promises";
import { createHash, generateKeyPairSync, sign as edSign } from "node:crypto";
import { fileURLToPath, pathToFileURL } from "node:url";
import { dirname, join } from "node:path";

const here = dirname(fileURLToPath(import.meta.url));

let failures = 0;
let checks = 0;

function check(name, condition, detail = "") {
  checks += 1;
  if (condition) {
    console.log(`  ok   ${name}`);
  } else {
    failures += 1;
    console.log(`  FAIL ${name}${detail ? `: ${detail}` : ""}`);
  }
}

function eq(name, actual, expected) {
  check(name, Object.is(actual, expected), `got ${JSON.stringify(actual)}, want ${JSON.stringify(expected)}`);
}

// ---------------------------------------------------------------- graph.wasm

async function testGraph() {
  console.log("graph.wasm");
  const bytes = await readFile(join(here, "graph.wasm"));
  check(
    "module carries the wasm magic and is non-trivial in size",
    bytes.length > 1024 && bytes[0] === 0x00 && bytes[1] === 0x61 && bytes[2] === 0x73 && bytes[3] === 0x6d,
  );
  const { instance } = await WebAssembly.instantiate(bytes, {});
  const ex = instance.exports;
  for (const name of ["memory", "aether_alloc", "aether_dealloc", "aether_layout"]) {
    check(`exports ${name}`, ex[name] !== undefined);
  }

  const memory = ex.memory;
  const N = 500;
  const E = 900;

  // A ring of 500 nodes, each linked to the next, is a shape a real identity
  // graph produces (delegation chains) and exercises the springs.
  const nodeBytes = N * 3 * 4;
  const edgeBytes = E * 2 * 4;
  const outBytes = N * 2 * 4;
  const nodePtr = ex.aether_alloc(nodeBytes);
  const edgePtr = ex.aether_alloc(edgeBytes);
  const outPtr = ex.aether_alloc(outBytes);
  const diagPtr = ex.aether_alloc(16);
  check("alloc returned non-null pointers", nodePtr !== 0 && edgePtr !== 0 && outPtr !== 0 && diagPtr !== 0);

  const seed = [];
  let nodes = new Float32Array(memory.buffer, nodePtr, N * 3);
  for (let i = 0; i < N; i += 1) {
    const angle = (i / N) * Math.PI * 2;
    const x = Math.cos(angle) * 500;
    const y = Math.sin(angle) * 500;
    seed.push([x, y]);
    nodes[i * 3] = x;
    nodes[i * 3 + 1] = y;
    nodes[i * 3 + 2] = i === 0 ? 1 : 0; // pin node 0
  }
  const edges = new Uint32Array(memory.buffer, edgePtr, E * 2);
  for (let e = 0; e < E; e += 1) {
    edges[e * 2] = e % N;
    edges[e * 2 + 1] = (e + 1) % N;
  }

  const call = () =>
    ex.aether_layout(N, nodePtr, E, edgePtr, 120, 300, 0.05, 0.85, 0.7, 12, outPtr, diagPtr);
  eq("aether_layout returns OK", call(), 0);

  // Views are rebuilt after every call: the layout can grow memory, which
  // detaches any previously created view.
  let out = new Float32Array(memory.buffer, outPtr, N * 2);
  const diag = new Uint32Array(memory.buffer, diagPtr, 4);
  check("every output coordinate is finite", out.every((v) => Number.isFinite(v)));
  eq("output count matches input", out.length, N * 2);
  check("quadtree reported cells", diag[0] > 0, `cells=${diag[0]}`);
  eq("iterations reported", diag[1], 120);
  check("some nodes moved", diag[2] > 0, `moved=${diag[2]}`);

  eq("pinned node kept its x", out[0], seed[0][0]);
  eq("pinned node kept its y", out[1], seed[0][1]);
  check("unpinned node 1 was displaced", out[2] !== seed[1][0] || out[3] !== seed[1][1]);
  check(
    "no output equals NaN from an uninitialised buffer",
    out.slice(2, 12).some((v) => v !== 0),
  );

  // Re-running with identical inputs must reproduce identical output: the
  // operator screenshots the graph and the evidence has to match.
  const first = Float32Array.from(out);
  eq("second run also returns OK", call(), 0);
  out = new Float32Array(memory.buffer, outPtr, N * 2);
  let identical = true;
  for (let i = 0; i < out.length; i += 1) {
    if (out[i] !== first[i]) {
      identical = false;
      break;
    }
  }
  check("layout is deterministic across runs", identical);

  // Error paths must return codes, not trap.
  eq("null input is rejected", ex.aether_layout(0, 0, 0, 0, 1, 1, 1, 1, 1, 1, outPtr, diagPtr), -1);
  eq("zero nodes is rejected", ex.aether_layout(0, nodePtr, 0, 0, 1, 1, 1, 1, 1, 1, outPtr, diagPtr), -3);
  eq("absurd node count is rejected", ex.aether_layout(9_000_000, nodePtr, 0, 0, 1, 1, 1, 1, 1, 1, outPtr, diagPtr), -2);
  eq("edges without a buffer are rejected", ex.aether_layout(4, nodePtr, 3, 0, 1, 1, 1, 1, 1, 1, outPtr, diagPtr), -1);

  ex.aether_dealloc(nodePtr, nodeBytes);
  ex.aether_dealloc(edgePtr, edgeBytes);
  ex.aether_dealloc(outPtr, outBytes);
  ex.aether_dealloc(diagPtr, 16);
  check("dealloc survived the call", true);

  // A single-node graph is the degenerate case the phyllotaxis seed has to
  // survive without dividing by zero.
  const oneIn = ex.aether_alloc(12);
  const oneOut = ex.aether_alloc(8);
  new Float32Array(memory.buffer, oneIn, 3).set([0, 0, 0]);
  eq("single node runs", ex.aether_layout(1, oneIn, 0, 0, 30, 300, 0.05, 0.85, 0.7, 12, oneOut, 0), 0);
  const single = new Float32Array(memory.buffer, oneOut, 2);
  check("single node stays finite", Number.isFinite(single[0]) && Number.isFinite(single[1]));
  ex.aether_dealloc(oneIn, 12);
  ex.aether_dealloc(oneOut, 8);
}

// --------------------------------------------------------------- verify.wasm

// The chain-building helpers mirror internal/store/audit.go byte for byte. If
// the Go format ever changes, these must change with it or this test lies.
const GENESIS = "0".repeat(64);

function preimage(seq, timestamp, command, result, prevHash) {
  return `${seq}|${timestamp}|${command}|${result}|${prevHash}`;
}

function sha256Hex(s) {
  return createHash("sha256").update(s, "utf8").digest("hex");
}

function buildChain(privateKey, commands) {
  const entries = [];
  let prev = GENESIS;
  commands.forEach(([command, result, timestamp], i) => {
    const seq = i + 1;
    const hash = sha256Hex(preimage(seq, timestamp, command, result, prev));
    // The signature covers the ASCII bytes of the hex hash, not the raw digest.
    // This is the single easiest thing to get wrong in a second implementation.
    const signature = edSign(null, Buffer.from(hash, "ascii"), privateKey).toString("base64");
    entries.push({ seq, timestamp, command, result, prev_hash: prev, hash, signature });
    prev = hash;
  });
  return entries;
}

function publicKeyBase64(spki) {
  // SPKI for Ed25519 is 44 bytes: a 12-byte prefix followed by the 32-byte key.
  return Buffer.from(spki).subarray(12).toString("base64");
}

async function testVerify() {
  console.log("verify.wasm");
  const bytes = await readFile(join(here, "verify.wasm"));
  check("module is non-trivial in size", bytes.length > 1024, `${bytes.length} bytes`);

  // Drive the real wasm-bindgen glue the browser loads, not a hand-rolled shim,
  // so a glue/module version mismatch fails here rather than in the operator's
  // browser.
  const glue = await import(pathToFileURL(join(here, "..", "static", "verify.js")).href);
  // wasm-bindgen's web target exports its initialiser as the module default.
  check("glue exports an initialiser", typeof glue.default === "function");
  check("glue exports verify_chain", typeof glue.verify_chain === "function");
  check("glue exports verify_chain_ndjson", typeof glue.verify_chain_ndjson === "function");

  // Passing the bytes explicitly avoids the glue's default `fetch` of a
  // relative URL, which has no meaning under Node.
  await glue.default({ module_or_path: bytes });
  // The self-test verifies an embedded known-answer vector signed by an
  // independent implementation, and then requires that same vector to be
  // rejected under an unrelated key. Checking only the "ok:" prefix would let a
  // module that skipped the negative control pass, so the count is checked too.
  const st = glue.selftest();
  check("selftest reports ok", st.startsWith("ok:"), `got ${JSON.stringify(st)}`);
  check("selftest verified its known-answer vector", st.includes("2/2"), st);
  check(
    "selftest rejected the vector under an unrelated key",
    st.includes("rejected under an unrelated key"),
    st
  );

  const { publicKey, privateKey } = generateKeyPairSync("ed25519");
  const pubB64 = publicKeyBase64(publicKey.export({ type: "spki", format: "der" }));

  const commands = [
    ["workspace.create", '{"workspace":"aether.test"}', "2026-09-26T01:15:30Z"],
    ["graph.addNode", '{"id":"S-1","label":"svc-deploy"}', "2026-09-26T01:15:31.5Z"],
    ["graph.addEdge", '{"source":"S-1","target":"U-1","type":"memberOf"}', "2026-09-26T01:15:32.25Z"],
  ];
  const chain = buildChain(privateKey, commands);

  const good = JSON.parse(glue.verify_chain(JSON.stringify(chain), pubB64, false));
  check("an honest chain verifies", good.valid === true, JSON.stringify(good.notes));
  eq("all entries counted valid", good.valid_count, chain.length);
  eq("entry count echoed", good.entry_count, chain.length);
  eq("head hash echoed", good.head_hash, chain[chain.length - 1].hash);
  eq("genesis reported", good.genesis, GENESIS);
  check("verifier identifies itself", String(good.verified_by).includes("wasm"));

  // Per-entry verdicts, when requested, must cover every entry.
  const withEntries = JSON.parse(glue.verify_chain(JSON.stringify(chain), pubB64, true));
  check("per-entry verdicts returned on request", Array.isArray(withEntries.entries) && withEntries.entries.length === chain.length);
  check("every entry verdict is positive", withEntries.entries.every((e) => e.hash_ok && e.signature_ok && e.linkage_ok));
  check("canonical timestamps recognised", withEntries.entries.every((e) => e.timestamp_canonical));

  // The NDJSON path must reach the same verdict.
  const nd = JSON.parse(glue.verify_chain_ndjson(chain.map((e) => JSON.stringify(e)).join("\n"), pubB64, false));
  check("ndjson input verifies identically", nd.valid === true && nd.head_hash === good.head_hash);

  // Tamper with the payload of entry 2: the hash no longer matches, and the
  // signature cannot rescue it because the hash is what was signed.
  const tampered = chain.map((e) => (e.seq === 2 ? { ...e, result: '{"id":"S-1","label":"svc-privileged"}' } : e));
  const t = JSON.parse(glue.verify_chain(JSON.stringify(tampered), pubB64, false));
  check("altered entry is detected", t.valid === false);
  check("altered entry is named", t.tampered_seq.includes(2), JSON.stringify(t.tampered_seq));
  check("the rest still verifies", t.valid_count === chain.length - 1, `valid_count=${t.valid_count}`);

  // Re-sign a tampered chain with a different key: hashes still line up, but no
  // signature is from the supplied key. This is the "server lies about the key"
  // case, and it must not pass.
  const { privateKey: other } = generateKeyPairSync("ed25519");
  const forged = buildChain(other, commands);
  const f = JSON.parse(glue.verify_chain(JSON.stringify(forged), pubB64, false));
  check("chain signed by another key is rejected", f.valid === false);
  eq("every forged entry is flagged", f.tampered_seq.length, forged.length);
  check(
    "rejection reason is the signature",
    f.notes.some((n) => n.includes("signature")),
    JSON.stringify(f.notes.slice(0, 2)),
  );

  // Break the linkage: drop the middle entry, keeping every hash intact.
  const gapped = [chain[0], chain[2]];
  const g = JSON.parse(glue.verify_chain(JSON.stringify(gapped), pubB64, false));
  check("a missing entry breaks linkage", g.valid === false);
  eq("linkage break is located", g.broken_chain_at, 3);

  // Reordering must be caught as a linkage failure, not silently accepted.
  const reordered = [chain[1], chain[0]];
  const r = JSON.parse(glue.verify_chain(JSON.stringify(reordered), pubB64, false));
  check("a reordered chain is rejected", r.valid === false);
  check("reorder is reported as a linkage break", r.broken_chain_at !== null);

  // An empty chain is internally consistent but attests to nothing, and the
  // result must say so rather than presenting a green tick over zero evidence.
  const empty = JSON.parse(glue.verify_chain("[]", pubB64, false));
  check("empty chain reports valid with a caveat", empty.valid === true && empty.notes.length > 0);

  // A malformed key must produce an error, not a panic and not a valid verdict.
  const badKey = JSON.parse(glue.verify_chain(JSON.stringify(chain), "not-base64!!", false));
  check("a bad key yields an error", badKey.valid === false && typeof badKey.error === "string");
  const shortKey = JSON.parse(glue.verify_chain(JSON.stringify(chain), "AAAA", false));
  check("a short key is rejected", shortKey.valid === false && String(shortKey.error).includes("32"));

  // A signature from a key that verifies, but where the hash bytes were signed
  // instead of the hex string: must fail, which is why the wire format is frozen.
  const rawSigned = chain.map((e) => ({ ...e, signature: edSign(null, Buffer.from(e.hash, "hex"), privateKey).toString("base64") }));
  const raw = JSON.parse(glue.verify_chain(JSON.stringify(rawSigned), pubB64, false));
  check("signing the raw digest instead of the hex string is rejected", raw.valid === false);
}

await testGraph();
await testVerify();

console.log(`\n${checks - failures}/${checks} checks passed`);
if (failures > 0) {
  console.error(`${failures} check(s) failed`);
  process.exit(1);
}
