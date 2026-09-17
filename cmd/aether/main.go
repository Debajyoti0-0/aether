package main

import (
	"errors"
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
		// Deterministic exit-code contract for
		// 'export verify-evidence' (documented on the command):
		// 2 - revoked, 3 - status unknown; every other error exits 1.
		switch {
		case errors.Is(err, cli.ErrEvidenceRevoked):
			os.Exit(2)
		case errors.Is(err, cli.ErrEvidenceUnknown):
			os.Exit(3)
		default:
			os.Exit(1)
		}
	}
}
