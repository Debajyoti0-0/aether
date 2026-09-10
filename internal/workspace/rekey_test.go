package workspace

import (
	"testing"
)

func TestRekey(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("USERPROFILE", dir)
	t.Setenv("AppData", dir)
	t.Setenv("HOME", dir)

	w, err := Create("RekeyWS", "old-pass")
	if err != nil {
		t.Fatal(err)
	}
	
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

	// Reopen with the NEW passphrase: records must decrypt.
	fresh, err := Open("RekeyWS", "new-pass")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
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

	// Old passphrase must no longer decrypt. After a rekey the key file
	// tag is keyed with the NEW key, so Open fails closed immediately.
	if _, err := Open("RekeyWS", "old-pass"); err == nil {
		t.Error("old passphrase should fail after rekey")
	}
}

func TestRekeyWrongOldPass(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("USERPROFILE", dir)
	t.Setenv("AppData", dir)
	t.Setenv("HOME", dir)

	w, err := Create("WrongOld", "real-pass")
	if err != nil {
		t.Fatal(err)
	}
	type tok struct{ V string }
	if err := w.SaveRecord(BucketTokens, "x", tok{V: "secret"}); err != nil {
		t.Fatal(err)
	}

	if _, err := w.Rekey("wrong-pass", "new"); err == nil {
		t.Error("wrong old passphrase should fail")
	}
	// The record must still decrypt with the original passphrase.
	check, err := Open("WrongOld", "real-pass")
	if err != nil {
		t.Fatalf("reopen with original passphrase after failed rekey: %v", err)
	}
	var out tok
	if err := check.LoadRecord(BucketTokens, "x", &out); err != nil {
		t.Errorf("original record damaged by failed rekey: %v", err)
	}
}

func TestRekeyEmptyNewPass(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("USERPROFILE", dir)
	t.Setenv("AppData", dir)
	t.Setenv("HOME", dir)

	w, _ := Create("EmptyNew", "old-pass")
	if _, err := w.Rekey("a", ""); err == nil {
		t.Error("empty new passphrase should fail")
	}
}
