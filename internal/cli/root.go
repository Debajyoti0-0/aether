package cli

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/Debajyoti0-0/aether/internal/paths"
	"github.com/Debajyoti0-0/aether/internal/version"
)

var rootCmd = &cobra.Command{
	Use:     "aether",
	Short:   "Aether - Modern Identity Security Testing Toolkit",
	Long:    "Aether is a unified toolkit for authorized security testing of modern identity fabrics.",
	Version: version.Version,
}

var (
	modulesMu sync.Mutex
	modules   []Module
)

type Module interface {
	Name() string
	Commands() []*cobra.Command
}

func RegisterModule(m Module) {
	modulesMu.Lock()
	defer modulesMu.Unlock()
	modules = append(modules, m)
}

func loadModules() {
	modulesMu.Lock()
	defer modulesMu.Unlock()
	for _, m := range modules {
		for _, cmd := range m.Commands() {
			rootCmd.AddCommand(cmd)
		}
	}
}

// SetVersion overrides the CLI version from build-time injection.
func SetVersion(v string) {
	if v != "" {
		rootCmd.Version = v
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().String("config", "", "Config file path")
	rootCmd.PersistentFlags().String("log-level", "info", "Log level (debug, info, warn, error)")

	_ = viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	_ = viper.BindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level"))
}

// NewRootCommand builds and returns the full command tree. It is used
// both by Execute() and by the docs/man generators.
func NewRootCommand() *cobra.Command {
	if !rootCmd.Runnable() {
		rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		}
	}
	return rootCmd
}

func Execute() error {
	loadModules()
	return NewRootCommand().Execute()
}

func initConfig() {
	if cfgFile := viper.GetString("config"); cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("aether")
		viper.SetConfigType("json")
		for _, dir := range paths.ConfigSearchPaths() {
			viper.AddConfigPath(dir)
		}
	}

	// The environment tier has to be declared, and the key replacer is what
	// makes a hyphenated key reachable at all: viper derives the variable name
	// from the key, so "log-level" would otherwise map to "LOG-LEVEL" and no
	// shell would ever set it. Without the prefix and the replacer an operator
	// could export AETHER_LOG_LEVEL, be told nothing was wrong, and get the
	// default anyway.
	applyEnvWiring(viper.GetViper())
	if err := viper.ReadInConfig(); err != nil {
		// A config file that exists but cannot be read must never be
		// silently ignored: an operator whose security-relevant settings
		// silently fall back to defaults is a fail-open configuration.
		// Only the "no config file found" case is expected and silent;
		// an explicit --config that fails to load, or any other read
		// error, is surfaced.
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			fmt.Fprintf(os.Stderr, "Warning: config file ignored: %v\n", err)
		}
	} else if viper.ConfigFileUsed() != "" {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}

	// Reject invalid --log-level values instead of silently accepting
	// them (F-34-2); the flag is reserved but its contract is strict.
	level := normalizeLogLevel(viper.GetString("log-level"))
	viper.Set("log-level", level)
	initLogging(level)
}

// applyEnvWiring declares the environment tier of the configuration chain.
//
// It is a separate function so the wiring can be exercised against a fresh
// viper in tests. initConfig cannot be: it ends by pinning the resolved level
// with viper.Set, which outranks the environment, so a second call in the same
// process can no longer observe a new environment.
func applyEnvWiring(v *viper.Viper) {
	v.SetEnvPrefix("AETHER")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()
}

// slogLevelFor maps a resolved --log-level onto a slog threshold. Anything
// unrecognised is treated as info, which normalizeLogLevel has already warned
// about by the time this is reached.
func slogLevelFor(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// initLogging gives --log-level an actual consumer.
//
// Until this existed the flag and the environment variable were inert: parsed,
// validated, written back to viper, and then read by nothing. A setting an
// operator believed they had applied produced no observable behaviour change at
// all, which is why the precedence chain could not be verified and G53R-25 had
// to be recorded as blocked.
//
// The handler is installed before the startup record below is emitted, so the
// level being verified is the one that decides whether that record appears. That
// makes each tier of the chain directly observable: a debug record on stderr at
// debug, and silence at info, warn and error.
func initLogging(level string) {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slogLevelFor(level)})))
	slog.Debug("configuration loaded",
		"log_level", level,
		"config_file", viper.ConfigFileUsed(),
	)
}

// validLogLevels is the set accepted by --log-level. The flag is
// currently reserved (no leveled logger consumes it yet) but invalid
// values are rejected so the contract is already strict.
var validLogLevels = map[string]bool{"debug": true, "info": true, "warn": true, "error": true}

// normalizeLogLevel returns the level if valid, or a warning plus
// "info" if not.
func normalizeLogLevel(level string) string {
	if validLogLevels[level] {
		return level
	}
	fmt.Fprintf(os.Stderr, "Warning: invalid --log-level %q; using %q (valid: debug, info, warn, error)\n", level, "info")
	return "info"
}
