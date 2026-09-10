// Command rmdir removes directories given as arguments (Makefile
// `clean` helper that works identically on Windows, macOS, and Linux).
package main

import (
	"fmt"
	"os"
)

func main() {
	for _, dir := range os.Args[1:] {
		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintf(os.Stderr, "rmdir %s: %v\n", dir, err)
			os.Exit(1)
		}
	}
}
