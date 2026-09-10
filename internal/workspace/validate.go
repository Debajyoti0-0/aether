package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Centralized path-validation policy for every user-controlled path
// component that reaches the filesystem (workspace names, record keys,
// artifact names). Security invariant (Stage 1 / forensic F1):
//
//	No unvalidated user-controlled path may reach a filesystem mutation.
//
// All construction of user-influenced paths inside this package must go
// through SafeJoin / validatePath, never through raw filepath.Join.

var errEmptyName = errors.New("name is required")

// ValidateName validates a workspace name. It rejects empty names,
// relative-navigation components, absolute paths, any path separator
// (including Windows-specific ones on every platform), NUL/control
// bytes, Windows drive letters and UNC prefixes, Windows reserved
// device names, and surrounding whitespace.
func ValidateName(name string) error {
	return validateComponent(name, "workspace name")
}

// ValidateRecordKey validates a record key inside a bucket.
func ValidateRecordKey(key string) error {
	return validateComponent(key, "record key")
}

// ValidateArtifactName validates an artifact file name.
func ValidateArtifactName(name string) error {
	return validateComponent(name, "artifact name")
}

func validateComponent(name, kind string) error {
	if name == "" {
		return fmt.Errorf("%s: %w", kind, errEmptyName)
	}
	if name != strings.TrimSpace(name) {
		return fmt.Errorf("%s %q: leading or trailing whitespace is not allowed", kind, name)
	}
	if name == "." || name == ".." {
		return fmt.Errorf("%s %q: relative-navigation component", kind, name)
	}
	// Reject separators on every platform regardless of the host OS so a
	// Windows-style traversal string cannot pass on Linux and vice versa.
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("%s %q: path separators are not allowed", kind, name)
	}
	// NUL and other control characters.
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("%s %q: control characters are not allowed", kind, name)
		}
	}
	// Absolute paths, Windows drive letters, UNC prefixes.
	if filepath.IsAbs(name) {
		return fmt.Errorf("%s %q: absolute paths are not allowed", kind, name)
	}
	if len(name) >= 2 && name[1] == ':' {
		return fmt.Errorf("%s %q: drive-letter paths are not allowed", kind, name)
	}
	if strings.HasPrefix(name, "\\\\") {
		return fmt.Errorf("%s %q: UNC paths are not allowed", kind, name)
	}
	if isWindowsReserved(name) {
		return fmt.Errorf("%s %q: reserved Windows device name", kind, name)
	}
	// Trailing dots/spaces are silently stripped by Windows file APIs and
	// can be used to smuggle a different effective name.
	if strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return fmt.Errorf("%s %q: trailing dot or space is not allowed", kind, name)
	}
	return nil
}

func isWindowsReserved(name string) bool {
	base := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	switch base {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return true
	}
	return false
}

// SafeJoin joins root with a validated single component and verifies the
// resolved path stays within root after symlink resolution.
//
// Symlink policy: the deepest already-existing ancestor of the joined
// path is resolved with filepath.EvalSymlinks; if the resolution escapes
// root (a symlink inside the workspace pointing outside), the join is
// rejected. Paths that do not exist yet are validated against the
// resolved real root.
func SafeJoin(root, name string) (string, error) {
	if err := validateComponent(name, "path component"); err != nil {
		return "", err
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}
	absRoot, err = resolveExisting(absRoot)
	if err != nil {
		return "", err
	}

	joined := filepath.Join(absRoot, name)

	// Resolve symlinks of the deepest existing ancestor.
	resolved, err := resolveExisting(joined)
	if err != nil {
		return "", err
	}
	if !containedWithin(resolved, absRoot) {
		return "", fmt.Errorf("path %q resolves outside the workspace root (symlink escape?)", name)
	}
	return joined, nil
}

// resolveExisting resolves symlinks on the deepest existing ancestor of
// p and re-appends the remaining segments.
func resolveExisting(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	remaining := []string{}
	cur := abs
	for {
		if _, err := os.Lstat(cur); err == nil {
			break
		}
		dir, base := filepath.Split(cur)
		if dir == cur { // reached filesystem root
			cur = strings.TrimRight(cur, string(os.PathSeparator))
			break
		}
		remaining = append([]string{base}, remaining...)
		cur = filepath.Clean(dir)
	}
	resolved, err := filepath.EvalSymlinks(cur)
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{resolved}, remaining...)...), nil
}

// containedWithin reports whether path is root itself or located under
// root (lexicographic containment on cleaned absolute paths).
func containedWithin(path, root string) bool {
	if path == root {
		return true
	}
	sep := string(os.PathSeparator)
	return strings.HasPrefix(path, strings.TrimRight(root, sep)+sep)
}
