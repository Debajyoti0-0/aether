package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateName(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "engagement1", false},
		{"valid dots-dashes", "red-team.2026", false},
		{"valid underscore", "client_work", false},
		{"empty", "", true},
		{"dot", ".", true},
		{"dotdot", "..", true},
		{"traversal", "../evil", true},
		{"deep traversal", "../../evil", true},
		{"inner traversal", "foo/../../evil", true},
		{"backslash traversal", `..\..\evil`, true},
		{"windows abs", `C:\evil`, true},
		{"windows abs fwd", "C:/evil", true},
		{"unc path", `\\server\share`, true},
		{"unix abs", "/absolute/path", true},
		{"slash", "a/b", true},
		{"backslash", `a\b`, true},
		{"nul byte", "a\x00b", true},
		{"trailing space", "ws ", true},
		{"leading space", " ws", true},
		{"trailing dot", "ws.", true},
		{"windows reserved", "CON", true},
		{"windows reserved nul", "NUL", true},
		{"windows reserved com1", "COM1", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateName(tc.input)
			if tc.wantErr && err == nil {
				t.Fatalf("ValidateName(%q) = nil, want error", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("ValidateName(%q) = %v, want nil", tc.input, err)
			}
		})
	}
}

func TestValidateRecordKeyAndArtifact(t *testing.T) {
	hostile := []string{"", "..", "../x", `..\x`, "/abs", "a/b", `a\b`, "k\x00", "CON", "x."}
	for _, k := range hostile {
		if err := ValidateRecordKey(k); err == nil {
			t.Errorf("ValidateRecordKey(%q) = nil, want error", k)
		}
		if err := ValidateArtifactName(k); err == nil {
			t.Errorf("ValidateArtifactName(%q) = nil, want error", k)
		}
	}
	for _, ok := range []string{"prt", "tokens-2026.json", "sess_1"} {
		if err := ValidateRecordKey(ok); err != nil {
			t.Errorf("ValidateRecordKey(%q) = %v, want nil", ok, err)
		}
	}
}

func TestSafeJoinRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("top secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Symlink INSIDE the workspace root pointing OUTSIDE it.
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable on this platform/filesystem: %v", err)
	}

	if _, err := SafeJoin(root, "escape"); err == nil {
		t.Fatal("SafeJoin(root, existing symlink dir) = nil error, want escape rejection")
	}
	// Path through the symlink must also be rejected.
	if _, err := SafeJoin(root, "escape"); err == nil {
		t.Fatal("SafeJoin via symlink = nil error, want rejection")
	}

	// Regular names under root still resolve.
	p, err := SafeJoin(root, "normal")
	if err != nil {
		t.Fatalf("SafeJoin(root, normal) = %v, want nil", err)
	}
	if !strings.HasPrefix(filepath.Clean(p), filepath.Clean(root)) {
		t.Fatalf("SafeJoin result %q escapes root %q", p, root)
	}
}

func TestSafeJoinRejectsInvalidComponents(t *testing.T) {
	root := t.TempDir()
	for _, bad := range []string{"..", "../evil", `C:\evil`, "/abs", "a/b", `a\b`, "", "x\x00y"} {
		if _, err := SafeJoin(root, bad); err == nil {
			t.Errorf("SafeJoin(root, %q) = nil error, want rejection", bad)
		}
	}
}

// F1 acceptance: Delete("../etc") must fail with a validation error and
// must not shred or remove anything outside the workspaces root.
func TestDeleteRejectsTraversal(t *testing.T) {
	t.Setenv("AETHER_CONFIG_DIR", t.TempDir())

	outside := filepath.Join(t.TempDir(), "etc")
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(outside, "passwd")
	if err := os.WriteFile(victim, []byte("root:x:0:0"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Delete("../etc"); err == nil {
		t.Fatal("Delete(\"../etc\") = nil, want validation error")
	}
	data, err := os.ReadFile(victim)
	if err != nil {
		t.Fatalf("victim file missing: %v (shred reached outside root?)", err)
	}
	if string(data) != "root:x:0:0" {
		t.Fatalf("victim file was modified: %q", data)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside dir removed by Delete: %v", err)
	}
}

func TestCreateOpenRejectTraversal(t *testing.T) {
	t.Setenv("AETHER_CONFIG_DIR", t.TempDir())

	for _, bad := range []string{"../evil", "a/b", `..\evil`, "/abs", "CON"} {
		if _, err := Create(bad); err == nil {
			t.Errorf("Create(%q) = nil error, want rejection", bad)
		}
		if _, err := Open(bad, "pass"); err == nil {
			t.Errorf("Open(%q) = nil error, want rejection", bad)
		}
		if Exists(bad) {
			t.Errorf("Exists(%q) = true, want false for hostile name", bad)
		}
	}

	// Records and artifacts must not escape either.
	ws, err := Create("traversal-guard")
	if err != nil {
		t.Fatal(err)
	}
	if err := ws.SaveRecord(BucketTokens, "../escape", map[string]string{"x": "y"}); err == nil {
		t.Error("SaveRecord with traversal key = nil, want error")
	}
	if err := ws.LoadRecord(BucketTokens, "../escape", &struct{}{}); err == nil {
		t.Error("LoadRecord with traversal key = nil, want error")
	}
	if err := ws.DeleteRecord(BucketTokens, "../escape"); err == nil {
		t.Error("DeleteRecord with traversal key = nil, want error")
	}
	if _, err := ws.SaveArtifact("../../escape.bin", []byte("x")); err == nil {
		t.Error("SaveArtifact with traversal name = nil, want error")
	}
	// Nothing was written outside the workspace root.
	if _, err := os.Stat(filepath.Join(filepath.Dir(ws.Root), "escape")); !os.IsNotExist(err) {
		t.Error("record key traversal escaped the workspace root")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(ws.Root), "escape.bin")); !os.IsNotExist(err) {
		t.Error("artifact traversal escaped the workspace root")
	}
}
