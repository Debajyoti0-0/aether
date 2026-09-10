package cli

import (
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
    if cfgFile := viper.GetString("config"); cfgFile != "" {
        viper.SetConfigFile(cfgFile)
    } else {
        viper.SetConfigName("aether")
        viper.SetConfigType("json")
        for _, dir := range paths.ConfigSearchPaths() {
            viper.AddConfigPath(dir)
        }
    }

    viper.AutomaticEnv()
    if err := viper.ReadInConfig(); err == nil {
        fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
    }
}
