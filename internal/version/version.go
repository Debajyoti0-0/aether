// Package version is the single source of truth for the Aether version.
//
// The default values are overridden at build time via ldflags:
//
//	-X github.com/Debajyoti0-0/aether/internal/version.Version=$(VERSION)
//	-X github.com/Debajyoti0-0/aether/internal/version.Commit=$(COMMIT)
//
// Every version display (CLI --version, SARIF driver version, man pages)
// must read from this package. Do not hardcode version strings elsewhere.
package version

// Version is the semantic version of the current build.
var Version = "dev"

// Commit is the git commit the binary was built from (may be empty).
var Commit = ""
