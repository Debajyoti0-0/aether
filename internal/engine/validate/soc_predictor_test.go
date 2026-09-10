package validate

import (
	"strings"
	"testing"
)

func TestSimulateKnownAction(t *testing.T) {
	p := NewPredictor()
	signals := p.Simulate("wstrust_relay", &Environment{})
	if len(signals) == 0 {
		t.Fatal("expected signals for wstrust_relay")
	}

	found50126 := false
	for _, s := range signals {
		if s.EntraIDLogID == "50126" {
			found50126 = true
		}
	}
	if !found50126 {
		t.Error("50126 signal missing")
	}
}

func TestSimulateUnknownAction(t *testing.T) {
	p := NewPredictor()
	if got := p.Simulate("nonexistent_action", &Environment{}); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestProbabilityEscalatesWithEnv(t *testing.T) {
	p := NewPredictor()

	quiet := &Environment{}
	loud := &Environment{UnknownLocation: true, NewDevice: true, TimeOfDay: "off_hours"}

	quietSignals := p.Simulate("prt_exchange", quiet)
	loudSignals := p.Simulate("prt_exchange", loud)

	probOf := func(signals []SOCSignal, id string) string {
		for _, s := range signals {
			if s.EntraIDLogID == id {
				return s.Probability
			}
		}
		return ""
	}

	q := probOf(quietSignals, "50177")
	l := probOf(loudSignals, "50177")
	if q == "" || l == "" {
		t.Fatalf("missing signals q=%q l=%q", q, l)
	}

	known := map[string]int{"low": 0, "medium": 1, "high": 2}
	if known[l] <= known[q] {
		t.Errorf("loud env should escalate probability: %s vs %s", q, l)
	}
}

func TestRenderSignals(t *testing.T) {
	p := NewPredictor()
	signals := p.Simulate("vm_runcommand", &Environment{})
	out := RenderSignals("vm_runcommand", signals)
	if !strings.Contains(out, "SOC Signal Prediction") {
		t.Errorf("render = %q", out)
	}
	if !strings.Contains(out, "RunCommand") {
		t.Errorf("render missing defender alert: %q", out)
	}

	empty := RenderSignals("unknown", nil)
	if !strings.Contains(empty, "No known SOC signals") {
		t.Errorf("empty render = %q", empty)
	}
}
