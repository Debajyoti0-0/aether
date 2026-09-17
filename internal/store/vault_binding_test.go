package store

import (
	"errors"
	bolt "go.etcd.io/bbolt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Regression tests for the Stage 34 forensic findings:
//   - F-34-4: a vault must be bound to the workspace directory it was
//     created in; cross-workspace substitution (even with the same
//     passphrase at the record-sealing layer) must be rejected.
//   - F-34-3: a truncated/corrupt vault must produce a typed error, and
//     a raw storage panic must never escape OpenVault.

// newBoundVault creates a vault inside a workspace directory named name
// under a fresh temp root, and returns its path.
func newBoundVault(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "vault.db")
	v, err := OpenVault(path)
	if err != nil {
		t.Fatalf("create vault %s: %v", name, err)
	}
	if err := v.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestVaultBindingAcceptsSameWorkspace(t *testing.T) {
	root := t.TempDir()
	path := newBoundVault(t, root, "wsA")
	v, err := OpenVault(path)
	if err != nil {
		t.Fatalf("reopen same workspace: %v", err)
	}
	_ = v.Close()
}

func TestVaultBindingRejectsSubstitution(t *testing.T) {
	root := t.TempDir()
	pathA := newBoundVault(t, root, "wsA")
	pathB := newBoundVault(t, root, "wsB")

	// Attack: substitute B's vault into A's workspace directory.
	raw, err := os.ReadFile(pathB)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pathA, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = OpenVault(pathA)
	if err == nil {
		t.Fatal("substituted vault accepted: identity binding not enforced")
	}
	if !errors.Is(err, ErrVaultIdentityMismatch) {
		t.Fatalf("expected ErrVaultIdentityMismatch, got: %v", err)
	}
	if !strings.Contains(err.Error(), "wsB") || !strings.Contains(err.Error(), "wsA") {
		t.Fatalf("error should name both identities, got: %v", err)
	}
}

func TestVaultBindingRejectsVaultOnlySwapBothDirections(t *testing.T) {
	root := t.TempDir()
	paths := []string{
		newBoundVault(t, root, "wsX"),
		newBoundVault(t, root, "wsY"),
	}
	rawX, _ := os.ReadFile(paths[0])
	rawY, _ := os.ReadFile(paths[1])
	_ = os.WriteFile(paths[0], rawY, 0o600)
	_ = os.WriteFile(paths[1], rawX, 0o600)
	for i, p := range paths {
		if _, err := OpenVault(p); !errors.Is(err, ErrVaultIdentityMismatch) {
			t.Fatalf("direction %d: expected ErrVaultIdentityMismatch, got %v", i, err)
		}
	}
}

func TestVaultTruncatedTypedErrorNoPanic(t *testing.T) {
	root := t.TempDir()
	path := newBoundVault(t, root, "wsT")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Corrupt a mid-file page header (zero its identity fields) — the
	// exact assertion that fired in the Stage 34 CLI reproduction
	// ("Page expected to be: N, but self identifies as 0"). Page size
	// is discovered from the file's declared 32KiB default: 4096 on
	// this platform, so page 3 starts at offset 12288.
	ps := 4096
	for i := 2 * ps; i < 3*ps; i += ps {
		hdr := raw[i : i+8]
		if hdr[0] != 0 { // skip zero/free pages
			for j := 0; j < 8; j++ {
				raw[i+j] = 0
			}
			break
		}
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	v, err := OpenVault(path)
	if err == nil {
		_ = v.Close()
		t.Fatal("truncated vault opened without error")
	}
	if !errors.Is(err, ErrVaultCorrupt) {
		t.Fatalf("expected ErrVaultCorrupt, got: %v", err)
	}
}

func TestVaultBindingMigratesPreBindingVault(t *testing.T) {
	root := t.TempDir()
	path := newBoundVault(t, root, "wsM")
	// Simulate a pre-binding vault by removing the meta key.
	v, err := OpenVault(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte("meta")).Delete([]byte(metaWorkspaceName))
	}); err != nil {
		t.Fatal(err)
	}
	_ = v.Close()
	// Migration: first open adopts the current directory name.
	v2, err := OpenVault(path)
	if err != nil {
		t.Fatalf("pre-binding vault must migrate, got: %v", err)
	}
	_ = v2.Close()
	// After migration, substitution is enforced again.
	other := newBoundVault(t, root, "wsN")
	raw, _ := os.ReadFile(other)
	_ = os.WriteFile(path, raw, 0o600)
	if _, err := OpenVault(path); !errors.Is(err, ErrVaultIdentityMismatch) {
		t.Fatalf("post-migration substitution not rejected: %v", err)
	}
}
