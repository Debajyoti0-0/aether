//go:build integration

// Stage 5 backfill evidence verification (B5-G11): 3 records from real
// spine runs verified from the signed audit chain, and tamper detection
// proven by mutating each record and confirming rejection.
package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Debajyoti0-0/aether/internal/engine/mutation"
	"github.com/Debajyoti0-0/aether/internal/store"
	"github.com/Debajyoti0-0/aether/internal/workspace"
)

// TestEvidenceVerification3Records: three governed spine runs produce
// six signed audit entries (before+after each); the exported trail
// verifies; mutating EACH of the three operation records is detected
// and pinpointed.
func TestEvidenceVerification3Records(t *testing.T) {
	ws, err := workspace.Create("EvidenceVerify-"+sanitizeName(t.Name()), "integration-pass")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ws.Close() })

	for i := 1; i <= 3; i++ {
		res, err := mutation.Run(context.Background(), ws, &fakeExec{
			kind:   "exec.azure",
			target: fmt.Sprintf("rg/evidence-%d", i),
			opID:   fmt.Sprintf("evidence-op-%d", i),
			undo:   &mutation.UndoSpec{Irreversible: true},
		})
		if err != nil || res.Status != "completed" {
			t.Fatalf("run %d: res=%+v err=%v", i, res, err)
		}
	}

	dir := t.TempDir()
	jsonl := filepath.Join(dir, "evidence.jsonl")
	log, err := ws.AuditLog()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ExportFile(log, jsonl); err != nil {
		t.Fatal(err)
	}

	// The chain is signed by the workspace's audit key (stored in the
	// vault meta bucket); export that seed for verification.
	seed, err := ws.Vault().MetaGet(store.MetaAuditKeyName)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(dir, "evidence.key")
	if err := os.WriteFile(keyPath, seed, 0o600); err != nil {
		t.Fatal(err)
	}

	// 3/3 records verified from real spine runs.
	vr, err := store.VerifyFile(jsonl, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if !vr.ValidAll || vr.Total != 6 || vr.Valid != 6 {
		t.Fatalf("baseline verify = %+v, want 6/6 valid", vr)
	}
	t.Logf("3/3 evidence records verified: %+v", vr)

	// Tamper detection: mutate each operation record (seq 2, 4, 6 =
	// the after-state entries of runs 1..3) and confirm rejection.
	raw, err := os.ReadFile(jsonl)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 6 {
		t.Fatalf("export lines = %d", len(lines))
	}
	for _, seq := range []int{2, 4, 6} {
		line := lines[seq-1]
		mutated := strings.Replace(line, "evidence-op-", "EVIDENCE-OP-", 1)
		if mutated == line {
			// Fall back to flipping a payload byte inside the JSON.
			idx := strings.LastIndex(line, `":"`)
			if idx < 0 || idx+4 >= len(line) {
				t.Fatalf("no mutation point for seq %d", seq)
			}
			b := []byte(line)
			b[idx+4] ^= 0x01
			mutated = string(b)
		}
		tampered := append([]string(nil), lines...)
		tampered[seq-1] = mutated
		tPath := filepath.Join(dir, fmt.Sprintf("tampered-%d.jsonl", seq))
		if err := os.WriteFile(tPath, []byte(strings.Join(tampered, "\n")+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		tv, err := store.VerifyFile(tPath, keyPath)
		if err == nil && tv != nil && tv.ValidAll {
			t.Fatalf("tampered record seq %d accepted", seq)
		}
		if err == nil && len(tv.Tampered) == 0 {
			t.Fatalf("tampered record seq %d not pinpointed: %+v", seq, tv)
		}
		t.Logf("tamper of seq %d detected: %+v (err=%v)", seq, tv, err)
	}
}
