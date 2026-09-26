package cli

import (
	"log/slog"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// These tests exist because G53R-25 could not be closed for want of any way to
// observe the configuration chain. Two separate defects were behind it:
//
//   - --log-level had no consumer. It was parsed, validated and written back to
//     viper, then read by nothing, so no setting had an observable effect.
//   - the environment tier was inoperative. viper derives an environment
//     variable name from the key, so the hyphenated key "log-level" mapped to
//     "LOG-LEVEL", which no shell sets. AETHER_LOG_LEVEL was silently ignored
//     and the operator was told nothing.
//
// Each case runs against a fresh viper rather than the global one. initConfig
// ends by pinning the resolved level with viper.Set, which outranks the
// environment, so it is not safe to call twice in one process and cannot be
// used to observe a second environment.

// boundLevelViper mirrors production wiring: a default, a bound flag, and the
// environment tier declared through applyEnvWiring.
func boundLevelViper(flagValue string, flagChanged bool) *viper.Viper {
	v := viper.New()
	v.SetDefault("log-level", "info")
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("log-level", "info", "")
	if err := fs.Set("log-level", flagValue); err != nil {
		panic(err)
	}
	fs.Lookup("log-level").Changed = flagChanged
	if err := v.BindPFlag("log-level", fs.Lookup("log-level")); err != nil {
		panic(err)
	}
	applyEnvWiring(v)
	return v
}

func TestEnvTierReachesTheSetting(t *testing.T) {
	t.Setenv("AETHER_LOG_LEVEL", "debug")

	if got := boundLevelViper("info", false).GetString("log-level"); got != "debug" {
		t.Errorf("AETHER_LOG_LEVEL=debug did not reach the setting: got %q, want %q", got, "debug")
	}
}

// TestEnvTierIsIgnoredWithoutTheWiring is the regression for the original
// defect: the same environment variable against a viper wired the old way.
func TestEnvTierIsIgnoredWithoutTheWiring(t *testing.T) {
	t.Setenv("AETHER_LOG_LEVEL", "debug")

	v := viper.New()
	v.SetDefault("log-level", "info")
	v.AutomaticEnv()

	got := v.GetString("log-level")
	if got == "debug" {
		t.Skip("this viper honours the environment without a prefix; the premise of the defect no longer holds")
	}
	if got != "info" {
		t.Errorf("unwired viper returned %q, want the default %q", got, "info")
	}
}

func TestFlagBeatsEnvironment(t *testing.T) {
	t.Setenv("AETHER_LOG_LEVEL", "debug")

	if got := boundLevelViper("error", true).GetString("log-level"); got != "error" {
		t.Errorf("the flag did not outrank the environment: got %q, want %q", got, "error")
	}
}

func TestUntouchedFlagDoesNotMaskEnvironment(t *testing.T) {
	t.Setenv("AETHER_LOG_LEVEL", "debug")

	if got := boundLevelViper("info", false).GetString("log-level"); got != "debug" {
		t.Errorf("an untouched flag masked the environment: got %q, want %q", got, "debug")
	}
}

func TestDefaultAppliesWithNoFlagOrEnvironment(t *testing.T) {
	t.Setenv("AETHER_LOG_LEVEL", "")

	if got := boundLevelViper("info", false).GetString("log-level"); got != "info" {
		t.Errorf("default tier: got %q, want %q", got, "info")
	}
}

func TestSlogLevelForCoversEveryAcceptedValue(t *testing.T) {
	for _, tc := range []struct {
		level string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		// normalizeLogLevel has already warned and substituted info by the
		// time this is reached, so an unrecognised value must not widen it.
		{"notalevel", slog.LevelInfo},
		{"", slog.LevelInfo},
	} {
		if got := slogLevelFor(tc.level); got != tc.want {
			t.Errorf("slogLevelFor(%q) = %v, want %v", tc.level, got, tc.want)
		}
	}
}

func TestInvalidLevelFallsBackToInfo(t *testing.T) {
	t.Setenv("AETHER_LOG_LEVEL", "notalevel")

	v := boundLevelViper("info", false)
	got := normalizeLogLevel(v.GetString("log-level"))
	if got != "info" {
		t.Errorf("an invalid level did not fall back to info: got %q", got)
	}
	if slogLevelFor(got) != slog.LevelInfo {
		t.Error("an invalid level left the threshold somewhere other than info")
	}
}
