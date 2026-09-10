package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateAndOpen(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("USERPROFILE", dir)
	t.Setenv("AppData", dir)
	t.Setenv("HOME", dir)

	w, err := Create("ClientX", "pw")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !Exists("ClientX") {
		t.Fatal("workspace should exist")
	}

	// Directory layout.
	for _, sub := range []string{"db", "artifacts", "reports"} {
		if _, err := os.Stat(filepath.Join(w.Root, sub)); err != nil {
			t.Errorf("missing dir %s", sub)
		}
	}

	opened, err := Open("ClientX", "pw")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if opened.Name != "ClientX" {
		t.Errorf("name = %q", opened.Name)
	}

	if _, err := Open("GhostWS", ""); err == nil {
		t.Error("missing workspace should fail to open")
	}
}

func TestSealOpenRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("USERPROFILE", dir)
	t.Setenv("AppData", dir)
	t.Setenv("HOME", dir)

	w, _ := Create("SealTest", "pw")
		if len(w.pass) != 32 {
		t.Fatalf("derive: len=%d", len(w.pass))
	}

	secret := []byte("prt-cookie-value-0.AAAA")
	sealed, err := w.Seal(secret)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if string(sealed) == string(secret) {
		t.Fatal("sealed output equals plaintext")
	}

	plain, err := w.Open(sealed)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if string(plain) != string(secret) {
		t.Errorf("plaintext = %q", plain)
	}

	// Tamper detection.
	sealed[5] ^= 0xFF
	if _, err := w.Open(sealed); err == nil {
		t.Error("tampered ciphertext should fail auth")
	}
}

func TestSealWithoutKey(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("USERPROFILE", dir)
	t.Setenv("AppData", dir)
	t.Setenv("HOME", dir)

	w, _ := Create("NoKey", "pw")
	// Key intentionally cleared.
	w.pass = nil
	if _, err := w.Seal([]byte("x")); err == nil {
		t.Error("seal without key should fail")
	}
}

func TestRecords(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("USERPROFILE", dir)
	t.Setenv("AppData", dir)
	t.Setenv("HOME", dir)

	w, _ := Open("RecWS", "pw")
	if w == nil {
		w, _ = Create("RecWS", "pw")
			}

	type token struct {
		Access string `json:"access"`
	}

	if err := w.SaveRecord(BucketTokens, "graph", token{Access: "tok-1"}); err != nil {
		t.Fatalf("save: %v", err)
	}

	var out token
	if err := w.LoadRecord(BucketTokens, "graph", &out); err != nil {
		t.Fatalf("load: %v", err)
	}
	if out.Access != "tok-1" {
		t.Errorf("access = %q", out.Access)
	}

	keys, err := w.ListRecords(BucketTokens)
	if err != nil || len(keys) != 1 {
		t.Errorf("keys = %v err=%v", keys, err)
	}

	// On-disk bytes must not contain the plaintext.
	data, _ := os.ReadFile(filepath.Join(w.Root, "db", BucketTokens, "graph"))
	if strings.Contains(string(data), "tok-1") {
		t.Fatal("record stored unencrypted")
	}

	if err := w.DeleteRecord(BucketTokens, "graph"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if keys, _ := w.ListRecords(BucketTokens); len(keys) != 0 {
		t.Errorf("keys after delete = %v", keys)
	}
}

func TestEventJournal(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("USERPROFILE", dir)
	t.Setenv("AppData", dir)
	t.Setenv("HOME", dir)

	w, _ := Create("EventWS", "pw")
	
	if err := w.LogEvent("token_added", "graph"); err != nil {
		t.Fatalf("log: %v", err)
	}
	if err := w.LogEvent("path_validated", "3 steps"); err != nil {
		t.Fatalf("log: %v", err)
	}

	events, err := w.Events()
	if err != nil {
		t.Fatalf("events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d", len(events))
	}
	if events[0].Kind != "token_added" || events[1].Kind != "path_validated" {
		t.Errorf("events = %+v", events)
	}
}

func TestSaveArtifact(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("USERPROFILE", dir)
	t.Setenv("AppData", dir)
	t.Setenv("HOME", dir)

	w, _ := Create("ArtWS", "pw")
	
	path, err := w.SaveArtifact("aether.ccache", []byte("krb5-ccache-data"))
	if err != nil {
		t.Fatalf("save artifact: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "krb5-ccache-data" {
		t.Errorf("artifact read err=%v data=%q", err, data)
	}
}

func TestDeleteShreds(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("USERPROFILE", dir)
	t.Setenv("AppData", dir)
	t.Setenv("HOME", dir)

	w, _ := Create("DoomedWS", "pw")
		if err := w.SaveRecord(BucketTokens, "graph", map[string]string{"access": "tok-1"}); err != nil {
		t.Fatal(err)
	}

	if err := Delete("DoomedWS"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if Exists("DoomedWS") {
		t.Fatal("workspace should be gone")
	}
}

func TestListWorkspaces(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("USERPROFILE", dir)
	t.Setenv("AppData", dir)
	t.Setenv("HOME", dir)

	if _, err := Create("WS-1", "pw"); err != nil {
		t.Fatal(err)
	}
	if _, err := Create("WS-2", "pw"); err != nil {
		t.Fatal(err)
	}

	names, err := List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(names) != 2 {
		t.Errorf("names = %v", names)
	}
}

func TestCreateEmptyName(t *testing.T) {
	if _, err := Create("", "pw"); err == nil {
		t.Error("empty name should fail")
	}
}
