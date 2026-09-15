//go:build integration

// Stage 4 backfill endurance evidence (B4-G13): 300 governed runs plus
// 20 reconnect cycles with bounded goroutines and monotonic-memory
// guard, asserting the audit chain stays intact end-to-end. FD-count
// assertions are platform-limited (CI ubuntu is authoritative).
package integration

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	"github.com/Debajyoti0-0/aether/internal/engine/mutation"
	"github.com/Debajyoti0-0/aether/internal/workspace"
)

func TestEndurance(t *testing.T) {
	const (
		runs       = 300
		reconnects = 20
	)
	name := "E2EEnduranceOwn-" + t.Name()

	ws, err := workspace.Create(name, "integration-pass")
	if err != nil {
		t.Fatal(err)
	}

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	goroutinesBefore := runtime.NumGoroutine()

	m := &fakeExec{
		kind:   "exec.azure",
		target: "rg/endurance",
		opID:   "endurance-op",
	}
	for i := 1; i <= runs; i++ {
		m.opID = fmt.Sprintf("endurance-op-%d", i)
		res, err := mutation.Run(context.Background(), ws, m)
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		if res.Status != "completed" {
			t.Fatalf("run %d: status %q", i, res.Status)
		}
		if i%100 == 0 {
			t.Logf("progress: %d/%d runs", i, runs)
		}
	}

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	goroutinesAfter := runtime.NumGoroutine()

	if goroutinesAfter > goroutinesBefore+10 {
		t.Fatalf("goroutine leak: before=%d after=%d", goroutinesBefore, goroutinesAfter)
	}
	if after.HeapAlloc > before.HeapAlloc+(64<<20) {
		t.Fatalf("monotonic memory growth: before=%d after=%d", before.HeapAlloc, after.HeapAlloc)
	}
	t.Logf("goroutines before=%d after=%d; heap before=%dKiB after=%dKiB",
		goroutinesBefore, goroutinesAfter, before.HeapAlloc>>10, after.HeapAlloc>>10)

	// Chain intact end-to-end after 300 governed runs.
	log, err := ws.AuditLog()
	if err != nil {
		t.Fatal(err)
	}
	vr, err := log.Verify()
	if err != nil {
		t.Fatal(err)
	}
	if !vr.ValidAll || vr.Valid != vr.Total {
		t.Fatalf("endurance chain verify = %+v", vr)
	}
	if vr.Total < runs {
		t.Fatalf("audit entries = %d, want >= %d", vr.Total, runs)
	}
	t.Logf("chain intact: %+v", vr)

	// 20 reconnect cycles: each reacquires the vault flock, reopens the
	// workspace, and re-verifies the chain tail.
	if err := ws.Close(); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= reconnects; i++ {
		w2, err := workspace.Open(name, "integration-pass")
		if err != nil {
			t.Fatalf("reconnect %d: %v", i, err)
		}
		l2, err := w2.AuditLog()
		if err != nil {
			t.Fatalf("reconnect %d audit log: %v", i, err)
		}
		v2, err := l2.Verify()
		if err != nil {
			t.Fatalf("reconnect %d verify: %v", i, err)
		}
		if !v2.ValidAll {
			t.Fatalf("reconnect %d chain = %+v", i, v2)
		}
		if err := w2.Close(); err != nil {
			t.Fatalf("reconnect %d close: %v", i, err)
		}
	}
	t.Logf("reconnect cycles: %d OK", reconnects)
}
