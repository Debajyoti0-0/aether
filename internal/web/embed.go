package web

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"sort"
)

// assets holds every byte the dashboard serves. Nothing is fetched at runtime:
// there is no CDN reference, no font CDN, and no package manager output. The
// embed patterns are explicit rather than a blanket "all:" so that a stray
// developer file dropped into static/ or templates/ is a build error to
// resolve, not a file that silently ships inside the release binary.
//
// wasm/*.wasm and wasm/*/*.wasm are the compiled modules produced by
// Makefile.wasm. They are embedded rather than shipped beside the binary so
// the single-binary deployment property holds.
//
//go:embed templates/*.html templates/partials/*.html
//go:embed static/*.css static/*.js
//go:embed wasm/*.wasm
var assets embed.FS

// Asset exposes the embedded filesystem for tests and for the static handler.
func Asset() fs.FS { return assets }

// staticFS returns the static subtree rooted so that a request for
// /static/style.css maps to static/style.css.
func staticFS() (fs.FS, error) {
	sub, err := fs.Sub(assets, "static")
	if err != nil {
		return nil, fmt.Errorf("static assets: %w", err)
	}
	return sub, nil
}

// wasmFS returns the WebAssembly subtree rooted so that a request for
// /wasm/graph.wasm maps to wasm/graph.wasm.
func wasmFS() (fs.FS, error) {
	sub, err := fs.Sub(assets, "wasm")
	if err != nil {
		return nil, fmt.Errorf("wasm assets: %w", err)
	}
	return sub, nil
}

// AssetPresence reports which embedded assets exist, for the startup self-check
// and for the certification evidence. It returns the error text rather than a
// bare bool so a missing module can be diagnosed from the log.
func AssetPresence() map[string]string {
	want := []string{
		"templates/dashboard.html",
		"templates/audit.html",
		"templates/graph.html",
		"templates/partials/chain-status.html",
		"templates/partials/workspace-summary.html",
		"templates/partials/graph-node-panel.html",
		"templates/partials/time-travel.html",
		"templates/partials/diff-mode.html",
		"templates/partials/choke-points.html",
		"static/style.css",
		"static/app.js",
		"static/graph.js",
		"static/verify.js",
		"static/verifier.js",
		"wasm/verify.wasm",
		"wasm/graph.wasm",
	}
	out := make(map[string]string, len(want))
	for _, p := range want {
		b, err := assets.ReadFile(p)
		if err != nil {
			out[p] = "MISSING"
			continue
		}
		out[p] = fmt.Sprintf("%d bytes", len(b))
	}
	return out
}

// placeholderPattern is the marker text that identifies an unbuilt asset.
//
// A committed placeholder is the failure mode that lets a broken build look
// healthy: the embed succeeds, the file is served, and the page silently does
// nothing. The startup check refuses to report "ok" while any asset still
// contains it.
const placeholderPattern = "PLACEHOLDER"

// placeholders returns the embedded assets that are still placeholders, so the
// caller can refuse to start or warn loudly.
func placeholders() []string {
	var bad []string
	for _, p := range assetPaths() {
		b, err := assets.ReadFile(p)
		if err != nil {
			continue
		}
		if bytes.Contains(b, []byte(placeholderPattern)) {
			bad = append(bad, p)
		}
	}
	sort.Strings(bad)
	return bad
}

// assetPaths is the authoritative asset list, shared by AssetPresence and
// placeholders so the two can never disagree about what should exist.
func assetPaths() []string {
	return []string{
		"templates/dashboard.html",
		"templates/audit.html",
		"templates/graph.html",
		"templates/partials/chain-status.html",
		"templates/partials/workspace-summary.html",
		"templates/partials/graph-node-panel.html",
		"templates/partials/time-travel.html",
		"templates/partials/diff-mode.html",
		"templates/partials/choke-points.html",
		"static/style.css",
		"static/app.js",
		"static/graph.js",
		"static/verify.js",
		"static/verifier.js",
		"wasm/verify.wasm",
		"wasm/graph.wasm",
	}
}
