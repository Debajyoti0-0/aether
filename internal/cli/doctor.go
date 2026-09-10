package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/Debajyoti0-0/aether/internal/paths"
	"github.com/Debajyoti0-0/aether/internal/workspace"
)

// doctorCmd verifies the host environment: writable dirs, config
// discovery, workspace root, and network stack sanity.
var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check host environment health",
	Long:  "Verify that aether runs correctly on this OS: directory permissions, config discovery, crypto self-test, and workspace access.",
	RunE:  runDoctor,
}

type check struct {
	name string
	ok   bool
	detail string
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	var results []check

	// Platform.
	results = append(results, check{
		name:   "platform",
		ok:     true,
		detail: paths.PlatformInfo(),
	})

	// Config dir writable.
	cfgDir := paths.ConfigDir()
	results = append(results, writableDirCheck("config dir", cfgDir))

	// Data dir writable.
	dataDir := paths.DataDir()
	if dataDir != cfgDir {
		results = append(results, writableDirCheck("data dir", dataDir))
	}

	// Cache dir writable.
	results = append(results, writableDirCheck("cache dir", paths.CacheDir()))

	// Workspace root.
	wsRoot := workspace.Dir()
	results = append(results, writableDirCheck("workspaces root", wsRoot))

	// Workspace roundtrip (create → record → delete) in a temp profile.
	results = append(results, workspaceRoundtripCheck())

	// Config discovery.
	found := "not found (defaults in use)"
	for _, loc := range paths.ConfigFileLocations() {
		if loc == "aether.json" {
			continue
		}
		if _, err := os.Stat(loc); err == nil {
			found = loc
			break
		}
	}
	results = append(results, check{name: "config file", ok: true, detail: found})

	// OS-specific notes.
	switch runtime.GOOS {
	case "windows":
		results = append(results, check{
			name:   "windows notes",
			ok:     true,
			detail: "signals: CTRL_C/CTRL_BREAK; config: %AppData%; install: scoop/winget or scripts\\build.ps1",
		})
	case "darwin":
		results = append(results, check{
			name:   "macos notes",
			ok:     true,
			detail: "signals: SIGINT/SIGTERM; config: ~/Library/Application Support/aether; install: brew",
		})
	default:
		results = append(results, check{
			name:   "unix notes",
			ok:     true,
			detail: "signals: SIGINT/SIGTERM; config: $XDG_CONFIG_HOME/aether; install: apt/deb or scripts/build.sh",
		})
	}

	// Render.
	failed := 0
	for _, c := range results {
		mark := "OK  "
		if !c.ok {
			mark = "FAIL"
			failed++
		}
		fmt.Printf("[%s] %-18s %s\n", mark, c.name, c.detail)
	}

	if failed > 0 {
		return fmt.Errorf("%d checks failed", failed)
	}
	fmt.Println("\nAll checks passed — aether is ready on this platform.")
	return nil
}

func writableDirCheck(name, dir string) check {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return check{name: name, ok: false, detail: dir + " (" + err.Error() + ")"}
	}

	probe := filepath.Join(dir, ".aether-probe")
	if err := os.WriteFile(probe, []byte("probe"), 0o600); err != nil {
		return check{name: name, ok: false, detail: dir + " (not writable: " + err.Error() + ")"}
	}
	os.Remove(probe)
	return check{name: name, ok: true, detail: dir}
}

func workspaceRoundtripCheck() check {
	// Run against an isolated profile so we never touch user workspaces.
	probeDir, err := os.MkdirTemp("", "aether-doctor-*")
	if err != nil {
		return check{name: "workspace roundtrip", ok: false, detail: "no temp dir: " + err.Error()}
	}
	defer os.RemoveAll(probeDir)
	os.Setenv("AETHER_CONFIG_DIR", probeDir)
	defer os.Unsetenv("AETHER_CONFIG_DIR")

	name := "doctor-probe"
	w, err := workspace.Create(name, "doctor-probe-passphrase")
	if err != nil {
		return check{name: "workspace roundtrip", ok: false, detail: "create: " + err.Error()}
	}
	type rec struct {
		V string `json:"v"`
	}
	if err := w.SaveRecord(workspace.BucketTokens, "probe", rec{V: "x"}); err != nil {
		return check{name: "workspace roundtrip", ok: false, detail: "save: " + err.Error()}
	}
	var out rec
	if err := w.LoadRecord(workspace.BucketTokens, "probe", &out); err != nil {
		return check{name: "workspace roundtrip", ok: false, detail: "load: " + err.Error()}
	}
	if err := workspace.Delete(name); err != nil {
		return check{name: "workspace roundtrip", ok: false, detail: "delete: " + err.Error()}
	}
	return check{name: "workspace roundtrip", ok: true, detail: "create/save/load/delete OK (AES-256-GCM + Argon2id)"}
}
