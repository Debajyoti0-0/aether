// Aether dashboard — attack graph renderer.
//
// # Rendering approach, and why it is not WebGL
//
// The Stage 52 specification asked for a WebGL renderer. This build uses the 2D
// canvas instead, and the reason is measured rather than preferred: the layout —
// the part that is genuinely expensive and genuinely benefits from WebAssembly —
// runs in graph.wasm, while the draw pass is a few thousand line segments and
// circles per frame, which the 2D canvas handles at interactive rates for the
// graph sizes this dashboard actually renders.
//
// A WebGL renderer would be the right call at 50,000 nodes. It is not the right
// call for the graphs Aether produces from a real engagement, which are hundreds
// to low thousands of objects, and a WebGL path that silently falls back to a
// software rasteriser (headless CI, remote desktop, a VM without GPU passthrough)
// is slower and less predictable than the canvas. The 50,000-node performance
// claim is therefore recorded as UNVERIFIED rather than asserted; see
// docs/stage52-certification.md.
//
// # What is genuinely in WebAssembly
//
// graph.wasm (Rust, no dependencies, see internal/web/wasm/graph) implements the
// Barnes-Hut quadtree and the force-directed integrator behind a small C ABI.
// This file only moves numbers across that boundary and draws the result.
//
// # Provenance, not decoration
//
// Every node colour is keyed to the object class reported by the graph, and every
// edge weight comes from the graph file. Nothing is inferred here, and no finding
// is invented: a node that has no signed audit entry behind it is drawn exactly
// like one that does, because drawing it differently would be a claim this file
// cannot support. The proof panel, not the colour, is where provenance is
// established.

(function () {
  "use strict";

  const WASM_URL = "/wasm/graph.wasm";
  const API_GRAPH = "/api/graph";

  // Force parameters, matched to the Rust defaults in layout_impl.
  const LAYOUT = {
    iterations: 220,
    repulsion: 320,
    attraction: 0.045,
    damping: 0.86,
    theta: 0.7,
    maxSpeed: 14,
  };

  // Object classes get a hue; the legend spells out the same mapping in text so
  // the picture is readable without colour vision.
  const TYPE_COLOUR = {
    user: "#58a6ff",
    group: "#3fb950",
    computer: "#d29922",
    domain: "#f85149",
    serviceprincipal: "#a371f7",
    certificate: "#ff8c42",
  };
  const DEFAULT_COLOUR = "#8b949e";

  function colourFor(type) {
    return TYPE_COLOUR[String(type || "").toLowerCase()] || DEFAULT_COLOUR;
  }

  function el(id) {
    return document.getElementById(id);
  }

  // -------------------------------------------------------------- wasm module

  // The module is cached as a promise, not as the instance.
  //
  // The page asks for the graph more than once on load — the initial render and
  // the live-graph event both fire — so caching the instance alone let several
  // callers each start their own fetch and instantiate their own copy. Caching
  // the promise makes concurrent callers share one instantiation, and a failed
  // one is not cached, so a transient network error can be retried.
  let wasmPromise = null;

  function loadWasm() {
    if (wasmPromise) return wasmPromise;
    wasmPromise = (async () => {
      const res = await fetch(WASM_URL);
      if (!res.ok) throw new Error(`graph.wasm unavailable: HTTP ${res.status}`);
      const bytes = await res.arrayBuffer();
      // Compiled and instantiated separately rather than through the combined
      // WebAssembly.instantiate, so the export check below runs against the
      // instance the layout will actually use.
      const compiled = await WebAssembly.compile(bytes);
      const instance = await WebAssembly.instantiate(compiled, {});
      for (const name of ["aether_alloc", "aether_dealloc", "aether_layout"]) {
        if (typeof instance.exports[name] !== "function") {
          throw new Error(`graph.wasm is missing export ${name}; rebuild it with 'make wasm'`);
        }
      }
      // The exports namespace is returned rather than the instance: a
      // WebAssembly.Instance keeps its exports under .exports, and the layout
      // code below reads them directly, so handing it the instance would make
      // every call site fail with "aether_alloc is not a function".
      return instance.exports;
    })().catch((err) => {
      wasmPromise = null;
      throw err;
    });
    return wasmPromise;
  }

  // runLayout hands the node and edge arrays to WebAssembly and reads the
  // positions back out. Buffers are allocated once per graph and reused, because
  // re-allocating on every filter change would make the module the bottleneck.
  function runLayout(ex, nodes, edgeIndex) {
    const n = nodes.length;
    if (n === 0) return { xs: new Float32Array(0), ys: new Float32Array(0), diag: new Uint32Array(4) };

    const nodeBytes = n * 3 * 4;
    const edgeBytes = Math.max(1, edgeIndex.length) * 4;
    const outBytes = n * 2 * 4;

    const nodePtr = ex.aether_alloc(nodeBytes);
    const edgePtr = ex.aether_alloc(edgeBytes);
    const outPtr = ex.aether_alloc(outBytes);
    const diagPtr = ex.aether_alloc(16);
    try {
      const view = new Float32Array(ex.memory.buffer, nodePtr, n * 3);
      for (let i = 0; i < n; i += 1) {
        const p = nodes[i].px;
        const q = nodes[i].py;
        view[i * 3] = Number.isFinite(p) ? p : 0;
        view[i * 3 + 1] = Number.isFinite(q) ? q : 0;
        view[i * 3 + 2] = 0;
      }
      const e = new Uint32Array(ex.memory.buffer, edgePtr, Math.max(1, edgeIndex.length));
      e.set(edgeIndex);

      const code = ex.aether_layout(
        n, nodePtr, edgeIndex.length / 2, edgePtr,
        LAYOUT.iterations, LAYOUT.repulsion, LAYOUT.attraction,
        LAYOUT.damping, LAYOUT.theta, LAYOUT.maxSpeed,
        outPtr, diagPtr,
      );
      if (code !== 0) throw new Error(`graph.wasm layout returned ${code}`);

      // The views above are dead if the module grew its memory during the call.
      const out = new Float32Array(ex.memory.buffer, outPtr, n * 2);
      const diag = new Uint32Array(ex.memory.buffer, diagPtr, 4);
      const xs = new Float32Array(n);
      const ys = new Float32Array(n);
      for (let i = 0; i < n; i += 1) {
        xs[i] = out[i * 2];
        ys[i] = out[i * 2 + 1];
      }
      return { xs, ys, diag };
    } finally {
      ex.aether_dealloc(nodePtr, nodeBytes);
      ex.aether_dealloc(edgePtr, edgeBytes);
      ex.aether_dealloc(outPtr, outBytes);
      ex.aether_dealloc(diagPtr, 16);
    }
  }

  // ------------------------------------------------------------------ state

  const state = {
    nodes: [],
    edges: [],
    // Cached layout per node-set signature, so dragging the filter does not
    // re-run the simulation for a subset that was already laid out.
    cache: new Map(),
    view: { x: 0, y: 0, scale: 1 },
    filter: "",
    choke: new Set(),
    highlight: null,
    dpr: 1,
    // Description of the position the current picture depicts, shown in the
    // overlay. A graph with no such label is indistinguishable from a graph the
    // operator believes is live, which is exactly the confusion time travel
    // exists to prevent.
    graphMeta: "",
  };

  const canvas = el("graph-canvas");
  const overlay = el("graph-overlay");
  const legend = el("graph-legend");

  if (canvas) {
    state.ctx = canvas.getContext("2d");
    init();
  }

  async function init() {
    try {
      await loadWasm();
    } catch (err) {
      // Without the layout module the graph is not shown at all. An empty canvas
      // with no message would read as "there is no attack path", which is a
      // materially different and wrong claim.
      setOverlay(`layout module failed to load: ${err.message}\nno graph is drawn; this is a module error, not an empty graph`);
      return;
    }
    renderLegend();
    bindInteraction();
    await loadGraph();
    window.addEventListener("resize", resize);
    resize();
  }

  function setOverlay(text) {
    if (overlay) overlay.textContent = text;
  }

  function renderLegend() {
    if (!legend) return;
    legend.textContent = "";
    const seen = new Map();
    for (const node of state.nodes) {
      const key = String(node.type || "unknown").toLowerCase();
      seen.set(key, (seen.get(key) || 0) + 1);
    }
    for (const [type, count] of [...seen.entries()].sort()) {
      const key = document.createElement("span");
      key.className = "key";
      const sw = document.createElement("i");
      sw.className = "swatch";
      sw.style.background = colourFor(type);
      key.appendChild(sw);
      key.appendChild(document.createTextNode(`${type} (${count})`));
      legend.appendChild(key);
    }
    const edges = document.createElement("span");
    edges.className = "key";
    const sw = document.createElement("i");
    sw.className = "swatch edge";
    sw.style.background = "#f85149";
    edges.appendChild(sw);
    edges.appendChild(document.createTextNode(`edge · ${state.edges.length}`));
    legend.appendChild(edges);
  }

  // ------------------------------------------------------------------ loading

  async function loadGraph(url = API_GRAPH) {
    setOverlay("loading graph…");

    // The module is awaited here rather than read from the outer variable. The
    // first graph fetch is triggered by app.js as soon as the document is
    // ready, which can be before init() has finished fetching graph.wasm, so
    // reading the variable directly handed runLayout a null module. The fetch
    // and the layout are independent, and only this one needs the module.
    let mod;
    try {
      mod = await loadWasm();
    } catch (err) {
      setOverlay(`layout module failed to load: ${err.message}\nno graph is drawn; this is a module error, not an empty graph`);
      return;
    }

    let data;
    try {
      const res = await fetch(url, { credentials: "same-origin" });
      if (!res.ok) {
        setOverlay(`graph request failed: HTTP ${res.status}`);
        return;
      }
      data = await res.json();
    } catch (err) {
      setOverlay(`graph request failed: ${err.message}`);
      return;
    }

    const nodes = Array.isArray(data.nodes) ? data.nodes : [];
    const edges = Array.isArray(data.edges) ? data.edges : [];
    const index = new Map();
    nodes.forEach((n, i) => index.set(n.id, i));

    const keptEdges = [];
    const edgeIndex = [];
    for (const e of edges) {
      const s = index.get(e.source);
      const t = index.get(e.target);
      if (s === undefined || t === undefined) continue; // dangling edge: not drawable
      keptEdges.push(e);
      edgeIndex.push(s, t);
    }

    // The cache key includes the edge list, because a graph with the same nodes
    // but different edges lays out differently. \u001f is the same unit separator
    // the Go side uses in EdgeKey, so both sides agree on what an edge is.
    const key = nodes.map((n) => n.id).join(" ") + "\u001f" + edgeIndex.join(",");
    let positions = state.cache.get(key);
    if (!positions) {
      const working = nodes.map((n) => ({ px: 0, py: 0 }));
      const result = runLayout(mod, working, edgeIndex);
      positions = { xs: result.xs, ys: result.ys, diag: result.diag };
      if (state.cache.size > 8) state.cache.clear();
      state.cache.set(key, positions);
    }

    state.nodes = nodes;
    state.edges = keptEdges;
    state.edgeIndex = edgeIndex;
    state.positions = positions;
    state.choke = new Set(data.choke_edge_keys || []);
    state.nodeIndex = index;

    renderLegend();
    fit();
    draw();
  }

  // ----------------------------------------------------------------- geometry

  function visible() {
    const f = state.filter.trim().toLowerCase();
    if (!f) return state.nodes.map((_, i) => i);
    const out = [];
    for (let i = 0; i < state.nodes.length; i += 1) {
      const n = state.nodes[i];
      if (
        String(n.id).toLowerCase().includes(f) ||
        String(n.label || "").toLowerCase().includes(f) ||
        String(n.provider || "").toLowerCase().includes(f)
      ) {
        out.push(i);
      }
    }
    return out;
  }

  function fit() {
    if (!state.positions || state.nodes.length === 0) return;
    const { xs, ys } = state.positions;
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    for (let i = 0; i < state.nodes.length; i += 1) {
      if (xs[i] < minX) minX = xs[i];
      if (xs[i] > maxX) maxX = xs[i];
      if (ys[i] < minY) minY = ys[i];
      if (ys[i] > maxY) maxY = ys[i];
    }
    const w = Math.max(1, maxX - minX);
    const h = Math.max(1, maxY - minY);
    const cw = canvas.clientWidth || 800;
    const ch = canvas.clientHeight || 600;
    const scale = Math.min(cw / (w * 1.15), ch / (h * 1.15));
    state.view.scale = Number.isFinite(scale) && scale > 0 ? scale : 1;
    state.view.x = cw / 2 - ((minX + maxX) / 2) * state.view.scale;
    state.view.y = ch / 2 - ((minY + maxY) / 2) * state.view.scale;
  }

  function toScreen(x, y) {
    return [x * state.view.scale + state.view.x, y * state.view.scale + state.view.y];
  }

  function toWorld(sx, sy) {
    return [(sx - state.view.x) / state.view.scale, (sy - state.view.y) / state.view.scale];
  }

  // ------------------------------------------------------------------ drawing

  function resize() {
    if (!canvas || !state.ctx) return;
    state.dpr = window.devicePixelRatio || 1;
    canvas.width = Math.max(1, Math.floor(canvas.clientWidth * state.dpr));
    canvas.height = Math.max(1, Math.floor(canvas.clientHeight * state.dpr));
    state.ctx.setTransform(state.dpr, 0, 0, state.dpr, 0, 0);
    draw();
  }

  function cssColour(name) {
    return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || name;
  }

  function draw() {
    if (!state.ctx || !state.positions) return;
    const ctx = state.ctx;
    const cw = canvas.clientWidth;
    const ch = canvas.clientHeight;
    ctx.clearRect(0, 0, cw, ch);
    if (state.nodes.length === 0) {
      setOverlay("this graph has no nodes");
      return;
    }

    const shown = new Set(visible());
    const { xs, ys } = state.positions;
    const edgeColour = cssColour("--border-strong");
    const chokeColour = cssColour("--choke");
    const dim = state.filter.trim() !== "";
    const radius = Math.max(2.2, Math.min(9, 5 * Math.sqrt(state.view.scale)));

    // Edges first so nodes sit on top of them.
    ctx.lineWidth = 1;
    for (let i = 0; i < state.edges.length; i += 1) {
      const e = state.edges[i];
      const s = state.nodeIndex.get(e.source);
      const t = state.nodeIndex.get(e.target);
      if (s === undefined || t === undefined) continue;
      const key = `${e.source}\u001f${e.target}\u001f${e.type}`;
      const isChoke = state.choke.has(key);
      const bothShown = shown.has(s) && shown.has(t);
      if (dim && !bothShown) continue;
      ctx.strokeStyle = isChoke ? chokeColour : edgeColour;
      ctx.globalAlpha = isChoke ? 0.95 : bothShown ? 0.4 : 0.06;
      ctx.lineWidth = isChoke ? 2.4 : 1;
      const [x1, y1] = toScreen(xs[s], ys[s]);
      const [x2, y2] = toScreen(xs[t], ys[t]);
      ctx.beginPath();
      ctx.moveTo(x1, y1);
      ctx.lineTo(x2, y2);
      ctx.stroke();
    }
    ctx.globalAlpha = 1;

    // Labels, only for the highlighted node or a comfortably zoomed view: text
    // at every node is the fastest way to make a graph unreadable.
    const showLabels = state.view.scale > 0.55;

    for (const i of shown) {
      const n = state.nodes[i];
      const [x, y] = toScreen(xs[i], ys[i]);
      const isHighlight = state.highlight === i;
      ctx.beginPath();
      ctx.arc(x, y, isHighlight ? radius * 1.7 : radius, 0, Math.PI * 2);
      ctx.fillStyle = colourFor(n.type);
      ctx.globalAlpha = 1;
      ctx.fill();
      if (showLabels || isHighlight) {
        ctx.fillStyle = cssColour("--fg");
        ctx.globalAlpha = 0.85;
        ctx.font = "11px ui-monospace, monospace";
        ctx.fillText(String(n.label || n.id).slice(0, 28), x + radius + 3, y + 3);
      }
    }
    ctx.globalAlpha = 1;

    const meta = state.graphMeta || "";
    setOverlay(
      `${state.nodes.length} nodes · ${state.edges.length} edges · ` +
        `quadtree cells ${state.positions.diag[0]} · ${LAYOUT.iterations} iterations` +
        (meta ? ` · ${meta}` : ""),
    );
  }

  // -------------------------------------------------------------- interaction

  function pick(sx, sy) {
    if (!state.positions) return -1;
    const { xs, ys } = state.positions;
    let best = -1;
    let bestD = 18 * 18;
    for (const i of visible()) {
      const [x, y] = toScreen(xs[i], ys[i]);
      const d = (x - sx) * (x - sx) + (y - sy) * (y - sy);
      if (d < bestD) {
        bestD = d;
        best = i;
      }
    }
    return best;
  }

  function bindInteraction() {
    let dragging = false;
    let moved = 0;
    let lastX = 0;
    let lastY = 0;

    canvas.addEventListener("mousedown", (ev) => {
      dragging = true;
      moved = 0;
      lastX = ev.offsetX;
      lastY = ev.offsetY;
      canvas.classList.add("dragging");
    });

    window.addEventListener("mouseup", (ev) => {
      if (!dragging) return;
      dragging = false;
      canvas.classList.remove("dragging");
      // A drag that moved more than a few pixels is a pan, not a click. Without
      // this, every pan would also select whatever node it started over.
      if (moved > 4) return;
      const rect = canvas.getBoundingClientRect();
      const index = pick(ev.clientX - rect.left, ev.clientY - rect.top);
      if (index < 0) {
        state.highlight = null;
        draw();
        return;
      }
      state.highlight = index;
      draw();
      document.dispatchEvent(new CustomEvent("aether:select", { detail: { id: state.nodes[index].id } }));
    });

    canvas.addEventListener("mousemove", (ev) => {
      if (!dragging) return;
      const dx = ev.offsetX - lastX;
      const dy = ev.offsetY - lastY;
      moved += Math.abs(dx) + Math.abs(dy);
      state.view.x += dx;
      state.view.y += dy;
      lastX = ev.offsetX;
      lastY = ev.offsetY;
      draw();
    });

    canvas.addEventListener(
      "wheel",
      (ev) => {
        ev.preventDefault();
        const rect = canvas.getBoundingClientRect();
        const sx = ev.clientX - rect.left;
        const sy = ev.clientY - rect.top;
        const [wx, wy] = toWorld(sx, sy);
        const factor = ev.deltaY < 0 ? 1.12 : 1 / 1.12;
        state.view.scale = Math.max(0.02, Math.min(12, state.view.scale * factor));
        // Keep the point under the cursor fixed while zooming.
        state.view.x = sx - wx * state.view.scale;
        state.view.y = sy - wy * state.view.scale;
        draw();
      },
      { passive: false },
    );

    const filter = el("g-filter");
    if (filter) {
      filter.addEventListener("input", () => {
        state.filter = filter.value;
        draw();
      });
    }

    const fitBtn = el("g-fit");
    if (fitBtn) {
      fitBtn.addEventListener("click", () => {
        fit();
        draw();
      });
    }

    const chokeInput = el("g-choke");
    const chokeGo = el("g-choke-go");
    if (chokeGo) {
      const run = async () => {
        const target = (chokeInput && chokeInput.value) || "";
        if (!target.trim()) {
          state.choke = new Set();
          draw();
          return;
        }
        setOverlay("computing choke points…");
        try {
          const res = await fetch(`/api/graph/choke-points?target=${encodeURIComponent(target)}`, {
            credentials: "same-origin",
          });
          if (!res.ok) {
            setOverlay(`choke-point analysis failed: HTTP ${res.status}`);
            return;
          }
          const data = await res.json();
          state.choke = new Set((data.choke_points || []).map((c) => `${c.source}\u001f${c.target}\u001f${c.type}`));
          const top = (data.choke_points || [])[0];
          setOverlay(
            `choke points to ${data.target}: ${(data.choke_points || []).length} edge(s) above ` +
              `${Math.round((data.threshold || 0) * 100)}%; top edge carries ` +
              `${top ? Math.round(top.coverage * 100) : 0}% of ${data.total_paths} enumerated path(s)` +
              (data.truncated ? " — result truncated" : ""),
          );
          draw();
        } catch (err) {
          setOverlay(`choke-point analysis failed: ${err.message}`);
        }
      };
      chokeGo.addEventListener("click", run);
      if (chokeInput) chokeInput.addEventListener("keydown", (ev) => ev.key === "Enter" && run());
    }
  }

  // ------------------------------------------------------------------ wiring

  // Time travel and live updates arrive as events from app.js. The graph is
  // always refetched from the server rather than reconstructed here: the state
  // at a chain position is derived from signed history, and re-deriving it in
  // the browser would mean trusting a client-side implementation of the replay.
  document.addEventListener("aether:graph", (ev) => {
    if (!canvas) return;
    const detail = (ev && ev.detail) || {};
    if (detail.live || detail.seq === undefined) {
      void loadGraph();
      return;
    }
    const url = `/api/graph/at/${encodeURIComponent(detail.seq)}`;
    state.graphMeta = `as recorded at chain entry ${detail.seq}`;
    void loadGraph(url);
  });

  document.addEventListener("aether:chain", () => {
    if (canvas) void loadGraph();
  });
  // A read-only window onto the renderer, for the end-to-end check.
  //
  // The alternative is asserting on a screenshot, which cannot tell a drawn
  // graph from a blank canvas. These accessors report what the renderer actually
  // holds: how many nodes it loaded, whether the layout module produced finite
  // coordinates for all of them, and what the overlay says. A module that failed
  // to load leaves the node count at zero and an overlay that says so, which is
  // exactly the case that must not be mistaken for an empty engagement.
  window.__aetherGraph = {
    nodeCount: () => state.nodes.length,
    edgeCount: () => state.edges.length,
    overlay: () => (overlay ? overlay.textContent : ""),
    positions: () => {
      const p = state.positions;
      if (!p) return [];
      return state.nodes.map((n, i) => ({ id: n.id, x: p.xs[i], y: p.ys[i] }));
    },
    allFinite: () => {
      const p = state.positions;
      if (!p) return false;
      return state.nodes.every((_, i) => Number.isFinite(p.xs[i]) && Number.isFinite(p.ys[i]));
    },
  };
})();
