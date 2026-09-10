// Command genman generates the aether man page tree (section 1) into
// a target directory, for distribution with deb/rpm/homebrew packages.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra/doc"

	"github.com/Debajyoti0-0/aether/internal/cli"
)

func main() {
	outDir := "docs/man"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}

	// Reuse the real command tree so the man page matches the binary.
	root := cli.NewRootCommand()
	header := &doc.GenManHeader{
		Title:   "AETHER",
		Section: "1",
		Source:  "Aether " + root.Version,
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "genman: %v\n", err)
		os.Exit(1)
	}
	if err := doc.GenManTree(root, header, outDir); err != nil {
		fmt.Fprintf(os.Stderr, "genman: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Man pages written to %s\n", outDir)
}
