package behavior

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestGaussianDelayBounded(t *testing.T) {
	p := NewPacer(PersonaEngineer, 42)
	for i := 0; i < 200; i++ {
		d := p.NextDelay()
		if d < p.profile.MinDelay || d > p.profile.MaxDelay {
			t.Fatalf("delay %v out of persona bounds [%v, %v]", d, p.profile.MinDelay, p.profile.MaxDelay)
		}
	}
}

func TestGaussianDelayDeterministicWithSeed(t *testing.T) {
	a := NewPacer(PersonaEngineer, 7)
	b := NewPacer(PersonaEngineer, 7)
	for i := 0; i < 50; i++ {
		if a.NextDelay() != b.NextDelay() {
			t.Fatalf("seeded pacers diverged at iteration %d", i)
		}
	}
}

func TestSessionLengthBounded(t *testing.T) {
	p := NewPacer(PersonaAnalyst, 99)
	for i := 0; i < 100; i++ {
		s := p.SessionLength()
		if s < p.profile.MinSession || s > p.profile.MaxSession {
			t.Fatalf("session %v out of bounds [%v, %v]", s, p.profile.MinSession, p.profile.MaxSession)
		}
	}
}

func TestWithinWorkHours(t *testing.T) {
	p := NewPacer(PersonaEngineer, 1) // 9-18, weekend inactive

	// Wednesday 14:00 local.
	wed := time.Date(2026, 9, 9, 14, 0, 0, 0, time.Local)
	if !p.WithinWorkHours(wed) {
		t.Error("Wednesday 14:00 should be within engineer hours")
	}

	// Wednesday 22:00.
	night := time.Date(2026, 9, 9, 22, 0, 0, 0, time.Local)
	if p.WithinWorkHours(night) {
		t.Error("Wednesday 22:00 should be outside hours")
	}

	// Saturday (Sept 12, 2026).
	sat := time.Date(2026, 9, 12, 14, 0, 0, 0, time.Local)
	if p.WithinWorkHours(sat) {
		t.Error("Saturday should be inactive for engineer persona")
	}
}

func TestNextActiveTime(t *testing.T) {
	p := NewPacer(PersonaEngineer, 1)

	// 03:00 local → next window starts at 09:00 the same day.
	night := time.Date(2026, 9, 9, 3, 0, 0, 0, time.Local)
	next := p.NextActiveTime(night)
	if want := time.Date(2026, 9, 9, 9, 0, 0, 0, time.Local); !next.Equal(want) {
		t.Errorf("next active = %v, want %v", next, want)
	}

	// Inside the window → now.
	noon := time.Date(2026, 9, 9, 12, 0, 0, 0, time.Local)
	if next := p.NextActiveTime(noon); !next.Equal(noon) {
		t.Errorf("in-window next = %v, want %v", next, noon)
	}

	// Saturday 14:00 → Monday 09:00 (weekend inactive).
	sat := time.Date(2026, 9, 12, 14, 0, 0, 0, time.Local)
	next = p.NextActiveTime(sat)
	if want := time.Date(2026, 9, 14, 9, 0, 0, 0, time.Local); !next.Equal(want) {
		t.Errorf("weekend next = %v, want %v", next, want)
	}

	// Delay helper is non-negative and correct.
	if d := DelayUntil(next, sat); d != 43*time.Hour {
		t.Errorf("weekend delay = %v, want 43h", d)
	}
}

func TestWaitRespectsContext(t *testing.T) {
	p := NewPacer(PersonaExec, 3) // long delays
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.Wait(ctx); err == nil {
		t.Error("cancelled context should abort the wait")
	}
}

func TestPersonasDiffer(t *testing.T) {
	// Executives pause far longer than analysts on average.
	analyst := NewPacer(PersonaAnalyst, 5)
	exec := NewPacer(PersonaExec, 5)

	var aSum, eSum float64
	for i := 0; i < 300; i++ {
		aSum += analyst.NextDelay().Seconds()
		eSum += exec.NextDelay().Seconds()
	}
	if eSum <= aSum {
		t.Errorf("executive mean (%.0f) should exceed analyst mean (%.0f)", eSum/300, aSum/300)
	}
}

func TestGetProfileDefaults(t *testing.T) {
	p := GetProfile("nonexistent")
	if p.Name != "engineer" {
		t.Errorf("unknown persona should default to engineer, got %q", p.Name)
	}
	if !strings.EqualFold(GetProfile("HR").Name, "hr") {
		t.Error("HR profile should resolve")
	}
}
