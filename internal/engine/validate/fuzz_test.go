package validate

import (
	"strings"
	"testing"
)

func TestFuzzTelemetryCount(t *testing.T) {
	base := map[string]any{
		"time":      "2026-09-09T12:00:00Z",
		"Operation": "SigninLogs",
		"User":      "admin@corp.com",
	}
	variants := FuzzTelemetry(base, 25)
	if len(variants) != 25 {
		t.Fatalf("variants = %d", len(variants))
	}
	for _, v := range variants {
		if len(v.Mutations) < 1 || len(v.Mutations) > 4 {
			t.Errorf("mutations = %v (out of range)", v.Mutations)
		}
	}
}

func TestFuzzTelemetryBounds(t *testing.T) {
	base := map[string]any{"time": "2026-09-09T12:00:00Z"}
	if got := len(FuzzTelemetry(base, 0)); got != 10 {
		t.Errorf("count 0 → %d, want 10 (default)", got)
	}
	if got := len(FuzzTelemetry(base, 500)); got != 100 {
		t.Errorf("count 500 → %d, want 100 (capped)", got)
	}
}

func TestFuzzDeterministicWithSeed(t *testing.T) {
	base := map[string]any{"time": "2026-09-09T12:00:00Z", "User": "a@b.c"}

	fuzzRNG = randNew(42)
	a := FuzzTelemetry(base, 20)
	fuzzRNG = randNew(42)
	b := FuzzTelemetry(base, 20)

	if len(a) != len(b) {
		t.Fatalf("length mismatch")
	}
	for i := range a {
		if strings.Join(a[i].Mutations, ",") != strings.Join(b[i].Mutations, ",") {
			t.Errorf("variant %d differs: %v vs %v", i, a[i].Mutations, b[i].Mutations)
		}
	}
}

func TestFuzzJSONNDJSON(t *testing.T) {
	base := map[string]any{"time": "2026-09-09T12:00:00Z", "User": "a@b.c"}
	variants := FuzzTelemetry(base, 5)

	out, err := FuzzJSON(variants)
	if err != nil {
		t.Fatalf("fuzz json: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 5 {
		t.Errorf("lines = %d, want 5", len(lines))
	}
	if !strings.HasPrefix(lines[0], `{"mutations":`) {
		t.Errorf("line 0 = %q", lines[0])
	}
}

func TestRenderFuzzSummary(t *testing.T) {
	base := map[string]any{"time": "2026-09-09T12:00:00Z", "User": "a@b.c"}
	variants := FuzzTelemetry(base, 20)

	out := RenderFuzzSummary(variants)
	if !strings.Contains(out, "Telemetry Fuzz Batch: 20 variants") {
		t.Errorf("summary = %q", out)
	}
}
