package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Debajyoti0-0/aether/internal/cli"
	_ "github.com/Debajyoti0-0/aether/internal/cli/ad"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// This generator used to shell out to artifacts/stage54/aether.exe and scrape
// each command's --help text. That had two failure modes, both of which
// produced a plausible-looking file rather than an error:
//
//   - the binary is gitignored (*.exe) and goes stale, so the inventory could
//     describe a build other than the one in the tree;
//   - when exec failed the error was discarded and the empty output was parsed,
//     which yielded a 1-command inventory that looked like a real result.
//
// It now walks the live command tree in process, the same source of truth the
// shipped binary is built from, so the inventory cannot drift from the code.

type CommandInfo struct {
	Path            string        `json:"path"`
	Parent          string        `json:"parent"`
	Description     string        `json:"description"`
	Aliases         []string      `json:"aliases,omitempty"`
	LocalFlags      []FlagInfo    `json:"local_flags,omitempty"`
	PersistentFlags []FlagInfo    `json:"persistent_flags,omitempty"`
	RequiredFlags   []string      `json:"required_flags,omitempty"`
	Subcommands     []CommandInfo `json:"subcommands,omitempty"`
	IsGroup         bool          `json:"is_group"`
	IsRunnable      bool          `json:"is_runnable"`
}

type FlagInfo struct {
	Name         string `json:"name"`
	Shorthand    string `json:"shorthand,omitempty"`
	Usage        string `json:"usage"`
	DefaultValue string `json:"default_value,omitempty"`
	Required     bool   `json:"required"`
}

const rootName = "aether"

func main() {
	rootCmd := cli.NewRootCommand()

	// cobra attaches these during Execute(); trigger them so the inventory
	// matches the shipped binary's help output.
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultCompletionCmd()

	commands := walkCommands(rootCmd, rootName)

	output := map[string]interface{}{
		"generated_at": "2026-09-26",
		"source":       "in-process cobra tree via cli.NewRootCommand()",
		"root":         rootName,
		"commands":     commands,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal inventory: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile("artifacts/stage54/command-inventory.json", data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write inventory: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Generated inventory with %d top-level commands\n", len(commands))
}

// walkCommands returns the direct children of cmd, each carrying its own
// recursive subtree, so the emitted document mirrors the command tree.
func walkCommands(cmd *cobra.Command, prefix string) []CommandInfo {
	var out []CommandInfo
	for _, sub := range cmd.Commands() {
		if sub.Name() == "" {
			continue
		}
		out = append(out, describeCommand(sub, prefix))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func describeCommand(cmd *cobra.Command, parentPath string) CommandInfo {
	path := parentPath + " " + cmd.Name()

	info := CommandInfo{
		Path:        path,
		Parent:      parentPath,
		Description: firstNonEmpty(cmd.Short, cmd.Long),
		Aliases:     cmd.Aliases,
		IsGroup:     !cmd.Runnable(),
		IsRunnable:  cmd.Runnable(),
	}

	info.LocalFlags = collectFlags(cmd.LocalFlags(), cmd)
	info.PersistentFlags = collectFlags(cmd.PersistentFlags(), cmd)
	info.RequiredFlags = requiredFlagNames(cmd)

	children := walkCommands(cmd, path)
	if len(children) > 0 {
		info.Subcommands = children
	}
	return info
}

func collectFlags(fs *pflag.FlagSet, cmd *cobra.Command) []FlagInfo {
	if fs == nil {
		return nil
	}
	var out []FlagInfo
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" {
			return
		}
		out = append(out, FlagInfo{
			Name:         f.Name,
			Shorthand:    f.Shorthand,
			Usage:        f.Usage,
			DefaultValue: f.DefValue,
			Required:     isRequired(f),
		})
	})
	return out
}

// requiredFlagNames reports the command's required flags across its whole
// inheritance chain, which is what the binary actually enforces.
func requiredFlagNames(cmd *cobra.Command) []string {
	var names []string
	seen := map[string]bool{}
	add := func(f *pflag.Flag) {
		if f == nil || !isRequired(f) || seen[f.Name] {
			return
		}
		seen[f.Name] = true
		names = append(names, f.Name)
	}
	for c := cmd; c != nil; c = c.Parent() {
		add(c.Flags().Lookup("config"))
		add(c.Flags().Lookup("log-level"))
	}
	cmd.NonInheritedFlags().VisitAll(func(f *pflag.Flag) { add(f) })
	cmd.InheritedFlags().VisitAll(func(f *pflag.Flag) { add(f) })
	sort.Strings(names)
	return names
}

func isRequired(f *pflag.Flag) bool {
	if f.Annotations == nil {
		return false
	}
	values := f.Annotations[cobra.BashCompOneRequiredFlag]
	return len(values) > 0 && values[0] == "true"
}

func firstNonEmpty(candidates ...string) string {
	for _, c := range candidates {
		if t := strings.TrimSpace(c); t != "" {
			return t
		}
	}
	return ""
}
