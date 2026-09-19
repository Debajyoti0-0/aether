package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// Regression (Stage 44-R D-002): `serve cert revoke --operator X` was
// unreachable because the --operator flag was only registered on
// `issue`, leaving tsCertOperator empty on revoke. The full command
// chain must accept the flag, append the operator to revoked.txt, and
// reject path-like names.
func TestServeCertRevokeAcceptsOperatorFlag(t *testing.T) {
	dir := t.TempDir()

	root := NewRootCommand()
	root.SetArgs([]string{"serve", "cert", "revoke", "--dir", dir, "--operator", "alice"})
	if err := root.Execute(); err != nil {
		t.Fatalf("revoke with --operator failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "revoked.txt"))
	if err != nil {
		t.Fatalf("revoked.txt not written: %v", err)
	}
	if string(data) != "alice\n" {
		t.Fatalf("revoked.txt = %q, want %q", data, "alice\n")
	}

	root = NewRootCommand()
	root.SetArgs([]string{"serve", "cert", "revoke", "--dir", dir, "--operator", "../evil"})
	if err := root.Execute(); err == nil {
		t.Fatal("path-like operator name must be rejected")
	}
}
