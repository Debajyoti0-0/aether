package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type CommandInfo struct {
	Path            string   `json:"path"`
	Parent          string   `json:"parent"`
	Description     string   `json:"description"`
	Aliases         []string `json:"aliases,omitempty"`
	LocalFlags      []FlagInfo `json:"local_flags,omitempty"`
	PersistentFlags []FlagInfo `json:"persistent_flags,omitempty"`
	RequiredFlags   []string `json:"required_flags,omitempty"`
	Subcommands     []CommandInfo `json:"subcommands,omitempty"`
	IsGroup         bool     `json:"is_group"`
	IsRunnable      bool     `json:"is_runnable"`
}

type FlagInfo struct {
	Name         string `json:"name"`
	Shorthand    string `json:"shorthand,omitempty"`
	Usage        string `json:"usage"`
	DefaultValue string `json:"default_value,omitempty"`
	Required     bool   `json:"required"`
}

func main() {
	rootCmd := ""
	binary := "artifacts/stage54/aether.exe"

	// Get all commands recursively
	commands := walkCommands(binary, rootCmd, "")

	output := map[string]interface{}{
		"generated_at": "2026-09-26",
		"binary":       binary,
		"commands":     commands,
	}

	data, _ := json.MarshalIndent(output, "", "  ")
	os.WriteFile("artifacts/stage54/command-inventory.json", data, 0644)
	fmt.Printf("Generated inventory with %d top-level commands\n", len(commands))
}

func walkCommands(binary, cmdPath, parent string) []CommandInfo {
	// Get help for this command
	var args []string
	if cmdPath != "" {
		args = append(strings.Fields(cmdPath), "--help")
	} else {
		args = []string{"--help"}
	}
	helpOut, err := exec.Command(binary, args...).CombinedOutput()
	if err != nil {
		// Try without --help for commands that might not support it
		if cmdPath != "" {
			helpOut, _ = exec.Command(binary, strings.Fields(cmdPath)...).CombinedOutput()
		} else {
			helpOut, _ = exec.Command(binary).CombinedOutput()
		}
	}
	helpText := string(helpOut)

	cmd := parseCommandHelp(cmdPath, parent, helpText)

	// If it has subcommands, recurse
	if len(cmd.Subcommands) > 0 {
		var subCmds []CommandInfo
		for _, sub := range cmd.Subcommands {
			subPath := cmdPath
			if subPath != "" {
				subPath += " " + sub.Path
			} else {
				subPath = sub.Path
			}
			subCmds = append(subCmds, walkCommands(binary, subPath, cmd.Path)...)
		}
		cmd.Subcommands = subCmds
	}

	return []CommandInfo{cmd}
}

func parseCommandHelp(cmdPath, parent, helpText string) CommandInfo {
	lines := strings.Split(helpText, "\n")

	path := cmdPath
	if strings.HasPrefix(path, "aether ") {
		path = strings.TrimPrefix(path, "aether ")
	}
	cmd := CommandInfo{
		Path:     strings.TrimSpace(path),
		Parent:   parent,
		IsGroup:  false,
		IsRunnable: true,
	}

	inCommands := false
	inFlags := false
	currentFlagSection := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)

if strings.HasPrefix(line, "Available Commands:") {
		inCommands = true
		inFlags = false
		continue
	}
	if strings.HasPrefix(line, "Flags:") {
		inCommands = false
		inFlags = true
		currentFlagSection = "local"
		continue
	}
	if strings.HasPrefix(line, "Global Flags:") {
		inCommands = false
		inFlags = true
		currentFlagSection = "persistent"
		continue
	}
		if strings.HasPrefix(line, "Use ") || strings.HasPrefix(line, "Usage:") || strings.HasPrefix(line, "Aliases:") || line == "" {
			if strings.HasPrefix(line, "Aliases:") {
				aliasPart := strings.TrimPrefix(line, "Aliases:")
				cmd.Aliases = strings.Fields(strings.TrimSpace(aliasPart))
			}
			continue
		}

		if inCommands && line != "" {
			// Parse subcommand: "  cmd    description"
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				subName := parts[0]
				desc := strings.Join(parts[1:], " ")
				cmd.Subcommands = append(cmd.Subcommands, CommandInfo{
					Path:        subName,
					Parent:      cmd.Path,
					Description: desc,
					IsGroup:     true, // assume group until proven otherwise
				})
			}
		}

		if inFlags && line != "" && !strings.HasPrefix(line, "-h, --help") {
			// Parse flag: "  --flag value   description (default \"val\")"
			flag := parseFlag(line)
			if flag.Name != "" {
				if currentFlagSection == "persistent" {
					cmd.PersistentFlags = append(cmd.PersistentFlags, flag)
				} else {
					cmd.LocalFlags = append(cmd.LocalFlags, flag)
				}
			}
		}

		// Try to extract description from first meaningful line
		if cmd.Description == "" && line != "" && !strings.HasPrefix(line, "Usage:") && !strings.HasPrefix(line, "Available") && !strings.HasPrefix(line, "Flags:") && !strings.HasPrefix(line, "Global") && !strings.HasPrefix(line, "Use ") {
			if !strings.Contains(line, "  ") || strings.HasPrefix(line, "aether") {
				cmd.Description = line
			}
		}
	}

	// A command is a group if it has subcommands but no Run/RunE of its own
	// We'll mark it as group if it has subcommands
	if len(cmd.Subcommands) > 0 {
		cmd.IsGroup = true
	}

	return cmd
}

func parseFlag(line string) FlagInfo {
	// Format: "  --flag value   description (default \"val\")"
	// or: "  -f, --flag value   description"
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "-h, --help") {
		return FlagInfo{}
	}

	// Split by double space or tab
	parts := strings.Split(line, "  ")
	if len(parts) < 2 {
		// Try single space split for flags with no description
		parts = strings.Fields(line)
		if len(parts) < 1 {
			return FlagInfo{}
		}
	}

	flagPart := strings.TrimSpace(parts[0])
	desc := ""
	if len(parts) > 1 {
		desc = strings.TrimSpace(parts[1])
	}

	// Extract default value
	defaultVal := ""
	if idx := strings.Index(desc, "(default "); idx >= 0 {
		end := strings.Index(desc[idx:], ")")
		if end >= 0 {
			defaultVal = desc[idx+9 : idx+end]
			desc = strings.TrimSpace(desc[:idx])
		}
	}

	// Parse flag name and shorthand
	flags := strings.Split(flagPart, ",")
	var name, shorthand string
	for _, f := range flags {
		f = strings.TrimSpace(f)
		if strings.HasPrefix(f, "--") {
			name = strings.TrimPrefix(f, "--")
			// Remove value placeholder
			if idx := strings.Index(name, " "); idx >= 0 {
				name = name[:idx]
			}
		} else if strings.HasPrefix(f, "-") && !strings.HasPrefix(f, "--") {
			shorthand = strings.TrimPrefix(f, "-")
		}
	}

	required := strings.Contains(desc, "required") || strings.Contains(desc, "Required")

	return FlagInfo{
		Name:         name,
		Shorthand:    shorthand,
		Usage:        desc,
		DefaultValue: defaultVal,
		Required:     required,
	}
}