package transport

import (
	"context"
	"math/rand"
	"time"
)

// Jitter is the injected delay source for stealth pacing.
type Jitter interface {
	Wait(ctx context.Context) error
}

// LowSlowJitter sleeps a random 1-5s between operations to mimic human
// pacing (--low-slow mode).
type LowSlowJitter struct {
	Min time.Duration
	Max time.Duration
	RNG *rand.Rand
}

// NewLowSlowJitter builds a 1-5s jitterer.
func NewLowSlowJitter() *LowSlowJitter {
	return &LowSlowJitter{Min: 1 * time.Second, Max: 5 * time.Second}
}

// Wait blocks for the jittered duration, honoring ctx cancellation.
func (j *LowSlowJitter) Wait(ctx context.Context) error {
	min := j.Min
	if min <= 0 {
		min = time.Second
	}
	max := j.Max
	if max <= min {
		max = min + 4*time.Second
	}

	delta := max - min
	delay := min + time.Duration(randInt63n(int64(delta)))

	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// randInt63n is an indirection for deterministic tests.
var randInt63n = rand.Int63n

// FixedJitter always waits a fixed duration (tests).
type FixedJitter struct {
	D time.Duration
}

// Wait sleeps for the fixed duration.
func (f *FixedJitter) Wait(ctx context.Context) error {
	t := time.NewTimer(f.D)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// NoJitter performs no delay (default fast mode).
type NoJitter struct{}

// Wait returns immediately.
func (NoJitter) Wait(context.Context) error { return nil }

// MemClean zeroes sensitive byte slices before exit (--mem-clean).
// Go cannot guarantee GC-level erasure, but overwriting the backing
// arrays shortens the window for memory scraping.
func MemClean(blobs ...[]byte) {
	for _, b := range blobs {
		for i := range b {
			b[i] = 0
		}
	}
}

// MemCleanStrings zeroes the underlying bytes of strings (copy-on-write
// safe: this duplicates before zeroing to avoid immutable-data faults).
func MemCleanStrings(strs ...string) {
	for _, s := range strs {
		b := []byte(s)
		for i := range b {
			b[i] = 0
		}
	}
}
