package cap

import (
	"strings"
	"testing"
	"time"
)

func obs(pid string, active bool, day int, hour int) Observation {
	return Observation{
		PolicyID:  pid,
		Active:    active,
		Timestamp: time.Date(2026, 9, day, hour, 0, 0, 0, time.UTC),
	}
}

// bizHoursHistory models a policy active 09:00-18:00 over 3 days.
func bizHoursHistory() []Observation {
	return []Observation{
		obs("mfa-window", true, 7, 9), obs("mfa-window", false, 7, 18),
		obs("mfa-window", true, 8, 9), obs("mfa-window", false, 8, 18),
		obs("mfa-window", true, 9, 9), obs("mfa-window", false, 9, 18),
	}
}

func TestWindowsInfersBusinessHours(t *testing.T) {
	p := NewPredictor(bizHoursHistory())
	windows := p.Windows()
	if len(windows) != 1 {
		t.Fatalf("windows = %d", len(windows))
	}
	w := windows[0]
	if w.ActiveFrom != 9 || w.ActiveUntil != 18 {
		t.Errorf("window = %d-%d, want 9-18", w.ActiveFrom, w.ActiveUntil)
	}
	if w.AlwaysOn {
		t.Error("time-bounded policy should not be always-on")
	}
	// Stage 2: predictions are never certain — 3 days saturate at 0.66.
	if w.Confidence < 0.65 || w.Confidence > 0.67 {
		t.Errorf("confidence = %.2f, want ~0.66", w.Confidence)
	}
}

func TestIsActiveAt(t *testing.T) {
	p := NewPredictor(bizHoursHistory())

	noon := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	active, w, known := p.IsActiveAt("mfa-window", noon)
	if !known || !active {
		t.Errorf("noon should be active: %t %t %+v", active, known, w)
	}

	night := time.Date(2026, 9, 10, 23, 0, 0, 0, time.UTC)
	active, _, _ = p.IsActiveAt("mfa-window", night)
	if active {
		t.Error("23:00 should be inactive")
	}
}

func TestIsActiveAtWrapsMidnight(t *testing.T) {
	// Policy active 22:00 → 06:00 (next day).
	hist := []Observation{
		obs("night-watch", true, 7, 22), obs("night-watch", false, 8, 6),
		obs("night-watch", true, 8, 22), obs("night-watch", false, 9, 6),
	}
	p := NewPredictor(hist)

	midnight := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	active, _, _ := p.IsActiveAt("night-watch", midnight)
	if !active {
		t.Error("00:00 should be active for a 22-06 window")
	}

	noon := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	if active, _, _ = p.IsActiveAt("night-watch", noon); active {
		t.Error("12:00 should be inactive")
	}
}

func TestAlwaysOnPolicy(t *testing.T) {
	// Only activation events observed → always on.
	hist := []Observation{obs("always", true, 7, 9), obs("always", true, 8, 9)}
	p := NewPredictor(hist)

	windows := p.Windows()
	if !windows[0].AlwaysOn {
		t.Error("expected always-on")
	}
	active, _, _ := p.IsActiveAt("always", time.Date(2026, 9, 9, 3, 0, 0, 0, time.UTC))
	if !active {
		t.Error("always-on should be active at any hour")
	}
}

func TestForecastFindsQuietWindow(t *testing.T) {
	// Policy M active 09-18: forecast at 06:00 → best window is a
	// quiet hour (before 09:00 or after 18:00).
	hist := bizHoursHistory()
	hist = append(hist, obs("always-on-p2", true, 7, 0), obs("always-on-p2", true, 8, 0))

	p := NewPredictor(hist)
	f := p.Forecast(time.Date(2026, 9, 9, 6, 0, 0, 0, time.UTC))

	// At 06:00 only the always-on policy is active.
	if len(f.ActivePolicies) != 1 || f.ActivePolicies[0] != "always-on-p2" {
		t.Errorf("active now = %v", f.ActivePolicies)
	}
	if f.BestWindowRisk != "medium" {
		t.Errorf("best window risk = %q", f.BestWindowRisk)
	}
	// Best hour must be one where only the always-on policy fires.
	hour := f.BestWindowStart.Hour()
	if hour >= 9 && hour < 18 {
		t.Errorf("best window %02d:00 falls inside the busy 9-18 window", hour)
	}
}

func TestForecastConfidenceAndRender(t *testing.T) {
	p := NewPredictor(bizHoursHistory())
	f := p.Forecast(time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC))
	// Stage 2: forecast confidence caps at 0.66 (predictions are never
	// certain).
	if f.Confidence < 0.65 || f.Confidence > 0.67 {
		t.Errorf("confidence = %.2f", f.Confidence)
	}

	out := RenderForecast(f)
	for _, want := range []string{"CAP Forecast", "mfa-window", "Least-restrictive window"} {
		if !strings.Contains(out, want) {
			t.Errorf("render missing %q", want)
		}
	}
}

func TestPredictorEmpty(t *testing.T) {
	p := NewPredictor(nil)
	if windows := p.Windows(); len(windows) != 0 {
		t.Errorf("windows = %d", len(windows))
	}
	f := p.Forecast(time.Now())
	if len(f.ActivePolicies) != 0 || f.BestWindowRisk != "low" {
		t.Errorf("empty forecast = %+v", f)
	}
	// Stage 2 (T3): zero observations ⇒ confidence 0 (ClassUnknown),
	// never the old fabricated 100%.
	if f.Confidence != 0 {
		t.Errorf("empty forecast confidence = %.2f, want 0", f.Confidence)
	}
}

func TestConfidenceWithFewSamples(t *testing.T) {
	// Single observed day → 1/3 confidence (saturates at 3 days).
	hist := []Observation{obs("partial", true, 7, 9), obs("partial", false, 7, 17)}
	p := NewPredictor(hist)
	if w := p.Windows(); w[0].Confidence < 0.32 || w[0].Confidence > 0.35 {
		t.Errorf("confidence = %.2f, want ~0.33", w[0].Confidence)
	}
}
