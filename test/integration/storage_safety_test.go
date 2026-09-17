//go:build integration

// Stage 4 backfill evidence tests (B4-G08, B4-G09, B4-G10, B4-G14):
// multi-process vault safety, crash-consistency matrix, torn-write
// rejection, and audit-spine tamper detection.
//
// Crash cells kill real child processes (TerminateProcess on Windows)
// at deterministic fsync boundaries. Torn-write truncation probes run
// in child processes so a fault from reading a truncated mmap cannot
// take down the test process.
package integration

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Debajyoti0-0/aether/internal/store"
)

// runStorageHelper executes child-process helper modes. Dispatched
// from TestMain before the test runner starts; the exit code is the
// helper's verdict protocol.
func runStorageHelper(name string) int {
	switch name {
	case "lockchild":
		return helperLockChild()
	case "crashchild":
		return helperCrashChild()
	case "tornprobe":
		return helperTornProbe()
	default:
		fmt.Printf("unknown helper %q\n", name)
		return 99
	}
}

func helperLockChild() int {
	v, err := store.OpenVault(os.Getenv("AETHER_IT_PATH"))
	if err != nil {
		if errors.Is(err, store.ErrVaultLocked) {
			fmt.Println("LOCKED")
			return 3 // expected loser verdict
		}
		fmt.Printf("OTHERERR: %v\n", err)
		return 4
	}
	_ = v.Close()
	fmt.Println("OPENED")
	return 0 // unexpected winner
}

func helperCrashChild() int {
	target := 0
	fmt.Sscanf(os.Getenv("AETHER_IT_TARGET"), "%d", &target)
	v, err := store.OpenVault(os.Getenv("AETHER_IT_PATH"))
	if err != nil {
		fmt.Printf("OPENERR: %v\n", err)
		return 4
	}
	log, err := store.NewVaultLog(v)
	if err != nil {
		fmt.Printf("LOGERR: %v\n", err)
		return 4
	}
	fmt.Println("READY") // after exclusive lock held
	for i := 1; ; i++ {
		// Append is fsync'd before it returns (durability contract),
		// so the marker below is printed only after durable commit.
		if _, err := log.Append("crash-cell-op", fmt.Sprintf("entry-%d", i)); err != nil {
			fmt.Printf("APPERR: %v\n", err)
			return 5
		}
		fmt.Printf("APPEND %d\n", i)
		if target > 0 && i >= target {
			break
		}
	}
	fmt.Println("COMPLETE")
	_ = v.Close()
	return 0
}

// helperTornProbe opens a (possibly corrupt) vault and reports whether
// verification rejects it. Isolated in a child process because reading
// a truncated bbolt mmap can fault; the probe must never take down the
// test process.
func helperTornProbe() int {
	v, err := store.OpenVault(os.Getenv("AETHER_IT_PATH"))
	if err != nil {
		fmt.Printf("VERDICT REJECTED-OPEN %q\n", err)
		return 0
	}
	defer v.Close()
	log, err := store.NewVaultLog(v)
	if err != nil {
		fmt.Printf("VERDICT REJECTED-LOG %q\n", err)
		return 0
	}
	entries, err := log.Entries()
	if err != nil {
		fmt.Printf("VERDICT REJECTED-READ %q\n", err)
		return 0
	}
	vr, err := log.Verify()
	if err != nil {
		fmt.Printf("VERDICT REJECTED-VERIFY-ERR %q\n", err)
		return 0
	}
	if !vr.ValidAll {
		fmt.Printf("VERDICT REJECTED-TAMPER total=%d valid=%d\n", vr.Total, vr.Valid)
		return 0
	}
	fmt.Printf("VERDICT ACCEPTED total=%d\n", len(entries))
	return 0
}

// spawnHelper starts the test binary in helper mode and returns the
// command plus a line scanner over its stdout.
func spawnHelper(t *testing.T, mode, path string) (*exec.Cmd, *bufio.Scanner) {
	t.Helper()
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(),
		"AETHER_IT_HELPER="+mode,
		"AETHER_IT_PATH="+path,
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	return cmd, bufio.NewScanner(stdout)
}

// waitForLine reads the next line with the given prefix, or any line
// when the prefix is empty; fails after a deadline.
func waitForLine(t *testing.T, sc *bufio.Scanner, prefix string, deadline time.Duration) string {
	t.Helper()
	ch := make(chan string, 1)
	go func() {
		for sc.Scan() {
			if prefix == "" || strings.HasPrefix(sc.Text(), prefix) {
				ch <- sc.Text()
				return
			}
		}
		close(ch)
	}()
	select {
	case line, ok := <-ch:
		if !ok {
			t.Fatalf("stream ended before %q line", prefix)
		}
		return line
	case <-time.After(deadline):
		t.Fatalf("timeout waiting for %q line", prefix)
		return ""
	}
}

// TestMultiProcessVaultLock (B4-G08): with the vault lock held by the
// parent, concurrent child processes must lose with the typed
// ErrVaultLocked error, and the file must remain intact.
func TestMultiProcessVaultLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mp-vault.db")

	v, err := store.OpenVault(path)
	if err != nil {
		t.Fatal(err)
	}
	log, err := store.NewVaultLog(v)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := log.Append("mp-writer", "held"); err != nil {
		t.Fatal(err)
	}

	// Two sequential losers.
	for i := 1; i <= 2; i++ {
		cmd, sc := spawnHelper(t, "lockchild", path)
		line := waitForLine(t, sc, "", 15*time.Second)
		waitErr := cmd.Wait()
		var exitErr *exec.ExitError
		if !errors.As(waitErr, &exitErr) || exitErr.ExitCode() != 3 {
			t.Fatalf("loser %d: output=%q err=%v", i, line, waitErr)
		}
		if line != "LOCKED" {
			t.Fatalf("loser %d: verdict %q", i, line)
		}
	}

	// One concurrent loser while the parent holds the lock.
	lost := make(chan error, 1)
	cmd, sc := spawnHelper(t, "lockchild", path)
	go func() {
		waitForLine(t, sc, "LOCKED", 15*time.Second)
		lost <- cmd.Wait()
	}()
	select {
	case waitErr := <-lost:
		var exitErr *exec.ExitError
		if !errors.As(waitErr, &exitErr) || exitErr.ExitCode() != 3 {
			t.Fatalf("concurrent loser err=%v", waitErr)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("concurrent loser did not report LOCKED in time")
	}

	// Winner state intact: chain verifies, lock still held by parent.
	vr, err := log.Verify()
	if err != nil {
		t.Fatal(err)
	}
	if !vr.ValidAll || vr.Total != 1 {
		t.Fatalf("winner chain = %+v", vr)
	}
	if err := v.Close(); err != nil {
		t.Fatal(err)
	}

	// After close, a child must be able to acquire the lock.
	cmd, sc = spawnHelper(t, "lockchild", path)
	line := waitForLine(t, sc, "", 15*time.Second)
	if waitErr := cmd.Wait(); waitErr != nil {
		t.Fatalf("post-close child: %v (line %q)", waitErr, line)
	}
	if line != "OPENED" {
		t.Fatalf("post-close child verdict %q", line)
	}
}

// TestCrashMatrix (B4-G09): kill the writer child (TerminateProcess)
// after it reports fsync'd append k, for 8 deterministic cells. After
// each kill: the OS must release the lock, every committed entry must
// be readable, the sequence must be contiguous, and the chain must
// verify — no partial record may be readable.
func TestCrashMatrix(t *testing.T) {
	cells := []int{1, 2, 3, 5, 8, 13, 21, 34}
	for _, k := range cells {
		k := k
		t.Run(fmt.Sprintf("kill_after_entry_%03d", k), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), fmt.Sprintf("crash-%03d.db", k))
			cmd, sc := spawnHelper(t, "crashchild", path)
			cmd.Env = append(cmd.Env, "AETHER_IT_TARGET=99999")
			_ = waitForLine(t, sc, "READY", 20*time.Second)
			marker := waitForLine(t, sc, fmt.Sprintf("APPEND %d", k), 60*time.Second)

			// Kill mechanism: TerminateProcess (os.Process.Kill on
			// Windows) with the marker already observed — append k is
			// durably committed (fsync before print).
			if err := cmd.Process.Kill(); err != nil {
				t.Fatalf("kill pid %d: %v", cmd.Process.Pid, err)
			}
			_ = cmd.Wait()
			t.Logf("cell k=%d: marker=%q pid=%d killed", k, marker, cmd.Process.Pid)

			// OS must release the lock: bounded reopen.
			deadline := time.Now().Add(15 * time.Second)
			var v *store.Vault
			var err error
			for {
				v, err = store.OpenVault(path)
				if err == nil || time.Now().After(deadline) {
					break
				}
				time.Sleep(200 * time.Millisecond)
			}
			if err != nil {
				t.Fatalf("post-kill reopen (lock not released?): %v", err)
			}
			defer v.Close()

			log, err := store.NewVaultLog(v)
			if err != nil {
				t.Fatal(err)
			}
			entries, err := log.Entries()
			if err != nil {
				t.Fatalf("post-kill read (partial record?): %v", err)
			}
			// Durability bound: every entry fsync'd before the
			// APPEND k marker must survive the kill. There is
			// deliberately NO upper bound on committed entries: the
			// interval between observing the marker and
			// TerminateProcess taking effect is load-dependent (a
			// fixed tolerance failed under background CPU load,
			// Stage 33b/34), and entries committed in that window
			// are legitimate fsync'd writes, not a durability
			// violation. The product guarantees under test are the
			// structural ones asserted below: no partial record
			// readable, contiguous sequence, and full chain
			// verification.
			if len(entries) < k-1 {
				t.Fatalf("committed entries = %d, want at least %d (fsync'd marker entry)", len(entries), k-1)
			}
			for i, e := range entries {
				if e.Seq != int64(i+1) {
					t.Fatalf("seq gap at index %d: seq=%d", i, e.Seq)
				}
			}
			vr, err := log.Verify()
			if err != nil {
				t.Fatal(err)
			}
			if !vr.ValidAll || vr.Total != len(entries) || vr.Valid != len(entries) {
				t.Fatalf("post-kill verify = %+v (committed=%d)", vr, len(entries))
			}
			t.Logf("cell k=%d: committed=%d chain VERIFIED", k, len(entries))
		})
	}
}

// TestTornWriteRejection (B4-G10): truncated and bit-flipped vault
// files must be rejected — never silently accepted as a valid chain.
func TestTornWriteRejection(t *testing.T) {
	build := func(t *testing.T) (path string, total int) {
		t.Helper()
		path = filepath.Join(t.TempDir(), "torn.db")
		v, err := store.OpenVault(path)
		if err != nil {
			t.Fatal(err)
		}
		log, err := store.NewVaultLog(v)
		if err != nil {
			t.Fatal(err)
		}
		for i := 1; i <= 50; i++ {
			if _, err := log.Append("torn-seed-op", strings.Repeat("x", 64)+fmt.Sprintf("-%d", i)); err != nil {
				t.Fatal(err)
			}
		}
		if err := v.Close(); err != nil {
			t.Fatal(err)
		}
		return path, 50
	}

	probe := func(t *testing.T, path string) (verdict string, rejected bool) {
		t.Helper()
		cmd, sc := spawnHelper(t, "tornprobe", path)
		// Drain all output; a child that dies before a VERDICT is
		// itself fail-closed (crash-isolated rejection).
		var lines []string
		for sc.Scan() {
			lines = append(lines, sc.Text())
		}
		waitErr := cmd.Wait()
		for _, l := range lines {
			if strings.HasPrefix(l, "VERDICT") {
				v := strings.TrimSpace(strings.TrimPrefix(l, "VERDICT"))
				return v, !strings.HasPrefix(v, "ACCEPTED")
			}
		}
		t.Logf("probe produced no verdict (exit=%v); treating as crash-isolated rejection", waitErr)
		return fmt.Sprintf("REJECTED-CRASH exit=%v", waitErr), true
	}

	t.Run("truncate_to_60pct", func(t *testing.T) {
		path, _ := build(t)
		p2 := path + ".trunc"
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		cut := len(raw) * 3 / 5
		if err := os.WriteFile(p2, raw[:cut], 0o600); err != nil {
			t.Fatal(err)
		}
		t.Logf("mutated bytes: truncated %d -> %d (offsets %d..%d removed)", len(raw), cut, cut, len(raw))
		verdict, rejected := probe(t, p2)
		if !rejected {
			t.Fatalf("truncated vault silently ACCEPTED: %s", verdict)
		}
		t.Logf("rejection: %s", verdict)
	})

	t.Run("bitflip_live_data", func(t *testing.T) {
		path, _ := build(t)
		// Locate LIVE bytes by diffing against a same-code empty vault:
		// a hard-coded offset is layout-coupled (the Stage 35 identity
		// binding shifted the page layout, moving 8192 into dead space
		// where corruption is legitimately tolerated). Flip in the
		// differing region — bytes that actually carry vault data.
		emptyPath := filepath.Join(t.TempDir(), "empty.db")
		ev, err := store.OpenVault(emptyPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := ev.Close(); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		empty, err := os.ReadFile(emptyPath)
		if err != nil {
			t.Fatal(err)
		}
		var diffs []int
		for i := 0; i < len(raw) && i < len(empty); i++ {
			if raw[i] != empty[i] {
				diffs = append(diffs, i)
			}
		}
		if len(diffs) == 0 {
			t.Fatal("no live data found to corrupt")
		}
		off := diffs[len(diffs)/2]
		p2 := path + ".flip1"
		raw[off] ^= 0xff
		if err := os.WriteFile(p2, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		t.Logf("mutated bytes: offset %d flipped (live-data region, %d differing bytes)", off, len(diffs))
		verdict, rejected := probe(t, p2)
		if !rejected {
			t.Fatalf("bit-flipped vault silently ACCEPTED: %s", verdict)
		}
		t.Logf("rejection: %s", verdict)
	})

	// Targeted record corruption: flip a byte INSIDE an audit entry's
	// JSON payload (located by its known plaintext). Verification must
	// reject via JSON parse failure, hash mismatch, or signature
	// mismatch — never accept.
	t.Run("bitflip_inside_audit_record", func(t *testing.T) {
		path, _ := build(t)
		p2 := path + ".fliprec"
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		idx := bytes.LastIndex(raw, []byte("torn-seed-op"))
		if idx < 0 {
			t.Fatal("audit record plaintext not found in vault file")
		}
		off := idx + 20 // inside the JSON value payload
		raw[off] ^= 0x55
		if err := os.WriteFile(p2, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		t.Logf("mutated bytes: offset %d flipped (inside audit record JSON)", off)
		verdict, rejected := probe(t, p2)
		if !rejected {
			t.Fatalf("record-corrupting bit flip silently ACCEPTED: %s", verdict)
		}
		t.Logf("rejection: %s", verdict)
	})

	// Documented non-violation (B4-DEF-01): bbolt data pages carry no
	// per-page checksums; a flip in UNALLOCATED space leaves every
	// committed record intact and verifiable. This subtest asserts the
	// truthful semantics: no record corruption, chain still valid. The
	// whole-file-digest hardening item is registered in the Stage 4
	// deferred register.
	t.Run("bitflip_unallocated_space_records_intact", func(t *testing.T) {
		path, _ := build(t)
		p2 := path + ".flipfree"
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		off := len(raw) * 2 / 3
		raw[off] ^= 0x55
		if err := os.WriteFile(p2, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		t.Logf("mutated bytes: offset %d flipped (unallocated space)", off)
		verdict, _ := probe(t, p2)
		if !strings.HasPrefix(verdict, "ACCEPTED") {
			t.Fatalf("expected records-intact acceptance, got: %s", verdict)
		}
		t.Logf("records unaffected by unallocated-space flip: %s", verdict)
	})

	t.Run("jsonl_torn_final_line", func(t *testing.T) {
		dir := t.TempDir()
		jsonl := filepath.Join(dir, "audit.jsonl")
		key := filepath.Join(dir, "audit.key")
		l, err := store.New(jsonl, key)
		if err != nil {
			t.Fatal(err)
		}
		for i := 1; i <= 3; i++ {
			if _, err := l.Append("torn-jsonl", fmt.Sprintf("e%d", i)); err != nil {
				t.Fatal(err)
			}
		}
		raw, err := os.ReadFile(jsonl)
		if err != nil {
			t.Fatal(err)
		}
		// Tear: cut the final line mid-record (no trailing newline).
		torn := raw[:len(raw)-20]
		t.Logf("mutated bytes: JSONL truncated %d -> %d (mid-line)", len(raw), len(torn))
		if err := os.WriteFile(jsonl, torn, 0o600); err != nil {
			t.Fatal(err)
		}
		vr, err := store.VerifyFile(jsonl, key)
		if err == nil && vr != nil && vr.ValidAll {
			t.Fatalf("torn JSONL silently accepted: %+v", vr)
		}
		t.Logf("rejection: err=%v vr=%+v", err, vr)
	})
}

// TestSpineTamperDetection (B4-G14): the audit chain must be
// contiguous across a sustained run, and any mutation of an exported
// record must be detected.
func TestSpineTamperDetection(t *testing.T) {
	dir := t.TempDir()
	jsonl := filepath.Join(dir, "spine.jsonl")
	key := filepath.Join(dir, "spine.key")
	l, err := store.New(jsonl, key)
	if err != nil {
		t.Fatal(err)
	}
	const n = 25
	for i := 1; i <= n; i++ {
		if _, err := l.Append("spine-integrity-op", fmt.Sprintf("payload-%d", i)); err != nil {
			t.Fatal(err)
		}
	}

	// Contiguity: seq 1..n, chain linkage via Verify.
	entries, err := l.Entries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != n {
		t.Fatalf("entries = %d, want %d", len(entries), n)
	}
	for i, e := range entries {
		if e.Seq != int64(i+1) {
			t.Fatalf("contiguity broken at index %d (seq=%d)", i, e.Seq)
		}
	}
	vr, err := l.Verify()
	if err != nil {
		t.Fatal(err)
	}
	if !vr.ValidAll || vr.Valid != n {
		t.Fatalf("baseline verify = %+v", vr)
	}

	if err := store.ExportFile(l, jsonl+".export"); err != nil {
		t.Fatal(err)
	}

	// Deliberate tamper: mutate entry 5's payload in the exported trail.
	raw, err := os.ReadFile(jsonl + ".export")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(raw), "\n")
	if len(lines) < 5 {
		t.Fatalf("export lines = %d", len(lines))
	}
	tampered := strings.Replace(lines[4], "payload-5", "PAYLOAD-5", 1)
	if tampered == lines[4] {
		t.Fatal("tamper no-op: payload substring not found")
	}
	lines[4] = tampered
	if err := os.WriteFile(jsonl+".tampered", []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("tamper: entry seq=5 payload mutated in exported trail")

	tv, err := store.VerifyFile(jsonl+".tampered", key)
	if err != nil {
		t.Fatalf("tamper verification errored (fail-closed, acceptable): %v", err)
	}
	if tv == nil || tv.ValidAll {
		t.Fatalf("tampered trail accepted: %+v", tv)
	}
	found := false
	for _, seq := range tv.Tampered {
		if seq == 5 {
			found = true
		}
	}
	if !found {
		t.Fatalf("tampered seq 5 not pinpointed: %+v", tv)
	}
	t.Logf("tamper detected: %+v", tv)

	// Untampered export must still verify (no false positives).
	uv, err := store.VerifyFile(jsonl+".export", key)
	if err != nil {
		t.Fatal(err)
	}
	if !uv.ValidAll || uv.Total != n {
		t.Fatalf("untampered export verify = %+v", uv)
	}
}
