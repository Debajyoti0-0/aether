package transport

import (
	"context"
	"testing"
	"time"
)

func TestNoJitter(t *testing.T) {
	j := NoJitter{}
	start := time.Now()
	if err := j.Wait(context.Background()); err != nil {
		t.Fatalf("wait: %v", err)
	}
	if time.Since(start) > 50*time.Millisecond {
		t.Error("NoJitter should return immediately")
	}
}

func TestFixedJitter(t *testing.T) {
	j := FixedJitter{D: 20 * time.Millisecond}
	start := time.Now()
	if err := j.Wait(context.Background()); err != nil {
		t.Fatalf("wait: %v", err)
	}
	if time.Since(start) < 20*time.Millisecond {
		t.Error("FixedJitter should wait the fixed duration")
	}
}

func TestLowSlowJitterBounded(t *testing.T) {
	j := NewLowSlowJitter()
	start := time.Now()
	if err := j.Wait(context.Background()); err != nil {
		t.Fatalf("wait: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < j.Min {
		t.Errorf("elapsed %v below min %v", elapsed, j.Min)
	}
	if elapsed > j.Max+500*time.Millisecond {
		t.Errorf("elapsed %v above max %v", elapsed, j.Max)
	}
}

func TestLowSlowJitterCancel(t *testing.T) {
	j := LowSlowJitter{Min: 5 * time.Second, Max: 10 * time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := j.Wait(ctx); err == nil {
		t.Error("cancelled context should return error")
	}
}

func TestMemClean(t *testing.T) {
	blob := []byte("supersecret")
	MemClean(blob)
	for _, b := range blob {
		if b != 0 {
			t.Fatal("blob not zeroed")
		}
	}
	// Strings and nil safety.
	MemCleanStrings("secret", "more")
	MemClean(nil)
}
