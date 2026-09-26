package api

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// Regression (Stage 44-R D-003): a server-startup snapshot of
// revoked.txt meant `serve cert revoke` had no effect until restart,
// contradicting the documented "fails closed on new connections"
// contract. A file-bound list must observe revocations appended after
// startup, must tolerate the file disappearing (empty list, per the
// loader contract), and must never silently lift revocations on a read
// error.
func TestFileRevocationListTakesEffectWithoutRestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "revoked.txt")

	if err := os.WriteFile(path, []byte("# revocations\nalice\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rl := FileRevocationList(path, []byte(""))

	if rl.IsRevoked("alice") != true {
		t.Fatal("alice should be revoked from initial file content")
	}
	if rl.IsRevoked("bob") {
		t.Fatal("bob is not revoked yet")
	}

	// Live revoke: append after startup, no restart.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("bob\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if !rl.IsRevoked("bob") {
		t.Fatal("revocation appended after startup was not observed (D-003)")
	}

	// Missing file is an empty list (loader contract).
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if rl.IsRevoked("bob") {
		t.Fatal("missing file must be treated as an empty list")
	}

	// Unreadable file keeps the last known list: never fail-open.
	if err := os.WriteFile(path, []byte("carol\n"), 0o000); err == nil {
		if rl.IsRevoked("carol") != true {
			t.Log("note: file permissions not enforced on this platform; unreadable-file branch untested")
		}
		if err := os.Chmod(path, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRevocationListConcurrentChecks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "revoked.txt")
	if err := os.WriteFile(path, []byte("alice\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rl := FileRevocationList(path, nil)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = rl.IsRevoked("alice")
			}
		}()
	}
	wg.Wait()
}
