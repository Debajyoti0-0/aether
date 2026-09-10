package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Resolution order for every directory:
//  1. AETHER_* environment override (portable/USB installs, CI)
//  2. OS-native config/data location
//  3. Legacy ~/.config/aether (pre-2.x workspaces are migrated)

// AppName is the directory name used under system config/data roots.
const AppName = "aether"

// Home returns the user home directory ("" if unavailable).
func Home() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// ConfigDir returns the OS-appropriate configuration directory.
//
//	Windows: %AppData%\aether         (C:\Users\<u>\AppData\Roaming\aether)
//	macOS:   ~/Library/Application Support/aether
//	Linux:   $XDG_CONFIG_HOME/aether  (~/.config/aether)
func ConfigDir() string {
	if override := strings.TrimSpace(os.Getenv("AETHER_CONFIG_DIR")); override != "" {
		return override
	}
	if base, err := os.UserConfigDir(); err == nil && base != "" {
		return filepath.Join(base, AppName)
	}
	// Fallback when UserConfigDir fails (rare).
	if home := Home(); home != "" {
		return filepath.Join(home, ".config", AppName)
	}
	return "." + AppName
}

// LegacyConfigDir is the pre-2.x location (~/.config/aether) used for
// one-time migration on all platforms.
func LegacyConfigDir() string {
	if override := strings.TrimSpace(os.Getenv("AETHER_CONFIG_DIR")); override != "" {
		return ""
	}
	if home := Home(); home != "" {
		return filepath.Join(home, ".config", AppName)
	}
	return ""
}

// DataDir returns the OS-appropriate application data directory.
//
//	Windows: %AppData%\aether (same as config; Windows has one roaming root)
//	macOS:   ~/Library/Application Support/aether
//	Linux:   $XDG_DATA_HOME/aether (~/.local/share/aether)
func DataDir() string {
	if override := strings.TrimSpace(os.Getenv("AETHER_DATA_DIR")); override != "" {
		return override
	}
	switch runtime.GOOS {
	case "linux", "freebsd", "openbsd", "netbsd":
		if base := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); base != "" {
			return filepath.Join(base, AppName)
		}
		if home := Home(); home != "" {
			return filepath.Join(home, ".local", "share", AppName)
		}
	default:
		// Windows and macOS: data lives beside config.
		return ConfigDir()
	}
	return ConfigDir()
}

// CacheDir returns the OS-appropriate cache directory.
//
//	Windows: %LocalAppData%\aether\cache
//	macOS:   ~/Library/Caches/aether
//	Linux:   $XDG_CACHE_HOME/aether
func CacheDir() string {
	if override := strings.TrimSpace(os.Getenv("AETHER_CACHE_DIR")); override != "" {
		return override
	}
	switch runtime.GOOS {
	case "windows":
		if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
			return filepath.Join(local, AppName, "cache")
		}
	case "darwin":
		if home := Home(); home != "" {
			return filepath.Join(home, "Library", "Caches", AppName)
		}
	default:
		if base := strings.TrimSpace(os.Getenv("XDG_CACHE_HOME")); base != "" {
			return filepath.Join(base, AppName)
		}
		if home := Home(); home != "" {
			return filepath.Join(home, ".cache", AppName)
		}
	}
	return filepath.Join(os.TempDir(), AppName)
}

// WorkspacesDir returns the workspace root, migrating the legacy
// ~/.config/aether/workspaces layout transparently.
func WorkspacesDir() string {
	current := filepath.Join(ConfigDir(), "workspaces")

	if legacy := LegacyConfigDir(); legacy != "" {
		legacyWS := filepath.Join(legacy, "workspaces")
		if _, err := os.Stat(legacyWS); err == nil {
			if _, err := os.Stat(current); err != nil {
				// One-time migration: move legacy workspaces into place.
				if err := os.MkdirAll(filepath.Dir(current), 0o700); err == nil {
					_ = os.Rename(legacyWS, current)
				}
			}
		}
	}
	return current
}

// ConfigFileLocations returns config file candidates in priority order.
// The system location (/etc/aether) is only included on unix-likes.
func ConfigFileLocations() []string {
	var out []string
	out = append(out, "aether.json") // cwd, for portable use
	out = append(out, filepath.Join(ConfigDir(), "config.json"))

	switch runtime.GOOS {
	case "linux", "freebsd", "openbsd", "netbsd":
		out = append(out, "/etc/aether/config.json")
	case "darwin":
		out = append(out, "/etc/aether/config.json")
	case "windows":
		// No system-wide convention on Windows; ProgramData is the closest.
		if pd := strings.TrimSpace(os.Getenv("ProgramData")); pd != "" {
			out = append(out, filepath.Join(pd, AppName, "config.json"))
		}
	}
	return out
}

// ConfigSearchPaths mirrors ConfigFileLocations as directories (viper).
func ConfigSearchPaths() []string {
	var out []string
	out = append(out, ".")
	out = append(out, ConfigDir())
	switch runtime.GOOS {
	case "linux", "freebsd", "openbsd", "netbsd", "darwin":
		out = append(out, "/etc/aether")
	case "windows":
		if pd := strings.TrimSpace(os.Getenv("ProgramData")); pd != "" {
			out = append(out, filepath.Join(pd, AppName))
		}
	}
	return out
}

// LogFile returns the default structured log location.
func LogFile() string {
	return filepath.Join(CacheDir(), "aether.log")
}

// PlatformInfo returns a human-readable platform line for diagnostics.
func PlatformInfo() string {
	return runtime.GOOS + "/" + runtime.GOARCH + " (" + runtime.Version() + ")"
}
