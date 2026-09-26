// Aether dashboard — page behaviour.
//
// # Scope
//
// This file does three things and nothing else:
//
//   1. refreshes server-rendered partials over GET, for the panels that are
//      plain server state (chain status, engagement summary, choke points, diff);
//   2. drives the two controls that are not plain state: the time-travel slider
//      and the replay button;
//   3. owns the live push channel and hands new chain state to the other
//      modules through DOM events.
//
// It is deliberately dependency-free. There is no framework, no build step and
// no vendored library: the partial-refresh logic below is about sixty lines,
// and a framework would have to be trusted with a page that displays signed
// security evidence and hands it a green banner. Everything the operator sees
// about integrity is computed by verify.wasm in this same tab, not by this file.
//
// # Read-only by construction
//
// Every request this file makes is a GET, and every endpoint it can reach is
// registered GET-only server-side. There is no code path here that could mutate
// anything, so there is nothing to disable and no CSRF surface to defend: a
// cross-site page cannot make this tab issue a state change because this tab
// issues no state changes at all.

// ---------------------------------------------------------------- partials

// escapeText renders a value for inclusion in HTML.
//
// The only strings passed through it are strings this file already holds (a URL,
// an error message, a status code), so this is a belt-and-braces measure rather
// than the primary defence. It exists because a page that displays security
// evidence should not have a code path that can emit an unescaped interpolation,
// even one that is currently unreachable from attacker-controlled input.
const HTML_ESCAPES = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" };

function escapeText(s) {
  return String(s).replace(/[&<>"']/g, (c) => HTML_ESCAPES[c]);
}

// swap performs a GET and replaces a target element's HTML.
//
// The response is HTML the server rendered, so inserting it is a DOM operation
// on already-escaped markup; no value from the response is ever evaluated.
async function swap(el, url) {
  if (!el) return;
  el.classList.add("aether-request");
  try {
    const res = await fetch(url, { credentials: "same-origin", headers: { Accept: "text/html" } });
    if (!res.ok) {
      el.innerHTML = `<div class="banner warn"><strong>Request failed.</strong> ${escapeText(
        `the server returned ${res.status} for ${url}`,
      )}</div>`;
      return;
    }
    el.innerHTML = await res.text();
    // The new markup can contain more partials (the choke-point panel's own
    // button, for example), so bind before the operator can touch them.
    bindPartials(el);
    applyDataWidths(el);
  } catch (err) {
    el.innerHTML = `<div class="banner warn"><strong>Request failed.</strong> ${escapeText(String(err))}</div>`;
  } finally {
    el.classList.remove("aether-request");
  }
}

// applyDataWidths turns data-w into a CSS custom property.
//
// The dashboard's CSP is `style-src 'self'`, which refuses style attributes in
// parsed markup, so a coverage bar cannot carry `style="width:52%"`. Writing the
// property through the CSSOM is not a style attribute and is not restricted,
// which is why the value travels in the markup as data and is applied here.
//
// The value is validated rather than interpolated: anything that is not a
// percentage is ignored, so a malformed number from a future template cannot
// turn into an arbitrary CSS declaration.
function applyDataWidths(root) {
  for (const el of root.querySelectorAll("[data-w]")) {
    const raw = String(el.getAttribute("data-w") || "").trim();
    if (!/^\d{1,3}(\.\d+)?%$/.test(raw)) continue;
    el.style.setProperty("--w", raw);
  }
}

// includeParams reads the values of the selectors named by an hx-include
// attribute. Only same-document ids are honoured: there is nothing here that
// should ever read a value from another origin.
function includeParams(attr) {
  const out = new URLSearchParams();
  for (const sel of attr.split(",")) {
    const id = sel.trim().replace(/^#/, "");
    if (!id) continue;
    const field = document.getElementById(id);
    if (!field) continue;
    if (field.type === "checkbox" || field.type === "radio") {
      if (field.checked) out.append(id, field.value);
    } else if (field.value !== undefined) {
      out.append(id, field.value.trim());
    }
  }
  return out;
}

function withParams(url, params) {
  if ([...params.keys()].length === 0) return url;
  return url + (url.includes("?") ? "&" : "?") + params.toString();
}

// triggerFor resolves an hx-trigger value into a listener name and an interval.
//
// Only the three forms the templates actually use are supported: no trigger at
// all (click), "load" (once, immediately) and "every <n>s". An unrecognised
// value is a no-op rather than a guess, so a typo in a template cannot turn into
// a request the operator did not ask for.
function triggerFor(value) {
  if (!value) return { event: "click", interval: 0 };
  const every = /^every\s+(\d+)s$/.exec(value.trim());
  if (every) return { event: "load", interval: Number(every[1]) * 1000 };
  if (value.trim() === "load") return { event: "load", interval: 0 };
  return { event: value.trim(), interval: 0 };
}

const bound = new WeakSet();

function bindPartials(root = document) {
  for (const el of root.querySelectorAll("[hx-get]")) {
    if (bound.has(el)) continue;
    bound.add(el);
    const url = el.getAttribute("hx-get");
    const include = el.getAttribute("hx-include") || "";
    const targetSel = el.getAttribute("hx-target");
    const swapMode = el.getAttribute("hx-swap") || "innerHTML";
    const { event, interval } = triggerFor(el.getAttribute("hx-trigger"));

    const fire = () => {
      let target = el;
      if (targetSel) {
        target = document.querySelector(targetSel);
        if (!target) return;
      }
      // outerHTML is used nowhere in the templates; if it ever is, honouring it
      // would mean replacing the very element that carries the trigger.
      if (swapMode !== "innerHTML") return;
      void swap(target, withParams(url, includeParams(include)));
    };

    if (event === "load") {
      if (interval > 0) {
        // Polling. The interval is cleared when the element leaves the document
        // so a swapped-out panel does not keep a timer alive for ever.
        const id = setInterval(() => {
          if (!el.isConnected) {
            clearInterval(id);
            return;
          }
          fire();
        }, interval);
      } else {
        fire();
      }
    } else {
      el.addEventListener(event, fire);
    }
  }
}

// ---------------------------------------------------------------- time travel

// The slider position is a real chain sequence number, so moving it asks the
// server to reconstruct that exact state from signed history. The picture is
// never recomputed client-side from an untrusted hint.
function initTimeTravel() {
  const host = document.getElementById("time-travel");
  if (!host) return;
  const slider = host.querySelector("#seq-slider");
  const label = host.querySelector("#seq-label");
  const play = host.querySelector("#play-btn");
  const live = host.querySelector("#live-btn");
  const status = host.querySelector("#replay-status");

  const showAt = (seq, note) => {
    if (label) label.textContent = `seq ${seq}`;
    if (status) status.textContent = note || "";
    document.dispatchEvent(new CustomEvent("aether:graph", { detail: { seq } }));
  };

  if (slider) {
    slider.addEventListener("change", () => {
      const seq = Number(slider.value);
      showAt(seq, `requesting the graph as recorded at seq ${seq}`);
      void swap(host, `/partials/time-travel?seq=${seq}`);
    });
  }

  if (live) {
    live.addEventListener("click", () => {
      if (slider) slider.value = slider.max;
      showAt(Number(slider ? slider.max : 0), "showing the live chain head");
      void swap(host, "/partials/time-travel");
    });
  }

  if (play) {
    play.addEventListener("click", () => {
      if (!slider) return;
      // Stepping is done with a timer rather than a tight loop so the browser
      // can paint between positions; a long chain would otherwise appear to
      // hang with no indication that anything is happening.
      let seq = Number(slider.min);
      const step = Math.max(1, Math.round((Number(slider.max) - Number(slider.min)) / 40));
      play.disabled = true;
      const tick = () => {
        seq = Math.min(Number(slider.max), seq + step);
        slider.value = seq;
        showAt(seq, `replaying to seq ${seq} of ${slider.max}`);
        void swap(host, `/partials/time-travel?seq=${seq}`);
        if (seq >= Number(slider.max)) {
          play.disabled = false;
          if (status) status.textContent = "replay finished at the chain head";
          return;
        }
        setTimeout(tick, 60);
      };
      tick();
    });
  }
}

// ---------------------------------------------------------------- proof panel

function initProofPanel() {
  const sidebar = document.getElementById("proof-sidebar");
  const host = document.getElementById("graph-node-panel");
  if (!sidebar || !host) return;

  // Delegated, because the panel's contents are replaced on every selection and
  // a directly-attached listener would die with the old markup.
  document.addEventListener("click", (ev) => {
    const closer = ev.target.closest("[data-close-panel]");
    if (closer) {
      sidebar.classList.remove("open");
      return;
    }
    const row = ev.target.closest(".choke-row[data-edge]");
    if (row) {
      // A choke-point row is an edge, and an edge has provenance too.
      void select(host, sidebar, `edge:${row.dataset.edge}`);
    }
  });

  document.addEventListener("aether:select", (ev) => {
    void select(host, sidebar, ev.detail && ev.detail.id);
  });
}

async function select(host, sidebar, id) {
  if (!id) return;
  await swap(host, `/partials/graph-node?id=${encodeURIComponent(id)}`);
  sidebar.classList.add("open");
}

// ---------------------------------------------------------------- push channel

// The WebSocket is push-only: the server never reads a command from it, and
// neither does this file. It exists so an append by another process shows up
// without the operator reloading.
function initPush() {
  if (!("WebSocket" in window)) return;
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  let socket;
  let retry;

  const badge = document.createElement("span");
  badge.className = "verdict-chip";
  badge.id = "live-badge";
  badge.textContent = "live: connecting";
  const verdicts = document.querySelector(".verdicts");
  if (verdicts) verdicts.appendChild(badge);

  const connect = () => {
    socket = new WebSocket(`${proto}//${location.host}/ws`);
    socket.addEventListener("open", () => {
      badge.textContent = "live: connected";
      badge.classList.add("ok");
    });
    socket.addEventListener("message", (ev) => {
      let msg;
      try {
        msg = JSON.parse(ev.data);
      } catch {
        return;
      }
      if (msg.type === "audit.overflow") {
        // The server dropped messages because this tab fell behind. Rather than
        // pretend the stream is complete, re-read the chain from the start.
        badge.textContent = "live: fell behind, reloading";
        badge.classList.remove("ok");
        document.dispatchEvent(new CustomEvent("aether:chain"));
        return;
      }
      if (msg.type === "audit.head") {
        document.dispatchEvent(new CustomEvent("aether:chain", { detail: msg }));
        refreshChainStatus();
        requestLiveGraph();
      }
    });
    socket.addEventListener("close", () => {
      badge.textContent = "live: disconnected";
      badge.classList.remove("ok");
      // Reconnect with a floor on the delay so a server that is down does not
      // turn into a reconnect storm from every open tab.
      clearTimeout(retry);
      retry = setTimeout(connect, 5000);
    });
    socket.addEventListener("error", () => socket.close());
  };

  connect();
}

function refreshChainStatus() {
  const host = document.getElementById("chain-status");
  if (host) void swap(host, "/partials/chain-status");
}

function requestLiveGraph() {
  if (!document.getElementById("graph-canvas")) return;
  document.dispatchEvent(new CustomEvent("aether:graph", { detail: { live: true } }));
}

// ------------------------------------------------------------------ startup

document.addEventListener("DOMContentLoaded", () => {
  bindPartials();
  // The server-rendered partials are in the document before any script runs, so
  // the bars they contain need the same treatment the swapped ones get.
  applyDataWidths(document);
  initTimeTravel();
  initProofPanel();
  initPush();
  requestLiveGraph();
});
