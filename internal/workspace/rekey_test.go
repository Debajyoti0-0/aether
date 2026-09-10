package workspace

import (
	"testing"
)

func TestRekey(t *testing.T) {
	t.Setenv("AETHER_CONFIG_DIR", t.TempDir())

	w, err := Create("RekeyWS", "old-pass")
	if err != nil {
		t.Fatal(err)
	}
	closeLater(t, w)

	type tok struct {
		Access string `json:"access"`
	}
	if err := w.SaveRecord(BucketTokens, "graph", tok{Access: "tok-1"}); err != nil {
		t.Fatal(err)
	}
	if err := w.SaveRecord(BucketIdentities, "alice", tok{Access: "id-1"}); err != nil {
		t.Fatal(err)
	}
	if err := w.LogEvent("boot", "rekey test"); err != nil {
		t.Fatal(err)
	}

	n, err := w.Rekey("old-pass", "new-pass")
	if err != nil {
		t.Fatalf("rekey: %v", err)
	}
	if n != 2 {
		t.Errorf("migrated = %d, want 2", n)
	}

	// Release and reopen with the NEW passphrase: records must decrypt.
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	fresh, err := Open("RekeyWS", "new-pass")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	closeLater(t, fresh)
	var out tok
	if err := fresh.LoadRecord(BucketTokens, "graph", &out); err != nil {
		t.Fatalf("load with new pass: %v", err)
	}
	if out.Access != "tok-1" {
		t.Errorf("access = %q", out.Access)
	}

	events, err := fresh.Events()
	if err != nil {
		t.Fatalf("events with new pass: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("events = %d", len(events))
	}

	// Old passphrase must fail closed (the key file tag is keyed with
	// the NEW key after a rekey).
	if _, err := Open("RekeyWS", "old-pass"); err == nil {
		t.Error("old passphrase should fail after rekey")
	}
}

func TestRekeyWrongOldPass(t *testing.T) {
	t.Setenv("AETHER_CONFIG_DIR", t.TempDir())

	w, err := Create("WrongOld", "real-pass")
	if err != nil {
		t.Fatal(err)
	}
	closeLater(t, w)
	type tok struct{ V string }
	if err := w.SaveRecord(BucketTokens, "x", tok{V: "secret"}); err != nil {
		t.Fatal(err)
	}

	if _, err := w.Rekey("wrong-pass", "new"); err == nil {
		t.Error("wrong old passphrase should fail")
	}
	// Release the lock; the record must still decrypt with the original
	// passphrase.
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	check, err := Open("WrongOld", "real-pass")
	if err != nil {
		t.Fatalf("reopen with original passphrase after failed rekey: %v", err)
	}
	closeLater(t, check)
	var out tok
	if err := check.LoadRecord(BucketTokens, "x", &out); err != nil {
		t.Errorf("original record damaged by failed rekey: %v", err)
	}
}

func TestRekeyEmptyNewPass(t *testing.T) {
	t.Setenv("AETHER_CONFIG_DIR", t.TempDir())

	w, err := Create("EmptyNew", "old-pass")
	if err != nil {
		t.Fatal(err)
	}
	closeLater(t, w)
	if _, err := w.Rekey("a", ""); err == nil {
		t.Error("empty new passphrase should fail")
	}
}
