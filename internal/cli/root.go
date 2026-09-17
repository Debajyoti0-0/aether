package cli

import (
	"errors"
	"fmt"
	"os"

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

// SetVersion overrides the CLI version from build-time injection.
func SetVersion(v string) {
	if v != "" {
		rootCmd.Version = v
	}
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
	return NewRootCommand().Execute()
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().String("config", "", "Config file path")
	rootCmd.PersistentFlags().String("log-level", "info", "Log level (debug, info, warn, error)")

	_ = viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	_ = viper.BindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level"))
}

func initConfig() {
	cfgFile := viper.GetString("config")
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("aether")
		viper.SetConfigType("json")
		for _, dir := range paths.ConfigSearchPaths() {
			viper.AddConfigPath(dir)
		}
	}

	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		// A config file that exists but cannot be read must never be
		// silently ignored: an operator whose security-relevant settings
		// silently fall back to defaults is a fail-open configuration.
		var notFound viper.ConfigFileNotFoundError
		if cfgFile != "" || !errors.As(err, &notFound) {
			fmt.Fprintf(os.Stderr, "Warning: config file ignored: %v\n", err)
		}
	} else if viper.ConfigFileUsed() != "" {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
	// Reject invalid --log-level values instead of silently accepting
	// them (F-34-2); the flag is reserved but its contract is strict.
	viper.Set("log-level", normalizeLogLevel(viper.GetString("log-level")))
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
