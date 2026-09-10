package behavior

import (
	"net/http"
	"sort"
	"strings"
)

// Shaper applies browser-realistic header sets and pacing metadata to
// outbound requests, closing the gap between protocol-accurate traffic
// and human-generated traffic.
type Shaper struct {
	// Persona drives the Accept-Language and timing hints.
	Persona Persona
	// ExtraHeaders are applied last (operator overrides).
	ExtraHeaders map[string]string
}

// browserHeaderSets are the header fingerprints of real browser stacks.
var browserHeaderSets = map[string][][2]string{
	"chrome": {
		{"sec-ch-ua", `"Chromium";v="120", "Google Chrome";v="120", "Not?A_Brand";v="99"`},
		{"sec-ch-ua-mobile", "?0"},
		{"sec-ch-ua-platform", `"Windows"`},
		{"sec-fetch-dest", "empty"},
		{"sec-fetch-mode", "cors"},
		{"sec-fetch-site", "same-origin"},
		{"accept", "application/json, text/plain, */*"},
		{"accept-language", "en-US,en;q=0.9"},
	},
	"edge": {
		{"sec-ch-ua", `"Chromium";v="120", "Microsoft Edge";v="120", "Not?A_Brand";v="99"`},
		{"sec-ch-ua-mobile", "?0"},
		{"sec-ch-ua-platform", `"Windows"`},
		{"sec-fetch-dest", "empty"},
		{"sec-fetch-mode", "cors"},
		{"sec-fetch-site", "same-origin"},
		{"accept", "application/json, text/plain, */*"},
		{"accept-language", "en-US,en;q=0.9"},
	},
	"firefox": {
		{"accept", "application/json, text/plain, */*"},
		{"accept-language", "en-US,en;q=0.5"},
		{"sec-fetch-dest", "empty"},
		{"sec-fetch-mode", "cors"},
		{"sec-fetch-site", "same-origin"},
	},
}

// localeVariants rotate Accept-Language like real multi-locale users.
var localeVariants = map[Persona]string{
	PersonaEngineer: "en-US,en;q=0.9",
	PersonaHR:       "en-US,en;q=0.8,es;q=0.6",
	PersonaExec:     "en-US,en;q=0.9,fr;q=0.7",
	PersonaAnalyst:  "en-GB,en;q=0.9",
}

// Shape applies the browser-realistic header set to a request. The
// fingerprint preset selects the header family (chrome/edge/firefox).
func (s *Shaper) Shape(req *http.Request, preset string) {
	set, ok := browserHeaderSets[strings.ToLower(preset)]
	if !ok {
		set = browserHeaderSets["chrome"]
	}
	for _, kv := range set {
		req.Header.Set(kv[0], kv[1])
	}

	// Persona-consistent Accept-Language.
	if lang, ok := localeVariants[s.Persona]; ok {
		req.Header.Set("accept-language", lang)
	}

	for k, v := range s.ExtraHeaders {
		req.Header.Set(k, v)
	}

	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	}
}

// HeaderNamesSorted returns the shaped header keys in sorted order
// (deterministic inspection for tests and reports).
func (s *Shaper) HeaderNamesSorted(preset string) []string {
	req, _ := http.NewRequest(http.MethodGet, "https://login.microsoftonline.com/", nil)
	s.Shape(req, preset)

	names := make([]string, 0, len(req.Header))
	for k := range req.Header {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
