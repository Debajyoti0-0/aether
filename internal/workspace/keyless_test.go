package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

// F2: Open with an empty passphrase must fail for a normally
// (passphrase-)protected workspace.
func TestOpenRejectsEmptyPassphrase(t *testing.T) {
	t.Setenv("AETHER_CONFIG_DIR", t.TempDir())

	if _, err := Create("Guarded", "real-pass"); err != nil {
		t.Fatal(err)
	}
	if _, err := Open("Guarded", ""); err == nil {
		t.Fatal("Open with empty passphrase on protected workspace = nil, want error")
	}
	if _, err := Open("Guarded", "   "); err == nil {
		t.Fatal("Open with whitespace passphrase = nil, want error (not a keyless bypass)")
	}
}

// F2: keyless workspaces exist only through the explicit KEYLESS marker
// and warn on every open.
func TestKeylessWorkspaceExplicitOnly(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AETHER_CONFIG_DIR", dir)

	w, err := Create("KeylessWS", "")
	if err != nil {
		t.Fatal(err)
	}
	if !w.Keyless {
		t.Fatal("Create with empty passphrase should mark workspace keyless")
	}
	if _, err := os.Stat(filepath.Join(w.Root, keylessMarker)); err != nil {
		t.Fatalf("KEYLESS marker missing: %v", err)
	}

	// Keyless open works but warns on every open.
	if _, err := Open("KeylessWS", ""); err != nil {
		t.Fatalf("keyless open: %v", err)
	}
	if _, err := Open("KeylessWS", ""); err != nil {
		t.Fatalf("second keyless open: %v", err)
	}

	// A keyless workspace cannot be opened with a non-empty passphrase.
	if _, err := Open("KeylessWS", "some-pass"); err == nil {
		t.Error("Open with non-empty passphrase on keyless workspace = nil, want error")
	}
}

// F2: wrong passphrase and tampered salt.bin must fail closed.
func TestOpenFailsClosedOnWrongPassAndSaltTamper(t *testing.T) {
	t.Setenv("AETHER_CONFIG_DIR", t.TempDir())

	if _, err := Create("TamperMe", "real-pass"); err != nil {
		t.Fatal(err)
	}

	if _, err := Open("TamperMe", "wrong-pass"); err == nil {
		t.Fatal("wrong passphrase = nil error, want rejection")
	}

	saltPath := filepath.Join(Dir(), "TamperMe", saltFileName)
	raw, err := os.ReadFile(saltPath)
	if err != nil {
		t.Fatal(err)
	}
	raw[5] ^= 0xFF // flip a salt byte
	if err := os.WriteFile(saltPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open("TamperMe", "real-pass"); err == nil {
		t.Fatal("tampered salt.bin = nil error, want rejection")
	}
}

// F2: the key is no longer derivable from the workspace name alone —
// two workspaces with the same name on different machines must have
// different salts, and a fresh workspace must not accept the empty
// passphrase.
func TestNoDeterministicKeyFromName(t *testing.T) {
	t.Setenv("AETHER_CONFIG_DIR", t.TempDir())

	w1, err := Create("SameName", "pass-a")
	if err != nil {
		t.Fatal(err)
	}
	root1 := w1.Root
	// Remove the workspace dir to create an identical-named workspace
	// with a fresh salt (simulating a different machine).
	if err := os.RemoveAll(filepath.Dir(root1) + string(os.PathSeparator) + "SameName"); err != nil {
		t.Fatal(err)
	}
	w2, err := Create("SameName", "pass-b")
	if err != nil {
		t.Fatal(err)
	}
	if string(w1.salt) == string(w2.salt) {
		t.Fatal("salts identical across workspace creations — deterministic key layout regression")
	}
	// The old derivation (name-only) must not open the workspace.
	if _, err := Open("SameName", ""); err == nil {
		t.Fatal("empty passphrase opened a fresh protected workspace")
	}
}

// F2: keyless → keyed migration via Rekey removes the marker and
// invalidates the keyless key.
func TestRekeyMigratesKeylessToKeyed(t *testing.T) {
	t.Setenv("AETHER_CONFIG_DIR", t.TempDir())

	w, err := Create("MigrateMe", "")
	if err != nil {
		t.Fatal(err)
	}
	type tok struct {
		Access string `json:"access"`
	}
	if err := w.SaveRecord(BucketTokens, "graph", tok{Access: "tok-1"}); err != nil {
		t.Fatal(err)
	}
	if err := w.LogEvent("boot", "keyless"); err != nil {
		t.Fatal(err)
	}

	// Rekey with an empty old passphrase is only allowed for keyless
	// workspaces and requires the migration path.
	n, err := w.Rekey("", "real-pass")
	if err != nil {
		t.Fatalf("keyless migration rekey: %v", err)
	}
	if n != 1 {
		t.Errorf("migrated = %d, want 1", n)
	}

	// KEYLESS marker removed.
	if _, err := os.Stat(filepath.Join(Dir(), "MigrateMe", keylessMarker)); !os.IsNotExist(err) {
		t.Fatal("KEYLESS marker still present after migration")
	}

	// New passphrase decrypts.
	fresh, err := Open("MigrateMe", "real-pass")
	if err != nil {
		t.Fatalf("open after migration: %v", err)
	}
	var out tok
	if err := fresh.LoadRecord(BucketTokens, "graph", &out); err != nil {
		t.Fatalf("load after migration: %v", err)
	}
	if out.Access != "tok-1" {
		t.Errorf("access = %q", out.Access)
	}
	events, err := fresh.Events()
	if err != nil || len(events) != 1 {
		t.Fatalf("events after migration = %d err=%v", len(events), err)
	}

	// Keyless mode no longer works.
	if _, err := Open("MigrateMe", ""); err == nil {
		t.Fatal("keyless open after migration = nil, want rejection")
	}
}

// F2: a protected workspace refuses an empty old passphrase in Rekey
// unless the workspace is actually keyless/legacy.
func TestRekeyRejectsEmptyOldPassOnProtected(t *testing.T) {
	t.Setenv("AETHER_CONFIG_DIR", t.TempDir())

	w, err := Create("ProtectedWS", "real-pass")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Rekey("", "new-pass"); err == nil {
		t.Fatal("Rekey with empty old passphrase on protected workspace = nil, want error")
	}
}
