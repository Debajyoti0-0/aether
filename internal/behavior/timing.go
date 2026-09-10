package behavior

import (
	"context"
	"math"
	"math/rand"
	"strings"
	"time"
)

// Personas model distinct human operating rhythms. Delays, session
// lengths, and activity windows are drawn from the persona profile.
type Persona string

const (
	PersonaEngineer Persona = "engineer"
	PersonaHR       Persona = "hr"
	PersonaExec     Persona = "executive"
	PersonaAnalyst  Persona = "analyst"
)

// Profile captures the behavioral statistics of a persona.
type Profile struct {
	Name string
	// MeanDelay / StdDelay: seconds between successive actions
	// (clamped to Min/Max). Drawn from a Gaussian.
	MeanDelay float64
	StdDelay  float64
	MinDelay  time.Duration
	MaxDelay  time.Duration
	// SessionLength bounds for a realistic continuous work session.
	MinSession time.Duration
	MaxSession time.Duration
	// WorkHours are the local hours this persona is normally active.
	WorkStartHour int
	WorkEndHour   int
	// WeekendActive: some personas touch systems off-hours.
	WeekendActive bool
}

// DefaultProfiles are the shipped personas (mean/std in seconds).
var DefaultProfiles = map[Persona]Profile{
	PersonaEngineer: {
		Name: "engineer", MeanDelay: 45, StdDelay: 25,
		MinDelay: 8 * time.Second, MaxDelay: 4 * time.Minute,
		MinSession: 25 * time.Minute, MaxSession: 3 * time.Hour,
		WorkStartHour: 9, WorkEndHour: 18, WeekendActive: false,
	},
	PersonaHR: {
		Name: "hr", MeanDelay: 90, StdDelay: 45,
		MinDelay: 20 * time.Second, MaxDelay: 8 * time.Minute,
		MinSession: 45 * time.Minute, MaxSession: 2 * time.Hour,
		WorkStartHour: 8, WorkEndHour: 17, WeekendActive: false,
	},
	PersonaExec: {
		Name: "executive", MeanDelay: 180, StdDelay: 90,
		MinDelay: 30 * time.Second, MaxDelay: 15 * time.Minute,
		MinSession: 10 * time.Minute, MaxSession: 1 * time.Hour,
		WorkStartHour: 7, WorkEndHour: 19, WeekendActive: true,
	},
	PersonaAnalyst: {
		Name: "analyst", MeanDelay: 30, StdDelay: 15,
		MinDelay: 5 * time.Second, MaxDelay: 2 * time.Minute,
		MinSession: 1 * time.Hour, MaxSession: 4 * time.Hour,
		WorkStartHour: 8, WorkEndHour: 20, WeekendActive: true,
	},
}

// GetProfile resolves a persona by name (case-insensitive; unknown
// names default to engineer).
func GetProfile(p Persona) Profile {
	for k, prof := range DefaultProfiles {
		if strings.EqualFold(string(k), string(p)) {
			return prof
		}
	}
	return DefaultProfiles[PersonaEngineer]
}

// Pacer generates human-like delays between successive actions using a
// Gaussian distribution (Box-Muller), clamped to the persona bounds.
type Pacer struct {
	profile Profile
	rng     *rand.Rand
}

// NewPacer builds a pacer for a persona.
func NewPacer(p Persona, seed int64) *Pacer {
	var rng *rand.Rand
	if seed >= 0 {
		rng = rand.New(rand.NewSource(seed))
	} else {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return &Pacer{profile: GetProfile(p), rng: rng}
}

// gauss returns a standard normal sample (Box-Muller).
func (p *Pacer) gauss() float64 {
	u1 := p.rng.Float64()
	if u1 < 1e-9 {
		u1 = 1e-9
	}
	u2 := p.rng.Float64()
	return math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
}

// NextDelay returns the next human-like inter-action delay.
func (p *Pacer) NextDelay() time.Duration {
	sample := p.profile.MeanDelay + p.profile.StdDelay*p.gauss()
	sec := math.Max(float64(p.profile.MinDelay.Seconds()), math.Min(sample, float64(p.profile.MaxDelay.Seconds())))
	return time.Duration(sec * float64(time.Second))
}

// SessionLength draws a plausible continuous-session duration.
func (p *Pacer) SessionLength() time.Duration {
	lo := p.profile.MinSession.Seconds()
	hi := p.profile.MaxSession.Seconds()
	mean := (lo + hi) / 2
	std := (hi - lo) / 6 // ~95% within bounds
	sample := mean + std*p.gauss()
	sec := math.Max(lo, math.Min(sample, hi))
	return time.Duration(sec * float64(time.Second))
}

// WithinWorkHours reports whether t falls inside the persona's active
// window (honoring the weekend preference, local time).
func (p *Pacer) WithinWorkHours(t time.Time) bool {
	h := t.Hour()
	if h < p.profile.WorkStartHour || h >= p.profile.WorkEndHour {
		return false
	}
	wd := t.Weekday()
	if wd == time.Saturday || wd == time.Sunday {
		return p.profile.WeekendActive
	}
	return true
}

// NextActiveTime returns the next instant the persona would plausibly
// be active (now itself when already within the window). Hour
// boundaries are constructed in local time (safe for half-hour zones
// and DST transitions).
func (p *Pacer) NextActiveTime(now time.Time) time.Time {
	if p.WithinWorkHours(now) {
		return now
	}

	// Bounded search: at most 8 days ahead, hour granularity.
	candidate := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
	for i := 0; i < 24*8; i++ {
		candidate = candidate.Add(time.Hour)
		candidate = time.Date(candidate.Year(), candidate.Month(), candidate.Day(),
			candidate.Hour(), 0, 0, 0, candidate.Location())
		if p.WithinWorkHours(candidate) {
			return candidate
		}
	}
	return now
}

// Profile returns the pacer's active persona profile.
func (p *Pacer) Profile() Profile { return p.profile }

// Wait sleeps for the next human-like delay, honoring cancellation.
func (p *Pacer) Wait(ctx context.Context) error {
	return sleepCtx(ctx, p.NextDelay())
}

// WaitActive sleeps until the persona's next active window starts.
func (p *Pacer) WaitActive(ctx context.Context, now time.Time) error {
	return sleepCtx(ctx, DelayUntil(p.NextActiveTime(now), now))
}

// DelayUntil returns the non-negative wait until target.
func DelayUntil(target, now time.Time) time.Duration {
	d := target.Sub(now)
	if d < 0 {
		return 0
	}
	return d
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
