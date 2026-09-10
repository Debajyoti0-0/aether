package graph

import (
	"encoding/json"
	"fmt"
	"strings"
)

// visNode is the JSON node shape embedded into the HTML export.
type visNode struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Type     string `json:"type"`
	Provider string `json:"provider"`
}

type visEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

// Visualize renders the identity graph as a self-contained interactive
// HTML file (inline SVG layout via a simple radial algorithm — no CDN,
// no external JS; works fully offline).
func Visualize(g *IdentityGraph, title string) (string, error) {
	nodes := make([]visNode, len(g.Nodes))
	for i, n := range g.Nodes {
		nodes[i] = visNode{ID: n.ID, Label: n.Label, Type: n.Type, Provider: n.Provider}
	}
	edges := make([]visEdge, len(g.Edges))
	for i, e := range g.Edges {
		edges[i] = visEdge{Source: e.Source, Target: e.Target, Type: e.Type}
	}

	nodesJSON, _ := json.Marshal(nodes)
	edgesJSON, _ := json.Marshal(edges)

	if title == "" {
		title = "Aether Identity Graph"
	}

	providerColors := map[string]string{
		"entra":  "#0078d4",
		"aws":    "#ff9900",
		"gcp":    "#34a853",
		"okta":   "#00297a",
		"onprem": "#a020f0",
	}
	colorsJSON, _ := json.Marshal(providerColors)

	var b strings.Builder
	fmt.Fprintf(&b, `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>%s</title>
<style>
 body{margin:0;background:#111;color:#eee;font-family:Segoe UI,monospace;overflow:hidden}
 #bar{padding:8px 14px;background:#1a1a1a;border-bottom:1px solid #333}
 .legend{display:inline-block;margin-left:16px;font-size:12px}
 .dot{display:inline-block;width:10px;height:10px;border-radius:5px;margin:0 4px 0 12px}
 svg text{fill:#eee;font-size:11px;pointer-events:none}
</style></head><body>
<div id="bar"><b>%s</b><span id="legend" class="legend"></span></div>
<svg id="g" width="100%%" height="97%%"></svg>
<script>
const NODES = %s;
const EDGES = %s;
const COLORS = %s;
const svg = document.getElementById('g');
const R = Math.min(innerWidth, innerHeight) * 0.38;
const CX = innerWidth/2, CY = innerHeight/2;

// legend
const provs = [...new Set(NODES.map(n=>n.provider))];
document.getElementById('legend').innerHTML = provs.map(p =>
  '<span class="dot" style="background:'+(COLORS[p]||'#888')+'"></span>'+p).join(' ');

// radial layout per provider ring
const counts = {}, idx = {};
provs.forEach(p => counts[p] = NODES.filter(n=>n.provider===p).length);
const pos = {};
NODES.forEach(n => {
  idx[n.provider] = (idx[n.provider]||0);
  const a = (idx[n.provider] / Math.max(1,counts[n.provider])) * 2*Math.PI;
  const rr = R * (0.45 + 0.55 * (provs.indexOf(n.provider)+1) / provs.length);
  pos[n.id] = {x: CX + rr*Math.cos(a) - CX*0 + CX*0 + rr*Math.sin(a)*0, y: 0};
  // proper placement
  pos[n.id] = {x: CX + rr*Math.cos(a), y: CY + rr*Math.sin(a)};
  idx[n.provider]++;
});

// edges
EDGES.forEach(e => {
  const s = pos[e.source], t = pos[e.target];
  if (!s || !t) return;
  const l = document.createElementNS('http://www.w3.org/2000/svg','line');
  l.setAttribute('x1',s.x); l.setAttribute('y1',s.y);
  l.setAttribute('x2',t.x); l.setAttribute('y2',t.y);
  l.setAttribute('stroke','#555'); l.setAttribute('stroke-width','1.5');
  svg.appendChild(l);
});

// nodes
NODES.forEach(n => {
  const p = pos[n.id]; if (!p) return;
  const g = document.createElementNS('http://www.w3.org/2000/svg','g');
  const c = document.createElementNS('http://www.w3.org/2000/svg','circle');
  c.setAttribute('cx',p.x); c.setAttribute('cy',p.y); c.setAttribute('r',14);
  c.setAttribute('fill',COLORS[n.provider]||'#888');
  c.setAttribute('stroke','#fff'); c.setAttribute('stroke-width','1');
  c.addEventListener('mouseenter', () => {
    c.setAttribute('r', 18);
    tip.textContent = n.id + ' [' + n.type + '/' + n.provider + '] ' + n.label;
  });
  c.addEventListener('mouseleave', () => { c.setAttribute('r',14); tip.textContent=''; });
  const t = document.createElementNS('http://www.w3.org/2000/svg','text');
  t.setAttribute('x',p.x); t.setAttribute('y',p.y-20); t.setAttribute('text-anchor','middle');
  t.textContent = n.label;
  g.appendChild(c); g.appendChild(t); svg.appendChild(g);
});

const tip = document.createElementNS('http://www.w3.org/2000/svg','text');
tip.setAttribute('x',16); tip.setAttribute('y',innerHeight-16);
svg.appendChild(tip);
</script></body></html>`, title, title, nodesJSON, edgesJSON, colorsJSON)

	return b.String(), nil
}

// SaveVisualize writes the HTML export.
func SaveVisualize(g *IdentityGraph, title, path string) error {
	html, err := Visualize(g, title)
	if err != nil {
		return err
	}
	return writeFile(path, []byte(html))
}
