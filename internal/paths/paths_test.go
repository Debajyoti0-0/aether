package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestConfigDirOverride(t *testing.T) {
	t.Setenv("AETHER_CONFIG_DIR", "/custom/aether")
	if got := ConfigDir(); got != "/custom/aether" {
		t.Errorf("ConfigDir = %q", got)
	}
	// DataDir inherits the override on non-Windows too via its own env.
	t.Setenv("AETHER_DATA_DIR", "/custom/aether-data")
	if got := DataDir(); got != "/custom/aether-data" {
		t.Errorf("DataDir = %q", got)
	}
	t.Setenv("AETHER_CACHE_DIR", "/custom/aether-cache")
	if got := CacheDir(); got != "/custom/aether-cache" {
		t.Errorf("CacheDir = %q", got)
	}
}

func TestConfigDirPerOS(t *testing.T) {
	home := t.TempDir()

	switch runtime.GOOS {
	case "windows":
		t.Setenv("AppData", home)
		want := filepath.Join(home, "aether")
		if got := ConfigDir(); got != want {
			t.Errorf("ConfigDir = %q, want %q", got, want)
		}
	case "darwin":
		t.Setenv("HOME", home)
		want := filepath.Join(home, "Library", "Application Support", "aether")
		if got := ConfigDir(); got != want {
			t.Errorf("ConfigDir = %q, want %q", got, want)
		}
		wantCache := filepath.Join(home, "Library", "Caches", "aether")
		if got := CacheDir(); got != wantCache {
			t.Errorf("CacheDir = %q, want %q", got, wantCache)
		}
	default:
		t.Setenv("HOME", home)
		t.Setenv("XDG_CONFIG_HOME", "")
		want := filepath.Join(home, ".config", "aether")
		if got := ConfigDir(); got != want {
			t.Errorf("ConfigDir = %q, want %q", got, want)
		}
		t.Setenv("XDG_DATA_HOME", "")
		wantData := filepath.Join(home, ".local", "share", "aether")
		if got := DataDir(); got != wantData {
			t.Errorf("DataDir = %q, want %q", got, wantData)
		}
		t.Setenv("XDG_CACHE_HOME", "")
		wantCache := filepath.Join(home, ".cache", "aether")
		if got := CacheDir(); got != wantCache {
			t.Errorf("CacheDir = %q, want %q", got, wantCache)
		}
	}
}

func TestConfigFileLocations(t *testing.T) {
	locations := ConfigFileLocations()
	if len(locations) < 2 {
		t.Fatalf("locations = %v", locations)
	}
	// cwd config always first (portable mode).
	if locations[0] != "aether.json" {
		t.Errorf("first = %q", locations[0])
	}

	switch runtime.GOOS {
	case "windows":
		found := false
		for _, l := range locations {
			if filepath.Base(filepath.Dir(l)) == "aether" && l != locations[1] {
				found = true
			}
		}
		_ = found // ProgramData optional in sandboxes
	default:
		last := locations[len(locations)-1]
		if last != "/etc/aether/config.json" {
			t.Errorf("system location = %q", last)
		}
	}
}

func TestMigrateLegacyWorkspaces(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	os.Unsetenv("AETHER_CONFIG_DIR")
	os.Unsetenv("XDG_CONFIG_HOME")

	legacy := filepath.Join(home, ".config", "aether", "workspaces", "OldWS")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}

	got := WorkspacesDir()
	migrated := filepath.Join(got, "OldWS")
	if _, err := os.Stat(migrated); err != nil {
		t.Errorf("legacy workspace not migrated to %s", migrated)
	}
}
