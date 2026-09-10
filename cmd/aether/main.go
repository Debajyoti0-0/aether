package main

import (
	"fmt"
	"os"

	"github.com/Debajyoti0-0/aether/internal/cli"
	"github.com/Debajyoti0-0/aether/internal/version"
)

func main() {
	// Version comes from internal/version (single source of truth),
	// injected at build time via -ldflags. See VERSION at repo root.
	cli.SetVersion(version.Version)
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
