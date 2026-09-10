package behavior

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestShapeChromeHeaders(t *testing.T) {
	s := &Shaper{Persona: PersonaEngineer}
	req := httptest.NewRequest("GET", "https://login.microsoftonline.com/", nil)
	s.Shape(req, "chrome")

	for _, h := range []string{"Sec-Ch-Ua", "Sec-Fetch-Site", "Accept-Language"} {
		if req.Header.Get(h) == "" {
			t.Errorf("missing header %s", h)
		}
	}
	if !strings.Contains(req.Header.Get("Sec-Ch-Ua"), "Google Chrome") {
		t.Errorf("chrome brand missing: %q", req.Header.Get("Sec-Ch-Ua"))
	}
	if !strings.Contains(req.Header.Get("User-Agent"), "Chrome/120") {
		t.Errorf("ua = %q", req.Header.Get("User-Agent"))
	}
}

func TestShapeEdgeHeaders(t *testing.T) {
	s := &Shaper{Persona: PersonaAnalyst}
	req := httptest.NewRequest("GET", "https://x.com/", nil)
	s.Shape(req, "edge")
	if !strings.Contains(req.Header.Get("Sec-Ch-Ua"), "Microsoft Edge") {
		t.Errorf("edge brand missing: %q", req.Header.Get("Sec-Ch-Ua"))
	}
}

func TestShapePersonaLanguage(t *testing.T) {
	req1 := httptest.NewRequest("GET", "https://x.com/", nil)
	(&Shaper{Persona: PersonaEngineer}).Shape(req1, "chrome")
	if got := req1.Header.Get("Accept-Language"); got != "en-US,en;q=0.9" {
		t.Errorf("engineer lang = %q", got)
	}

	req2 := httptest.NewRequest("GET", "https://x.com/", nil)
	(&Shaper{Persona: PersonaHR}).Shape(req2, "chrome")
	if got := req2.Header.Get("Accept-Language"); got != "en-US,en;q=0.8,es;q=0.6" {
		t.Errorf("hr lang = %q", got)
	}
}

func TestShapeUnknownPresetDefaultsToChrome(t *testing.T) {
	s := &Shaper{Persona: PersonaEngineer}
	req := httptest.NewRequest("GET", "https://x.com/", nil)
	s.Shape(req, "safari-unknown")
	if !strings.Contains(req.Header.Get("Sec-Ch-Ua"), "Google Chrome") {
		t.Error("unknown preset should fall back to chrome set")
	}
}

func TestShapeOperatorOverridesWin(t *testing.T) {
	s := &Shaper{
		Persona:      PersonaEngineer,
		ExtraHeaders: map[string]string{"X-Custom": "yes", "User-Agent": "CustomAgent/1.0"},
	}
	req := httptest.NewRequest("GET", "https://x.com/", nil)
	s.Shape(req, "chrome")
	if req.Header.Get("X-Custom") != "yes" {
		t.Error("extra header not applied")
	}
	if req.Header.Get("User-Agent") != "CustomAgent/1.0" {
		t.Errorf("override UA = %q", req.Header.Get("User-Agent"))
	}
}

func TestHeaderNamesSortedDeterministic(t *testing.T) {
	s := &Shaper{Persona: PersonaEngineer}
	a := s.HeaderNamesSorted("chrome")
	b := s.HeaderNamesSorted("chrome")
	if strings.Join(a, ",") != strings.Join(b, ",") {
		t.Errorf("sorted headers differ: %v vs %v", a, b)
	}
}
