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

	w, err := Create("Guarded", "real-pass")
	if err != nil {
		t.Fatal(err)
	}
	closeLater(t, w)

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
	t.Setenv("AETHER_CONFIG_DIR", t.TempDir())

	w, err := Create("KeylessWS", "")
	if err != nil {
		t.Fatal(err)
	}
	closeLater(t, w)
	if !w.Keyless {
		t.Fatal("Create with empty passphrase should mark workspace keyless")
	}
	if _, err := os.Stat(filepath.Join(w.Root, keylessMarker)); err != nil {
		t.Fatalf("KEYLESS marker missing: %v", err)
	}

	// The creator holds the vault lock; release before reopening.
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	// Keyless open works but warns on every open.
	reopened, err := Open("KeylessWS", "")
	if err != nil {
		t.Fatalf("keyless open: %v", err)
	}
	closeLater(t, reopened)
	if _, err := Open("KeylessWS", ""); err == nil {
		t.Fatalf("second concurrent keyless open must fail on the vault lock")
	}
}

// F2: wrong passphrase and tampered salt.bin must fail closed.
func TestOpenFailsClosedOnWrongPassAndSaltTamper(t *testing.T) {
	t.Setenv("AETHER_CONFIG_DIR", t.TempDir())

	w, err := Create("TamperMe", "real-pass")
	if err != nil {
		t.Fatal(err)
	}
	closeLater(t, w)

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
// recreating an identically named workspace must produce a fresh salt.
func TestNoDeterministicKeyFromName(t *testing.T) {
	t.Setenv("AETHER_CONFIG_DIR", t.TempDir())

	w1, err := Create("SameName", "pass-a")
	if err != nil {
		t.Fatal(err)
	}
	salt1 := append([]byte(nil), w1.salt...)
	// Close (releases the lock), then recreate the same-named workspace
	// with a fresh salt (simulating a different machine).
	if err := w1.Close(); err != nil {
		t.Fatal(err)
	}
	if err := Delete("SameName"); err != nil {
		t.Fatal(err)
	}
	w2, err := Create("SameName", "pass-b")
	if err != nil {
		t.Fatal(err)
	}
	closeLater(t, w2)
	if string(salt1) == string(w2.salt) {
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

	// Release the lock and reopen with the new passphrase.
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	fresh, err := Open("MigrateMe", "real-pass")
	if err != nil {
		t.Fatalf("open after migration: %v", err)
	}
	closeLater(t, fresh)
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
	closeLater(t, w)
	if _, err := w.Rekey("", "new-pass"); err == nil {
		t.Fatal("Rekey with empty old passphrase on protected workspace = nil, want error")
	}
}
