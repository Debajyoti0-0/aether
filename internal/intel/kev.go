package intel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// KEVEntry is one record from the CISA Known Exploited Vulnerabilities
// catalog (normalized from the official JSON feed).
type KEVEntry struct {
	CVEID      string `json:"cveID"`
	Vendor     string `json:"vendorProject"`
	Product    string `json:"product"`
	Severity   string `json:"-"`
	DateAdded  string `json:"dateAdded"`
	DueDate    string `json:"dueDate"`
	KnownRansomware bool `json:"knownRansomwareUse"`
}

// kevFeed is the official catalog document shape.
type kevFeed struct {
	Title   string     `json:"title"`
	Catalog string     `json:"catalogVersion"`
	Count   int        `json:"count"`
	Vulns   []KEVEntry `json:"vulnerabilities"`
}

// Official CISA KEV JSON feed.
const DefaultKEVURL = "https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json"

// PriorityBoostKEV is the multiplier applied to paths whose techniques
// exploit a KEV-listed vulnerability (+30% per the blueprint).
const PriorityBoostKEV = 1.3

// Catalog is a loaded KEV catalog with lookup support.
type Catalog struct {
	entries map[string]KEVEntry // CVE → entry
	Loaded  time.Time
	Source  string
}

// LoadKEVFromURL fetches the live CISA feed.
func LoadKEVFromURL(ctx context.Context, client *http.Client, url string) (*Catalog, error) {
	if url == "" {
		url = DefaultKEVURL
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch kev: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kev http %d", resp.StatusCode)
	}
	return parseKEV(body, url)
}

// LoadKEVFromFile loads a locally cached catalog copy.
func LoadKEVFromFile(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read kev file: %w", err)
	}
	return parseKEV(data, path)
}

func parseKEV(body []byte, source string) (*Catalog, error) {
	feed := &kevFeed{}
	if err := json.Unmarshal(body, feed); err != nil {
		return nil, fmt.Errorf("parse kev: %w", err)
	}
	c := &Catalog{
		entries: map[string]KEVEntry{},
		Loaded:  time.Now().UTC(),
		Source:  source,
	}
	for _, e := range feed.Vulns {
		c.entries[strings.ToUpper(e.CVEID)] = e
	}
	return c, nil
}

// IsKnownExploited checks a CVE against the catalog.
func (c *Catalog) IsKnownExploited(cve string) (KEVEntry, bool) {
	e, ok := c.entries[strings.ToUpper(strings.TrimSpace(cve))]
	return e, ok
}

// Len returns the catalog size.
func (c *Catalog) Len() int { return len(c.entries) }

// PrioritizedPath is an attack path with intel-adjusted priority.
type PrioritizedPath struct {
	Path        []string `json:"path"`
	BaseScore   float64  `json:"base_score"`
	KEVCVEs     []string `json:"kev_cves,omitempty"`
	Priority    float64  `json:"priority"` // base × boost
	Critical    bool     `json:"critical"`
}

// Score is the base priority of a path before intel boosts.
func Score(path []string) float64 {
	if len(path) == 0 {
		return 0
	}
	// Longer paths are more complex; weight by hop count with the
	// final hop (execution) dominating.
	base := 20.0 + 10.0*float64(len(path))
	if base > 100 {
		base = 100
	}
	return base
}

// Prioritize orders paths by intel-adjusted priority: paths exploiting
// KEV-listed CVEs get +30%. cvesByPath maps a path key (joined with
// ">") to the CVEs that path leverages.
func Prioritize(paths [][]string, cvesByPath map[string][]string, kev *Catalog) []PrioritizedPath {
	out := make([]PrioritizedPath, 0, len(paths))
	for _, p := range paths {
		key := strings.Join(p, ">")
		pp := PrioritizedPath{Path: p, BaseScore: Score(p)}

		if kev != nil {
			for _, cve := range cvesByPath[key] {
				if _, exploited := kev.IsKnownExploited(cve); exploited {
					pp.KEVCVEs = append(pp.KEVCVEs, cve)
				}
			}
		}
		pp.Priority = pp.BaseScore
		if len(pp.KEVCVEs) > 0 {
			pp.Priority = pp.BaseScore * PriorityBoostKEV
			if pp.Priority > 100 {
				pp.Priority = 100
			}
			pp.Critical = true
		}
		out = append(out, pp)
	}

	// Highest priority first.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Priority > out[i].Priority {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// RenderPrioritized formats prioritized paths for the terminal.
func RenderPrioritized(paths []PrioritizedPath) string {
	var b strings.Builder
	b.WriteString("=== Intel-Prioritized Attack Paths ===\n")
	for i, p := range paths {
		mark := fmt.Sprintf("%.0f", p.Priority)
		if p.Critical {
			mark += " [KEV-CRITICAL]"
		}
		fmt.Fprintf(&b, "%d. %s — %v\n", i+1, mark, p.Path)
	}
	return b.String()
}

// CacheKEV persists a fetched catalog for offline use.
func CacheKEV(c *Catalog, path string) error {
	data, err := os.ReadFile(c.Source) // re-read source bytes if it's a file
	if err != nil {
		// Source was a URL; re-marshal the parsed entries.
		feed := kevFeed{Title: "CISA KEV (aether cache)", Catalog: "aether-1"}
		for _, e := range c.entries {
			feed.Vulns = append(feed.Vulns, e)
		}
		data, err = json.MarshalIndent(feed, "", "  ")
		if err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0o600)
}
