package cli

import "testing"

// Regression tests for the Stage 34 forensic findings:
//   - F-34-1: malformed config files must produce a visible warning
//     instead of being silently ignored (fail-open configuration).
//     Behavior verified live; the level helper is unit-tested here.
//   - F-34-2: invalid --log-level values must be rejected (normalized
//     to "info" with a warning), never silently accepted.

func TestNormalizeLogLevelAcceptsValid(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		if got := normalizeLogLevel(level); got != level {
			t.Errorf("normalizeLogLevel(%q) = %q, want %q", level, got, level)
		}
	}
}

func TestNormalizeLogLevelRejectsInvalid(t *testing.T) {
	for _, level := range []string{"notalevel", "", "DEBUG", "Info", "trace"} {
		if got := normalizeLogLevel(level); got != "info" {
			t.Errorf("normalizeLogLevel(%q) = %q, want fallback %q", level, got, "info")
		}
	}
}
